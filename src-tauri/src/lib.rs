mod commands;
mod terminal;

use commands::apps::{application_icon, list_applications, open_application, ApplicationState};
use commands::config::{get_settings, load_settings, save_settings};
use commands::custom::{
    add_custom_command, delete_custom_command, execute_custom_command, get_custom_commands,
    update_custom_command, CommandState,
};
use commands::terminal::{
    open_in_default_terminal, term_close, term_input, term_resize, term_scroll, term_scroll_to,
    term_spawn, TerminalState,
};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use tauri::{
    menu::{Menu, MenuItem},
    tray::{TrayIconBuilder, TrayIconEvent},
    AppHandle, Emitter, LogicalPosition, LogicalSize, Manager, Monitor, PhysicalPosition,
    UserAttentionType, WebviewWindow, Wry,
};
use tauri_plugin_global_shortcut::{GlobalShortcutExt, ShortcutState};
use terminal::session::TerminalManager;

const INPUT_WINDOW_WIDTH: f64 = 720.0;
const TERMINAL_WINDOW_WIDTH: f64 = 860.0;
const INPUT_WINDOW_HEIGHT: f64 = 56.0;
const TERMINAL_WINDOW_HEIGHT: f64 = 460.0;

struct TrayMenuItems {
    show: MenuItem<Wry>,
    quit: MenuItem<Wry>,
}

struct AppState {
    window_visible: AtomicBool,
    terminal_mode: AtomicBool,
    /// Height (logical px) the terminal window actually takes once the canvas is
    /// fitted to whole rows. Reported by the frontend; it is the centering target
    /// the collapsed input anchors against.
    terminal_height: Mutex<f64>,
    tray_items: Mutex<Option<TrayMenuItems>>,
}

impl AppState {
    fn terminal_height(&self) -> f64 {
        self.terminal_height
            .lock()
            .map(|height| *height)
            .unwrap_or(TERMINAL_WINDOW_HEIGHT)
    }
}

fn tray_labels(language: &str) -> (&'static str, &'static str) {
    match language {
        "zh" => ("显示 floter", "退出"),
        _ => ("Show floter", "Quit"),
    }
}

/// Retitle the tray menu in the given language. Called on startup and whenever
/// the language setting changes, so the tray never lags behind the UI.
pub fn apply_tray_language(app: &AppHandle, language: &str) {
    let (show, quit) = tray_labels(language);
    let state = app.state::<AppState>();
    let Ok(items) = state.tray_items.lock() else {
        return;
    };
    if let Some(items) = items.as_ref() {
        let _ = items.show.set_text(show);
        let _ = items.quit.set_text(quit);
    }
}

/// The monitor the user is working on. Follows the mouse cursor, which is what
/// macOS itself uses to decide where Spotlight-style panels appear, and falls
/// back to the window's own monitor when the cursor position is unavailable.
fn focused_monitor(window: &WebviewWindow) -> Option<Monitor> {
    if let Ok(cursor) = window.cursor_position() {
        if let Ok(Some(monitor)) = window.monitor_from_point(cursor.x, cursor.y) {
            return Some(monitor);
        }
    }
    window
        .current_monitor()
        .ok()
        .flatten()
        .or_else(|| window.primary_monitor().ok().flatten())
}

/// Where the window belongs when summoned: horizontally centered on the focused
/// monitor, with its top edge placed where the *expanded terminal* would be
/// vertically centered. The input row therefore sits high on screen, and because
/// both modes share that top edge the window never jumps when the terminal opens.
///
/// Everything is computed in logical points. A monitor's reported position is
/// physical (points × that monitor's scale), while `set_position` converts a
/// physical value back using the *window's* scale — so mixing the two would
/// misplace the window across monitors with different scale factors.
fn default_position(
    window: &WebviewWindow,
    logical_width: f64,
    terminal_height: f64,
) -> Option<LogicalPosition<f64>> {
    let monitor = focused_monitor(window)?;
    let scale = monitor.scale_factor();
    if scale <= 0.0 {
        return None;
    }
    let area = monitor.work_area();
    let area_x = area.position.x as f64 / scale;
    let area_y = area.position.y as f64 / scale;
    let area_width = area.size.width as f64 / scale;
    let area_height = area.size.height as f64 / scale;

    // Clamp against the terminal height (not the current height) so the expanded
    // window still fits on screen after the input row grows into it.
    let max_x = area_x + (area_width - logical_width).max(0.0);
    let max_y = area_y + (area_height - terminal_height).max(0.0);
    let x = (area_x + (area_width - logical_width) / 2.0).clamp(area_x, max_x);
    let y = (area_y + (area_height - terminal_height) / 2.0).clamp(area_y, max_y);

    Some(LogicalPosition::new(x, y))
}

fn move_to_default_position(
    window: &WebviewWindow,
    logical_width: f64,
    state: &AppState,
) -> Result<(), String> {
    match default_position(window, logical_width, state.terminal_height()) {
        Some(position) => window.set_position(position).map_err(|e| e.to_string()),
        None => window.center().map_err(|e| e.to_string()),
    }
}

