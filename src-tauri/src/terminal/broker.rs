//! Persistent terminal sessions backed by qscreen's PTY daemon.
//!
//! Floter uses qscreen's byte protocol only. The daemon owns the PTY and keeps
//! it alive while UI and system-terminal clients come and go; rendering and
//! keyboard policy remain Floter concerns.

use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::process::{Command as ProcessCommand, Stdio};
use std::time::{Duration, Instant};

use anyhow::{Context, Result};
use qscreen_protocol::{
    AttachMode, Command, EventType, Message, MessageKind, SessionInfo, MAX_PAYLOAD_SIZE,
};
use serde::Serialize;
use tokio::io::{AsyncBufReadExt, AsyncRead, AsyncWrite, AsyncWriteExt, BufReader};
use tokio::sync::mpsc::{self, UnboundedReceiver, UnboundedSender};

const NAMESPACE_ENV: &str = "QSCREEN_NAMESPACE";
const NAMESPACE: &str = "floter";
const DAEMON_ARGUMENT: &str = "--terminal-daemon";
const ATTACH_ARGUMENT: &str = "--terminal-attach";
const TERMINAL_COMMAND: &str = "terminal";
const DAEMON_START_TIMEOUT: Duration = Duration::from_secs(5);
const RESIZE_POLL: Duration = Duration::from_millis(150);

#[cfg(unix)]
type BrokerStream = tokio::net::UnixStream;
#[cfg(windows)]
type BrokerStream = tokio::net::windows::named_pipe::NamedPipeClient;

pub struct BrokerCallbacks {
    pub output: Box<dyn FnMut(Vec<u8>) + Send>,
    pub exit: Box<dyn FnMut(Option<i32>) + Send>,
}

#[derive(Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SpawnCommand {
    pub program: String,
    pub args: Vec<String>,
    pub environment: std::collections::BTreeMap<String, String>,
    pub inherit_environment: bool,
}

enum BrokerCommand {
    Input(Vec<u8>),
    Resize(u16, u16),
    Detach,
    Kill,
}

/// Handle to a qscreen session and the UI client's attach stream.
pub struct BrokerSession {
    session_id: String,
    sender: UnboundedSender<BrokerCommand>,
}

#[derive(Clone, Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BrokerSessionInfo {
    pub session_id: String,
    pub name: String,
    pub attached: bool,
    pub exited: bool,
    pub exit_code: i64,
    pub created_at: String,
    pub width: u32,
    pub height: u32,
    pub size: String,
    pub cwd: String,
}

impl From<SessionInfo> for BrokerSessionInfo {
    fn from(info: SessionInfo) -> Self {
        Self {
            session_id: info.session_id,
            name: info.name,
            attached: info.attached,
            exited: info.exited,
            exit_code: info.exit_code,
            created_at: info.created_at.to_rfc3339(),
            width: info.width,
            height: info.height,
            size: info.size,
            cwd: info.cwd,
        }
    }
}

#[derive(Clone)]
pub struct BrokerInput {
    sender: UnboundedSender<BrokerCommand>,
}

impl BrokerInput {
    pub fn send(&self, data: Vec<u8>) -> Result<(), String> {
        self.sender
            .send(BrokerCommand::Input(data))
            .map_err(|_| "terminal broker is no longer connected".to_string())
    }
}

impl BrokerSession {
    pub fn session_id(&self) -> &str {
        &self.session_id
    }

    pub fn input(&self, data: Vec<u8>) -> Result<(), String> {
        self.writer().send(data)
    }

    pub fn writer(&self) -> BrokerInput {
        BrokerInput {
            sender: self.sender.clone(),
        }
    }

    pub fn resize(&self, cols: u16, rows: u16) -> Result<(), String> {
        self.sender
            .send(BrokerCommand::Resize(cols, rows))
            .map_err(|_| "terminal broker is no longer connected".to_string())
    }

    pub fn detach(&self) {
        let _ = self.sender.send(BrokerCommand::Detach);
    }

    pub fn kill(&self) {
        let _ = self.sender.send(BrokerCommand::Kill);
    }
}

/// Set the endpoint namespace before qscreen resolves its socket/pipe path.
/// This variable is intentionally inherited by helper and shell processes; it
/// is Floter-specific and does not alter the user's identity or shell config.
pub fn initialize_environment() {
    std::env::set_var(NAMESPACE_ENV, NAMESPACE);
}

/// Handle terminal-only process modes before Tauri initializes.
pub fn run_helper(arguments: &[String]) -> Option<Result<()>> {
    match arguments.get(1).map(String::as_str) {
        Some(DAEMON_ARGUMENT) => Some(run_daemon()),
        Some(ATTACH_ARGUMENT) => Some(
            arguments
                .get(2)
                .context("missing terminal session id")
                .and_then(|session_id| run_external_attach(session_id)),
        ),
        Some(TERMINAL_COMMAND) => Some(run_terminal_cli(&arguments[2..])),
        _ => None,
    }
}

