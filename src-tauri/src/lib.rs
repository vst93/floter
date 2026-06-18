mod commands;
mod terminal;

use commands::config::{get_settings, save_settings};
use commands::custom::{
    add_custom_command, delete_custom_command, execute_custom_command, get_custom_commands,
    update_custom_command, CommandState,
};
use commands::terminal::{pty_close, pty_resize, pty_spawn, pty_write, PtyState};
use std::sync::{Arc, Mutex};
use terminal::pty::PtyManager;
use tauri::{LogicalSize, Manager};
use tauri_plugin_global_shortcut::{GlobalShortcutExt, ShortcutState};

#[tauri::command]
fn show_terminal(window: tauri::Window) -> Result<(), String> {
    window
        .set_size(LogicalSize::new(780.0, 520.0))
        .map_err(|e| e.to_string())?;
    window.show().map_err(|e| e.to_string())?;
    window.set_focus().map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
fn hide_window(window: tauri::Window) -> Result<(), String> {
    window.hide().map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
fn start_drag(window: tauri::Window) -> Result<(), String> {
    window.start_dragging().map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
fn show_input(window: tauri::Window) -> Result<(), String> {
    window
        .set_size(LogicalSize::new(560.0, 56.0))
        .map_err(|e| e.to_string())?;
    window.show().map_err(|e| e.to_string())?;
    window.set_focus().map_err(|e| e.to_string())?;
    Ok(())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let pty_manager = PtyManager::new();

    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(tauri_plugin_store::Builder::new().build())
        .plugin(tauri_plugin_shell::init())
        .manage(PtyState(Arc::new(Mutex::new(pty_manager))))
        .manage(CommandState::new())
        .setup(|app| {
            let window = app.get_webview_window("main").unwrap();
            let window_clone = window.clone();

            app.global_shortcut()
                .on_shortcut("CmdOrCtrl+Shift+Space", move |_app, _shortcut, event| {
                    if event.state == ShortcutState::Pressed {
                        if window_clone.is_visible().unwrap_or(false) {
                            let _ = window_clone.hide();
                        } else {
                            let _ = window_clone.show();
                            let _ = window_clone.set_focus();
                        }
                    }
                })?;

            let window = app.get_webview_window("main").unwrap();
            window.on_window_event(move |event| {
                if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                    api.prevent_close();
                }
            });

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            pty_spawn,
            pty_write,
            pty_resize,
            pty_close,
            get_settings,
            save_settings,
            get_custom_commands,
            add_custom_command,
            update_custom_command,
            delete_custom_command,
            execute_custom_command,
            show_terminal,
            hide_window,
            show_input,
            start_drag,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
