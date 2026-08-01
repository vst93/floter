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
use alacritty_terminal::term::{Config, Term, TermMode};
use alacritty_terminal::tty;
use alacritty_terminal::tty::{Options as TtyOptions, Shell};
use alacritty_terminal::vte::ansi::{CursorShape, CursorStyle, Processor, Rgb};
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

#[derive(Clone, Copy, PartialEq, Eq)]
enum TerminalTheme {
    Dark,
    Light,
}

impl TerminalTheme {
    fn from_name(value: Option<&str>) -> Self {
        match value {
            Some("light") => Self::Light,
            _ => Self::Dark,
        }
    }
}

const LIGHT_ANSI: [Rgb; 16] = [
    Rgb { r: 0x1d, g: 0x1f, b: 0x21 },
    Rgb { r: 0xc7, g: 0x2e, b: 0x3c },
    Rgb { r: 0x19, g: 0x73, b: 0x3a },
    Rgb { r: 0x7a, g: 0x5c, b: 0x00 },
    Rgb { r: 0x16, g: 0x59, b: 0xb7 },
    Rgb { r: 0x9c, g: 0x2c, b: 0xa0 },
    Rgb { r: 0x00, g: 0x7c, b: 0x86 },
    Rgb { r: 0x53, g: 0x57, b: 0x62 },
    Rgb { r: 0x76, g: 0x7b, b: 0x86 },
    Rgb { r: 0xd6, g: 0x3d, b: 0x4b },
    Rgb { r: 0x1a, g: 0x6e, b: 0x38 },
    Rgb { r: 0x7a, g: 0x59, b: 0x00 },
    Rgb { r: 0x24, g: 0x6f, b: 0xd1 },
    Rgb { r: 0xac, g: 0x38, b: 0xa9 },
    Rgb { r: 0x00, g: 0x6e, b: 0x7a },
    Rgb { r: 0x2c, g: 0x30, b: 0x37 },
];

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

fn apply_terminal_theme<T: EventListener>(term: &mut Term<T>, theme: TerminalTheme) {
    let mut sequence = String::new();
    // Remove any palette set by a prior app theme before applying the new one.
    for index in 0..16 {
        sequence.push_str(&format!("\x1b]104;{index}\x07"));
    }
    if theme == TerminalTheme::Light {
        for (index, color) in LIGHT_ANSI.iter().enumerate() {
            sequence.push_str(&format!(
                "\x1b]4;{index};#{:02x}{:02x}{:02x}\x07",
                color.r, color.g, color.b
            ));
        }
    }
    let mut processor: Processor = Processor::new();
    processor.advance(term, sequence.as_bytes());
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
    /// PID of the spawned shell, used to look up its working directory and
    /// whether the launcher-started command is still running.
    shell_pid: u32,
    /// Command entered in the launcher. A system terminal can restart this when
    /// it is still the shell's foreground job; the PTY itself cannot be moved.
    initial_command: Option<String>,
}