/// Query the persistent daemon without starting it. A missing daemon simply
/// means there are no resumable sessions yet.
pub fn list_sessions() -> Result<Vec<BrokerSessionInfo>, String> {
    let runtime = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|error| error.to_string())?;
    runtime.block_on(async {
        let mut stream = match connect().await {
            Ok(stream) => stream,
            Err(_) => return Ok(Vec::new()),
        };
        let response = request_on_stream(
            &mut stream,
            Message {
                kind: MessageKind::Request,
                id: "list".into(),
                command: Some(Command::List),
                ..Default::default()
            },
        )
        .await
        .map_err(|error| error.to_string())?;
        check_response(&response, "list").map_err(|error| error.to_string())?;
        Ok(response.sessions.into_iter().map(Into::into).collect())
    })
}

pub fn kill_existing_session(session_id: &str) -> Result<(), String> {
    qscreen_protocol::validate_session_id(session_id).map_err(|error| error.to_string())?;
    let runtime = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|error| error.to_string())?;
    runtime
        .block_on(existing_control_request(Message {
            kind: MessageKind::Request,
            id: "kill".into(),
            command: Some(Command::Kill),
            session_id: session_id.to_string(),
            ..Default::default()
        }))
        .and_then(|response| check_response(&response, "kill"))
        .map_err(|error| error.to_string())
}

#[cfg(test)]
pub fn spawn_session(
    name: String,
    shell: Option<String>,
    cwd: PathBuf,
    initial_command: Option<String>,
    cols: u16,
    rows: u16,
    callbacks: BrokerCallbacks,
) -> Result<BrokerSession, String> {
    spawn_session_with_command(
        name,
        shell,
        cwd,
        initial_command,
        None,
        cols,
        rows,
        callbacks,
    )
}

#[allow(clippy::too_many_arguments)]
pub fn spawn_session_with_command(
    name: String,
    shell: Option<String>,
    cwd: PathBuf,
    initial_command: Option<String>,
    command: Option<SpawnCommand>,
    cols: u16,
    rows: u16,
    callbacks: BrokerCallbacks,
) -> Result<BrokerSession, String> {
    start_session_worker(
        None,
        name,
        shell,
        cwd,
        initial_command,
        command,
        cols,
        rows,
        callbacks,
    )
}

pub fn attach_session(
    session_id: String,
    cols: u16,
    rows: u16,
    callbacks: BrokerCallbacks,
) -> Result<BrokerSession, String> {
    qscreen_protocol::validate_session_id(&session_id).map_err(|error| error.to_string())?;
    start_session_worker(
        Some(session_id),
        String::new(),
        None,
        PathBuf::new(),
        None,
        None,
        cols,
        rows,
        callbacks,
    )
}

#[allow(clippy::too_many_arguments)]
fn start_session_worker(
    existing_session_id: Option<String>,
    name: String,
    shell: Option<String>,
    cwd: PathBuf,
    initial_command: Option<String>,
    command: Option<SpawnCommand>,
    cols: u16,
    rows: u16,
    callbacks: BrokerCallbacks,
) -> Result<BrokerSession, String> {
    let (command_tx, command_rx) = mpsc::unbounded_channel();
    let (ready_tx, ready_rx) = std::sync::mpsc::sync_channel(1);
    std::thread::Builder::new()
        .name("term-broker".into())
        .spawn(move || {
            let runtime = match tokio::runtime::Builder::new_current_thread()
                .enable_all()
                .build()
            {
                Ok(runtime) => runtime,
                Err(error) => {
                    let _ = ready_tx.send(Err(error.to_string()));
                    return;
                }
            };
            runtime.block_on(session_worker(
                existing_session_id,
                name,
                shell,
                cwd,
                initial_command,
                command,
                cols,
                rows,
                command_rx,
                callbacks,
                ready_tx,
            ));
        })
        .map_err(|error| error.to_string())?;

    let session_id = ready_rx
        .recv_timeout(DAEMON_START_TIMEOUT + Duration::from_secs(2))
        .map_err(|_| "terminal broker did not become ready".to_string())??;
    Ok(BrokerSession {
        session_id,
        sender: command_tx,
    })
}

