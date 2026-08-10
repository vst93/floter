mod commands;
mod extensions;
#[cfg(target_os = "linux")]
pub mod ipc;
#[cfg(target_os = "linux")]
mod linux_render;
mod terminal;

use commands::actions::{open_path, open_url};
use commands::apps::{
    application_icon, check_applications, list_applications, open_application, ApplicationState,
};
use commands::autostart::{ensure_launch_at_startup, set_launch_at_startup};
use commands::config::{
    app_version, get_settings, get_shortcuts, load_settings, reset_shortcuts, resolved_shortcuts,
    resume_shortcuts, save_settings, save_terminal_size as persist_terminal_size,
    saved_terminal_size, suspend_shortcuts, update_shortcut, DEFAULT_TOGGLE_WINDOW, TOGGLE_WINDOW,
};
#[allow(deprecated)]
use commands::custom::{
    add_custom_command, delete_custom_command, execute_custom_command, get_custom_commands,
    update_custom_command, CommandState,
};
use commands::extensions::{
    catalog_complete, catalog_search, extensions_config_get, extensions_config_set,
    extensions_describe, extensions_diagnose, extensions_disable, extensions_enable,
    extensions_export, extensions_import, extensions_install, extensions_list,
    extensions_permissions_summary, extensions_rollback, extensions_search, extensions_uninstall,
    extensions_update,
};
use commands::system::system_power;
use commands::terminal::{
    open_in_default_terminal, term_close, term_input, term_mouse, term_resize, term_scroll,
    term_scroll_to, term_set_theme, term_spawn, term_wheel, TerminalState,
};
use extensions::ExtensionState;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
#[cfg(target_os = "windows")]
use tauri::webview::Color;
#[cfg(target_os = "macos")]
use tauri::ActivationPolicy;
use tauri::{
    menu::{Menu, MenuItem},
    tray::{TrayIconBuilder, TrayIconEvent},
    AppHandle, Emitter, LogicalPosition, LogicalSize, Manager, Monitor, PhysicalPosition,
    WebviewWindow, Wry,
};
#[cfg(target_os = "macos")]
use tauri_nspanel::{
    tauri_panel, CollectionBehavior, ManagerExt as NSPanelManagerExt, PanelLevel, StyleMask,
    WebviewWindowExt as NSPanelWebviewWindowExt,
};
use tauri_plugin_global_shortcut::{GlobalShortcutExt, ShortcutState};
use terminal::session::TerminalManager;

const INPUT_WINDOW_WIDTH: f64 = 720.0;
const INPUT_WINDOW_HEIGHT: f64 = 56.0;
const TERMINAL_WINDOW_HEIGHT: f64 = 600.0;

/// Configure and, when requested, run the terminal broker's process-only
/// modes before any GUI runtime is initialized.
pub fn prepare_terminal_process(arguments: &[String]) -> Option<Result<(), String>> {
    terminal::broker::initialize_environment();
    terminal::broker::run_helper(arguments).map(|result| result.map_err(|error| error.to_string()))
}

#[cfg(target_os = "macos")]
tauri_panel! {
    panel!(FloterPanel {
        config: {
            can_become_main_window: false,
            can_become_key_window: true,
            becomes_key_only_if_needed: false,
            is_floating_panel: true
        }
    })
}

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
    /// The toggle shortcut currently held with the OS, which is not always the
    /// one in the settings file: a stored binding another app owns falls back to
    /// the default, and the next rebind has to release what was really taken.
    toggle_shortcut: Mutex<String>,
    /// Physical origin of the monitor the panel was last seen on, used to
    /// identify that monitor again in `available_monitors()`. Wayland hands out
    /// no cursor position at all, so remembering where the panel was dismissed
    /// is the only way a later summon can return to the screen the user chose.
    last_monitor: Mutex<Option<PhysicalPosition<i32>>>,
}

impl AppState {
    fn terminal_height(&self) -> f64 {
        self.terminal_height
            .lock()
            .map(|height| *height)
            .unwrap_or(TERMINAL_WINDOW_HEIGHT)
    }

    fn remembered_monitor(&self) -> Option<PhysicalPosition<i32>> {
        self.last_monitor.lock().ok().and_then(|origin| *origin)
    }

