use crate::terminal::session::TerminalManager;
use std::sync::{Arc, Mutex};
use tauri::{AppHandle, State};

pub struct TerminalState(pub Arc<Mutex<TerminalManager>>);

#[tauri::command]
pub fn term_spawn(
    state: State<'_, TerminalState>,
    app: AppHandle,
    id: String,
    shell: Option<String>,
    cols: Option<u16>,
    rows: Option<u16>,
) -> Result<(), String> {
    let manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.spawn(id, app, shell, cols.unwrap_or(80), rows.unwrap_or(24))
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