#[allow(clippy::too_many_arguments)]
async fn session_worker(
    existing_session_id: Option<String>,
    name: String,
    shell: Option<String>,
    cwd: PathBuf,
    initial_command: Option<String>,
    command: Option<SpawnCommand>,
    cols: u16,
    rows: u16,
    mut commands: UnboundedReceiver<BrokerCommand>,
    mut callbacks: BrokerCallbacks,
    ready: std::sync::mpsc::SyncSender<Result<String, String>>,
) {
    let attached_existing = existing_session_id.is_some();
    let session_id = match existing_session_id {
        Some(session_id) => session_id,
        None => match create_session(&name, shell.as_deref(), &cwd, command.as_ref(), cols, rows)
            .await
        {
            Ok(session_id) => session_id,
            Err(error) => {
                let _ = ready.send(Err(error.to_string()));
                return;
            }
        },
    };
    let setup = async {
        let stream = connect().await?;
        let (read_half, mut writer) = tokio::io::split(stream);
        let mut reader = BufReader::new(read_half);
        send_message(
            &mut writer,
            Message {
                kind: MessageKind::Request,
                id: "1".into(),
                command: Some(Command::Attach),
                session_id: session_id.clone(),
                width: u32::from(cols.max(2)),
                height: u32::from(rows.max(1)),
                attach_mode: AttachMode::Bytes,
                ..Default::default()
            },
        )
        .await?;
        let response = receive_message(&mut reader).await?;
        check_response(&response, "1")?;
        Ok::<_, anyhow::Error>((reader, writer))
    }
    .await;

    let (mut reader, mut writer) = match setup {
        Ok(value) => value,
        Err(error) => {
            if !attached_existing {
                let _ = kill_session(&session_id).await;
            }
            let _ = ready.send(Err(error.to_string()));
            return;
        }
    };

    if ready.send(Ok(session_id.clone())).is_err() {
        if !attached_existing {
            let _ = kill_session(&session_id).await;
        }
        return;
    }

    let mut message_id = 2_u64;
    if let Some(command) = initial_command.filter(|command| !command.is_empty()) {
        let mut payload = command.into_bytes();
        payload.push(b'\r');
        if send_input(&mut writer, &session_id, &mut message_id, payload)
            .await
            .is_err()
        {
            (callbacks.exit)(None);
            return;
        }
    }

    let mut line = String::new();
    loop {
        tokio::select! {
            read = receive_message_reusing(&mut reader, &mut line) => {
                match read {
                    Ok(message) if message.kind == MessageKind::Event
                        && message.event == Some(EventType::Output) => {
                            (callbacks.output)(message.payload);
                        }
                    Ok(message) if message.kind == MessageKind::Event
                        && message.event == Some(EventType::Exit) => {
                            let code = i32::try_from(message.exit_code).ok();
                            (callbacks.exit)(code);
                            return;
                        }
                    Ok(message) if message.kind == MessageKind::Response && !message.error.is_empty() => {
                        (callbacks.exit)(None);
                        return;
                    }
                    Ok(_) => {}
                    Err(_) => {
                        (callbacks.exit)(None);
                        return;
                    }
                }
            }
            command = commands.recv() => {
                match command {
                    Some(BrokerCommand::Input(data)) => {
                        if send_input(&mut writer, &session_id, &mut message_id, data).await.is_err() {
                            (callbacks.exit)(None);
                            return;
                        }
                    }
                    Some(BrokerCommand::Resize(cols, rows)) => {
                        message_id += 1;
                        if send_message(&mut writer, Message {
                            kind: MessageKind::Request,
                            id: message_id.to_string(),
                            command: Some(Command::Resize),
                            session_id: session_id.clone(),
                            width: u32::from(cols.max(2)),
                            height: u32::from(rows.max(1)),
                            ..Default::default()
                        }).await.is_err() {
                            (callbacks.exit)(None);
                            return;
                        }
                    }
                    Some(BrokerCommand::Detach) => return,
                    Some(BrokerCommand::Kill) | None => {
                        drop(writer);
                        let _ = kill_session(&session_id).await;
                        return;
                    }
                }
            }
        }
    }
}

async fn send_input<W: AsyncWrite + Unpin>(
    writer: &mut W,
    session_id: &str,
    message_id: &mut u64,
    data: Vec<u8>,
) -> Result<()> {
    for chunk in data.chunks(MAX_PAYLOAD_SIZE) {
        *message_id += 1;
        send_message(
            writer,
            Message {
                kind: MessageKind::Request,
                id: message_id.to_string(),
                command: Some(Command::Input),
                session_id: session_id.to_string(),
                payload: chunk.to_vec(),
                ..Default::default()
            },
        )
        .await?;
    }
    Ok(())
}

async fn create_session(
    name: &str,
    shell: Option<&str>,
    cwd: &Path,
    command: Option<&SpawnCommand>,
    cols: u16,
    rows: u16,
) -> Result<String> {
    let payload = command
        .map(serde_json::to_vec)
        .transpose()
        .context("serialize structured terminal command")?
        .unwrap_or_default();
    let response = control_request(Message {
        kind: MessageKind::Request,
        id: "new".into(),
        command: Some(Command::New),
        name: name.to_string(),
        shell: shell.unwrap_or_default().to_string(),
        cwd: cwd.to_string_lossy().into_owned(),
        payload,
        width: u32::from(cols.max(2)),
        height: u32::from(rows.max(1)),
        ..Default::default()
    })
    .await?;
    check_response(&response, "new")?;
    if response.session_id.is_empty() {
        anyhow::bail!("terminal broker returned an empty session id");
    }
    Ok(response.session_id)
}

