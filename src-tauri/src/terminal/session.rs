//! Alacritty-backed terminal session.
//!
//! Owns an [`alacritty_terminal::term::Term`] behind a `FairMutex`, fed by
//! alacritty's PTY event loop. A dedicated render thread coalesces the
//! terminal's `Wakeup` events, serializes the visible grid, and pushes binary
//! frames to the frontend.

use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::mpsc::{self, Receiver, Sender};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use alacritty_terminal::event::{Event, EventListener};
use alacritty_terminal::grid::{Dimensions, Scroll};
use alacritty_terminal::sync::FairMutex;
use alacritty_terminal::term::{Config, Term, TermMode};
use alacritty_terminal::vte::ansi::{CursorShape, CursorStyle, Processor, Rgb};
use base64::Engine;
use serde::Serialize;
use tauri::{AppHandle, Emitter};

use super::broker::{self, BrokerCallbacks, BrokerInput, BrokerSession};
use super::frame;

/// Maximum render rate cap, expressed as the idle window used to coalesce a
/// burst of `Wakeup` notifications into a single frame.
const COALESCE: Duration = Duration::from_millis(8);

/// Poll interval for the render thread's shutdown check.
const POLL: Duration = Duration::from_millis(100);

/// qscreen disconnects a byte client when its own 16-event queue fills. Drain
/// IPC into a bounded parsing queue first so ordinary large logs cannot trip
/// that limit, while still placing a hard cap on Floter's buffered output.
const PARSE_QUEUE_CAPACITY: usize = 512;

/// Payload emitted to the frontend on every rendered frame.
#[derive(Clone, Serialize)]
struct FrameEvent<'a> {
    id: &'a str,
    generation: u64,
    frame: String,
}

/// Payload emitted when the PTY child exits.
#[derive(Clone, Serialize)]
struct ExitEvent<'a> {
    id: &'a str,
    generation: u64,
    code: Option<i32>,
}

enum RenderEvent {
    Wake,
    ChildExit(Option<i32>),
}

enum ParserEvent {
    Output(Vec<u8>),
    Exit(Option<i32>),
}

#[derive(Clone)]
struct SessionIdentity {
    id: String,
    generation: u64,
}

/// Event listener that forwards redraw-worthy events to the render thread, and
/// routes terminal query responses (`PtyWrite`) back into the PTY.
#[derive(Clone)]
struct RenderListener {
    tx: Sender<RenderEvent>,
    /// The broker write path, filled in once its attach stream is ready.
    ///
    /// `Arc<Mutex<Option<_>>>` because the listener is cloned — it is handed to
    /// both `Term::new` and `EventLoop::new` — while the sender only exists
    /// *after* the event loop it belongs to, i.e. after both of those clones
    /// have been made. The shared cell lets the later assignment reach the
    /// copies that were already given away.
    pty_sender: Arc<Mutex<Option<BrokerInput>>>,
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
                        let _ = sender.send(text.into_bytes());
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
    Rgb {
        r: 0x1d,
        g: 0x1f,
        b: 0x21,
    },
    Rgb {
        r: 0xc7,
        g: 0x2e,
        b: 0x3c,
    },
    Rgb {
        r: 0x19,
        g: 0x73,
        b: 0x3a,
    },
    Rgb {
        r: 0x7a,
        g: 0x5c,
        b: 0x00,
    },
    Rgb {
        r: 0x16,
        g: 0x59,
        b: 0xb7,
    },
    Rgb {
        r: 0x9c,
        g: 0x2c,
        b: 0xa0,
    },
    Rgb {
        r: 0x00,
        g: 0x7c,
        b: 0x86,
    },
    Rgb {
        r: 0x53,
        g: 0x57,
        b: 0x62,
    },
    Rgb {
        r: 0x76,
        g: 0x7b,
        b: 0x86,
    },
    Rgb {
        r: 0xd6,
        g: 0x3d,
        b: 0x4b,
    },
    Rgb {
        r: 0x1a,
        g: 0x6e,
        b: 0x38,
    },
    Rgb {
        r: 0x7a,
        g: 0x59,
        b: 0x00,
    },
    Rgb {
        r: 0x24,
        g: 0x6f,
        b: 0xd1,
    },
    Rgb {
        r: 0xac,
        g: 0x38,
        b: 0xa9,
    },
    Rgb {
        r: 0x00,
        g: 0x6e,
        b: 0x7a,
    },
    Rgb {
        r: 0x2c,
        g: 0x30,
        b: 0x37,
    },
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

/// A single Alacritty terminal view attached to a broker-owned PTY.
pub struct TerminalSession {
    identity: SessionIdentity,
    broker: BrokerSession,
    cwd: PathBuf,
    preserve_broker: AtomicBool,
    closed: AtomicBool,
    terminal: Arc<FairMutex<Term<RenderListener>>>,
    /// Manual wake channel (e.g. after an out-of-band resize).
    wakeup: Sender<RenderEvent>,
    alive: Arc<AtomicBool>,
}

impl TerminalSession {
    /// Spawn a new terminal session running `shell` (or the user's default).
    #[allow(clippy::too_many_arguments)]
    fn new(
        identity: SessionIdentity,
        app: AppHandle,
        shell: Option<String>,
        initial_command: Option<String>,
        command: Option<broker::SpawnCommand>,
        cwd: Option<PathBuf>,
        theme: Option<String>,
        cols: u16,
        rows: u16,
    ) -> Result<Self, String> {
        let cwd = cwd.unwrap_or_else(|| dirs::home_dir().unwrap_or_default());
        let name = format!("floter-{}-{}", std::process::id(), identity.generation);
        Self::from_broker(
            identity,
            app,
            cwd.clone(),
            theme,
            cols,
            rows,
            false,
            move |callbacks| {
                broker::spawn_session_with_command(
                    name,
                    shell,
                    cwd,
                    initial_command,
                    command,
                    cols,
                    rows,
                    callbacks,
                )
            },
        )
    }