    fn set_remembered_monitor(&self, origin: Option<PhysicalPosition<i32>>) {
        if let Ok(mut last) = self.last_monitor.lock() {
            *last = origin;
        }
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

/// The monitor the user is working on, answered by the first strategy that can.
///
/// 1. The mouse cursor, which is what macOS itself uses to decide where
///    Spotlight-style panels appear, and is equally right on Windows and X11.
/// 2. The monitor the panel was last dismissed from. This is the Wayland path:
///    there is no cursor position to be had there, so the panel returns to the
///    screen the user last left it on instead of jumping back to the primary.
/// 3. The focused X11 window, for the keyboard-driven case where the mouse was
///    left behind on another screen. Best-effort, and deliberately behind the
///    cache because it shells out on a latency-sensitive path.
/// 4. The panel's own monitor, then the primary one.
fn focused_monitor(window: &WebviewWindow, state: &AppState) -> Option<Monitor> {
    if let Some(monitor) = cursor_monitor(window) {
        return Some(monitor);
    }

    // A remembered monitor that has since been unplugged matches nothing and
    // simply falls through to the next strategy.
    if let Some(origin) = state.remembered_monitor() {
        let remembered = window
            .available_monitors()
            .unwrap_or_default()
            .into_iter()
            .find(|monitor| *monitor.position() == origin);
        if remembered.is_some() {
            return remembered;
        }
    }

    #[cfg(target_os = "linux")]
    {
        if let Some(monitor) = active_window_monitor(window) {
            return Some(monitor);
        }
    }

    window
        .current_monitor()
        .ok()
        .flatten()
        .or_else(|| window.primary_monitor().ok().flatten())
}

/// The monitor under the mouse, or `None` when the platform will not say where
/// the mouse is. Wayland is the awkward case: `tao` returns a hardcoded
/// `(0, 0)` there rather than an error, so on Wayland a zero reading has to be
/// taken as "unknown" instead of as the top-left corner.
///
/// Deliberately does *not* fall back to the primary monitor: the callers behind
/// [`focused_monitor`] — the remembered monitor, the focused X11 window — are
/// better answers than "the primary one", and returning a monitor here would
/// hide them.
fn cursor_monitor(window: &WebviewWindow) -> Option<Monitor> {
    let cursor = window.cursor_position().ok()?;
    if cursor.x == 0.0 && cursor.y == 0.0 && on_wayland() {
        return None;
    }
    // Matched by hand first; `monitor_from_point` is only the fallback for a
    // reading that lands outside every monitor's bounds. See
    // [`monitor_containing`] for why tao's own answer cannot be trusted.
    let monitor = monitor_containing(window, cursor)
        .or_else(|| window.monitor_from_point(cursor.x, cursor.y).ok().flatten());
    eprintln!(
        "floter: cursor {:?} -> monitor {:?}",
        (cursor.x, cursor.y),
        monitor
            .as_ref()
            .map(|found| (found.name(), *found.position(), found.scale_factor())),
    );
    monitor
}

/// The monitor whose bounds contain the cursor.
///
/// This exists because `monitor_from_point` compares the two values in
/// *different coordinate spaces* on both macOS and Linux, so it answers wrongly
/// — or, worse, not at all — as soon as a scale factor other than 1 is
/// involved. In `tao`:
///
/// - `cursor_position()` reads the pointer in points and multiplies by the
///   **primary** monitor's scale factor.
/// - A monitor's `position()`/`size()` are points multiplied by **that
///   monitor's own** scale factor.
/// - `monitor_from_point()` hands the value straight to `CGRectContainsPoint`
///   against `CGDisplayBounds` (macOS) or `gdk_display_monitor_at_point`
///   (Linux), both of which are quoted in **points**.
///
/// So on a Retina laptop (scale 2) with an external display to its right, a
/// cursor at point 2400 is reported as 4800, which is past the right edge of
/// every display: `monitor_from_point` returns `None` and the panel falls back
/// to the primary screen. Dividing each reading by the scale factor that was
/// applied to it puts them back in one shared space, where the comparison
/// means something.
///
/// Windows needs none of this — every value there is already in one physical
/// pixel space — hence the platform-dependent divisors below.
fn monitor_containing(window: &WebviewWindow, cursor: PhysicalPosition<f64>) -> Option<Monitor> {
    let cursor_scale = cursor_bounds_scale(window);
    if cursor_scale <= 0.0 {
        return None;
    }
    let x = cursor.x / cursor_scale;
    let y = cursor.y / cursor_scale;

    window
        .available_monitors()
        .ok()?
        .into_iter()
        .find(|monitor| {
            let scale = monitor_bounds_scale(monitor);
            if scale <= 0.0 {
                return false;
            }
            let position = monitor.position();
            let size = monitor.size();
            let min_x = f64::from(position.x) / scale;
            let min_y = f64::from(position.y) / scale;
            // Half-open, so a cursor on the seam between two screens belongs to
            // exactly one of them.
            let max_x = min_x + f64::from(size.width) / scale;
            let max_y = min_y + f64::from(size.height) / scale;
            x >= min_x && x < max_x && y >= min_y && y < max_y
        })
}

/// The factor `cursor_position()` applied to the pointer's position in points.
#[cfg(target_os = "windows")]
fn cursor_bounds_scale(_window: &WebviewWindow) -> f64 {
    1.0
}

#[cfg(not(target_os = "windows"))]
fn cursor_bounds_scale(window: &WebviewWindow) -> f64 {
    window
        .primary_monitor()
        .ok()
        .flatten()
        .map_or(1.0, |monitor| monitor.scale_factor())
}

/// The factor a monitor's reported bounds were multiplied by.
#[cfg(target_os = "windows")]
fn monitor_bounds_scale(_monitor: &Monitor) -> f64 {
    1.0
}

#[cfg(not(target_os = "windows"))]
fn monitor_bounds_scale(monitor: &Monitor) -> f64 {
    monitor.scale_factor()
}

#[cfg(target_os = "linux")]
fn on_wayland() -> bool {
    std::env::var_os("WAYLAND_DISPLAY").is_some()
        || std::env::var("XDG_SESSION_TYPE").is_ok_and(|kind| kind.eq_ignore_ascii_case("wayland"))
}

#[cfg(not(target_os = "linux"))]
fn on_wayland() -> bool {
    false
}

/// The monitor holding the focused window, asked of X11 through `xprop` and
/// then `xdotool` or `xwininfo`. This is the keyboard user's answer to "which
/// screen am I on": it stays right even when the mouse was left elsewhere.
///
/// Every step is best-effort. Missing tools, a session with no X server, or a
/// focused window that is not an X11 client all just return `None`, and the
/// caller moves on to the next strategy.
#[cfg(target_os = "linux")]
fn active_window_monitor(window: &WebviewWindow) -> Option<Monitor> {
    let id = x11_active_window_id()?;
    let (x, y, width, height) = xdotool_geometry(&id).or_else(|| xwininfo_geometry(&id))?;
    // The center rather than the origin: a window straddling two screens belongs
    // to the one showing most of it, and a maximized window's top-left corner
    // can sit a pixel outside its own monitor.
    //
    // Both tools report X11 pixels, i.e. the same physical space a cursor
    // reading is in, so the match goes through [`monitor_containing`] for the
    // same coordinate-space reason.
    let center = PhysicalPosition::new(x + width / 2.0, y + height / 2.0);
    monitor_containing(window, center)
        .or_else(|| window.monitor_from_point(center.x, center.y).ok().flatten())
}

#[cfg(target_os = "linux")]
fn x11_active_window_id() -> Option<String> {
    let output = std::process::Command::new("xprop")
        .args(["-root", "_NET_ACTIVE_WINDOW"])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    // "_NET_ACTIVE_WINDOW(WINDOW): window id # 0x3400007"
    let stdout = String::from_utf8_lossy(&output.stdout);
    let digits: String = stdout
        .split("0x")
        .nth(1)?
        .chars()
        .take_while(char::is_ascii_hexdigit)
        .collect();
    // 0x0 means nothing is focused, which is how every native Wayland client
    // looks from Xwayland's side of the fence.
    if digits.trim_start_matches('0').is_empty() {
        return None;
    }
    // Keep the 0x prefix: both tools parse the id with base 0, so a bare
    // "3400007" would silently be read as decimal and name the wrong window.
    Some(format!("0x{digits}"))
}

/// Parses `X=1920 / Y=100 / WIDTH=800 / HEIGHT=600` out of `--shell` output.
#[cfg(target_os = "linux")]
fn xdotool_geometry(id: &str) -> Option<(f64, f64, f64, f64)> {
    let output = std::process::Command::new("xdotool")
        .args(["getwindowgeometry", "--shell", id])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    let text = String::from_utf8_lossy(&output.stdout);
    let field = |key: &str| {
        text.lines()
            .find_map(|line| line.strip_prefix(key)?.trim().parse::<f64>().ok())
    };
    Some((
        field("X=")?,
        field("Y=")?,
        field("WIDTH=")?,
        field("HEIGHT=")?,
    ))
}

/// Parses the same four numbers out of `xwininfo`'s `Key: value` listing, for
/// the many systems that ship `xprop` and `xwininfo` but not `xdotool`.
#[cfg(target_os = "linux")]
fn xwininfo_geometry(id: &str) -> Option<(f64, f64, f64, f64)> {
    let output = std::process::Command::new("xwininfo")
        .args(["-id", id])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    let text = String::from_utf8_lossy(&output.stdout);
    let field = |key: &str| {
        text.lines().find_map(|line| {
            let (name, value) = line.split_once(':')?;
            if name.trim() != key {
                return None;
            }
            value.trim().parse::<f64>().ok()
        })
    };
    Some((
        field("Absolute upper-left X")?,
        field("Absolute upper-left Y")?,
        field("Width")?,
        field("Height")?,
    ))
}

/// Note which screen the panel is on before it disappears. `current_monitor()`
/// only means anything while the window is mapped, so this has to run *before*
/// `hide()`.
fn remember_monitor(window: &WebviewWindow, state: &AppState) {
    if let Ok(Some(monitor)) = window.current_monitor() {
        state.set_remembered_monitor(Some(*monitor.position()));
    }
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
    state: &AppState,
) -> Option<LogicalPosition<f64>> {
    let monitor = focused_monitor(window, state)?;
    let terminal_height = state.terminal_height();
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

    eprintln!(
        "floter: placing on {:?} at {:?} work_area {:?}/{:?} scale {scale} -> ({x}, {y})",
        monitor.name(),
        *monitor.position(),
        area.position,
        area.size,
    );

    Some(LogicalPosition::new(x, y))
}

fn move_to_default_position(
    window: &WebviewWindow,
    logical_width: f64,
    state: &AppState,
) -> Result<(), String> {
    match default_position(window, logical_width, state) {
        Some(position) => window.set_position(position).map_err(|e| e.to_string()),
        None => window.center().map_err(|e| e.to_string()),
    }
}

#[cfg(target_os = "macos")]
fn is_main_thread() -> bool {
    unsafe { libc::pthread_main_np() == 1 }
}

/// Convert Tauri's NSWindow into the same non-activating NSPanel shape used by
/// native launchers. The WebView and Tauri handle remain attached to the object;
/// only its Objective-C class and panel behavior change.
#[cfg(target_os = "macos")]
fn configure_macos_panel(window: &WebviewWindow) -> Result<(), String> {
    let panel = window
        .to_panel::<FloterPanel>()
        .map_err(|error| error.to_string())?;

    panel.set_level(PanelLevel::Floating.value());
    panel.set_style_mask(StyleMask::empty().nonactivating_panel().resizable().into());
    panel.set_collection_behavior(
        CollectionBehavior::new()
            .can_join_all_spaces()
            .full_screen_auxiliary()
            .into(),
    );
    panel.set_floating_panel(true);
    panel.set_hides_on_deactivate(false);
    panel.set_works_when_modal(true);
    panel.set_released_when_closed(false);
    // `shadow: false` is required by the Windows frame path in tauri.conf.json,
    // but macOS panels should use the window server's native soft shadow. It
    // follows the alpha outline and is recomputed after every resize below.
    window.set_shadow(true).map_err(|error| error.to_string())?;
    refresh_macos_shadow(window);
    Ok(())
}

#[cfg(target_os = "macos")]
fn show_macos_panel(window: &WebviewWindow) -> Result<(), String> {
    if !is_main_thread() {
        let window = window.clone();
        let handle = window.app_handle().clone();
        handle
            .run_on_main_thread(move || {
                let _ = show_macos_panel(&window);
            })
            .map_err(|error| error.to_string())?;
        return Ok(());
    }

    let panel = window
        .app_handle()
        .get_webview_panel(window.label())
        .map_err(|_| "macOS panel is not initialized".to_string())?;
    panel.show_and_make_key();
    panel.order_front_regardless();

    // A panel summoned before the accessory app has ever activated can lose the
    // first key request. Tinycast reasserts it on the next main-loop turn too.
    let label = window.label().to_string();
    let handle = window.app_handle().clone();
    let retry_handle = handle.clone();
    let _ = handle.run_on_main_thread(move || {
        if let Ok(panel) = retry_handle.get_webview_panel(&label) {
            if panel.is_visible() && !panel.as_panel().isKeyWindow() {
                panel.show_and_make_key();
            }
        }
    });
    Ok(())
}

#[cfg(target_os = "macos")]
fn hide_macos_panel(window: &WebviewWindow) -> Result<(), String> {
    if !is_main_thread() {
        let window = window.clone();
        let handle = window.app_handle().clone();
        handle
            .run_on_main_thread(move || {
                let _ = hide_macos_panel(&window);
            })
            .map_err(|error| error.to_string())?;
        return Ok(());
    }

    window
        .app_handle()
        .get_webview_panel(window.label())
        .map_err(|_| "macOS panel is not initialized".to_string())?
        .hide();
    Ok(())
}

/// Windows draws two independent edges around an undecorated window: the DWM
/// border — a hard line the CSS surface would otherwise disagree with — and
/// the drop shadow, which is the gray outline that shows around the rounded
/// corners of a transparent window. Both are native frame, so both are off:
/// the edge is drawn by CSS (an inset ring in `App.css`) and the depth the
/// shadow used to give is drawn there too, by a pseudo-element falloff. The
/// webview's own background is cleared to transparent in `setup` so no opaque
/// fill can peek around the radius.
///
/// The DWM attributes below stay: the corners keep following the system's own
/// rounding — floter draws them itself (`DWMWCP_DONOTROUND`) so CSS is the
/// sole source of the corner shape — and the border colour is pinned to none
/// so no residual edge can survive the CSS one.
#[cfg(target_os = "windows")]
fn configure_windows_frame(window: &WebviewWindow) -> Result<(), String> {
    use windows::Win32::Graphics::Dwm::{
        DwmSetWindowAttribute, DWMWA_BORDER_COLOR, DWMWA_COLOR_NONE,
        DWMWA_WINDOW_CORNER_PREFERENCE, DWMWCP_DONOTROUND,
    };

    window
        .set_shadow(false)
        .map_err(|error| error.to_string())?;
    let hwnd = window.hwnd().map_err(|error| error.to_string())?;
    let preference = DWMWCP_DONOTROUND;
    let border_color = DWMWA_COLOR_NONE;
    unsafe {
        // These attributes were added in Windows 11. Their failure is expected
        // and harmless on Windows 10, where DWM does not apply rounded corners.
        let _ = DwmSetWindowAttribute(
            hwnd,
            DWMWA_WINDOW_CORNER_PREFERENCE,
            &preference as *const _ as *const _,
            std::mem::size_of_val(&preference) as u32,
        );
        let _ = DwmSetWindowAttribute(
            hwnd,
            DWMWA_BORDER_COLOR,
            &border_color as *const _ as *const _,
            std::mem::size_of_val(&border_color) as u32,
        );
    }
    suppress_alt_space_system_menu(window)
}

/// Whether the settings panel is recording a shortcut right now.
///
/// The subclass below swallows Alt+Space so the system menu never opens over
/// the panel, which also means the combination never reaches the webview — and
/// a shortcut the recorder cannot see is a shortcut that cannot be bound. The
/// flag opens that door for exactly as long as the recorder is listening. It is
/// a static rather than a field of [`AppState`] because a window procedure is
/// handed nothing but its `HWND`.
#[cfg(target_os = "windows")]
static SHORTCUT_RECORDING: AtomicBool = AtomicBool::new(false);

/// Hand Alt+Space to the webview while a shortcut is being recorded.
///
/// Windows is the only platform that intercepts the combination at all: the X11
/// grab Linux uses is made by the shortcut plugin itself, and macOS has no
/// window menu on that key. The frontend therefore only calls this there, and
/// the command is a no-op everywhere else.
#[tauri::command]
fn set_recording_flag(on: bool) {
    #[cfg(target_os = "windows")]
    SHORTCUT_RECORDING.store(on, Ordering::SeqCst);
    #[cfg(not(target_os = "windows"))]
    let _ = on;
}

#[cfg(target_os = "windows")]
unsafe extern "system" fn floter_window_subclass(
    hwnd: windows::Win32::Foundation::HWND,
    message: u32,
    wparam: windows::Win32::Foundation::WPARAM,
    lparam: windows::Win32::Foundation::LPARAM,
    _subclass_id: usize,
    _reference_data: usize,
) -> windows::Win32::Foundation::LRESULT {
    use windows::Win32::Foundation::LRESULT;
    use windows::Win32::UI::Input::KeyboardAndMouse::VK_SPACE;
    use windows::Win32::UI::Shell::DefSubclassProc;
    use windows::Win32::UI::WindowsAndMessaging::{
        KF_ALTDOWN, SC_KEYMENU, WM_SYSCHAR, WM_SYSCOMMAND, WM_SYSKEYDOWN,
    };

    let alt_is_down = ((lparam.0 as usize >> 16) & KF_ALTDOWN as usize) != 0;
    let alt_space_key = matches!(message, WM_SYSKEYDOWN | WM_SYSCHAR)
        && wparam.0 == VK_SPACE.0 as usize
        && alt_is_down;
    // WebView2 can translate the child-window key event before the top-level
    // window sees the resulting system command. Catch that final path too.
    let alt_space_menu = message == WM_SYSCOMMAND
        && wparam.0 & 0xfff0 == SC_KEYMENU as usize
        && lparam.0 == VK_SPACE.0 as isize;
    // The key messages are released to the webview while the recorder is
    // listening, so Alt+Space can be bound like any other combination. The
    // system command never is: it is not what carries the key to the page, and
    // the menu it opens would take focus and cancel the recording it was meant
    // to serve.
    if alt_space_key && !SHORTCUT_RECORDING.load(Ordering::SeqCst) {
        return LRESULT(0);
    }
    if alt_space_menu {
        return LRESULT(0);
    }

    unsafe { DefSubclassProc(hwnd, message, wparam, lparam) }
}

#[cfg(target_os = "windows")]
fn suppress_alt_space_system_menu(window: &WebviewWindow) -> Result<(), String> {
    use windows::Win32::UI::Shell::SetWindowSubclass;

    // SetWindowSubclass keys registrations by callback + id, so calling this
    // again on reveal reasserts the handler without stacking another callback.
    const SUBCLASS_ID: usize = 0x464c_4f54;
    let hwnd = window.hwnd().map_err(|error| error.to_string())?;
    let installed =
        unsafe { SetWindowSubclass(hwnd, Some(floter_window_subclass), SUBCLASS_ID, 0) };
    if installed.as_bool() {
        Ok(())
    } else {
        Err("Failed to install the Windows message handler".to_string())
    }
}

/// Tell the window server to recompute the panel's shadow.
///
/// macOS derives the shadow of a transparent window from the alpha of what it
/// draws, but only when it is asked to: a window that has been resized keeps
/// the shadow of the shape it used to be. The launcher changes height on every
/// keystroke that changes the result list, and the settings and terminal
/// windows are resized the moment they open, so the leftover is a square
/// outline standing a little way outside the rounded card — an edge nobody
/// drew, along the sides and around the bottom corners.
#[cfg(target_os = "macos")]
fn refresh_macos_shadow(window: &WebviewWindow) {
    if !is_main_thread() {
        let window = window.clone();
        let handle = window.app_handle().clone();
        let _ = handle.run_on_main_thread(move || refresh_macos_shadow(&window));
        return;
    }

    if let Ok(panel) = window.app_handle().get_webview_panel(window.label()) {
        panel.as_panel().invalidateShadow();
    }
}

fn reveal_window(window: &WebviewWindow) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    show_macos_panel(window)?;
    #[cfg(not(target_os = "macos"))]
    {
        let _ = window.unminimize();
        #[cfg(target_os = "windows")]
        configure_windows_frame(window)?;
        window.show().map_err(|e| e.to_string())?;
        let _ = window.set_always_on_top(true);
        window.set_focus().map_err(|e| e.to_string())?;
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
        saved_terminal_size().0
    } else {
        INPUT_WINDOW_WIDTH
    };
    let _ = window.set_resizable(terminal);

    // Reveal first, position second: macOS' window server ignores geometry set
    // on an unmapped window, so a `set_position` made while hidden is discarded
    // and `show()` puts the panel back wherever it last was. Doing it in this
    // order costs nothing on the other platforms — both calls land in the same
    // event loop tick, so there is no visible jump.
    reveal_window(window)?;
    let _ = move_to_default_position(window, width, state);
    state.window_visible.store(true, Ordering::SeqCst);
    let _ = window.emit("floter://revealed", mode);
    Ok(())
}

fn resize_window(
    window: &WebviewWindow,
    width: f64,
    height: f64,
    preserve_anchor: bool,
) -> Result<(), String> {
    let previous_position = window.outer_position().ok();
    let previous_size = window.outer_size().ok();
    let scale_factor = window.scale_factor().unwrap_or(1.0);

    // The anchor below compares two *outer* readings against the width being
    // set, which is an inner one. On Windows the undecorated DWM shadow used to
    // put a frame between the two, and left uncorrected that difference was
    // added to the window's x on every collapse and expand — the panel walked
    // across the screen a few pixels at a time. The native frame is gone now
    // (`shadow: false`), so outer and inner are the same size everywhere and
    // the correction is zero.
    let frame_width = 0;

    window
        .set_size(LogicalSize::new(width, height))
        .map_err(|e| e.to_string())?;

    if preserve_anchor {
        if let (Some(position), Some(size)) = (previous_position, previous_size) {
            let next_width = (width * scale_factor).round() as i32 + frame_width;
            let next_x = position.x + (size.width as i32 - next_width) / 2;
            window
                .set_position(PhysicalPosition::new(next_x, position.y))
                .map_err(|e| e.to_string())?;
        }
    }

    // No default-position fallback here: this runs while the window may still be
    // hidden, and macOS drops geometry set on an unmapped window. Callers that
    // are not preserving an anchor place the window themselves, after revealing
    // it.
    Ok(())
}

fn terminal_size_for_monitor(
    window: &WebviewWindow,
    state: &AppState,
    width: f64,
    height: f64,
) -> (f64, f64) {
    let Some(monitor) = focused_monitor(window, state) else {
        return (width, height);
    };
    let scale = monitor.scale_factor();
    if scale <= 0.0 {
        return (width, height);
    }
    let area = monitor.work_area();
    let available_width = (area.size.width as f64 / scale - 24.0).max(320.0);
    let available_height = (area.size.height as f64 / scale - 24.0).max(240.0);
    (width.min(available_width), height.min(available_height))
}

#[tauri::command]
fn show_terminal(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    let preserve_anchor = state.window_visible.load(Ordering::SeqCst);
    let (width, height) = saved_terminal_size();
    let (width, height) = terminal_size_for_monitor(&window, &state, width, height);
    if let Ok(mut current) = state.terminal_height.lock() {
        *current = height;
    }
    window
        .set_resizable(true)
        .map_err(|error| error.to_string())?;
    resize_window(&window, width, height, preserve_anchor)?;
    reveal_window(&window)?;
    if !preserve_anchor {
        let _ = move_to_default_position(&window, width, &state);
    }
    state.terminal_mode.store(true, Ordering::SeqCst);
    state.window_visible.store(true, Ordering::SeqCst);
    Ok(())
}

/// Persist the terminal dimensions after an edge resize, and use the new
/// height when positioning the compact launcher above its expanded counterpart.
#[tauri::command]
fn save_terminal_size(
    width: f64,
    height: f64,
    state: tauri::State<'_, AppState>,
) -> Result<(), String> {
    let (_, height) = persist_terminal_size(width, height)?;
    if let Ok(mut current) = state.terminal_height.lock() {
        *current = height;
    }
    Ok(())
}

#[tauri::command]
fn hide_window(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    remember_monitor(&window, &state);
    #[cfg(target_os = "macos")]
    hide_macos_panel(&window)?;
    #[cfg(not(target_os = "macos"))]
    window.hide().map_err(|e| e.to_string())?;
    state.window_visible.store(false, Ordering::SeqCst);
    Ok(())
}

#[tauri::command]
fn quit_app(app: tauri::AppHandle) {
    app.exit(0);
}

#[tauri::command]
fn start_drag(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    // The user is about to pick a screen by hand, which makes the monitor
    // remembered from the last dismissal stale. It is refilled on the next hide,
    // from wherever the drag left the panel.
    state.set_remembered_monitor(None);
    // One path on every platform: `start_dragging` hands the click to the OS as
    // a caption drag — on Windows it performs the same WM_NCLBUTTONDOWN
    // hand-off the removed `start_windows_drag` used to make by hand.
    window.start_dragging().map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
fn show_input(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    let preserve_anchor = state.window_visible.load(Ordering::SeqCst);
    window
        .set_resizable(false)
        .map_err(|error| error.to_string())?;
    resize_window(
        &window,
        INPUT_WINDOW_WIDTH,
        INPUT_WINDOW_HEIGHT,
        preserve_anchor,
    )?;
    reveal_window(&window)?;
    if !preserve_anchor {
        let _ = move_to_default_position(&window, INPUT_WINDOW_WIDTH, &state);
    }
    state.terminal_mode.store(false, Ordering::SeqCst);
    state.window_visible.store(true, Ordering::SeqCst);
    Ok(())
}

/// Show the panel when it is hidden, hide it when it is up: the behaviour bound
/// to the global toggle shortcut.
fn toggle_window_visibility(app: &AppHandle) {
    let Some(window) = app.get_webview_window("main") else {
        return;
    };
    let state = app.state::<AppState>();
    if state.window_visible.load(Ordering::SeqCst) {
        remember_monitor(&window, &state);
        #[cfg(target_os = "macos")]
        let hidden = hide_macos_panel(&window);
        #[cfg(not(target_os = "macos"))]
        let hidden = window.hide().map_err(|error| error.to_string());
        if hidden.is_ok() {
            state.window_visible.store(false, Ordering::SeqCst);
        }
    } else {
        let _ = reveal_saved_mode(&window, &state);
    }
}

/// Register `shortcut` with the OS as the global toggle.
pub fn register_toggle_shortcut(app: &AppHandle, shortcut: &str) -> Result<(), String> {
    let handle = app.clone();
    app.global_shortcut()
        .on_shortcut(shortcut, move |_app, _shortcut, event| {
            if event.state == ShortcutState::Pressed {
                toggle_window_visibility(&handle);
            }
        })
        .map_err(|e| e.to_string())?;

    if let Ok(mut active) = app.state::<AppState>().toggle_shortcut.lock() {
        *active = shortcut.to_string();
    }
    Ok(())
}

/// Move the global toggle to `next`.
///
/// A combination another application already owns is refused by the OS; the
/// previous binding is restored in that case, so the panel never ends up with
/// no way to be summoned.
pub fn rebind_toggle_shortcut(app: &AppHandle, next: &str) -> Result<(), String> {
    let previous = app
        .state::<AppState>()
        .toggle_shortcut
        .lock()
        .map(|active| active.clone())
        .unwrap_or_default();

    if !previous.is_empty() {
        let _ = app.global_shortcut().unregister(previous.as_str());
    }
    if let Err(error) = register_toggle_shortcut(app, next) {
        if !previous.is_empty() {
            let _ = register_toggle_shortcut(app, previous.as_str());
        }
        return Err(error);
    }
    Ok(())
}

/// Point the user at the `--toggle` escape hatch.
///
/// Wayland hands global key bindings to the compositor and to nobody else, so
/// the X11 grab the shortcut plugin performs either fails outright or — with
/// Xwayland in the picture — is accepted and then never fires. Either way the
/// panel needs a compositor-owned binding, and the user is the only one who can
/// create it.
#[cfg(target_os = "linux")]
fn print_toggle_hint(reason: &str) {
    eprintln!("{reason}");
    eprintln!("Bind 'floter --toggle' as a custom shortcut in your compositor settings.");
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
#[allow(deprecated)]
pub fn run() {
    // Before the builder, and therefore before anything initializes GTK: this
    // is the last point at which the renderer WebKitGTK will use can still be
    // chosen. See `linux_render` for why that choice cannot be made later.
    #[cfg(target_os = "linux")]
    linux_render::prepare(std::env::args_os());

    let builder = tauri::Builder::default().plugin(tauri_plugin_single_instance::init(
        |app, arguments, _working_directory| {
            if arguments.iter().any(|argument| argument == "--background") {
                return;
            }
            let handle = app.clone();
            let _ = app.run_on_main_thread(move || {
                if let Some(window) = handle.get_webview_window("main") {
                    let state = handle.state::<AppState>();
                    let _ = reveal_saved_mode(&window, &state);
                }
            });
        },
    ));
    #[cfg(target_os = "macos")]
    let builder = builder.plugin(tauri_nspanel::init());

    builder
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(tauri_plugin_store::Builder::new().build())
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_dialog::init())
        .manage(ApplicationState::new())
        .manage(TerminalState(Arc::new(Mutex::new(TerminalManager::new()))))
        .manage(CommandState::new())
        .manage(AppState {
            window_visible: AtomicBool::new(false),
            terminal_mode: AtomicBool::new(false),
            terminal_height: Mutex::new(saved_terminal_size().1),
            tray_items: Mutex::new(None),
            toggle_shortcut: Mutex::new(String::new()),
            last_monitor: Mutex::new(None),
        })
        .setup(|app| {
            app.manage(ExtensionState::new().map_err(std::io::Error::other)?);
            // floter is tray-resident, and the non-activating NSPanel must not
            // promote the process or switch away from another app's fullscreen
            // Space when it takes key focus.
            #[cfg(target_os = "macos")]
            app.set_activation_policy(ActivationPolicy::Accessory);

            let settings = load_settings();
            if let Err(error) = ensure_launch_at_startup(settings.launch_at_startup) {
                eprintln!("failed to reconcile launch-at-startup registration: {error}");
            }
            let (show_label, quit_label) = tray_labels(&settings.language);
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
                    if let TrayIconEvent::Click { button, .. } = event {
                        if button != tauri::tray::MouseButton::Left {
                            return;
                        }
                        if let Some(window) = tray.app_handle().get_webview_window("main") {
                            let state = tray.app_handle().state::<AppState>();
                            let _ = reveal_saved_mode(&window, &state);
                        }
                    }
                })
                .build(app)?;

            let window = app
                .get_webview_window("main")
                .ok_or("missing main webview window")?;
            // The webview exists, which is as far as a machine with a broken EGL
            // ever gets — so this run counts as a successful start and the next
            // one is free to use the GPU again.
            #[cfg(target_os = "linux")]
            linux_render::mark_started();
            #[cfg(target_os = "macos")]
            configure_macos_panel(&window)?;
            #[cfg(target_os = "windows")]
            configure_windows_frame(&window).map_err(std::io::Error::other)?;
            // The webview paints its own opaque background by default; with the
            // native frame gone, that fill would be what shows around the CSS
            // radius instead of the desktop. Clear it so the window's own
            // transparency is the only background there is.
            #[cfg(target_os = "windows")]
            window
                .set_background_color(Some(Color(0, 0, 0, 0)))
                .map_err(std::io::Error::other)?;
            window.set_resizable(false)?;
            let shadow_window = window.clone();
            window.on_window_event(move |event| {
                match event {
                    tauri::WindowEvent::CloseRequested { api, .. } => api.prevent_close(),
                    // Every resize the panel goes through — the frontend sizing
                    // the launcher to its rows, an edge drag on the terminal,
                    // the settings panel opening.
                    #[cfg(target_os = "macos")]
                    tauri::WindowEvent::Resized(_) => refresh_macos_shadow(&shadow_window),
                    _ => {}
                }
            });
            #[cfg(not(target_os = "macos"))]
            let _ = shadow_window;

            let shortcut = resolved_shortcuts(&settings)
                .remove(TOGGLE_WINDOW)
                .unwrap_or_else(|| DEFAULT_TOGGLE_WINDOW.to_string());

            // Start the control socket before the shortcut attempt: under
            // Wayland it is the only route by which a key press can ever reach
            // the panel, so it should already be listening if the grab below
            // turns out to be useless.
            #[cfg(target_os = "linux")]
            ipc::serve(app.handle());

            if let Err(error) = register_toggle_shortcut(app.handle(), &shortcut) {
                // A stored combination can be rejected by the OS (another app
                // owns it, or the settings file was hand-edited); fall back to
                // the default so the panel stays reachable.
                eprintln!("failed to register global shortcut {shortcut}: {error}");
                if shortcut != DEFAULT_TOGGLE_WINDOW {
                    let _ = register_toggle_shortcut(app.handle(), DEFAULT_TOGGLE_WINDOW);
                }
                #[cfg(target_os = "linux")]
                if on_wayland() {
                    print_toggle_hint(
                        "Global shortcut registration failed (likely Wayland - X11 grabs don't work there).",
                    );
                }
            } else {
                eprintln!("registered global shortcut: {shortcut}");
                // The grab was accepted by Xwayland, which is not the same as it
                // ever being delivered: the compositor keeps the key to itself.
                #[cfg(target_os = "linux")]
                if on_wayland() {
                    print_toggle_hint(
                        "Running under Wayland, where an X11 grab is accepted but never fires.",
                    );
                }
            }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            term_spawn,
            term_input,
            term_resize,
            term_scroll,
            term_set_theme,
            term_wheel,
            term_mouse,
            term_scroll_to,
            save_terminal_size,
            application_icon,
            check_applications,
            list_applications,
            open_application,
            open_in_default_terminal,
            open_path,
            open_url,
            term_close,
            get_settings,
            save_settings,
            set_launch_at_startup,
            app_version,
            get_shortcuts,
            reset_shortcuts,
            update_shortcut,
            suspend_shortcuts,
            resume_shortcuts,
            set_recording_flag,
            get_custom_commands,
            add_custom_command,
            update_custom_command,
            delete_custom_command,
            execute_custom_command,
            show_terminal,
            hide_window,
            quit_app,
            show_input,
            start_drag,
            system_power,
            extensions_list,
            extensions_export,
            extensions_import,
            extensions_install,
            extensions_permissions_summary,
            extensions_uninstall,
            extensions_enable,
            extensions_disable,
            extensions_update,
            extensions_rollback,
            extensions_describe,
            extensions_diagnose,
            extensions_search,
            extensions_config_get,
            extensions_config_set,
            catalog_search,
            catalog_complete,
        ])
        .build(tauri::generate_context!())
        .expect("error while running tauri application")
        .run(|app, event| {
            // Shut down sessions still owned by Floter. A broker session handed
            // to a system terminal has already left the manager and stays alive.
            if let tauri::RunEvent::Exit = event {
                if let Ok(manager) = app.state::<TerminalState>().0.lock() {
                    manager.shutdown_all();
                }
                terminal::broker::shutdown_if_idle();
                // The socket node outlives the process that made it, so the next
                // start would have to reclaim it as stale.
                #[cfg(target_os = "linux")]
                ipc::cleanup();
            }
        });
}