async fn kill_session(session_id: &str) -> Result<()> {
    let response = control_request(Message {
        kind: MessageKind::Request,
        id: "kill".into(),
        command: Some(Command::Kill),
        session_id: session_id.to_string(),
        ..Default::default()
    })
    .await?;
    check_response(&response, "kill")
}

async fn control_request(message: Message) -> Result<Message> {
    let mut stream = ensure_connected().await?;
    request_on_stream(&mut stream, message).await
}

async fn existing_control_request(message: Message) -> Result<Message> {
    let mut stream = connect().await?;
    request_on_stream(&mut stream, message).await
}

async fn request_on_stream(stream: &mut BrokerStream, message: Message) -> Result<Message> {
    send_message(stream, message).await?;
    let mut reader = BufReader::new(stream);
    receive_message(&mut reader).await
}

async fn ensure_connected() -> Result<BrokerStream> {
    if let Ok(stream) = connect().await {
        return Ok(stream);
    }
    spawn_daemon_process()?;
    let deadline = Instant::now() + DAEMON_START_TIMEOUT;
    loop {
        if let Ok(stream) = connect().await {
            return Ok(stream);
        }
        if Instant::now() >= deadline {
            anyhow::bail!("terminal daemon did not start within 5 seconds");
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }
}

#[cfg(unix)]
async fn connect() -> Result<BrokerStream> {
    let path = qscreen_shared::pipe_name();
    tokio::net::UnixStream::connect(&path)
        .await
        .with_context(|| format!("connect to terminal daemon socket {path}"))
}

#[cfg(windows)]
async fn connect() -> Result<BrokerStream> {
    use tokio::net::windows::named_pipe::ClientOptions;
    let path = qscreen_shared::pipe_name();
    ClientOptions::new()
        .open(&path)
        .with_context(|| format!("connect to terminal daemon pipe {path}"))
}

fn spawn_daemon_process() -> Result<()> {
    let executable = std::env::current_exe().context("resolve Floter executable")?;
    let mut command = ProcessCommand::new(executable);
    command.arg(DAEMON_ARGUMENT).env(NAMESPACE_ENV, NAMESPACE);
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        const DETACHED_PROCESS: u32 = 0x0000_0008;
        command.creation_flags(CREATE_NO_WINDOW | DETACHED_PROCESS);
    }
    #[cfg(not(windows))]
    {
        command
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null());
    }
    command.spawn().context("start terminal daemon")?;
    Ok(())
}

fn run_daemon() -> Result<()> {
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .context("create terminal daemon runtime")?;
    runtime.block_on(qscreen_daemon::run())
}

fn run_external_attach(session_id: &str) -> Result<()> {
    qscreen_protocol::validate_session_id(session_id)?;
    let runtime = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .context("create terminal attach runtime")?;
    runtime.block_on(external_attach(session_id))
}

fn run_terminal_cli(arguments: &[String]) -> Result<()> {
    match arguments.first().map(String::as_str) {
        Some("list") => {
            let sessions = list_sessions().map_err(anyhow::Error::msg)?;
            if arguments.iter().skip(1).any(|argument| argument == "--json") {
                println!("{}", serde_json::to_string_pretty(&sessions)?);
                return Ok(());
            }
            if sessions.is_empty() {
                return Ok(());
            }
            println!("ID\tSTATE\tSIZE\tCWD\tNAME");
            for session in sessions {
                let state = if session.exited {
                    "exited"
                } else if session.attached {
                    "attached"
                } else {
                    "detached"
                };
                println!(
                    "{}\t{}\t{}\t{}\t{}",
                    session.session_id, state, session.size, session.cwd, session.name
                );
            }
            Ok(())
        }
        Some("attach") => arguments
            .get(1)
            .context("usage: floter terminal attach <session-id>")
            .and_then(|session_id| run_external_attach(session_id)),
        Some("kill") => arguments
            .get(1)
            .context("usage: floter terminal kill <session-id>")
            .and_then(|session_id| kill_existing_session(session_id).map_err(anyhow::Error::msg)),
        Some("switch") => run_terminal_switch(&arguments[1..]),
        Some("detach") => anyhow::bail!(
            "detach is local to the currently attached client; close that terminal client to detach without killing the session"
        ),
        Some("help") | Some("--help") | Some("-h") | None => {
            println!(
                "floter terminal list [--json]\n\
                 floter terminal attach <session-id>\n\
                 floter terminal kill <session-id>\n\
                 floter terminal switch <session-id> [--terminal <name>]"
            );
            Ok(())
        }
        Some(command) => anyhow::bail!("unknown terminal command '{command}'"),
    }
}