    /// Attach the renderer to a daemon session that outlived its previous UI.
    fn attach(
        identity: SessionIdentity,
        app: AppHandle,
        broker_session_id: String,
        theme: Option<String>,
        cols: u16,
        rows: u16,
    ) -> Result<Self, String> {
        let info = broker::list_sessions()?
            .into_iter()
            .find(|session| session.session_id == broker_session_id)
            .ok_or_else(|| "terminal session no longer exists".to_string())?;
        let cwd = if info.cwd.is_empty() {
            dirs::home_dir().unwrap_or_default()
        } else {
            PathBuf::from(info.cwd)
        };
        Self::from_broker(
            identity,
            app,
            cwd,
            theme,
            cols,
            rows,
            true,
            move |callbacks| broker::attach_session(broker_session_id, cols, rows, callbacks),
        )
    }

    #[allow(clippy::too_many_arguments)]
    fn from_broker(
        identity: SessionIdentity,
        app: AppHandle,
        cwd: PathBuf,
        theme: Option<String>,
        cols: u16,
        rows: u16,
        preserve_on_close: bool,
        broker_factory: impl FnOnce(BrokerCallbacks) -> Result<BrokerSession, String>,
    ) -> Result<Self, String> {
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
        let (parser_tx, parser_rx) = mpsc::sync_channel(PARSE_QUEUE_CAPACITY);
        let output_tx = parser_tx.clone();
        let broker = broker_factory(BrokerCallbacks {
            output: Box::new(move |output| {
                let _ = output_tx.send(ParserEvent::Output(output));
            }),
            exit: Box::new(move |code| {
                let _ = parser_tx.send(ParserEvent::Exit(code));
            }),
        })?;

        // Install the terminal-query reply path before parsing any queued PTY
        // output. Programs probing terminal capabilities can write immediately.
        if let Ok(mut slot) = pty_sender.lock() {
            *slot = Some(broker.writer());
        }

        let alive = Arc::new(AtomicBool::new(true));
        if let Err(error) =
            spawn_broker_parser_thread(terminal.clone(), parser_rx, tx.clone(), alive.clone())
        {
            if preserve_on_close {
                broker.detach();
            } else {
                broker.kill();
            }
            return Err(error);
        }
        if let Err(error) =
            spawn_render_thread(identity.clone(), app, terminal.clone(), rx, alive.clone())
        {
            if preserve_on_close {
                broker.detach();
            } else {
                broker.kill();
            }
            return Err(error);
        }

        Ok(Self {
            identity,
            broker,
            cwd,
            preserve_broker: AtomicBool::new(preserve_on_close),
            closed: AtomicBool::new(false),
            terminal,
            wakeup: tx,
            alive,
        })
    }

