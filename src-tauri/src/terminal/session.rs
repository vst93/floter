//! Alacritty-backed terminal session.
//!
//! Owns an [`alacritty_terminal::term::Term`] behind a `FairMutex`, fed by
//! alacritty's PTY event loop. A dedicated render thread coalesces the
//! terminal's `Wakeup` events, serializes the visible grid, and pushes binary
//! frames to the frontend.

use std::borrow::Cow;
use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc::{self, Receiver, Sender};
use std::sync::{Arc, Mutex};
use std::time::Duration;

#[cfg(unix)]
use std::os::unix::fs::PermissionsExt;

use alacritty_terminal::event::{Event, EventListener, WindowSize};
use alacritty_terminal::event_loop::{EventLoop, EventLoopSender, Msg};
use alacritty_terminal::grid::{Dimensions, Scroll};
use alacritty_terminal::sync::FairMutex;
use alacritty_terminal::term::{Config, Term};
use alacritty_terminal::tty;
use alacritty_terminal::tty::{Options as TtyOptions, Shell};
use alacritty_terminal::vte::ansi::{CursorShape, CursorStyle};
use base64::Engine;
use serde::Serialize;
use tauri::{AppHandle, Emitter};

use super::frame;

/// Maximum render rate cap, expressed as the idle window used to coalesce a
/// burst of `Wakeup` notifications into a single frame.
const COALESCE: Duration = Duration::from_millis(8);

/// Poll interval for the render thread's shutdown check.
const POLL: Duration = Duration::from_millis(100);

/// Payload emitted to the frontend on every rendered frame.
#[derive(Clone, Serialize)]
struct FrameEvent<'a> {
    id: &'a str,
    frame: String,
}

/// Payload emitted when the PTY child exits.
#[derive(Clone, Serialize)]
struct ExitEvent<'a> {
    id: &'a str,
    code: Option<i32>,
}

enum RenderEvent {
    Wake,
    ChildExit(Option<i32>),
}

/// Event listener that forwards redraw-worthy events to the render thread.
#[derive(Clone)]
struct RenderListener {
    tx: Sender<RenderEvent>,
}

impl EventListener for RenderListener {
    fn send_event(&self, event: Event) {
        match event {
            Event::ChildExit(status) => {
                let _ = self.tx.send(RenderEvent::ChildExit(status.code()));
            }
            Event::Exit => {
                let _ = self.tx.send(RenderEvent::ChildExit(None));
            }
            Event::Wakeup | Event::Bell | Event::CursorBlinkingChange => {
                let _ = self.tx.send(RenderEvent::Wake);
            }
            _ => {}
        }
    }
}

/// Terminal dimensions used when constructing / resizing the `Term`.
struct TermSize {
    cols: usize,
    rows: usize,
}

/// Build the alacritty `Config`, applying the user-configured default cursor
/// style (defaults to a blinking beam).
fn terminal_config() -> Config {
    let mut config = Config::default();
    let shape = match crate::commands::config::load_settings()
        .cursor_shape
        .as_str()
    {
        "block" => CursorShape::Block,
        "underline" => CursorShape::Underline,
        "beam" => CursorShape::Beam,
        _ => CursorShape::Beam,
    };
    config.default_cursor_style = CursorStyle {
        shape,
        blinking: true,
    };
    config
}

impl Dimensions for TermSize {
    fn total_lines(&self) -> usize {
        self.rows
    }
    fn screen_lines(&self) -> usize {
        self.rows
    }
    fn columns(&self) -> usize {
        self.cols
    }
}

/// A single live terminal backed by alacritty_terminal.
///
/// The PTY event loop and render thread run detached; both self-terminate on
/// [`TerminalSession::close`] (via `Msg::Shutdown` and the `alive` flag).
pub struct TerminalSession {
    terminal: Arc<FairMutex<Term<RenderListener>>>,
    sender: EventLoopSender,
    /// Manual wake channel (e.g. after an out-of-band resize).
    wakeup: Sender<RenderEvent>,
    alive: Arc<AtomicBool>,
    /// PID of the spawned shell, used to look up its working directory.
    shell_pid: u32,
}