fn run_terminal_switch(arguments: &[String]) -> Result<()> {
    let session_id = arguments
        .first()
        .context("usage: floter terminal switch <session-id> [--terminal <name>]")?;
    qscreen_protocol::validate_session_id(session_id)?;
    let mut preferred = None;
    let mut index = 1;
    while index < arguments.len() {
        let argument = &arguments[index];
        if let Some(value) = argument.strip_prefix("--terminal=") {
            preferred = Some(value);
            index += 1;
        } else if argument == "--terminal" {
            preferred = Some(
                arguments
                    .get(index + 1)
                    .context("--terminal requires an emulator name")?,
            );
            index += 2;
        } else {
            anyhow::bail!("unexpected terminal switch argument '{argument}'");
        }
    }

    let session = list_sessions()
        .map_err(anyhow::Error::msg)?
        .into_iter()
        .find(|session| session.session_id == *session_id)
        .context("terminal session does not exist")?;
    if session.exited {
        anyhow::bail!("terminal session has already exited");
    }
    let cwd = if session.cwd.is_empty() {
        std::env::current_dir().context("resolve current directory")?
    } else {
        PathBuf::from(session.cwd)
    };

    #[cfg(unix)]
    {
        let command = attach_command(session_id).map_err(anyhow::Error::msg)?;
        super::session::open_terminal_session(&cwd, &command, preferred)
            .map(|_| ())
            .map_err(anyhow::Error::msg)
    }
    #[cfg(windows)]
    {
        if preferred.is_some_and(|name| !matches!(name, "default" | "system" | "windows-terminal"))
        {
            anyhow::bail!("Windows currently supports only the default terminal host");
        }
        open_windows_terminal(session_id, &cwd).map_err(anyhow::Error::msg)
    }
}

async fn external_attach(session_id: &str) -> Result<()> {
    let _raw_mode = RawModeGuard::enter()?;
    let (cols, rows) = crossterm::terminal::size().unwrap_or((80, 24));
    let stream = connect().await?;
    let (read_half, mut writer) = tokio::io::split(stream);
    let mut reader = BufReader::new(read_half);
    send_message(
        &mut writer,
        Message {
            kind: MessageKind::Request,
            id: "1".into(),
            command: Some(Command::Attach),
            session_id: session_id.to_string(),
            width: u32::from(cols.max(1)),
            height: u32::from(rows.max(1)),
            attach_mode: AttachMode::Bytes,
            ..Default::default()
        },
    )
    .await?;
    let response = receive_message(&mut reader).await?;
    check_response(&response, "1")?;

    let (input_tx, mut input_rx) = mpsc::unbounded_channel::<Vec<u8>>();
    std::thread::Builder::new()
        .name("term-attach-input".into())
        .spawn(move || {
            let mut stdin = std::io::stdin();
            let mut buffer = [0_u8; 4096];
            loop {
                match stdin.read(&mut buffer) {
                    Ok(0) | Err(_) => break,
                    Ok(read) if input_tx.send(buffer[..read].to_vec()).is_err() => break,
                    Ok(_) => {}
                }
            }
        })
        .context("start terminal input reader")?;

    let mut output = std::io::stdout();
    let mut message_id = 1_u64;
    let mut last_size = (cols, rows);
    let mut resize_timer = tokio::time::interval(RESIZE_POLL);
    let mut line = String::new();
    loop {
        tokio::select! {
            message = receive_message_reusing(&mut reader, &mut line) => {
                let message = message?;
                if message.kind == MessageKind::Event
                    && message.event == Some(EventType::Output)
                {
                    output.write_all(&message.payload)?;
                    output.flush()?;
                } else if message.kind == MessageKind::Event
                    && message.event == Some(EventType::Exit)
                {
                    break;
                } else if message.kind == MessageKind::Response && !message.error.is_empty() {
                    anyhow::bail!(message.error);
                }
            }
            input = input_rx.recv() => {
                let Some(input) = input else { break; };
                send_input(&mut writer, session_id, &mut message_id, input).await?;
            }
            _ = resize_timer.tick() => {
                let size = crossterm::terminal::size().unwrap_or(last_size);
                if size != last_size {
                    last_size = size;
                    message_id += 1;
                    send_message(&mut writer, Message {
                        kind: MessageKind::Request,
                        id: message_id.to_string(),
                        command: Some(Command::Resize),
                        session_id: session_id.to_string(),
                        width: u32::from(size.0.max(1)),
                        height: u32::from(size.1.max(1)),
                        ..Default::default()
                    }).await?;
                }
            }
        }
    }
    drop(writer);
    drop(reader);
    stop_daemon_when_idle(Duration::from_millis(300)).await;
    Ok(())
}

/// Stop the broker after managed sessions have drained, while leaving it alive
/// when a system-terminal client still owns a handed-off session.
pub fn shutdown_if_idle() {
    let Ok(runtime) = tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
    else {
        return;
    };
    runtime.block_on(stop_daemon_when_idle(Duration::from_millis(500)));
}