impl TerminalSession {
    /// Spawn a new terminal session running `shell` (or the user's default).
    pub fn new(
        id: String,
        app: AppHandle,
        shell: Option<String>,
        initial_command: Option<String>,
        theme: Option<String>,
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
        let mut term = Term::new(config, &size, listener.clone());
        apply_terminal_theme(&mut term, TerminalTheme::from_name(theme.as_deref()));
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
        // `Pty::child()` is Unix-only: alacritty_terminal's Windows backend does
        // not expose the child process handle. The PID is used solely by
        // `open_in_default_terminal` to read the shell's cwd, which is itself a
        // Unix-only path — on Windows the feature falls back to the home dir.
        #[cfg(unix)]
        let shell_pid = pty.child().id();
        #[cfg(not(unix))]
        let shell_pid = 0u32;

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
            initial_command,
        })
    }

    pub fn set_theme(&self, theme: &str) {
        let mut term = self.terminal.lock();
        apply_terminal_theme(&mut term, TerminalTheme::from_name(Some(theme)));
        let _ = self.wakeup.send(RenderEvent::Wake);
    }

    fn send_pty(&self, data: Vec<u8>) -> Result<(), String> {
        self.sender
            .send(Msg::Input(Cow::Owned(data)))
            .map_err(|error| error.to_string())?;
        let _ = self.wakeup.send(RenderEvent::Wake);
        Ok(())
    }

    /// Write user input (keystrokes) to the PTY.
    pub fn input(&self, data: &[u8]) -> Result<(), String> {
        // Reset the scrollback view to the bottom before forwarding input,
        // mirroring the behavior of most terminals.
        {
            let mut term = self.terminal.lock();
            term.scroll_display(Scroll::Bottom);
        }
        self.send_pty(data.to_vec())
    }

    /// Handle a wheel gesture using normal terminal semantics.
    ///
    /// Full-screen applications either receive mouse-wheel reports or, when
    /// alternate scrolling is enabled, cursor-key input. Only the normal screen
    /// moves through the emulator's scrollback history.
    pub fn wheel(
        &self,
        delta: i32,
        column: u16,
        row: u16,
        modifiers: u8,
    ) -> Result<(), String> {
        let delta = delta.clamp(-32, 32);
        if delta == 0 {
            return Ok(());
        }

        let input = {
            let mut term = self.terminal.lock();
            let mode = *term.mode();
            if mode.intersects(TermMode::MOUSE_MODE) {
                mouse_wheel_input(mode, delta, column, row, modifiers)
            } else if mode.contains(TermMode::ALT_SCREEN) {
                mode.contains(TermMode::ALTERNATE_SCROLL)
                    .then(|| alternate_scroll_input(mode, delta))
            } else {
                term.scroll_display(Scroll::Delta(delta));
                None
            }
        };

        if let Some(input) = input {
            self.send_pty(input)
        } else {
            let _ = self.wakeup.send(RenderEvent::Wake);
            Ok(())
        }
    }

    /// Forward a mouse button or motion event when the application requested
    /// terminal mouse reporting. Holding Shift is handled in the frontend and
    /// keeps local text selection available.
    pub fn mouse(
        &self,
        kind: &str,
        button: u8,
        column: u16,
        row: u16,
        modifiers: u8,
    ) -> Result<(), String> {
        let mode = *self.terminal.lock().mode();
        let Some(input) = mouse_input(mode, kind, button, column, row, modifiers) else {
            return Ok(());
        };
        self.send_pty(input)
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

    /// Continue in the system terminal as closely as a standalone PTY allows.
    ///
    /// A PTY's controlling process and live screen cannot be attached to a
    /// second terminal without a persistent session broker. We carry over the
    /// working directory and restart the launcher command only while its direct
    /// foreground child still matches it, avoiding stale command replay.
    pub fn open_in_default_terminal(&self) -> Result<(), String> {
        let cwd = read_cwd(self.shell_pid).unwrap_or_else(|| dirs::home_dir().unwrap_or_default());
        let command = self
            .initial_command
            .as_deref()
            .filter(|command| initial_command_is_running(self.shell_pid, command));
        open_terminal_at(&cwd, command)
    }
}

fn alternate_scroll_input(mode: TermMode, delta: i32) -> Vec<u8> {
    let app_cursor = mode.contains(TermMode::APP_CURSOR);
    let sequence = match (delta.is_positive(), app_cursor) {
        (true, true) => b"\x1bOA".as_slice(),
        (true, false) => b"\x1b[A".as_slice(),
        (false, true) => b"\x1bOB".as_slice(),
        (false, false) => b"\x1b[B".as_slice(),
    };
    sequence.repeat(delta.unsigned_abs() as usize)
}

fn mouse_wheel_input(
    mode: TermMode,
    delta: i32,
    column: u16,
    row: u16,
    modifiers: u8,
) -> Option<Vec<u8>> {
    let button = if delta.is_positive() { 64 } else { 65 };
    let report = encode_mouse_report(mode, "press", button, column, row, modifiers)?;
    Some(report.repeat(delta.unsigned_abs() as usize))
}

