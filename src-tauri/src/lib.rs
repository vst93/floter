mod commands;
#[cfg(target_os = "linux")]
pub mod ipc;
mod terminal;

use commands::actions::{open_path, open_url};
use commands::apps::{
    application_icon, check_applications, list_applications, open_application, ApplicationState,
};
use commands::config::{
    app_version, get_settings, get_shortcuts, load_settings, resolved_shortcuts, save_settings,
    suspend_shortcuts, resume_shortcuts, update_shortcut,
    DEFAULT_TOGGLE_WINDOW, TOGGLE_WINDOW,
};
use commands::custom::{
    add_custom_command, delete_custom_command, execute_custom_command, get_custom_commands,
    update_custom_command, CommandState,
};
use commands::system::system_power;
use commands::terminal::{
    open_in_default_terminal, term_close, term_input, term_resize, term_scroll, term_scroll_to,
    term_spawn, TerminalState,
};
#[cfg(target_os = "macos")]
use objc2::MainThreadMarker;
#[cfg(target_os = "macos")]
use objc2_app_kit::{NSApp, NSPopUpMenuWindowLevel, NSWindow, NSWindowCollectionBehavior};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
#[cfg(target_os = "macos")]
use tauri::{ActivationPolicy, UserAttentionType};
use tauri::{
    menu::{Menu, MenuItem},
    tray::{TrayIconBuilder, TrayIconEvent},
    AppHandle, Emitter, LogicalPosition, LogicalSize, Manager, Monitor, PhysicalPosition,
    WebviewWindow, Wry,
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

/// Raise the panel to a window level that clears a fullscreen app, and let it
/// share that app's space.
///
/// [`WebviewWindow::set_always_on_top`] is not enough on macOS: it is fixed at
/// `NSFloatingWindowLevel` (3), which wins against ordinary windows and loses
/// against a fullscreen one — the panel is summoned over a fullscreen editor or
/// browser and simply does not appear. `NSStatusWindowLevel` (25) is where the
/// menu bar and Spotlight sit, which is the company a summoned panel wants to
/// keep; it stays deliberately below `NSPopUpMenuWindowLevel` (101) so a native
/// menu opened *from* the panel still draws over it.
///
/// The level alone only settles the stacking order. `CanJoinAllSpaces` is what
/// puts the window on the fullscreen app's space at all — [`reveal_window`] has
/// already asked for that, and it is repeated here because the two flags are one
/// decision — and `FullScreenAuxiliary` is what lets it *share* that space
/// instead of pushing macOS to switch away to the desktop the panel belongs to.
/// Both are OR-ed into the current behaviour rather than replacing it, because
/// tao keeps its own bits in the same field.
///
/// This is why `set_always_on_top` is not called on macOS at all: tao implements
/// it with `set_level_async`, which posts `setLevel:` to the main dispatch queue
/// *even when it is already on the main thread*, so its level 3 would land after
/// the 25 set here and quietly undo it.
#[cfg(target_os = "macos")]
fn raise_window_level(window: &WebviewWindow) {
    if !is_main_thread() {
        let window = window.clone();
        let handle = window.app_handle().clone();
        let _ = handle.run_on_main_thread(move || {
            raise_window_level(&window);
        });
        return;
    }
    let Ok(ns_window) = window.ns_window() else {
        return;
    };
    let ns_window: &NSWindow = unsafe { &*ns_window.cast::<NSWindow>() };
    ns_window.setLevel(NSPopUpMenuWindowLevel);
    ns_window.setCollectionBehavior(
        NSWindowCollectionBehavior::CanJoinAllSpaces
            | NSWindowCollectionBehavior::FullScreenAuxiliary,
    );
}

/// Bring the panel forward and give it the keyboard, on a fullscreen app's space.
///
/// This replaces [`WebviewWindow::set_focus`] on macOS, which cannot do the job
/// for an accessory application:
///
/// * tao guards its whole implementation behind `isVisible()`, and a window the
///   window server has accepted but not yet placed on the current space reads as
///   *not* visible — so the activation is silently skipped in exactly the case
///   it is needed.
/// * It then calls `makeKeyAndOrderFront:` *before* activating. That method is
///   documented to order a window in front of its own application's windows
///   only, when the application is not the active one; an accessory app summoned
///   over a fullscreen editor is never the active one at that moment, so the
///   call lands and does nothing.
///
/// The order here is the fix. Activating first makes the ordering below mean
/// something globally, `orderFrontRegardless` raises the window whether or not
/// the activation was honoured — it is the one AppKit call that ignores the
/// active-application rule — and `makeKeyAndOrderFront:` then makes it the key
/// window, which is what actually routes keystrokes into the webview.
///
/// `activateIgnoringOtherApps:` is deprecated in favour of `activate`, which is
/// not used here for two reasons: it only exists on macOS 14 and later, and
/// objc2 performs no availability check, so an older system would take an
/// unrecognized selector. It is also the weaker call by design — `activate`
/// waits for the frontmost application to yield, which a fullscreen app has no
/// reason to do.
#[cfg(target_os = "macos")]
fn is_main_thread() -> bool {
    // pthread_main_np returns 1 if called on the main thread
    unsafe { libc::pthread_main_np() == 1 }
}

#[cfg(target_os = "macos")]
fn force_activate(window: &WebviewWindow) {
    if !is_main_thread() {
        // Global shortcut callbacks may fire on a background thread.
        // NSWindow calls require the main thread - re-dispatch synchronously.
        let window = window.clone();
        let handle = window.app_handle().clone();
        let _ = handle.run_on_main_thread(move || {
            force_activate(&window);
        });
        return;
    }
    let (Ok(ns_window), Some(mtm)) = (window.ns_window(), MainThreadMarker::new()) else {
        return;
    };
    let ns_window: &NSWindow = unsafe { &*ns_window.cast::<NSWindow>() };
    ns_window.orderFrontRegardless();
    #[allow(deprecated)]
    NSApp(mtm).activateIgnoringOtherApps(true);
    ns_window.makeKeyAndOrderFront(None);
}

/// On Windows 11, tell DWM not to paint its own rounded corners. The CSS
/// `border-radius` is the sole source of the corner shape, so the two never
/// disagree and leave a seam. On Windows 10 this is a no-op (DWM already
/// uses square corners), and the call fails silently.
#[cfg(target_os = "windows")]
fn disable_dwm_rounding(window: &WebviewWindow) {
    use windows::Win32::Graphics::Dwm::{
        DwmSetWindowAttribute, DWMWA_WINDOW_CORNER_PREFERENCE, DWMWCP_DONOTROUND,
    };

    let hwnd = window.hwnd().expect("window handle");
    let preference = DWMWCP_DONOTROUND;
    unsafe {
        let _ = DwmSetWindowAttribute(
            hwnd,
            DWMWA_WINDOW_CORNER_PREFERENCE,
            &preference as *const _ as *const _,
            std::mem::size_of_val(&preference) as u32,
        );
    }
}

fn reveal_window(window: &WebviewWindow) -> Result<(), String> {
    #[cfg(target_os = "macos")]
    {
        if !is_main_thread() {
            let window = window.clone();
            let handle = window.app_handle().clone();
            let _ = handle.run_on_main_thread(move || {
                let _ = reveal_window(&window);
            });
            return Ok(());
        }
        let _ = window.app_handle().show();
        raise_window_level(window);
        if let Ok(ns_window) = window.ns_window() {
            let ns_window: &NSWindow = unsafe { &*ns_window.cast::<NSWindow>() };
            ns_window.orderFrontRegardless();
        }
        force_activate(window);
        raise_window_level(window);
        let _ = window.request_user_attention(Some(UserAttentionType::Informational));
    }
    #[cfg(not(target_os = "macos"))]
    {
        let _ = window.unminimize();
        window.show().map_err(|e| e.to_string())?;
        #[cfg(target_os = "windows")]
        disable_dwm_rounding(window);
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
        TERMINAL_WINDOW_WIDTH
    } else {
        INPUT_WINDOW_WIDTH
    };

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
        }
    }

    // No default-position fallback here: this runs while the window may still be
    // hidden, and macOS drops geometry set on an unmapped window. Callers that
    // are not preserving an anchor place the window themselves, after revealing
    // it.
    Ok(())
}