impl TerminalSession {
    /// Spawn a new terminal session running `shell` (or the user's default).
    pub fn new(
        id: String,
        app: AppHandle,
        shell: Option<String>,
        cols: u16,
        rows: u16,
    ) -> Result<Self, String> {
        tty::setup_env();

        let size = TermSize {
            cols: cols.max(2) as usize,
            rows: rows.max(1) as usize,
        };

        let (tx, rx) = mpsc::channel::<RenderEvent>();
        let listener = RenderListener { tx: tx.clone() };

        let config = terminal_config();
        let term = Term::new(config, &size, listener.clone());
        let terminal = Arc::new(FairMutex::new(term));

        let mut tty_options = TtyOptions::default();
        tty_options.working_directory = dirs::home_dir();
        if let Some(program) = shell {
            tty_options.shell = Some(Shell::new(program, Vec::new()));
        }

        let window_size = WindowSize {
            num_lines: size.rows as u16,
            num_cols: size.cols as u16,
            cell_width: 0,
            cell_height: 0,
        };

        let pty = tty::new(&tty_options, window_size, 0).map_err(|e| e.to_string())?;
        let shell_pid = pty.child().id();

        let event_loop = EventLoop::new(terminal.clone(), listener, pty, false, false)
            .map_err(|e| e.to_string())?;
        let sender = event_loop.channel();
        // Detach: the loop owns itself and exits on `Msg::Shutdown`.
        let _ = event_loop.spawn();

        let alive = Arc::new(AtomicBool::new(true));
        spawn_render_thread(id.clone(), app, terminal.clone(), rx, alive.clone());

        Ok(Self {
            terminal,
            sender,
            wakeup: tx,
            alive,
            shell_pid,
        })
    }

    /// Write user input (keystrokes) to the PTY.
    pub fn input(&self, data: &[u8]) -> Result<(), String> {
        // Reset the scrollback view to the bottom before forwarding input,
        // mirroring the behavior of most terminals.
        {
            let mut term = self.terminal.lock();
            term.scroll_display(Scroll::Bottom);
        }
        self.sender
            .send(Msg::Input(Cow::Owned(data.to_vec())))
            .map_err(|e| e.to_string())?;
        let _ = self.wakeup.send(RenderEvent::Wake);
        Ok(())
    }

    /// Scroll the viewport by `delta` lines (positive = into history).
    pub fn scroll(&self, delta: i32) -> Result<(), String> {
        {
            let mut term = self.terminal.lock();
            term.scroll_display(Scroll::Delta(delta));
        }
        let _ = self.wakeup.send(RenderEvent::Wake);
        Ok(())
    }

    /// Set the absolute scrollback offset (0 = bottom).
    pub fn scroll_to(&self, offset: u32) -> Result<(), String> {
        let delta = {
            let term = self.terminal.lock();
            let current = term.grid().display_offset() as i32;
            offset as i32 - current
        };
        self.scroll(delta)
    }

    /// Resize both the terminal grid and the PTY.
    pub fn resize(&self, cols: u16, rows: u16) -> Result<(), String> {
        let size = TermSize {
            cols: cols.max(2) as usize,
            rows: rows.max(1) as usize,
        };
        {
            let mut term = self.terminal.lock();
            term.resize(size);
        }
        self.sender
            .send(Msg::Resize(WindowSize {
                num_lines: rows,
                num_cols: cols,
                cell_width: 0,
                cell_height: 0,
            }))
            .map_err(|e| e.to_string())?;
        // Prompt an immediate re-render so the new dimensions show up.
        let _ = self.wakeup.send(RenderEvent::Wake);
        Ok(())
    }

    /// Shut the session down: stop the PTY event loop and render thread.
    pub fn close(&self) {
        self.alive.store(false, Ordering::SeqCst);
        let _ = self.sender.send(Msg::Shutdown);
        let _ = self.wakeup.send(RenderEvent::Wake);
    }

    /// Open the current working directory of the session's shell in the
    /// system's default terminal, so work can continue there.
    pub fn open_in_default_terminal(&self) -> Result<(), String> {
        let cwd = read_cwd(self.shell_pid).unwrap_or_else(|| dirs::home_dir().unwrap_or_default());
        open_terminal_at(&cwd)
    }
}

impl Drop for TerminalSession {
    fn drop(&mut self) {
        self.close();
    }
}

#[allow(clippy::needless_pass_by_value)]
fn spawn_render_thread(
    id: String,
    app: AppHandle,
    terminal: Arc<FairMutex<Term<RenderListener>>>,
    rx: Receiver<RenderEvent>,
    alive: Arc<AtomicBool>,
) {
    std::thread::Builder::new()
        .name("term-render".into())
        .spawn(move || render_loop(id, app, terminal, rx, alive))
        .expect("spawn render thread");
}