fn mouse_input(
    mode: TermMode,
    kind: &str,
    button: u8,
    column: u16,
    row: u16,
    modifiers: u8,
) -> Option<Vec<u8>> {
    if !mode.intersects(TermMode::MOUSE_MODE) {
        return None;
    }
    match kind {
        "press" | "release" if button <= 2 => {}
        "move"
            if mode.contains(TermMode::MOUSE_MOTION)
                || (mode.contains(TermMode::MOUSE_DRAG) && button <= 2) => {}
        _ => return None,
    }
    encode_mouse_report(mode, kind, button, column, row, modifiers)
}

fn encode_mouse_report(
    mode: TermMode,
    kind: &str,
    button: u8,
    column: u16,
    row: u16,
    modifiers: u8,
) -> Option<Vec<u8>> {
    let modifiers = modifiers & 0b1_1100;
    let mut code = button | modifiers;
    if kind == "move" {
        code |= 32;
    }

    if mode.contains(TermMode::SGR_MOUSE) {
        let suffix = if kind == "release" { 'm' } else { 'M' };
        return Some(
            format!(
                "\x1b[<{code};{};{}{suffix}",
                u32::from(column) + 1,
                u32::from(row) + 1
            )
            .into_bytes(),
        );
    }

    if kind == "release" {
        code = 3 | modifiers;
    }
    let values = [u32::from(code) + 32, u32::from(column) + 33, u32::from(row) + 33];
    let mut report = b"\x1b[M".to_vec();
    if mode.contains(TermMode::UTF8_MOUSE) {
        for value in values {
            let character = char::from_u32(value)?;
            let mut encoded = [0; 4];
            report.extend_from_slice(character.encode_utf8(&mut encoded).as_bytes());
        }
    } else {
        for value in values {
            report.push(u8::try_from(value).ok()?);
        }
    }
    Some(report)
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

    // This mirrors the named Tauri command arguments without a lossy wrapper.
    #[allow(clippy::too_many_arguments)]
    pub fn spawn(
        &self,
        id: String,
        app: AppHandle,
        shell: Option<String>,
        initial_command: Option<String>,
        theme: Option<String>,
        cols: u16,
        rows: u16,
    ) -> Result<(), String> {
        let mut sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        if let Some(session) = sessions.remove(&id) {
            session.close();
        }
        let session = TerminalSession::new(
            id.clone(),
            app,
            shell,
            initial_command,
            theme,
            cols,
            rows,
        )?;
        sessions.insert(id, session);
        Ok(())
    }

    pub fn set_theme(&self, id: &str, theme: &str) -> Result<(), String> {
        let sessions = self.sessions.lock().map_err(|error| error.to_string())?;
        if let Some(session) = sessions.get(id) {
            session.set_theme(theme);
        }
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

    pub fn wheel(
        &self,
        id: &str,
        delta: i32,
        column: u16,
        row: u16,
        modifiers: u8,
    ) -> Result<(), String> {
        let sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        match sessions.get(id) {
            Some(session) => session.wheel(delta, column, row, modifiers),
            None => Ok(()),
        }
    }

    pub fn mouse(
        &self,
        id: &str,
        kind: &str,
        button: u8,
        column: u16,
        row: u16,
        modifiers: u8,
    ) -> Result<(), String> {
        let sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        match sessions.get(id) {
            Some(session) => session.mouse(kind, button, column, row, modifiers),
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

/// Whether the shell's direct foreground child still matches the command that
/// opened this launcher session. Complex shell expressions are deliberately not
/// resumed: carrying the directory is better than replaying a pipeline or a
/// partially completed command in a second terminal.
fn initial_command_is_running(pid: u32, command: &str) -> bool {
    let Some(expected) = resumable_program(command) else {
        return false;
    };
    direct_child_programs(pid)
        .into_iter()
        .any(|program| program == expected)
}

fn resumable_program(command: &str) -> Option<String> {
    let command = command.trim();
    if command.is_empty() || command.contains(['|', ';', '&', '>', '<', '`', '$', '\n']) {
        return None;
    }
    let program = command.split_whitespace().next()?;
    if program.contains('=') {
        return None;
    }
    Path::new(program)
        .file_name()
        .and_then(|name| name.to_str())
        .map(str::to_string)
}

#[cfg(target_os = "linux")]
fn direct_child_programs(pid: u32) -> Vec<String> {
    let children = std::fs::read_to_string(format!("/proc/{pid}/task/{pid}/children"))
        .unwrap_or_default();
    children
        .split_whitespace()
        .filter_map(|child| std::fs::read_to_string(format!("/proc/{child}/comm")).ok())
        .map(|program| program.trim().to_string())
        .collect()
}

#[cfg(target_os = "macos")]
fn direct_child_programs(pid: u32) -> Vec<String> {
    if pid == 0 {
        return Vec::new();
    }
    let Ok(children) = Command::new("pgrep")
        .args(["-P", &pid.to_string()])
        .output()
    else {
        return Vec::new();
    };
    if !children.status.success() {
        return Vec::new();
    }
    children
        .stdout
        .split(|byte| *byte == b'\n')
        .filter_map(|child| std::str::from_utf8(child).ok()?.trim().parse::<u32>().ok())
        .filter_map(|child| {
            Command::new("ps")
                .args(["-p", &child.to_string(), "-o", "comm="])
                .output()
                .ok()
        })
        .filter(|output| output.status.success())
        .filter_map(|output| {
            let program = String::from_utf8_lossy(&output.stdout).trim().to_string();
            Path::new(&program)
                .file_name()
                .and_then(|name| name.to_str())
                .map(str::to_string)
        })
        .collect()
}

#[cfg(not(any(target_os = "linux", target_os = "macos")))]
fn direct_child_programs(_pid: u32) -> Vec<String> {
    Vec::new()
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
#[cfg(unix)]
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
fn open_terminal_at(dir: &Path, resume_command: Option<&str>) -> Result<(), String> {
    // Terminal owns the new PTY, so the live emulator state cannot be copied.
    // Re-running a still-active launcher command after `cd` gives full-screen
    // applications a visually continuous handoff without keeping every shell
    // behind a session broker.
    let mut command = format!("cd {}", sh_quote(&dir.to_string_lossy()));
    if let Some(resume_command) = resume_command {
        command.push_str(" && ");
        command.push_str(resume_command);
    }
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
fn open_terminal_at(dir: &Path, resume_command: Option<&str>) -> Result<(), String> {
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
        if let Some(resume_command) = resume_command {
            let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".to_string());
            let script = format!("{resume_command}\nexec {} -l", sh_quote(&shell));
            let terminal = Path::new(name)
                .file_name()
                .and_then(|name| name.to_str())
                .unwrap_or(name);
            match terminal {
                "gnome-terminal" => {
                    command.args(["--", &shell, "-lc", &script]);
                }
                "kitty" | "foot" => {
                    command.args([&shell, "-lc", &script]);
                }
                "wezterm" => {
                    command.args(["start", "--", &shell, "-lc", &script]);
                }
                "xfce4-terminal" | "tilix" => {
                    command
                        .arg("--command")
                        .arg(format!("{} -lc {}", sh_quote(&shell), sh_quote(&script)));
                }
                "terminator" => {
                    command.args(["-x", &shell, "-lc", &script]);
                }
                _ => {
                    command.args(["-e", &shell, "-lc", &script]);
                }
            }
        }
        if command.spawn().is_ok() {
            return Ok(());
        }
    }
    Err("no terminal emulator found".to_string())
}

/// Open Windows Terminal (or `cmd` as a fallback) at `dir`.
#[cfg(target_os = "windows")]
fn open_terminal_at(dir: &Path, resume_command: Option<&str>) -> Result<(), String> {
    // Windows Terminal (preferred, but not pre-installed on all systems).
    let mut windows_terminal = Command::new("wt");
    windows_terminal.arg("-d").arg(dir);
    if let Some(resume_command) = resume_command {
        windows_terminal.args(["cmd", "/K", resume_command]);
    }
    if windows_terminal.spawn().is_ok() {
        return Ok(());
    }
    // Fallback: a new cmd window. `start cmd /K "cd /D <dir>"` explicitly
    // changes directory rather than relying on `current_dir` inheritance,
    // which `start` does not always forward to the spawned child.
    let dir_str = dir.to_string_lossy();
    let mut script = format!("cd /D \"{dir_str}\"");
    if let Some(resume_command) = resume_command {
        script.push_str(" && ");
        script.push_str(resume_command);
    }
    Command::new("cmd")
        .args(["/C", "start", "cmd", "/K", &script])
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn alternate_scroll_uses_the_active_cursor_mode() {
        assert_eq!(
            alternate_scroll_input(TermMode::ALTERNATE_SCROLL, 2),
            b"\x1b[A\x1b[A"
        );
        assert_eq!(
            alternate_scroll_input(TermMode::ALTERNATE_SCROLL | TermMode::APP_CURSOR, -1),
            b"\x1bOB"
        );
    }

    #[test]
    fn sgr_mouse_reports_wheel_position_and_modifiers() {
        let mode = TermMode::MOUSE_REPORT_CLICK | TermMode::SGR_MOUSE;
        assert_eq!(
            mouse_wheel_input(mode, 1, 3, 2, 4).unwrap(),
            b"\x1b[<68;4;3M"
        );
        assert_eq!(
            mouse_input(mode, "release", 0, 3, 2, 0).unwrap(),
            b"\x1b[<0;4;3m"
        );
    }

    #[test]
    fn legacy_mouse_uses_x10_coordinates() {
        let mode = TermMode::MOUSE_REPORT_CLICK;
        assert_eq!(
            mouse_input(mode, "press", 0, 3, 2, 0).unwrap(),
            vec![0x1b, b'[', b'M', 32, 36, 35]
        );
        assert!(mouse_input(mode, "move", 0, 3, 2, 0).is_none());
    }

    #[test]
    fn only_simple_launcher_commands_are_resumable() {
        assert_eq!(resumable_program("ttm --all"), Some("ttm".to_string()));
        assert_eq!(
            resumable_program("/usr/local/bin/ttm --all"),
            Some("ttm".to_string())
        );
        assert_eq!(resumable_program("ttm | less"), None);
        assert_eq!(resumable_program("ttm && echo done"), None);
        assert_eq!(resumable_program("FOO=bar ttm"), None);
    }

    #[cfg(unix)]
    #[test]
    fn recognizes_a_matching_direct_child() {
        let mut child = Command::new("sleep").arg("5").spawn().unwrap();
        std::thread::sleep(Duration::from_millis(20));
        assert!(initial_command_is_running(std::process::id(), "sleep 5"));
        let _ = child.kill();
        let _ = child.wait();
    }

    #[test]
    fn light_theme_uses_high_contrast_named_ansi_colors() {
        use alacritty_terminal::event::VoidListener;
        use alacritty_terminal::vte::ansi::{Color, NamedColor};

        let size = TermSize { cols: 80, rows: 24 };
        let mut term = Term::new(Config::default(), &size, VoidListener);
        apply_terminal_theme(&mut term, TerminalTheme::Light);
        assert_eq!(
            crate::terminal::color::resolve(&Color::Named(NamedColor::Red), term.colors()),
            LIGHT_ANSI[NamedColor::Red as usize]
        );
        assert_eq!(
            crate::terminal::color::resolve(&Color::Named(NamedColor::White), term.colors()),
            LIGHT_ANSI[NamedColor::White as usize]
        );
    }
}