fn reveal_window(window: &WebviewWindow) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        let _ = window.app_handle().show();
        let _ = window.set_visible_on_all_workspaces(true);
        let _ = window.set_always_on_top(true);
        let _ = window.unminimize();
    }
    window.show().map_err(|e| e.to_string())?;
    window.set_focus().map_err(|e| e.to_string())?;
    #[cfg(target_os = "macos")]
    {
        let _ = window.request_user_attention(Some(UserAttentionType::Informational));
    }
    Ok(())
}

fn reveal_saved_mode(window: &WebviewWindow, state: &AppState) -> Result<(), String> {
    // Keep the window's last size; the frontend restores the matching layout.
    // The position, however, is re-homed onto whichever monitor the user is on
    // so the panel always opens under their attention rather than where it was
    // last dismissed.
    let terminal = state.terminal_mode.load(Ordering::SeqCst);
    let mode = if terminal { "terminal" } else { "collapsed" };
    let width = if terminal {
        TERMINAL_WINDOW_WIDTH
    } else {
        INPUT_WINDOW_WIDTH
    };

    let _ = move_to_default_position(window, width, state);
    reveal_window(window)?;
    state.window_visible.store(true, Ordering::SeqCst);
    let _ = window.emit("floter://revealed", mode);
    Ok(())
}

fn resize_window(
    window: &WebviewWindow,
    width: f64,
    height: f64,
    preserve_anchor: bool,
    state: &AppState,
) -> Result<(), String> {
    let previous_position = window.outer_position().ok();
    let previous_size = window.outer_size().ok();
    let scale_factor = window.scale_factor().unwrap_or(1.0);

    window
        .set_size(LogicalSize::new(width, height))
        .map_err(|e| e.to_string())?;

    if preserve_anchor {
        if let (Some(position), Some(size)) = (previous_position, previous_size) {
            let next_width = (width * scale_factor).round() as i32;
            let next_x = position.x + (size.width as i32 - next_width) / 2;
            window
                .set_position(PhysicalPosition::new(next_x, position.y))
                .map_err(|e| e.to_string())?;
            return Ok(());
        }
    }

    move_to_default_position(window, width, state)
}

#[tauri::command]
fn show_terminal(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    let preserve_anchor = state.window_visible.load(Ordering::SeqCst);
    resize_window(
        &window,
        TERMINAL_WINDOW_WIDTH,
        TERMINAL_WINDOW_HEIGHT,
        preserve_anchor,
        &state,
    )?;
    reveal_window(&window)?;
    state.terminal_mode.store(true, Ordering::SeqCst);
    state.window_visible.store(true, Ordering::SeqCst);
    Ok(())
}

/// Record the terminal's real height (rows rounded to whole cells) so the
/// summon anchor centers the terminal exactly rather than approximately.
#[tauri::command]
fn set_terminal_height(height: f64, state: tauri::State<'_, AppState>) {
    if !height.is_finite() || height <= 0.0 {
        return;
    }
    if let Ok(mut current) = state.terminal_height.lock() {
        *current = height;
    }
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
    let preserve_anchor = state.window_visible.load(Ordering::SeqCst);
    resize_window(
        &window,
        INPUT_WINDOW_WIDTH,
        INPUT_WINDOW_HEIGHT,
        preserve_anchor,
        &state,
    )?;
    reveal_window(&window)?;
    state.terminal_mode.store(false, Ordering::SeqCst);
    state.window_visible.store(true, Ordering::SeqCst);
    Ok(())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(tauri_plugin_store::Builder::new().build())
        .plugin(tauri_plugin_shell::init())
        .manage(ApplicationState::new())
        .manage(TerminalState(Arc::new(Mutex::new(TerminalManager::new()))))
        .manage(CommandState::new())
        .manage(AppState {
            window_visible: AtomicBool::new(false),
            terminal_mode: AtomicBool::new(false),
            terminal_height: Mutex::new(TERMINAL_WINDOW_HEIGHT),
            tray_items: Mutex::new(None),
        })
        .setup(|app| {
            let language = load_settings().language;
            let (show_label, quit_label) = tray_labels(&language);
            let show_item = MenuItem::with_id(app, "show", show_label, true, None::<&str>)?;
            let quit_item = MenuItem::with_id(app, "quit", quit_label, true, None::<&str>)?;
            let tray_menu = Menu::with_items(app, &[&show_item, &quit_item])?;
            if let Ok(mut items) = app.state::<AppState>().tray_items.lock() {
                *items = Some(TrayMenuItems {
                    show: show_item.clone(),
                    quit: quit_item.clone(),
                });
            }
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
            term_spawn,
            term_input,
            term_resize,
            term_scroll,
            term_scroll_to,
            application_icon,
            list_applications,
            open_application,
            open_in_default_terminal,
            term_close,
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
            set_terminal_height,
            start_drag,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
