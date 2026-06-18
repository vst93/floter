use crate::terminal::pty::PtyManager;
use std::sync::{Arc, Mutex};
use tauri::{AppHandle, State};

pub struct PtyState(pub Arc<Mutex<PtyManager>>);

#[tauri::command]
pub fn pty_spawn(
    state: State<'_, PtyState>,
    app: AppHandle,
    id: String,
    shell: Option<String>,
) -> Result<(), String> {
    let mut manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.spawn(id, app, shell)
}

#[tauri::command]
pub fn pty_write(state: State<'_, PtyState>, id: String, data: Vec<u8>) -> Result<(), String> {
    let mut manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.write(&id, &data)
}

#[tauri::command]
pub fn pty_resize(
    state: State<'_, PtyState>,
    id: String,
    rows: u16,
    cols: u16,
) -> Result<(), String> {
    let mut manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.resize(&id, rows, cols)
}

#[tauri::command]
pub fn pty_close(state: State<'_, PtyState>, id: String) -> Result<(), String> {
    let mut manager = state.0.lock().map_err(|e| e.to_string())?;
    manager.close(&id)
}
