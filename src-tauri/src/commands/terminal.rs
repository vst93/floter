use crate::terminal::session::TerminalManager;
use std::sync::{Arc, Mutex};
use tauri::{AppHandle, State};

pub struct TerminalState(pub Arc<Mutex<TerminalManager>>);

// Tauri maps this public IPC command from named frontend arguments. Keeping the
// parameters explicit makes the contract stable and avoids an extra wrapper.
#[allow(clippy::too_many_arguments)]
#[tauri::command]
pub fn term_spawn(
    state: State<'_, TerminalState>,
    app: AppHandle,
    id: String,
    shell: Option<String>,
    initial_command: Option<String>,
    theme: Option<String>,
    cols: Option<u16>,
    rows: Option<u16>,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.spawn(
        id,
        app,
        shell,
        initial_command,
        theme,
        cols.unwrap_or(80),
        rows.unwrap_or(24),
    )
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
pub fn open_in_default_terminal(state: State<'_, TerminalState>, id: String) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.open_in_terminal(&id)
}

#[tauri::command]
pub fn term_close(state: State<'_, TerminalState>, id: String) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.close(&id)
}