#[tauri::command]
fn show_terminal(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    let preserve_anchor = state.window_visible.load(Ordering::SeqCst);
    resize_window(
        &window,
        TERMINAL_WINDOW_WIDTH,
        TERMINAL_WINDOW_HEIGHT,
        preserve_anchor,
    )?;
    reveal_window(&window)?;
    if !preserve_anchor {
        let _ = move_to_default_position(&window, TERMINAL_WINDOW_WIDTH, &state);
    }
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
    remember_monitor(&window, &state);
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
        let _ = window.hide();
        state.window_visible.store(false, Ordering::SeqCst);
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
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(tauri_plugin_store::Builder::new().build())
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .manage(ApplicationState::new())
        .manage(TerminalState(Arc::new(Mutex::new(TerminalManager::new()))))
        .manage(CommandState::new())
        .manage(AppState {
            window_visible: AtomicBool::new(false),
            terminal_mode: AtomicBool::new(false),
            terminal_height: Mutex::new(TERMINAL_WINDOW_HEIGHT),
            tray_items: Mutex::new(None),
            toggle_shortcut: Mutex::new(String::new()),
            last_monitor: Mutex::new(None),
        })
        .setup(|app| {
            // The window level in [`raise_window_level`] settles where the panel
            // draws; this settles whether the user is still looking at the same
            // screen when it does. `set_focus` ends in
            // `NSApp.activateIgnoringOtherApps:`, and a *regular* application
            // answers that by bringing its own space forward — so summoning the
            // panel over a fullscreen editor switched the user out of it, which
            // is the half of the problem no window level can fix. An accessory
            // application activates in place, leaving the fullscreen space where
            // it is for the panel to appear on.
            //
            // The Dock icon and the app's own menu bar go with it, which is the
            // shape floter already has: a tray-resident panel that starts hidden
            // and is quit from the tray. The default menu Tauri installs on macOS
            // stays in place, so Cmd+C / Cmd+V keep working in the webview even
            // though the menu bar is no longer drawn.
            #[cfg(target_os = "macos")]
            app.set_activation_policy(ActivationPolicy::Accessory);

            let settings = load_settings();
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
            term_scroll_to,
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
            app_version,
            get_shortcuts,
            update_shortcut,
            suspend_shortcuts,
            resume_shortcuts,
            get_custom_commands,
            add_custom_command,
            update_custom_command,
            delete_custom_command,
            execute_custom_command,
            show_terminal,
            hide_window,
            quit_app,
            show_input,
            set_terminal_height,
            start_drag,
            system_power,
        ])
        .build(tauri::generate_context!())
        .expect("error while running tauri application")
        .run(|app, event| {
            // Quitting is the only moment the terminal sessions are really
            // gone; returning to input mode deliberately keeps them (and any
            // tmux session behind them) alive.
            if let tauri::RunEvent::Exit = event {
                if let Ok(manager) = app.state::<TerminalState>().0.lock() {
                    manager.shutdown_all();
                }
                // The socket node outlives the process that made it, so the next
                // start would have to reclaim it as stale.
                #[cfg(target_os = "linux")]
                ipc::cleanup();
            }
        });
}
