//! Alacritty-backed terminal session.
//!
//! Owns an [`alacritty_terminal::term::Term`] behind a `FairMutex`, fed by
//! alacritty's PTY event loop. A dedicated render thread coalesces the
//! terminal's `Wakeup` events, serializes the visible grid, and pushes binary
//! frames to the frontend.

use std::borrow::Cow;
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc::{self, Receiver, Sender};
use std::sync::{Arc, Mutex};
use std::time::Duration;

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

/// Event listener that forwards redraw-worthy events to the render thread, and
/// routes terminal query responses (`PtyWrite`) back into the PTY.
#[derive(Clone)]
struct RenderListener {
    tx: Sender<RenderEvent>,
    /// The PTY sender, filled in once the event loop has been created.
    ///
    /// `Arc<Mutex<Option<_>>>` because the listener is cloned — it is handed to
    /// both `Term::new` and `EventLoop::new` — while the sender only exists
    /// *after* the event loop it belongs to, i.e. after both of those clones
    /// have been made. The shared cell lets the later assignment reach the
    /// copies that were already given away.
    pty_sender: Arc<Mutex<Option<EventLoopSender>>>,
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
            Event::PtyWrite(text) => {
                // Every reply the emulator owes the program on the other end of
                // the PTY arrives here: DA1/DA2 device attributes, DSR status
                // and cursor position reports, DECRQM mode queries (how a TUI
                // probes for synchronized output or bracketed paste), kitty
                // keyboard protocol queries, cell-size reports. Dropping them
                // does not fail loudly — the program simply waits out its own
                // timeout, so every TUI that asks what it is talking to stalls
                // for a second or three before drawing its first frame.
                if let Ok(guard) = self.pty_sender.lock() {
                    if let Some(sender) = guard.as_ref() {
                        let _ = sender.send(Msg::Input(Cow::Owned(text.into_bytes())));
                    }
                }
                // The reply itself changes nothing on screen, but whatever the
                // program does once unblocked will, and a spurious wake costs
                // one coalesced frame.
                let _ = self.tx.send(RenderEvent::Wake);
            }
            _ => {}
        }
    }
}

/// Terminfo entry advertised to the shell and everything it runs.
///
/// [`tty::setup_env`] writes `TERM=alacritty` whenever it can find that entry,
/// and its search path is narrower than the one ncurses actually uses: it misses
/// `/usr/local/share/terminfo` and the Homebrew prefixes, and it looks in
/// `~/.terminfo` only when `$TERMINFO` is unset. So it can settle on `alacritty`
/// on a system where a TUI program will not find the description — and a program
/// that cannot resolve `TERM` falls back to dumb-terminal behaviour after
/// searching for the description it was promised.
///
/// `xterm-256color` ships with ncurses itself, so it is present everywhere and
/// close enough to what this emulator implements.
const TERMINFO: &str = "xterm-256color";

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
        // Written *after* `setup_env`, which overwrites `TERM` unconditionally
        // (with either `alacritty` or `xterm-256color`, so there is nothing of
        // the user's to preserve here); the `COLORTERM` it also sets is left
        // alone. This only covers readers inside our own process — the child
        // gets its copy from `tty_options.env` below.
        std::env::set_var("TERM", TERMINFO);

        let size = TermSize {
            cols: cols.max(2) as usize,
            rows: rows.max(1) as usize,
        };

        let (tx, rx) = mpsc::channel::<RenderEvent>();
        let pty_sender = Arc::new(Mutex::new(None));
        let listener = RenderListener {
            tx: tx.clone(),
            pty_sender: pty_sender.clone(),
        };

        let config = terminal_config();
        let term = Term::new(config, &size, listener.clone());
        let terminal = Arc::new(FairMutex::new(term));

        let mut tty_options = TtyOptions::default();
        tty_options.working_directory = dirs::home_dir();
        // `tty::new` never puts `TERM` on the command it builds, so without this
        // the child would only pick the variable up by inheriting our process
        // environment. Everything in this map goes through `Command::env`, which
        // sets it on the child directly: no dependence on a process-wide global
        // that any other thread in a Tauri app may have rewritten between here
        // and the fork, and an explicit value for `login` to carry over on
        // macOS, where the shell is started as
        // `login -flp <user> /bin/zsh -fc 'exec -a -zsh <shell>'`.
        tty_options
            .env
            .insert("TERM".to_string(), TERMINFO.to_string());
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
        // Hand the listener its write path before the loop starts reading, so
        // that a query arriving in the child's very first output has somewhere
        // to send its answer.
        if let Ok(mut slot) = pty_sender.lock() {
            *slot = Some(sender.clone());
        }
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

    /// Tear down every live session. Called once on application exit.
    pub fn shutdown_all(&self) {
        if let Ok(mut sessions) = self.sessions.lock() {
            for (_, session) in sessions.drain() {
                session.close();
            }
        }
    }
}