async fn stop_daemon_when_idle(wait: Duration) {
    let deadline = Instant::now() + wait;
    loop {
        let response = existing_control_request(Message {
            kind: MessageKind::Request,
            id: "list-idle".into(),
            command: Some(Command::List),
            ..Default::default()
        })
        .await;
        let Ok(response) = response else {
            return;
        };
        if response.error.is_empty() && response.sessions.is_empty() {
            let _ = existing_control_request(Message {
                kind: MessageKind::Request,
                id: "stop-idle".into(),
                command: Some(Command::Stop),
                ..Default::default()
            })
            .await;
            return;
        }
        if Instant::now() >= deadline {
            return;
        }
        tokio::time::sleep(Duration::from_millis(25)).await;
    }
}

struct RawModeGuard;

impl RawModeGuard {
    fn enter() -> Result<Self> {
        crossterm::terminal::enable_raw_mode().context("enable terminal raw mode")?;
        Ok(Self)
    }
}

impl Drop for RawModeGuard {
    fn drop(&mut self) {
        let _ = crossterm::terminal::disable_raw_mode();
    }
}

async fn send_message<W: AsyncWrite + Unpin>(writer: &mut W, message: Message) -> Result<()> {
    writer.write_all(&message.to_json_line()?).await?;
    Ok(())
}

async fn receive_message<R: AsyncRead + Unpin>(reader: &mut BufReader<R>) -> Result<Message> {
    let mut line = String::new();
    receive_message_reusing(reader, &mut line).await
}

async fn receive_message_reusing<R: AsyncRead + Unpin>(
    reader: &mut BufReader<R>,
    line: &mut String,
) -> Result<Message> {
    line.clear();
    if reader.read_line(line).await? == 0 {
        anyhow::bail!("terminal daemon connection closed");
    }
    Message::from_json(line).context("decode terminal daemon message")
}

fn check_response(message: &Message, expected_id: &str) -> Result<()> {
    if message.kind != MessageKind::Response || message.id != expected_id {
        anyhow::bail!("unexpected terminal daemon response");
    }
    if !message.error.is_empty() {
        anyhow::bail!(message.error.clone());
    }
    if !message.ok {
        anyhow::bail!("terminal daemon rejected the request");
    }
    Ok(())
}

/// Command invoked in an interactive system shell on macOS/Linux.
#[cfg(unix)]
pub fn attach_command(session_id: &str) -> Result<String, String> {
    let executable = std::env::current_exe().map_err(|error| error.to_string())?;
    Ok(format!(
        "{} {} {}",
        shell_quote(&executable.to_string_lossy()),
        ATTACH_ARGUMENT,
        shell_quote(session_id)
    ))
}

#[cfg(unix)]
fn shell_quote(value: &str) -> String {
    format!("'{}'", value.replace('\'', "'\\''"))
}

