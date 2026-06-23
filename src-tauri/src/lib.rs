mod commands;
mod terminal;

use commands::config::{get_settings, save_settings};
use commands::custom::{
    add_custom_command, delete_custom_command, execute_custom_command, get_custom_commands,
    update_custom_command, CommandState,
};
use commands::terminal::{pty_close, pty_resize, pty_spawn, pty_write, PtyState};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use tauri::{
    menu::{Menu, MenuItem},
    tray::{TrayIconBuilder, TrayIconEvent},
    Emitter, LogicalSize, Manager, UserAttentionType, WebviewWindow,
};
use tauri_plugin_global_shortcut::{GlobalShortcutExt, ShortcutState};
use terminal::pty::PtyManager;

struct AppState {
    window_visible: AtomicBool,
    terminal_mode: AtomicBool,
}

fn reveal_window(window: &WebviewWindow) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        let _ = window.app_handle().show();
        let _ = window.set_visible_on_all_workspaces(true);
        let _ = window.set_always_on_top(true);
        let _ = window.unminimize();
    }
    window.center().map_err(|e| e.to_string())?;
    window.show().map_err(|e| e.to_string())?;
    window.set_focus().map_err(|e| e.to_string())?;
    #[cfg(target_os = "macos")]
    {
        let _ = window.request_user_attention(Some(UserAttentionType::Informational));
    }
    Ok(())
}

fn reveal_saved_mode(window: &WebviewWindow, state: &AppState) -> Result<(), String> {
    let mode = if state.terminal_mode.load(Ordering::SeqCst) {
        window
            .set_size(LogicalSize::new(820.0, 520.0))
            .map_err(|e| e.to_string())?;
        "terminal"
    } else {
        window
            .set_size(LogicalSize::new(640.0, 60.0))
            .map_err(|e| e.to_string())?;
        "collapsed"
    };

    reveal_window(window)?;
    state.window_visible.store(true, Ordering::SeqCst);
    let _ = window.emit("floter://revealed", mode);
    Ok(())
}

#[tauri::command]
fn show_terminal(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    window
        .set_size(LogicalSize::new(820.0, 520.0))
        .map_err(|e| e.to_string())?;
    reveal_window(&window)?;
    state.terminal_mode.store(true, Ordering::SeqCst);
    state.window_visible.store(true, Ordering::SeqCst);
    Ok(())
}

#[tauri::command]
fn hide_window(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    window.hide().map_err(|e| e.to_string())?;
    state.window_visible.store(false, Ordering::SeqCst);
    Ok(())
}

#[tauri::command]
fn start_drag(window: WebviewWindow) -> Result<(), String> {
    window.start_dragging().map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
fn show_input(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    window
        .set_size(LogicalSize::new(640.0, 60.0))
        .map_err(|e| e.to_string())?;
    reveal_window(&window)?;
    state.terminal_mode.store(false, Ordering::SeqCst);
    state.window_visible.store(true, Ordering::SeqCst);
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
        .manage(AppState {
            window_visible: AtomicBool::new(false),
            terminal_mode: AtomicBool::new(false),
        })
        .setup(|app| {
            let show_item = MenuItem::with_id(app, "show", "Show floter", true, None::<&str>)?;
            let quit_item = MenuItem::with_id(app, "quit", "Quit", true, None::<&str>)?;
            let tray_menu = Menu::with_items(app, &[&show_item, &quit_item])?;
            let tray_icon = app
                .default_window_icon()
                .cloned()
                .ok_or("missing default window icon")?;
            TrayIconBuilder::with_id("main-tray")
                .icon(tray_icon)
                .tooltip("floter")
                .menu(&tray_menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id().as_ref() {
                    "show" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let state = app.state::<AppState>();
                            let _ = reveal_saved_mode(&window, &state);
                        }
                    }
                    "quit" => app.exit(0),
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click { .. } = event {
                        if let Some(window) = tray.app_handle().get_webview_window("main") {
                            let state = tray.app_handle().state::<AppState>();
                            let _ = reveal_saved_mode(&window, &state);
                        }
                    }
                })
                .build(app)?;

            let window = app.get_webview_window("main").unwrap();
            window.on_window_event(move |event| {
                if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                    api.prevent_close();
                }
            });

            let shortcuts = vec!["Control+Shift+Space"];

            for shortcut in shortcuts {
                let app_handle = app.handle().clone();
                app.global_shortcut()
                    .on_shortcut(shortcut, move |_app, _shortcut, event| {
                        if event.state == ShortcutState::Pressed {
                            eprintln!("global shortcut triggered");
                            let state = app_handle.state::<AppState>();
                            if let Some(window) = app_handle.get_webview_window("main") {
                                if state.window_visible.load(Ordering::SeqCst) {
                                    let _ = window.hide();
                                    state.window_visible.store(false, Ordering::SeqCst);
                                } else {
                                    let _ = reveal_saved_mode(&window, &state);
                                }
                            }
                        }
                    })?;
                eprintln!("registered global shortcut: {shortcut}");
            }

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