fn render_loop(
    id: String,
    app: AppHandle,
    terminal: Arc<FairMutex<Term<RenderListener>>>,
    rx: Receiver<RenderEvent>,
    alive: Arc<AtomicBool>,
) {
    while alive.load(Ordering::SeqCst) {
        match rx.recv_timeout(POLL) {
            Ok(first) => {
                let mut child_exit = match first {
                    RenderEvent::Wake => None,
                    RenderEvent::ChildExit(code) => Some(code),
                };

                // Coalesce a burst of wakeups into one frame while preserving
                // the fact that the PTY child exited.
                while let Ok(event) = rx.try_recv() {
                    if let RenderEvent::ChildExit(code) = event {
                        child_exit = Some(code);
                    }
                }
                std::thread::sleep(COALESCE);
                while let Ok(event) = rx.try_recv() {
                    if let RenderEvent::ChildExit(code) = event {
                        child_exit = Some(code);
                    }
                }

                let frame = {
                    let term = terminal.lock();
                    frame::serialize(&term)
                };
                let encoded = base64::engine::general_purpose::STANDARD.encode(&frame);
                let _ = app.emit(
                    "term://frame",
                    FrameEvent {
                        id: &id,
                        frame: encoded,
                    },
                );

                if let Some(code) = child_exit {
                    alive.store(false, Ordering::SeqCst);
                    let _ = app.emit("term://exit", ExitEvent { id: &id, code });
                    break;
                }
            }
            Err(mpsc::RecvTimeoutError::Timeout) => continue,
            Err(mpsc::RecvTimeoutError::Disconnected) => break,
        }
    }
}

/// Manager owning all live terminal sessions keyed by id.
pub struct TerminalManager {
    sessions: Mutex<HashMap<String, TerminalSession>>,
}

impl TerminalManager {
    pub fn new() -> Self {
        Self {
            sessions: Mutex::new(HashMap::new()),
        }
    }

    pub fn spawn(
        &self,
        id: String,
        app: AppHandle,
        shell: Option<String>,
        cols: u16,
        rows: u16,
    ) -> Result<(), String> {
        let mut sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        if let Some(session) = sessions.remove(&id) {
            session.close();
        }
        let session = TerminalSession::new(id.clone(), app, shell, cols, rows)?;
        sessions.insert(id, session);
        Ok(())
    }

    pub fn input(&self, id: &str, data: &[u8]) -> Result<(), String> {
        let sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        match sessions.get(id) {
            Some(session) => session.input(data),
            // Session may not be spawned yet (startup race); drop early input.
            None => Ok(()),
        }
    }

    pub fn resize(&self, id: &str, cols: u16, rows: u16) -> Result<(), String> {
        let sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        match sessions.get(id) {
            Some(session) => session.resize(cols, rows),
            // Resize before spawn completes is harmless; the spawn uses 80x24
            // and the next change will resize properly.
            None => Ok(()),
        }
    }

    pub fn scroll(&self, id: &str, delta: i32) -> Result<(), String> {
        let sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        match sessions.get(id) {
            Some(session) => session.scroll(delta),
            None => Ok(()),
        }
    }

    pub fn scroll_to(&self, id: &str, offset: u32) -> Result<(), String> {
        let sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        match sessions.get(id) {
            Some(session) => session.scroll_to(offset),
            None => Ok(()),
        }
    }

    pub fn close(&self, id: &str) -> Result<(), String> {
        let mut sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        if let Some(session) = sessions.remove(id) {
            session.close();
        }
        Ok(())
    }

    pub fn open_in_terminal(&self, id: &str) -> Result<(), String> {
        let sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        match sessions.get(id) {
            Some(session) => session.open_in_default_terminal(),
            None => Ok(()),
        }
    }
}

/// Resolve the current working directory of `pid` via `lsof`.
fn read_cwd(pid: u32) -> Option<PathBuf> {
    let output = Command::new("lsof")
        .args(["-a", "-p", &pid.to_string(), "-d", "cwd", "-Fn"])
        .output()
        .ok()?;
    let stdout = String::from_utf8_lossy(&output.stdout);
    stdout.lines().find_map(|line| {
        line.strip_prefix('n')
            .filter(|p| !p.is_empty())
            .map(PathBuf::from)
    })
}

/// Open a Terminal.app window at `dir` by writing an executable `.command`
/// shim and handing it to `open` (the system's default handler for
/// `.command` files).
fn open_terminal_at(dir: &std::path::Path) -> Result<(), String> {
    let cache_dir = dirs::cache_dir()
        .ok_or_else(|| "no cache directory".to_string())?
        .join("floter");
    fs::create_dir_all(&cache_dir).map_err(|e| e.to_string())?;

    let shim = cache_dir.join("open-here.command");
    let escaped = dir.to_string_lossy().replace('\'', "'\\''");
    let content = format!("#!/bin/sh\ncd '{}'\nexec \"$SHELL\" -l\n", escaped);
    fs::write(&shim, content).map_err(|e| e.to_string())?;

    #[cfg(unix)]
    {
        fs::set_permissions(&shim, fs::Permissions::from_mode(0o755)).map_err(|e| e.to_string())?;
    }

    Command::new("open")
        .arg(&shim)
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(())
}