/// Launch the attach-only process in a new console. On current Windows this is
/// routed through the user's selected default terminal host; older versions use
/// conhost. Running the helper itself avoids an outer PowerShell competing for
/// console input or returning early because Floter is a GUI-subsystem binary.
#[cfg(windows)]
pub fn open_windows_terminal(session_id: &str, cwd: &Path) -> Result<(), String> {
    use std::os::windows::process::CommandExt;

    const CREATE_NEW_CONSOLE: u32 = 0x0000_0010;
    let executable = std::env::current_exe().map_err(|error| error.to_string())?;
    ProcessCommand::new(executable)
        .args([ATTACH_ARGUMENT, session_id])
        .current_dir(cwd)
        .creation_flags(CREATE_NEW_CONSOLE)
        .spawn()
        .map(|_| ())
        .map_err(|error| error.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    static DAEMON_TEST: Mutex<()> = Mutex::new(());

    #[test]
    fn helper_arguments_are_private_and_unambiguous() {
        assert_ne!(DAEMON_ARGUMENT, ATTACH_ARGUMENT);
        assert!(DAEMON_ARGUMENT.starts_with("--terminal-"));
        assert!(ATTACH_ARGUMENT.starts_with("--terminal-"));
    }

    #[cfg(unix)]
    #[test]
    fn shell_quote_handles_single_quotes() {
        assert_eq!(shell_quote("a'b"), "'a'\\''b'");
    }

    #[test]
    fn broker_executes_command_and_reports_exit() {
        let _serial = DAEMON_TEST.lock().unwrap();
        std::env::set_var(NAMESPACE_ENV, format!("floter-test-{}", std::process::id()));

        let daemon = std::thread::spawn(|| {
            let runtime = tokio::runtime::Builder::new_multi_thread()
                .enable_all()
                .build()
                .unwrap();
            runtime.block_on(qscreen_daemon::run()).unwrap();
        });
        let runtime = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .unwrap();
        runtime.block_on(async {
            let deadline = Instant::now() + Duration::from_secs(3);
            while connect().await.is_err() {
                assert!(Instant::now() < deadline, "test daemon did not start");
                tokio::time::sleep(Duration::from_millis(20)).await;
            }
        });

        #[cfg(unix)]
        let (shell, command) = (
            Some("/bin/sh".to_string()),
            "printf floter-broker-ready; exit",
        );
        #[cfg(windows)]
        let (shell, command) = (Some("cmd".to_string()), "echo floter-broker-ready & exit");
        let (event_tx, event_rx) = std::sync::mpsc::channel();
        let output_tx = event_tx.clone();
        let session = spawn_session(
            "floter-test".into(),
            shell,
            dirs::home_dir().unwrap_or_default(),
            Some(command.into()),
            80,
            24,
            BrokerCallbacks {
                output: Box::new(move |bytes| {
                    let _ = output_tx.send((Some(bytes), None));
                }),
                exit: Box::new(move |code| {
                    let _ = event_tx.send((None, Some(code)));
                }),
            },
        )
        .unwrap();

        let deadline = Instant::now() + Duration::from_secs(5);
        let mut output = Vec::new();
        let mut exited = false;
        while Instant::now() < deadline {
            match event_rx.recv_timeout(Duration::from_millis(100)) {
                Ok((Some(bytes), None)) => output.extend_from_slice(&bytes),
                Ok((None, Some(_))) => {
                    exited = true;
                    break;
                }
                Ok(_) => {}
                Err(std::sync::mpsc::RecvTimeoutError::Timeout) => {}
                Err(error) => panic!("broker event channel failed: {error}"),
            }
        }
        drop(session);

        runtime.block_on(async {
            let response = control_request(Message {
                kind: MessageKind::Request,
                id: "stop".into(),
                command: Some(Command::Stop),
                ..Default::default()
            })
            .await
            .unwrap();
            check_response(&response, "stop").unwrap();
        });
        daemon.join().unwrap();

        assert!(exited, "brokered shell did not exit");
        assert!(
            String::from_utf8_lossy(&output).contains("floter-broker-ready"),
            "broker output did not include the command marker"
        );
        std::env::set_var(NAMESPACE_ENV, NAMESPACE);
    }

    #[test]
    fn detached_session_can_be_listed_and_reattached() {
        let _serial = DAEMON_TEST.lock().unwrap();
        std::env::set_var(
            NAMESPACE_ENV,
            format!("floter-handoff-test-{}", std::process::id()),
        );

        let daemon = std::thread::spawn(|| {
            let runtime = tokio::runtime::Builder::new_multi_thread()
                .enable_all()
                .build()
                .unwrap();
            runtime.block_on(qscreen_daemon::run()).unwrap();
        });
        let runtime = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .unwrap();
        runtime.block_on(async {
            let deadline = Instant::now() + Duration::from_secs(3);
            while connect().await.is_err() {
                assert!(Instant::now() < deadline, "test daemon did not start");
                tokio::time::sleep(Duration::from_millis(20)).await;
            }
        });

        #[cfg(unix)]
        let (shell, command) = (
            Some("/bin/sh".to_string()),
            "printf floter-handoff-ready; while :; do sleep 1; done",
        );
        #[cfg(windows)]
        let (shell, command) = (
            Some("cmd".to_string()),
            "echo floter-handoff-ready & ping -t 127.0.0.1 >nul",
        );
        let (initial_tx, initial_rx) = std::sync::mpsc::channel();
        let session = spawn_session(
            "floter-handoff-test".into(),
            shell,
            dirs::home_dir().unwrap_or_default(),
            Some(command.into()),
            80,
            24,
            BrokerCallbacks {
                output: Box::new(move |bytes| {
                    let _ = initial_tx.send(bytes);
                }),
                exit: Box::new(|_| {}),
            },
        )
        .unwrap();
        let session_id = session.session_id().to_string();

        let mut initial_output = Vec::new();
        let deadline = Instant::now() + Duration::from_secs(5);
        while !String::from_utf8_lossy(&initial_output).contains("floter-handoff-ready") {
            assert!(
                Instant::now() < deadline,
                "initial session output did not arrive"
            );
            if let Ok(bytes) = initial_rx.recv_timeout(Duration::from_millis(100)) {
                initial_output.extend_from_slice(&bytes);
            }
        }
        session.detach();
        drop(session);

        let deadline = Instant::now() + Duration::from_secs(3);
        loop {
            let detached = list_sessions()
                .unwrap()
                .into_iter()
                .find(|session| session.session_id == session_id)
                .is_some_and(|session| !session.attached && !session.exited);
            if detached {
                break;
            }
            assert!(Instant::now() < deadline, "session did not detach cleanly");
            std::thread::sleep(Duration::from_millis(20));
        }

        let (replay_tx, replay_rx) = std::sync::mpsc::channel();
        let attached = attach_session(
            session_id.clone(),
            80,
            24,
            BrokerCallbacks {
                output: Box::new(move |bytes| {
                    let _ = replay_tx.send(bytes);
                }),
                exit: Box::new(|_| {}),
            },
        )
        .unwrap();
        let mut replay = Vec::new();
        let deadline = Instant::now() + Duration::from_secs(5);
        while !String::from_utf8_lossy(&replay).contains("floter-handoff-ready") {
            assert!(
                Instant::now() < deadline,
                "reattached client did not receive replay"
            );
            if let Ok(bytes) = replay_rx.recv_timeout(Duration::from_millis(100)) {
                replay.extend_from_slice(&bytes);
            }
        }
        attached.detach();
        drop(attached);
        kill_existing_session(&session_id).unwrap();

        runtime.block_on(async {
            let response = control_request(Message {
                kind: MessageKind::Request,
                id: "stop".into(),
                command: Some(Command::Stop),
                ..Default::default()
            })
            .await
            .unwrap();
            check_response(&response, "stop").unwrap();
        });
        daemon.join().unwrap();
        std::env::set_var(NAMESPACE_ENV, NAMESPACE);
    }

    #[cfg(unix)]
    #[test]
    #[ignore = "manual terminal throughput benchmark"]
    fn broker_sustains_large_output_with_terminal_parsing() {
        use std::sync::atomic::{AtomicUsize, Ordering};

        use alacritty_terminal::event::VoidListener;
        use alacritty_terminal::grid::Dimensions;
        use alacritty_terminal::sync::FairMutex;
        use alacritty_terminal::term::{Config, Term};
        use alacritty_terminal::vte::ansi::Processor;

        struct Size;
        impl Dimensions for Size {
            fn total_lines(&self) -> usize {
                24
            }
            fn screen_lines(&self) -> usize {
                24
            }
            fn columns(&self) -> usize {
                80
            }
        }

        enum BenchEvent {
            Output(Vec<u8>),
            Exit(Option<i32>),
        }

        const PAYLOAD: usize = 8 * 1024 * 1024;
        let _serial = DAEMON_TEST.lock().unwrap();
        std::env::set_var(
            NAMESPACE_ENV,
            format!("floter-bench-{}", std::process::id()),
        );
        let daemon = std::thread::spawn(|| {
            let runtime = tokio::runtime::Builder::new_multi_thread()
                .enable_all()
                .build()
                .unwrap();
            runtime.block_on(qscreen_daemon::run()).unwrap();
        });
        let runtime = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .unwrap();
        runtime.block_on(async {
            let deadline = Instant::now() + Duration::from_secs(3);
            while connect().await.is_err() {
                assert!(Instant::now() < deadline, "benchmark daemon did not start");
                tokio::time::sleep(Duration::from_millis(20)).await;
            }
        });

        let terminal = std::sync::Arc::new(FairMutex::new(Term::new(
            Config::default(),
            &Size,
            VoidListener,
        )));
        let parsed = std::sync::Arc::new(AtomicUsize::new(0));
        let parsed_output = parsed.clone();
        let (parse_tx, parse_rx) = std::sync::mpsc::sync_channel(512);
        let output_tx = parse_tx.clone();
        let (exit_tx, exit_rx) = std::sync::mpsc::channel();
        let parser = std::thread::spawn(move || {
            let mut processor: Processor = Processor::new();
            while let Ok(event) = parse_rx.recv() {
                match event {
                    BenchEvent::Output(bytes) => {
                        parsed_output.fetch_add(bytes.len(), Ordering::Relaxed);
                        processor.advance(&mut *terminal.lock(), &bytes);
                    }
                    BenchEvent::Exit(code) => {
                        let _ = exit_tx.send(code);
                        break;
                    }
                }
            }
        });
        let started = Instant::now();
        let session = spawn_session(
            "floter-benchmark".into(),
            Some("/bin/sh".into()),
            dirs::home_dir().unwrap_or_default(),
            Some(format!(
                "head -c {PAYLOAD} /dev/zero | LC_ALL=C tr '\\000' x; exit"
            )),
            80,
            24,
            BrokerCallbacks {
                output: Box::new(move |bytes| {
                    let _ = output_tx.send(BenchEvent::Output(bytes));
                }),
                exit: Box::new(move |code| {
                    let _ = parse_tx.send(BenchEvent::Exit(code));
                }),
            },
        )
        .unwrap();
        let code = exit_rx.recv_timeout(Duration::from_secs(15)).unwrap();
        let elapsed = started.elapsed();
        parser.join().unwrap();
        session.kill();
        drop(session);
        shutdown_if_idle();
        daemon.join().unwrap();

        let parsed = parsed.load(Ordering::Relaxed);
        assert_eq!(code, Some(0));
        assert!(parsed >= PAYLOAD, "large output was truncated: {parsed}");
        tracing::debug!(
            "broker + Alacritty: {:.1} MiB/s ({parsed} bytes in {:.3}s)",
            parsed as f64 / 1024.0 / 1024.0 / elapsed.as_secs_f64(),
            elapsed.as_secs_f64()
        );
        std::env::set_var(NAMESPACE_ENV, NAMESPACE);
    }
}