/// Resolve the current working directory of `pid`.
///
/// Uses `/proc` where available and falls back to `lsof` elsewhere (macOS).
fn read_cwd(pid: u32) -> Option<PathBuf> {
    #[cfg(target_os = "linux")]
    if let Ok(path) = std::fs::read_link(format!("/proc/{pid}/cwd")) {
        return Some(path);
    }

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

/// Wrap `value` in single quotes for safe interpolation into a `/bin/sh` script.
#[cfg(target_os = "macos")]
fn sh_quote(value: &str) -> String {
    format!("'{}'", value.replace('\'', "'\\''"))
}

/// Escape `value` for interpolation into a double-quoted AppleScript string.
///
/// Applied *on top of* [`sh_quote`]: the shell quoting protects the path from
/// the shell that ends up running the line, this protects the surrounding
/// AppleScript literal from the quoting itself. macOS allows `"` and `\` in file
/// names, and either one would otherwise end the literal early.
#[cfg(target_os = "macos")]
fn applescript_quote(value: &str) -> String {
    value.replace('\\', "\\\\").replace('"', "\\\"")
}

/// Open a Terminal.app window at `dir`.
#[cfg(target_os = "macos")]
fn open_terminal_at(dir: &Path) -> Result<(), String> {
    // Terminal opens every window through `login` and `do script` types its
    // argument into that window, so the first two lines would otherwise be the
    // "Last login:" banner and an echo of this command. `clear` wipes both and
    // leaves the window sitting on a bare prompt in the right directory — the
    // point of the whole feature.
    //
    // No `exec $SHELL`: `do script` already runs inside the shell Terminal is
    // configured to launch, so re-execing would only source the user's rc files
    // a second time. That is also why the old `.command` shim looked so noisy —
    // Terminal ran it as `<path to shim> ; exit;` and echoed that line.
    let command = format!("cd {}; clear", sh_quote(&dir.to_string_lossy()));
    let script = format!(
        "tell application \"Terminal\"\nactivate\ndo script \"{}\"\nend tell",
        applescript_quote(&command)
    );
    let dir = dir.to_path_buf();

    // Detached, because this is called from a synchronous Tauri command — i.e.
    // on the main thread, holding the session manager's lock. The first Apple
    // event floter sends raises the Automation consent dialog, and `osascript`
    // sits there until it is answered.
    std::thread::spawn(move || {
        let delivered = Command::new("osascript")
            .args(["-e", &script])
            .output()
            .is_ok_and(|out| out.status.success());
        if delivered {
            return;
        }
        // Consent refused, or Terminal rejected the event: `open` needs no
        // Apple events, and Terminal treats a directory argument as "new window
        // rooted here". Only the login banner comes back. `-a Terminal` pins the
        // handler — a bare `open` on a directory goes to Finder.
        let _ = Command::new("open")
            .args(["-a", "Terminal"])
            .arg(dir)
            .spawn();
    });
    Ok(())
}

/// Terminal emulators tried in order when `$TERMINAL` is unset.
#[cfg(target_os = "linux")]
const TERMINALS: &[&str] = &[
    "x-terminal-emulator",
    "gnome-terminal",
    "konsole",
    "xfce4-terminal",
    "kitty",
    "alacritty",
    "wezterm",
    "ghostty",
    "foot",
    "tilix",
    "terminator",
    "urxvt",
    "xterm",
];

/// Locate `name` on `$PATH`, or treat it as a path if it contains a separator.
#[cfg(target_os = "linux")]
fn which(name: &str) -> Option<PathBuf> {
    if name.contains('/') {
        let path = PathBuf::from(name);
        return path.is_file().then_some(path);
    }
    let paths = std::env::var_os("PATH")?;
    std::env::split_paths(&paths)
        .map(|dir| dir.join(name))
        .find(|candidate| candidate.is_file())
}

/// Spawn the first available terminal emulator at `dir`.
#[cfg(target_os = "linux")]
fn open_terminal_at(dir: &Path) -> Result<(), String> {
    let preferred = std::env::var("TERMINAL").ok();
    let candidates = preferred
        .as_deref()
        .into_iter()
        .chain(TERMINALS.iter().copied());

    for name in candidates {
        if which(name).is_none() {
            continue;
        }
        let mut command = Command::new(name);
        // All of these inherit their shell's cwd, so no per-emulator flag is
        // needed to land in the right directory.
        command.current_dir(dir);
        if command.spawn().is_ok() {
            return Ok(());
        }
    }
    Err("no terminal emulator found".to_string())
}

/// Open Windows Terminal (or `cmd` as a fallback) at `dir`.
#[cfg(target_os = "windows")]
fn open_terminal_at(dir: &Path) -> Result<(), String> {
    if Command::new("wt").arg("-d").arg(dir).spawn().is_ok() {
        return Ok(());
    }
    Command::new("cmd")
        .args(["/C", "start", "cmd"])
        .current_dir(dir)
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(())
}
