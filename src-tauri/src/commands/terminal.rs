use crate::extensions::ExtensionState;
use crate::terminal::broker::{self, BrokerSessionInfo, SpawnCommand};
use crate::terminal::session::{ExternalTerminalOutcome, TerminalManager};
use serde::Deserialize;
use std::collections::BTreeMap;
use std::sync::{Arc, Mutex};
use tauri::{AppHandle, State};

pub struct TerminalState(pub Arc<Mutex<TerminalManager>>);

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TerminalExecutionPlan {
    program: String,
    #[serde(default)]
    args: Vec<String>,
    cwd: Option<String>,
    #[serde(default)]
    environment: BTreeMap<String, String>,
    #[serde(default = "default_true")]
    inherit_environment: bool,
    plan_token: Option<String>,
    argument_override: Option<Vec<String>>,
}

fn default_true() -> bool {
    true
}

// Tauri maps this public IPC command from named frontend arguments. Keeping the
// parameters explicit makes the contract stable and avoids an extra wrapper.
#[allow(clippy::too_many_arguments)]
#[tauri::command]
pub fn term_spawn(
    state: State<'_, TerminalState>,
    extension_state: State<'_, ExtensionState>,
    app: AppHandle,
    id: String,
    generation: u64,
    shell: Option<String>,
    initial_command: Option<String>,
    execution: Option<TerminalExecutionPlan>,
    theme: Option<String>,
    cols: Option<u16>,
    rows: Option<u16>,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    let (cwd, command) = match execution {
        Some(plan) => {
            let (program, args, cwd, environment, inherit_environment) = if let Some(token) =
                plan.plan_token.as_deref()
            {
                let mut protected = extension_state.take_execution_plan(token)?;
                if let Some(argument_override) = plan.argument_override {
                    let start = protected.user_args_start.ok_or_else(|| {
                        "Extension execution plan does not accept argument overrides".to_string()
                    })?;
                    protected.args.truncate(start);
                    protected.args.extend(argument_override);
                }
                (
                    protected.program,
                    protected.args,
                    protected.cwd,
                    protected.environment,
                    protected.inherit_environment,
                )
            } else {
                (
                    plan.program,
                    plan.args,
                    plan.cwd,
                    plan.environment,
                    plan.inherit_environment,
                )
            };
            (
                cwd.map(std::path::PathBuf::from),
                Some(SpawnCommand {
                    program,
                    args,
                    environment,
                    inherit_environment,
                }),
            )
        }
        None => (None, None),
    };
    manager.spawn(
        id,
        generation,
        app,
        shell,
        initial_command,
        command,
        cwd,
        theme,
        cols.unwrap_or(80),
        rows.unwrap_or(24),
    )
}

#[tauri::command]
pub fn term_list_sessions() -> Result<Vec<BrokerSessionInfo>, String> {
    broker::list_sessions()
}

#[tauri::command]
pub fn term_attach_existing(
    state: State<'_, TerminalState>,
    app: AppHandle,
    id: String,
    generation: u64,
    broker_session_id: String,
    theme: Option<String>,
    cols: Option<u16>,
    rows: Option<u16>,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.attach_existing(
        id,
        generation,
        app,
        broker_session_id,
        theme,
        cols.unwrap_or(80),
        rows.unwrap_or(24),
    )
}

#[tauri::command]
pub fn term_kill_session(session_id: String) -> Result<(), String> {
    broker::kill_existing_session(&session_id)
}

#[tauri::command]
pub fn term_set_theme(
    state: State<'_, TerminalState>,
    id: String,
    theme: String,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.set_theme(&id, &theme)
}

#[tauri::command]
pub fn term_input(
    state: State<'_, TerminalState>,
    id: String,
    data: Vec<u8>,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.input(&id, &data)
}

#[tauri::command]
pub fn term_resize(
    state: State<'_, TerminalState>,
    id: String,
    cols: u16,
    rows: u16,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.resize(&id, cols, rows)
}

#[tauri::command]
pub fn term_scroll(state: State<'_, TerminalState>, id: String, delta: i32) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.scroll(&id, delta)
}

#[tauri::command]
pub fn term_wheel(
    state: State<'_, TerminalState>,
    id: String,
    delta: i32,
    column: u16,
    row: u16,
    modifiers: u8,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.wheel(&id, delta, column, row, modifiers)
}

#[tauri::command]
pub fn term_mouse(
    state: State<'_, TerminalState>,
    id: String,
    kind: String,
    button: u8,
    column: u16,
    row: u16,
    modifiers: u8,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.mouse(&id, &kind, button, column, row, modifiers)
}

#[tauri::command]
pub fn term_scroll_to(
    state: State<'_, TerminalState>,
    id: String,
    offset: u32,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.scroll_to(&id, offset)
}

#[tauri::command]
pub async fn open_in_default_terminal(
    state: State<'_, TerminalState>,
    id: String,
) -> Result<ExternalTerminalOutcome, String> {
    let terminal_state = state.0.clone();
    let request = {
        let manager = terminal_state.lock().map_err(|e| e.to_string())?;
        manager.external_terminal_request(&id)?
    };
    let generation = request.generation();
    let preserve_session = request.preserves_session();
    let outcome = tauri::async_runtime::spawn_blocking(move || request.open())
        .await
        .map_err(|error| error.to_string())??;

    // Detach the source client immediately after the external host has accepted
    // the session. The frontend still performs its idempotent cleanup to reset
    // renderer state.
    if outcome.session_handed_off {
        let manager = terminal_state.lock().map_err(|e| e.to_string())?;
        manager.close_if_generation(&id, generation, preserve_session)?;
    }
    Ok(outcome)
}

#[tauri::command]
pub fn term_close(state: State<'_, TerminalState>, id: String) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.close(&id)
}