    pub fn set_theme(&self, theme: &str) {
        let mut term = self.terminal.lock();
        apply_terminal_theme(&mut term, TerminalTheme::from_name(Some(theme)));
        let _ = self.wakeup.send(RenderEvent::Wake);
    }

    fn send_pty(&self, data: Vec<u8>) -> Result<(), String> {
        self.broker.input(data)?;
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
    pub fn wheel(&self, delta: i32, column: u16, row: u16, modifiers: u8) -> Result<(), String> {
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
        let cols = cols.max(2);
        let rows = rows.max(1);
        let size = TermSize {
            cols: cols as usize,
            rows: rows as usize,
        };
        {
            let mut term = self.terminal.lock();
            term.resize(size);
        }
        self.broker.resize(cols, rows)?;
        // Prompt an immediate re-render so the new dimensions show up.
        let _ = self.wakeup.send(RenderEvent::Wake);
        Ok(())
    }

    /// Shut down the view and either kill or detach its broker session.
    pub fn close(&self) {
        if self.closed.swap(true, Ordering::SeqCst) {
            return;
        }
        self.alive.store(false, Ordering::SeqCst);
        let _ = self.wakeup.send(RenderEvent::Wake);
        if self.preserve_broker.load(Ordering::SeqCst) {
            self.broker.detach();
        } else {
            self.broker.kill();
        }
    }

    fn detach_for_handoff(&self) {
        self.preserve_broker.store(true, Ordering::SeqCst);
        self.close();
    }

    /// Continue the same broker-owned PTY in the system terminal.
    fn external_terminal_request(&self) -> ExternalTerminalRequest {
        ExternalTerminalRequest {
            generation: self.identity.generation,
            cwd: self.cwd.clone(),
            session_id: self.broker.session_id().to_string(),
        }
    }
}

/// Everything needed to open a system terminal, copied out while the session
/// manager is locked so the potentially slow platform launcher runs without
/// blocking terminal input or another IPC command.
pub struct ExternalTerminalRequest {
    generation: u64,
    cwd: PathBuf,
    session_id: String,
}

#[derive(Serialize)]
pub struct ExternalTerminalOutcome {
    pub session_handed_off: bool,
}

impl ExternalTerminalRequest {
    pub fn generation(&self) -> u64 {
        self.generation
    }

    pub fn preserves_session(&self) -> bool {
        true
    }

    pub fn open(self) -> Result<ExternalTerminalOutcome, String> {
        #[cfg(unix)]
        let external_command = broker::attach_command(&self.session_id)?;
        #[cfg(unix)]
        let session_handed_off = open_terminal_at(&self.cwd, Some(&external_command))?;
        #[cfg(windows)]
        let session_handed_off = {
            broker::open_windows_terminal(&self.session_id, &self.cwd)?;
            true
        };
        Ok(ExternalTerminalOutcome { session_handed_off })
    }
}

#[cfg(target_os = "macos")]
pub(crate) fn open_terminal_session(
    dir: &Path,
    resume_command: &str,
    preferred: Option<&str>,
) -> Result<bool, String> {
    let preferred = preferred.filter(|name| !matches!(*name, "default" | "system" | "terminal"));
    match preferred {
        Some(name) => open_named_terminal_at(name, dir, resume_command),
        None => open_terminal_at(dir, Some(resume_command)),
    }
}

#[cfg(target_os = "linux")]
pub(crate) fn open_terminal_session(
    dir: &Path,
    resume_command: &str,
    preferred: Option<&str>,
) -> Result<bool, String> {
    open_terminal_at_with_preference(dir, Some(resume_command), preferred)
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
    let values = [
        u32::from(code) + 32,
        u32::from(column) + 33,
        u32::from(row) + 33,
    ];
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

fn spawn_broker_parser_thread(
    terminal: Arc<FairMutex<Term<RenderListener>>>,
    rx: Receiver<ParserEvent>,
    wakeup: Sender<RenderEvent>,
    alive: Arc<AtomicBool>,
) -> Result<(), String> {
    std::thread::Builder::new()
        .name("term-parser".into())
        .spawn(move || {
            let mut processor: Processor = Processor::new();
            while alive.load(Ordering::SeqCst) {
                match rx.recv_timeout(POLL) {
                    Ok(ParserEvent::Output(output)) => {
                        {
                            let mut term = terminal.lock();
                            processor.advance(&mut *term, &output);
                        }
                        let _ = wakeup.send(RenderEvent::Wake);
                    }
                    Ok(ParserEvent::Exit(code)) => {
                        let _ = wakeup.send(RenderEvent::ChildExit(code));
                        break;
                    }
                    Err(mpsc::RecvTimeoutError::Timeout) => continue,
                    Err(mpsc::RecvTimeoutError::Disconnected) => {
                        let _ = wakeup.send(RenderEvent::ChildExit(None));
                        break;
                    }
                }
            }
        })
        .map(|_| ())
        .map_err(|error| error.to_string())
}

#[allow(clippy::needless_pass_by_value)]
fn spawn_render_thread(
    identity: SessionIdentity,
    app: AppHandle,
    terminal: Arc<FairMutex<Term<RenderListener>>>,
    rx: Receiver<RenderEvent>,
    alive: Arc<AtomicBool>,
) -> Result<(), String> {
    std::thread::Builder::new()
        .name("term-render".into())
        .spawn(move || render_loop(identity, app, terminal, rx, alive))
        .map(|_| ())
        .map_err(|error| error.to_string())
}

fn render_loop(
    identity: SessionIdentity,
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
                        id: &identity.id,
                        generation: identity.generation,
                        frame: encoded,
                    },
                );

                if let Some(code) = child_exit {
                    alive.store(false, Ordering::SeqCst);
                    let _ = app.emit(
                        "term://exit",
                        ExitEvent {
                            id: &identity.id,
                            generation: identity.generation,
                            code,
                        },
                    );
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
        generation: u64,
        app: AppHandle,
        shell: Option<String>,
        initial_command: Option<String>,
        command: Option<broker::SpawnCommand>,
        cwd: Option<PathBuf>,
        theme: Option<String>,
        cols: u16,
        rows: u16,
    ) -> Result<(), String> {
        let mut sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        if let Some(session) = sessions.remove(&id) {
            session.close();
        }
        let identity = SessionIdentity {
            id: id.clone(),
            generation,
        };
        let session = TerminalSession::new(
            identity,
            app,
            shell,
            initial_command,
            command,
            cwd,
            theme,
            cols,
            rows,
        )?;
        sessions.insert(id, session);
        Ok(())
    }

    #[allow(clippy::too_many_arguments)]
    pub fn attach_existing(
        &self,
        id: String,
        generation: u64,
        app: AppHandle,
        broker_session_id: String,
        theme: Option<String>,
        cols: u16,
        rows: u16,
    ) -> Result<(), String> {
        let identity = SessionIdentity {
            id: id.clone(),
            generation,
        };
        let session = TerminalSession::attach(identity, app, broker_session_id, theme, cols, rows)?;
        let mut sessions = self.sessions.lock().map_err(|error| error.to_string())?;
        if let Some(previous) = sessions.remove(&id) {
            previous.close();
        }
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

    pub fn close_if_generation(
        &self,
        id: &str,
        generation: u64,
        preserve_session: bool,
    ) -> Result<(), String> {
        let mut sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        if sessions
            .get(id)
            .is_some_and(|session| session.identity.generation == generation)
        {
            if let Some(session) = sessions.remove(id) {
                if preserve_session {
                    session.detach_for_handoff();
                } else {
                    session.close();
                }
            }
        }
        Ok(())
    }

    pub fn external_terminal_request(&self, id: &str) -> Result<ExternalTerminalRequest, String> {
        let sessions = self.sessions.lock().map_err(|e| e.to_string())?;
        match sessions.get(id) {
            Some(session) => Ok(session.external_terminal_request()),
            None => Err("terminal session is not running".to_string()),
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

/// Wrap `value` in single quotes for safe interpolation into a `/bin/sh` script.
#[cfg(unix)]
fn sh_quote(value: &str) -> String {
    format!("'{}'", value.replace('\'', "'\\''"))
}

/// AppleScript used to start Floter's attach helper in Terminal.app's
/// interactive shell. Terminal creates a default window while cold-starting,
/// before it handles even the first `do script` Apple Event. Reuse that window
/// only on a cold start; when Terminal was already running, a target-less
/// `do script` creates a separate window without touching the user's current
/// shell. Either branch leaves its shell open after the brokered session exits.
#[cfg(target_os = "macos")]
const MACOS_TERMINAL_SCRIPT: &str = r#"on run argv
    set workingDirectory to item 1 of argv
    set commandText to item 2 of argv
    set handoffCommand to "cd -- " & quoted form of workingDirectory & " && " & commandText
    set terminalWasRunning to application id "com.apple.Terminal" is running
    tell application id "com.apple.Terminal"
        if terminalWasRunning then
            do script handoffCommand
        else
            activate
            repeat 40 times
                if exists window 1 then exit repeat
                delay 0.05
            end repeat
            if not (exists window 1) then error "Terminal did not create its initial window"
            do script handoffCommand in selected tab of window 1
        end if
        activate
    end tell
end run"#;

/// Open the macOS system Terminal at `dir`. Active commands are entered through
/// its interactive shell so the command is visible and saved in shell history.
/// If Automation permission is unavailable, open the directory without moving
/// the command and report that the internal session must remain alive.
#[cfg(target_os = "macos")]
fn open_terminal_at(dir: &Path, resume_command: Option<&str>) -> Result<bool, String> {
    if let Some(command) = resume_command {
        let status = Command::new("/usr/bin/osascript")
            .args(["-e", MACOS_TERMINAL_SCRIPT, "--"])
            .arg(dir)
            .arg(command)
            .status()
            .map_err(|error| error.to_string())?;
        if status.success() {
            return Ok(true);
        }
        return Err(format!(
            "Terminal did not accept the session handoff (osascript exited with {status})"
        ));
    }

    let status = Command::new("/usr/bin/open")
        .args(["-b", "com.apple.Terminal"])
        .arg(dir)
        .status()
        .map_err(|error| error.to_string())?;
    if status.success() {
        Ok(false)
    } else {
        Err(format!("system terminal opener exited with {status}"))
    }
}

#[cfg(target_os = "macos")]
fn open_named_terminal_at(name: &str, dir: &Path, resume_command: &str) -> Result<bool, String> {
    let executable =
        which(name).ok_or_else(|| format!("terminal emulator '{name}' was not found"))?;
    let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".to_string());
    let script = format!("{resume_command}\nexec {} -l", sh_quote(&shell));
    let dir_arg = dir.to_string_lossy();
    let terminal = Path::new(name)
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or(name);
    let mut command = Command::new(executable);
    command.current_dir(dir);
    match terminal {
        "kitty" => {
            command.args(["--directory", dir_arg.as_ref(), &shell, "-lc", &script]);
        }
        "alacritty" => {
            command.args([
                "--working-directory",
                dir_arg.as_ref(),
                "-e",
                &shell,
                "-lc",
                &script,
            ]);
        }
        "ghostty" => {
            command.arg(format!("--working-directory={dir_arg}"));
            command.args(["-e", &shell, "-lc", &script]);
        }
        "wezterm" => {
            command.args([
                "start",
                "--cwd",
                dir_arg.as_ref(),
                "--",
                &shell,
                "-lc",
                &script,
            ]);
        }
        _ => {
            command.args(["-e", &shell, "-lc", &script]);
        }
    }
    command
        .spawn()
        .map(|_| true)
        .map_err(|error| format!("failed to start terminal emulator '{name}': {error}"))
}

/// Terminal emulators tried in order when `$TERMINAL` is unset.
#[cfg(target_os = "linux")]
const TERMINALS: &[&str] = &[
    "xdg-terminal-exec",
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
#[cfg(unix)]
fn which(name: &str) -> Option<PathBuf> {
    if name.contains('/') {
        let path = PathBuf::from(name);
        return path.is_file().then_some(path);
    }
    let from_path = std::env::var_os("PATH").and_then(|paths| {
        std::env::split_paths(&paths)
            .map(|dir| dir.join(name))
            .find(|candidate| candidate.is_file())
    });
    if from_path.is_some() {
        return from_path;
    }
    #[cfg(target_os = "macos")]
    {
        let app_executable = match name {
            "kitty" => "/Applications/kitty.app/Contents/MacOS/kitty",
            "alacritty" => "/Applications/Alacritty.app/Contents/MacOS/alacritty",
            "ghostty" => "/Applications/Ghostty.app/Contents/MacOS/ghostty",
            "wezterm" => "/Applications/WezTerm.app/Contents/MacOS/wezterm",
            _ => return None,
        };
        return PathBuf::from(app_executable)
            .is_file()
            .then(|| PathBuf::from(app_executable));
    }
    #[cfg(not(target_os = "macos"))]
    None
}

/// Spawn the first available terminal emulator at `dir`.
#[cfg(target_os = "linux")]
fn open_terminal_at(dir: &Path, resume_command: Option<&str>) -> Result<bool, String> {
    open_terminal_at_with_preference(dir, resume_command, None)
}

#[cfg(target_os = "linux")]
fn open_terminal_at_with_preference(
    dir: &Path,
    resume_command: Option<&str>,
    requested: Option<&str>,
) -> Result<bool, String> {
    let environment_preference = std::env::var("TERMINAL").ok();
    let requested = requested.filter(|name| !matches!(*name, "default" | "system"));
    let candidates = requested
        .into_iter()
        .chain(
            environment_preference
                .as_deref()
                .filter(|_| requested.is_none()),
        )
        .chain(TERMINALS.iter().copied().filter(|_| requested.is_none()));

    for name in candidates {
        if which(name).is_none() {
            continue;
        }
        let mut command = Command::new(name);
        command.current_dir(dir);
        let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/sh".to_string());
        let script = resume_command
            .map(|resume_command| format!("{resume_command}\nexec {} -l", sh_quote(&shell)));
        let terminal = Path::new(name)
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or(name);
        let dir_arg = dir.to_string_lossy();

        match terminal {
            "gnome-terminal" => {
                command.arg(format!("--working-directory={dir_arg}"));
                if let Some(script) = script.as_deref() {
                    command.args(["--", &shell, "-lc", script]);
                }
            }
            "konsole" => {
                command.args(["--workdir", dir_arg.as_ref()]);
                if let Some(script) = script.as_deref() {
                    command.args(["-e", &shell, "-lc", script]);
                }
            }
            "kitty" => {
                command.args(["--directory", dir_arg.as_ref()]);
                if let Some(script) = script.as_deref() {
                    command.args([&shell, "-lc", script]);
                }
            }
            "alacritty" => {
                command.args(["--working-directory", dir_arg.as_ref()]);
                if let Some(script) = script.as_deref() {
                    command.args(["-e", &shell, "-lc", script]);
                }
            }
            "wezterm" => {
                command.args(["start", "--cwd", dir_arg.as_ref()]);
                if let Some(script) = script.as_deref() {
                    command.args(["--", &shell, "-lc", script]);
                }
            }
            "xfce4-terminal" | "tilix" => {
                command.arg(format!("--working-directory={dir_arg}"));
                if let Some(script) = script.as_deref() {
                    command.arg("--command").arg(format!(
                        "{} -lc {}",
                        sh_quote(&shell),
                        sh_quote(script)
                    ));
                }
            }
            "terminator" => {
                command.arg(format!("--working-directory={dir_arg}"));
                if let Some(script) = script.as_deref() {
                    command.args(["-x", &shell, "-lc", script]);
                }
            }
            "ghostty" => {
                command.arg(format!("--working-directory={dir_arg}"));
                if let Some(script) = script.as_deref() {
                    command.args(["-e", &shell, "-lc", script]);
                }
            }
            "xdg-terminal-exec" => {
                if let Some(script) = script.as_deref() {
                    command.args(["--", &shell, "-lc", script]);
                }
            }
            _ => {
                if let Some(script) = script.as_deref() {
                    command.args(["-e", &shell, "-lc", script]);
                }
            }
        }
        if command.spawn().is_ok() {
            return Ok(resume_command.is_some());
        }
    }
    Err("no terminal emulator found".to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[cfg(target_os = "macos")]
    #[test]
    fn macos_terminal_reuses_only_its_cold_start_window() {
        assert!(MACOS_TERMINAL_SCRIPT.contains("quoted form of workingDirectory"));
        assert!(MACOS_TERMINAL_SCRIPT.contains("application id \"com.apple.Terminal\" is running"));
        assert!(MACOS_TERMINAL_SCRIPT
            .contains("if terminalWasRunning then\n            do script handoffCommand"));
        assert!(
            MACOS_TERMINAL_SCRIPT.contains("do script handoffCommand in selected tab of window 1")
        );
    }

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
