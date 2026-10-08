#[cfg(feature = "clipboard-history")]
mod clipboard_history;
// R26-A: the built-in browser plugin's data layer (bookmarks / history search).
mod browser_data;
// R50 · the built-in calculator plugin's history store.
mod calculator_history;
mod commands;
pub mod deep_link;
pub mod extensions;
#[cfg(target_os = "linux")]
pub mod ipc;
#[cfg(target_os = "linux")]
mod linux_render;
mod notifications;
pub mod plugin_pages;
// R85 · the detached plugin window's remembered geometry.
mod plugin_window_geometry;
// R91 · more than one detached plugin window: the label allocator and the
// content identity a window is keyed by (pure, unit-tested there).
mod plugin_windows;
pub use plugin_windows::PLUGIN_WINDOW_LABEL;
// R41 · Hyprland (Wayland) window shaping: float the panel instead of letting
// the tiling compositor fill the screen.
mod hyprland;
// R41 · the one detached-spawn helper every "start a program" path uses.
mod process_launch;
mod terminal;
// R45 · the tray's Linux identity (SNI id / object path / menu path / icon
// file), derived once so it cannot collide with another Tauri app's tray.
mod tray_identity;

use commands::actions::{open_path, open_url, run_silent_command, system_spawn_detached};
use commands::apps::{
    application_icon, check_applications, list_applications, open_application, ApplicationState,
};
use commands::autostart::{ensure_launch_at_startup, set_launch_at_startup};
use commands::clipboard::{clipboard_read_text, clipboard_write_text};
use commands::config::set_custom_shortcuts;
use commands::config::{
    app_version, get_settings, get_shortcuts, load_settings, normalize_terminal_size,
    reset_shortcuts, resolved_shortcuts, resume_shortcuts, save_settings,
    save_terminal_size as persist_terminal_size, saved_terminal_size, suspend_shortcuts,
    update_shortcut, DEFAULT_TOGGLE_WINDOW, TOGGLE_WINDOW,
};
use commands::drops::resolve_dropped_files;
use commands::extensions::{
    catalog_complete, catalog_search, extensions_cancel_operation, extensions_config_copy,
    extensions_config_export, extensions_config_get, extensions_config_set,
    extensions_connect_recommended, extensions_connect_tool, extensions_create_custom,
    extensions_custom_export_script, extensions_custom_get, extensions_custom_update,
    extensions_describe, extensions_diagnose, extensions_disable, extensions_enable,
    extensions_export, extensions_health, extensions_import, extensions_install, extensions_launch,
    extensions_list, extensions_local_manifest_review, extensions_pick_local_manifest,
    extensions_pick_local_package, extensions_recommended_permissions, extensions_reconnect_system,
    extensions_repair, extensions_reprobe, extensions_reprobe_commands, extensions_run,
    extensions_run_output, extensions_script_runtime_check, extensions_search_tools,
    extensions_tool_catalog, extensions_uninstall, extensions_uninstall_componentized,
};
use commands::extensions::{external_plugin_commands, external_plugin_run};
use commands::system::system_power;
use commands::terminal::{
    open_in_default_terminal, term_attach_existing, term_close, term_input, term_kill_session,
    term_list_sessions, term_mouse, term_resize, term_scroll, term_scroll_to,
    term_set_cursor_style, term_set_theme, term_spawn, term_wheel, TerminalState,
};
use extensions::ExtensionState;
use std::collections::HashMap;
#[cfg(target_os = "macos")]
use std::sync::atomic::AtomicU64;
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
/// Height of the launcher card as CSS lays it out, plus the shell padding that
/// gives the card's box-shadow room outside it (`base.css`'s platform blocks).
/// Allocating less than this clips the card until the frontend's first resize
/// lands, so each platform states its own sum at `--ui-scale: 1`:
///
///   * macOS (and any other non-Windows/Linux target) — 58px: a 56px input row
///     plus the card's 1px top and bottom border, and no shell padding (the
///     window server owns the frame there, see `set_shadow(true)` below);
///   * Linux — 78px: the same 58px card in the shell's 10px top + 10px bottom
///     gutter (`R46`; it was 4px + 4px, i.e. 66, before the launcher's margin
///     was converged on the panels' own 10u);
///   * Windows — 72px: a borderless 56px card (`.platform-windows
///     .collapsed-card` paints its edge as an inset stroke, not a border) in
///     the shell's documented asymmetric 4px top + 12px bottom gutter.
#[cfg(target_os = "windows")]
const INPUT_WINDOW_HEIGHT: f64 = 72.0;
#[cfg(target_os = "linux")]
const INPUT_WINDOW_HEIGHT: f64 = 78.0;
#[cfg(not(any(target_os = "windows", target_os = "linux")))]
const INPUT_WINDOW_HEIGHT: f64 = 58.0;

/// The launcher's fallback height for the current interface-size step.
///
/// `INPUT_WINDOW_HEIGHT` is a **scale-1** measurement, exactly like the CSS box
/// values R7-13b tokenized: the collapsed card's input row, its border and the
/// shell padding that gives it shadow room are all written as
/// `calc(var(--u) * N)` and therefore grow with `--ui-scale`. The frontend
/// measures the real height on the first resize, but the native reset paths run
/// *before* that measurement lands, so they must scale the baseline themselves
/// or the window would open at the old step's height and be corrected a frame
/// later (a visible jump, and the exact blank-space bug the reset exists to
/// prevent).
///
/// The step is read from the settings file on demand rather than cached: a
/// reset can happen at any time and the user changes the step in settings, so
/// the file is the only current truth. The width is deliberately not scaled —
/// the launcher's 720px column is the window contract (R7-13a), and the
/// reference keeps its width fixed at every step too.
fn input_window_height() -> f64 {
    scaled_input_window_height(&load_settings().ui_scale)
}

/// The pure half of [`input_window_height`], split out so the arithmetic is a
/// unit test's subject rather than something only a running window shows:
/// `base × factor(step)`, with the width and every other dimension untouched.
fn scaled_input_window_height(step: &str) -> f64 {
    INPUT_WINDOW_HEIGHT * commands::config::ui_scale_factor(step)
}
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
    settings: MenuItem<Wry>,
    reload: MenuItem<Wry>,
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
    /// R55 · the custom global shortcut keys currently held with the OS. Kept
    /// apart from the settings file because the OS may refuse a key another app
    /// owns; only the keys actually registered are tracked here.
    custom_shortcuts: Mutex<Vec<String>>,
    /// A plugin page requested by the launch arguments (`floter clip` on a
    /// cold start), consumed once by the frontend once its listeners are up.
    pending_plugin_open: Mutex<Option<String>>,
    /// A manifest-connect request that arrived over the `floter://` scheme.
    /// Stored as well as emitted: a cold start dispatches before the webview
    /// has mounted its listeners, and the frontend consumes the slot once it
    /// is ready (the same contract `pending_plugin_open` uses).
    pending_deep_link: Mutex<Option<deep_link::ConnectRequest>>,
    /// A `floter://register` request that arrived over the scheme. Same
    /// cold-start contract as `pending_deep_link`, with its own cell because
    /// the payload shape differs (a resolved candidate, not a manifest path).
    pending_deep_link_register: Mutex<Option<deep_link::RegisterRequest>>,
    /// R91 · one parked request per detached window label (it was a single
    /// `Option` through R84-R90). A detach parks the request under the label it
    /// belongs to; the window's own page pulls it once and marks it delivered.
    pending_plugin_window_requests: Mutex<HashMap<String, plugin_windows::PendingSlot>>,
    /// R91 · the content key each live detached label is showing, so the
    /// geometry watcher can persist a move under the right key. Written when a
    /// window is (re)parked. R91 left the entry behind on close (the map only
    /// grew, and a reused label was overwritten before it was read again);
    /// R99 retires that tolerance — a native close drops the label's entry
    /// through `AppState::forget_plugin_window`, so a long session of
    /// pin/close cycles cannot leak one pair per window.
    plugin_window_keys: Mutex<HashMap<String, String>>,
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

    /// R91 · the content key a detached label is currently showing, for the
    /// geometry watcher. `None` while the label has no entry (already closed
    /// and its slot removed before the map's tolerated residue is read).
    fn plugin_window_key(&self, label: &str) -> Option<String> {
        self.plugin_window_keys
            .lock()
            .ok()
            .and_then(|keys| keys.get(label).cloned())
    }

    /// R99 · drop everything a detached label held in memory once its window
    /// is gone: the content key the geometry watcher would resolve (R91) and
    /// the parked request slot the page pulls from (R91's per-label slots).
    /// Called from the `Destroyed` arm of the window watcher, so a native
    /// close (title-bar X, system close) leaves neither behind and a long
    /// session of pin/close cycles cannot leak one pair per window.
    ///
    /// The remembered geometry **file** is deliberately not touched: R85's
    /// contract is that pinning the same content again returns to where the
    /// user last left that window, so the on-disk store has to outlive the
    /// window.
    ///
    /// Idempotent: a label with no entry — one already forgotten, or one this
    /// state never saw — is a no-op rather than a panic, because the event
    /// handler cannot know how many times it will be called. A poisoned lock
    /// is skipped, the same way the rest of this file treats one.
    fn forget_plugin_window(&self, label: &str) {
        if let Ok(mut keys) = self.plugin_window_keys.lock() {
            keys.remove(label);
        }
        if let Ok(mut slots) = self.pending_plugin_window_requests.lock() {
            slots.remove(label);
        }
    }
}

fn tray_labels(language: &str) -> (&'static str, &'static str, &'static str, &'static str) {
    match language {
        "zh" => ("显示 floter", "设置…", "重新扫描", "退出"),
        _ => ("Show floter", "Settings…", "Reload", "Quit"),
    }
}

/// Retitle the tray menu in the given language. Called on startup and whenever
/// the language setting changes, so the tray never lags behind the UI.
pub fn apply_tray_language(app: &AppHandle, language: &str) {
    let (show, settings, reload, quit) = tray_labels(language);
    let state = app.state::<AppState>();
    let Ok(items) = state.tray_items.lock() else {
        return;
    };
    if let Some(items) = items.as_ref() {
        let _ = items.show.set_text(show);
        let _ = items.settings.set_text(settings);
        let _ = items.reload.set_text(reload);
        let _ = items.quit.set_text(quit);
    }
}

/// R7-10c: show or hide the macOS menu bar status item / Windows+Linux tray
/// icon from the persisted setting.
///
/// Deliberately kept apart from [`apply_tray_language`]: that one only ever
/// retitles the menu, so a language change can never reset visibility. This
/// one only ever calls `set_visible`, and is idempotent, so the settings-save
/// path can call it on every write without tracking a previous value.
///
/// Hiding the icon removes exactly one summon path. The global toggle hotkey
/// (`register_toggle_shortcut`), the deep-link router, the Linux IPC socket and
/// the settings page (reachable from the panel the hotkey opens) are all
/// independent, so the app stays reachable with the icon gone.
pub fn apply_tray_visibility(app: &AppHandle, show_icon: bool) {
    if let Some(tray) = app.tray_by_id(tray_identity::TRAY_ICON_ID) {
        let _ = tray.set_visible(desired_tray_visibility(show_icon));
    }
}

/// The whole of the visibility contract, as a value: the status item is on
/// exactly when the setting is on. Split out of [`apply_tray_visibility`] so
/// the contract is a unit test rather than something only observable through a
/// live `AppHandle` — an inverted application (`set_visible(!setting)`) is the
/// mistake this round is guarding against, and it fails here.
pub fn desired_tray_visibility(show_icon: bool) -> bool {
    show_icon
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
    tracing::debug!(
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

/// Lock or unlock user resizing, with the Wayland caveat handled.
///
/// Everywhere else, `set_resizable(false)` only stops the *user* from dragging
/// the edges; programmatic `set_size` still goes through — X11 honours
/// geometry changes regardless of the size hints, and Windows and macOS keep
/// the two paths apart the same way. Wayland cannot make that distinction:
/// GTK implements a non-resizable window by pinning its min and max size
/// hints to the current size, and the compositor clamps EVERY resize — the
/// panel's own included — into that box. The launcher is sized from the
/// webview on every keystroke and the settings and plugin panels resize
/// themselves when they open, so a window locked on Wayland is a window whose
/// height never follows its content again: the result list grows underneath a
/// fixed surface, every page switch keeps the previous page's height, and the
/// leftover transparent surface keeps answering clicks the user aimed at the
/// desktop beside the card — which is why click-outside-to-hide appeared
/// broken there too.
///
/// So on Wayland a lock request is answered with "stay unlocked". Nothing is
/// given up: an undecorated window has no frame the compositor lets the user
/// grab, so the flag buys no user-resizability there in the first place.
/// Callers that need the window re-locked once their geometry has landed (the
/// plugin page) do it themselves, explicitly.
fn set_panel_resizable(window: &WebviewWindow, resizable: bool) -> Result<(), String> {
    #[cfg(target_os = "linux")]
    if on_wayland() {
        return window
            .set_resizable(true)
            .map_err(|error| error.to_string());
    }
    window
        .set_resizable(resizable)
        .map_err(|error| error.to_string())
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

    tracing::debug!(
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

/// Put the `WKWebView` — not the parent content view that tauri-nspanel's
/// `show_and_make_key` installs — at the head of the panel's responder chain,
/// which is what WebKit needs before it will draw a caret or route keys into
/// the focused editable element. Reuses wry's own `makeFirstResponder` path
/// through `Webview::set_focus`; no direct objc2 dependency, so it cannot drift
/// from whatever handle wry actually installed the web view with. Safe to call
/// on every reveal: if the web view is already responder this is a no-op, and
/// the dispatch is synchronous because the caller runs on the main thread.
#[cfg(target_os = "macos")]
fn arm_macos_webview_responder(window: &WebviewWindow) -> Result<(), String> {
    // Fully qualified on purpose: `Webview` is deliberately not in the shared
    // import block, because importing it there would be an unused-import
    // warning on every non-macOS build (and this fn is the only macOS user).
    let webview: &tauri::Webview<Wry> = window.as_ref();
    webview.set_focus().map_err(|error| error.to_string())
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
    // `show_and_make_key` (just above) leaves the CONTENT VIEW as first
    // responder; the caret needs the web view itself, so hand the web view the
    // keyboard before returning.
    let _ = arm_macos_webview_responder(window);

    // A panel summoned before the accessory app has ever activated can lose the
    // first key request. Tinycast reasserts it on the next main-loop turn too.
    let label = window.label().to_string();
    let handle = window.app_handle().clone();
    let retry_handle = handle.clone();
    let retry_window = window.clone();
    let _ = handle.run_on_main_thread(move || {
        if let Ok(panel) = retry_handle.get_webview_panel(&label) {
            if panel.is_visible() && !panel.as_panel().isKeyWindow() {
                panel.show_and_make_key();
                let _ = arm_macos_webview_responder(&retry_window);
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

/// How long a resize burst must be quiet before the shadow is recomputed.
///
/// R67 · longer than one frame of the launcher's height walk (~16ms) and shorter
/// than the eye's patience, so a walk draws its shadow once at the end.
#[cfg(target_os = "macos")]
const SHADOW_SETTLE_MS: u64 = 80;

/// The resize generation the pending shadow refresh belongs to. Every
/// [`refresh_macos_shadow_debounced`] bumps it, and a refresh only runs when its
/// own generation is still current after the quiet window.
#[cfg(target_os = "macos")]
static SHADOW_GENERATION: AtomicU64 = AtomicU64::new(0);

/// Recompute the panel's shadow once a burst of resizes has settled.
///
/// R67 · the user's report, verbatim: 「还是抖动」 while the launcher walked its
/// height. The walk resizes the window once per painted frame, and the resize
/// handler below used to call `invalidateShadow` on every one of those events —
/// the window server recomputed (and redrew) the shadow nine times in 150ms, and
/// a shadow that is recomputed from a shape that is still moving is the flicker
/// the user read as the list moving. The generation check collapses the burst:
/// only the last resize of a walk reaches [`refresh_macos_shadow`].
#[cfg(target_os = "macos")]
fn refresh_macos_shadow_debounced(window: &WebviewWindow) {
    let generation = SHADOW_GENERATION.fetch_add(1, Ordering::SeqCst) + 1;
    let window = window.clone();
    tauri::async_runtime::spawn(async move {
        tokio::time::sleep(std::time::Duration::from_millis(SHADOW_SETTLE_MS)).await;
        if SHADOW_GENERATION.load(Ordering::SeqCst) == generation {
            refresh_macos_shadow(&window);
        }
    });
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
        // R41 · Hyprland tiles a new toplevel; ask the compositor to float the
        // window we just focused, before the geometry calls that follow. No-op
        // on every other compositor. See `hyprland`.
        #[cfg(target_os = "linux")]
        hyprland::ensure_floating();
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
    let _ = set_panel_resizable(window, terminal);

    // Reveal first, position second: macOS' window server ignores geometry set
    // on an unmapped window, so a `set_position` made while hidden is discarded
    // and `show()` puts the panel back wherever it last was. Doing it in this
    // order costs nothing on the other platforms — both calls land in the same
    // event loop tick, so there is no visible jump.
    reveal_window(window)?;
    if !terminal {
        // A collapsed reveal must never inherit whatever geometry the panel
        // died with: a height left over from a long result list rides every
        // later reveal as blank space below the input (and, when the reveal
        // event lands before the webview has mounted its listeners, nothing
        // ever corrects it afterwards). Re-home the window onto the launcher's
        // own baseline now that it is mapped and the geometry will stick; the
        // frontend then grows it to the content it measures.
        let _ = resize_window(window, INPUT_WINDOW_WIDTH, input_window_height(), false);
    }
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
    // A terminal-to-launcher transition resizes the native window before React
    // has necessarily removed its terminal resize listener. Never let that
    // compact-window event overwrite the user's terminal dimensions.
    if !state.terminal_mode.load(Ordering::SeqCst) {
        return Ok(());
    }
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

/// Re-arm the web view as the window's native first responder.
///
/// Caret rendering is a native-responder property, not a DOM one. WebKit only
/// blinks the caret (and routes key events into the focused editable element)
/// when the `WKWebView` is the `NSWindow`'s first responder. The macOS panel is
/// a non-activating `NSPanel` re-fronted by tauri-nspanel's `show_and_make_key`,
/// which calls `makeFirstResponder:` on the *content view* (the container wry
/// installs above the web view) and then `makeKeyWindow` — so the window is key
/// while the web view itself may not be the first responder. Once a sandboxed
/// plugin iframe has held the keyboard, the first responder sits on a view
/// *inside* that iframe, and a DOM `focus()` on the launcher input moves
/// `document.activeElement` without moving the native responder back: the input
/// accepts no keystrokes and shows no caret until a mouse click makes the web
/// view responder again. The frontend calls this on every return to the
/// collapsed surface, after its DOM focus, so the two views of focus agree.
///
/// `Webview::set_focus` is wry's `WKWebView` `makeFirstResponder:`
/// (`wry/src/wkwebview/mod.rs`), dispatched to the main thread by the runtime's
/// user-message machinery. On WebKitGTK and WebView2 the same method is
/// `grab_focus` / `MoveFocus` — harmless there, and unnecessary, because those
/// engines paint the caret straight from DOM focus. That is the whole platform
/// difference: on Linux focus() alone is enough, on macOS it is not.
#[tauri::command]
fn refocus_webview(window: WebviewWindow) -> Result<(), String> {
    // Bind the view before calling: `WebviewWindow` derefs to `Webview`, but the
    // explicit binding keeps the `AsRef` conversion (and its lifetime) obvious.
    let webview: &tauri::Webview<Wry> = window.as_ref();
    webview.set_focus().map_err(|error| error.to_string())
}

/// R84 · the detached plugin window. R90 · the request is a discriminated
/// union now, one arm per kind of thing a window can be pinned to. R91 · there
/// can be more than one window: the request's content identity decides whether
/// it replaces an open window or opens another (see `detach_plugin_window`).
///
/// The user's ask: 「插件页面可以独立固定在界面上而不自动消失……脱离原来的整个
/// 软件主体，不再跟随呼出和隐藏」— a real second window a plugin page can be
/// pinned into, which the launcher's summon/hide lifecycle never touches. R90
/// extended it to 「钉住单条内容」: one selected result's text becomes its own
/// window, no command to run.
///
/// Delivery uses the established pull-slot contract (`pending_plugin_open`,
/// `pending_deep_link`): the request is parked in `AppState` *before* the
/// window exists, the detached page pulls it on mount. A live window gets the
/// `plugin-detach-request` emit on top, so a second detach of the *same*
/// content replaces its run without rebuilding anything. The emit alone could
/// race the window's first listener; the slot cannot, and the slot is the
/// truth.
///
/// `external` is R84's arm, field for field unchanged; `text` is R90's. The
/// `kind` tag is the same discriminator the frontend's `DetachRequest` union
/// uses (`src/plugin-window/detach.ts`), and the two sides share the JSON
/// shape — a rename on one side is a silent deserialisation failure on the
/// other, which the round-trip tests pin.
#[derive(Debug, Clone, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(tag = "kind", rename_all = "camelCase")]
pub enum DetachRequest {
    #[serde(rename_all = "camelCase")]
    External {
        extension_id: String,
        command_id: String,
        command_label: String,
        args: Vec<String>,
    },
    #[serde(rename_all = "camelCase")]
    Text { title: String, text: String },
}

impl DetachRequest {
    /// A request is only as good as its routing truth. The external arm needs
    /// its command id and labels (empty ids and labels are refused here as
    /// well as in the frontend's `validateDetachRequest` — the two sides share
    /// the convention, neither trusts the other). The text arm needs only its
    /// title: the body may be the empty string, because a command that printed
    /// nothing is still content the user pinned.
    fn valid(&self) -> bool {
        match self {
            DetachRequest::External {
                extension_id,
                command_id,
                command_label,
                ..
            } => {
                !extension_id.trim().is_empty()
                    && !command_id.trim().is_empty()
                    && !command_label.trim().is_empty()
            }
            DetachRequest::Text { title, .. } => !title.trim().is_empty(),
        }
    }

    /// The window title this request wants, before the window exists. Both
    /// arms carry one: the external arm's command label, the text arm's own
    /// row title.
    fn window_title(&self) -> &str {
        match self {
            DetachRequest::External { command_label, .. } => command_label,
            DetachRequest::Text { title, .. } => title,
        }
    }
}

/// Park a request and open the detached window that should show it.
///
/// R91 · the single-window assumption is retired: a request whose content is
/// **already** shown by an open window replaces that window's content (the
/// R84-R90 behaviour, kept for the case it was written for), while a request
/// for *different* content opens another window. The new window's label is the
/// lowest free instance label
/// ([`plugin_windows::next_plugin_window_label`]); there is no instance
/// ceiling — the user's desktop is theirs to manage, taskbar entries and all.
///
/// The request is parked *before* the window exists (the pull-slot contract
/// `pending_plugin_open` and `pending_deep_link` use); a live window also gets
/// the `plugin-detach-request` emit, because the emit alone could race its
/// first listener. The slot is the truth.
#[tauri::command]
fn detach_plugin_window(app: AppHandle, request: DetachRequest) -> Result<(), String> {
    if !request.valid() {
        return Err("detach request: missing its title or routing truth".into());
    }
    let state = app.state::<AppState>();
    let key = plugin_windows::content_key(&request);
    let live = app.webview_windows();

    // Same content already pinned → reuse its window: overwrite the request,
    // park it undelivered, nudge the page and raise the window.
    let same_content = state
        .pending_plugin_window_requests
        .lock()
        .ok()
        .and_then(|slots| {
            slots
                .iter()
                .find(|(label, slot)| {
                    plugin_windows::content_key(&slot.request) == key
                        && live.contains_key(label.as_str())
                })
                .map(|(label, _)| label.clone())
        });
    if let Some(label) = same_content {
        if let Ok(mut slots) = state.pending_plugin_window_requests.lock() {
            slots.insert(
                label.clone(),
                plugin_windows::PendingSlot {
                    request,
                    delivered: false,
                },
            );
        }
        if let Some(existing) = app.get_webview_window(&label) {
            let _ = existing.emit("plugin-detach-request", ());
            let _ = existing.set_focus();
        }
        return Ok(());
    }

    // Different content → a new window at the lowest free label.
    let open_labels: Vec<String> = live.keys().cloned().collect();
    let label = plugin_windows::next_plugin_window_label(&open_labels);
    if let Ok(mut slots) = state.pending_plugin_window_requests.lock() {
        slots.insert(
            label.clone(),
            plugin_windows::PendingSlot {
                request: request.clone(),
                delivered: false,
            },
        );
    }
    if let Ok(mut keys) = state.plugin_window_keys.lock() {
        keys.insert(label.clone(), key.clone());
    }
    let title = request.window_title().to_string();
    // R85 · open where the user left *this content*. The stored geometry is
    // resolved against the displays that exist *now*: a size the current
    // primary can hold, and a position only if it still lands on a screen (a
    // saved spot on an unplugged monitor falls back to the system's default
    // placement). R91 · the lookup is keyed by content, so two windows no
    // longer fight over one remembered rectangle.
    let placement = plugin_window_geometry::resolve_placement(
        plugin_window_geometry::load_geometry_for(&key),
        primary_display_area(&app),
        &display_areas(&app),
    );
    let mut builder = tauri::WebviewWindowBuilder::new(
        &app,
        label.as_str(),
        tauri::WebviewUrl::App("index.html".into()),
    )
    .title(title)
    .inner_size(placement.width, placement.height)
    .min_inner_size(
        plugin_window_geometry::MIN_WIDTH,
        plugin_window_geometry::MIN_HEIGHT,
    )
    .resizable(true)
    .skip_taskbar(false);
    if let Some((x, y)) = placement.position {
        builder = builder.position(x, y);
    }
    let window = builder.build().map_err(|error| {
        // R148 · a build that never produced a window must not leave the
        // parked pair behind. Both maps are keyed by the freshly allocated
        // label, so removing exactly that label undoes this call and leaves a
        // sibling window's entry alone (a map-wide `clear` would not).
        rollback_pending_plugin_window(&state, &label);
        error.to_string()
    })?;
    watch_plugin_window_geometry(&app, &window);
    Ok(())
}

/// R148 · undo the slot/key pair [`detach_plugin_window`] parked before its
/// `builder.build()`: the failure branch calls this with the label it had just
/// allocated. The two maps are taken one at a time and no IO runs under either
/// guard. An already-gone label is a silent no-op, matching
/// [`AppState::forget_plugin_window`]'s idempotence — the event handler and a
/// failed build can both want the same label gone.
fn rollback_pending_plugin_window(state: &AppState, label: &str) {
    if let Ok(mut slots) = state.pending_plugin_window_requests.lock() {
        slots.remove(label);
    }
    if let Ok(mut keys) = state.plugin_window_keys.lock() {
        keys.remove(label);
    }
}

/// Every monitor, in the logical space [`plugin_window_geometry`] works in.
fn display_areas(app: &AppHandle) -> Vec<plugin_window_geometry::DisplayArea> {
    app.available_monitors()
        .unwrap_or_default()
        .iter()
        .map(display_area)
        .collect()
}

/// The primary monitor's bounds, or `None` when the platform will not name
/// one — in which case the size clamp keeps its floor and skips the ceiling.
fn primary_display_area(app: &AppHandle) -> Option<plugin_window_geometry::DisplayArea> {
    app.primary_monitor()
        .ok()
        .flatten()
        .as_ref()
        .map(display_area)
}

/// A monitor's bounds converted from the physical pixels Tauri reports to the
/// logical points a window geometry is quoted in. A monitor with a scale
/// factor of zero (a broken reading) is treated as 1:1 rather than dividing by
/// it.
fn display_area(monitor: &tauri::Monitor) -> plugin_window_geometry::DisplayArea {
    let scale = monitor.scale_factor();
    let scale = if scale > 0.0 { scale } else { 1.0 };
    let position = monitor.position();
    let size = monitor.size();
    plugin_window_geometry::DisplayArea {
        x: f64::from(position.x) / scale,
        y: f64::from(position.y) / scale,
        width: f64::from(size.width) / scale,
        height: f64::from(size.height) / scale,
    }
}

/// Follow the detached window's own moves and resizes and record each one for
/// persistence. Attached to the window rather than the page on purpose: the
/// user can drag the native frame while the webview is busy or gone, and the
/// OS keeps delivering these events either way. R91 · the geometry is stored
/// per content key, so the watcher resolves its label's current key at event
/// time (`plugin_window_keys`). R99 · the same handler also learns when the
/// window is destroyed, which is the one moment the label's in-memory entries
/// can be dropped without racing a reopen.
fn watch_plugin_window_geometry(app: &AppHandle, window: &WebviewWindow) {
    let watched = window.clone();
    let state_app = app.clone();
    let label = window.label().to_string();
    window.on_window_event(move |event| match event {
        tauri::WindowEvent::Moved(_) | tauri::WindowEvent::Resized(_) => {
            if let Some(geometry) = current_geometry(&watched) {
                if let Some(key) = state_app.state::<AppState>().plugin_window_key(&label) {
                    plugin_window_geometry::note_geometry(key, geometry);
                }
            }
        }
        // R99 · the window is gone for good (title-bar X / system close), so
        // its key and parked slot are dropped. The geometry file stays (R85).
        // The main window never goes through this function, so it is untouched.
        tauri::WindowEvent::Destroyed => {
            state_app.state::<AppState>().forget_plugin_window(&label);
        }
        _ => {}
    });
}

/// The window's placement right now, in logical coordinates. `None` when the
/// platform will not report it — nothing to save is better than saving a
/// wrong value.
fn current_geometry(window: &WebviewWindow) -> Option<plugin_window_geometry::WindowGeometry> {
    let scale = window.scale_factor().ok()?;
    if scale <= 0.0 {
        return None;
    }
    let position = window.outer_position().ok()?;
    let size = window.inner_size().ok()?;
    Some(plugin_window_geometry::WindowGeometry {
        width: f64::from(size.width) / scale,
        height: f64::from(size.height) / scale,
        x: f64::from(position.x) / scale,
        y: f64::from(position.y) / scale,
    })
}

/// The detached page pulls its run request here, naming itself by label (on
/// mount, and again whenever a `plugin-detach-request` emit nudges it).
/// Read-once per label: the slot's `delivered` flag flips on the first pull, so
/// a remount never re-runs a command the user already consumed.
#[tauri::command]
fn take_plugin_window_request(
    state: tauri::State<'_, AppState>,
    label: String,
) -> Result<Option<DetachRequest>, String> {
    let mut slots = state
        .pending_plugin_window_requests
        .lock()
        .map_err(|error| error.to_string())?;
    Ok(slots
        .get_mut(&label)
        .and_then(plugin_windows::PendingSlot::take))
}

/// Close one detached window and drop its parked request. Called by the
/// window's own close button, which names itself by label; the launcher's
/// lifecycle never calls this. The label → key map entry is deliberately left
/// behind (see `AppState::plugin_window_keys`).
#[tauri::command]
fn close_plugin_window(app: AppHandle, label: String) -> Result<(), String> {
    if let Some(window) = app.get_webview_window(&label) {
        let _ = window.close();
    }
    if let Ok(mut slots) = app
        .state::<AppState>()
        .pending_plugin_window_requests
        .lock()
    {
        slots.remove(&label);
    }
    Ok(())
}

#[tauri::command]
fn show_input(window: WebviewWindow, state: tauri::State<'_, AppState>) -> Result<(), String> {
    let preserve_anchor = state.window_visible.load(Ordering::SeqCst);
    // Flip the mode before resizing so an in-flight frontend resize callback
    // cannot persist the compact launcher dimensions as the terminal size.
    state.terminal_mode.store(false, Ordering::SeqCst);
    // Deliberately left unlocked on Wayland: the frontend keeps calling
    // `setSize` on this window as the result list grows and shrinks, and a
    // Wayland lock would clamp every one of those calls to the height the
    // window happens to carry right now (see [`set_panel_resizable`]).
    set_panel_resizable(&window, false)?;
    resize_window(
        &window,
        INPUT_WINDOW_WIDTH,
        input_window_height(),
        preserve_anchor,
    )?;
    reveal_window(&window)?;
    if !preserve_anchor {
        let _ = move_to_default_position(&window, INPUT_WINDOW_WIDTH, &state);
    }
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

/// R55 · claim one user-defined global shortcut. On press the action string is
/// delivered to the main window, which interprets it (silent command run or a
/// plugin mode open). Any OS error (nearly always "already owned by another
/// application") is returned so the settings page can name the row.
pub fn register_custom_shortcut(app: &AppHandle, key: &str, action: &str) -> Result<(), String> {
    let handle = app.clone();
    let action = action.to_string();
    app.global_shortcut()
        .on_shortcut(key, move |_app, _shortcut, event| {
            if event.state == ShortcutState::Pressed {
                // The main window owns the launcher and the command surface.
                let _ = handle.emit_to("main", "custom-shortcut://trigger", action.clone());
            }
        })
        .map_err(|error| error.to_string())
}

/// R55 · release every custom shortcut currently held with the OS.
pub fn unregister_custom_shortcuts(app: &AppHandle) {
    let keys = app
        .state::<AppState>()
        .custom_shortcuts
        .lock()
        .map(|keys| keys.clone())
        .unwrap_or_default();
    for key in keys {
        let _ = app.global_shortcut().unregister(key.as_str());
    }
    set_registered_custom_shortcuts(app, &[]);
}

/// R55 · record which custom keys the OS actually accepted.
pub fn set_registered_custom_shortcuts(
    app: &AppHandle,
    entries: &[commands::config::CustomShortcut],
) {
    if let Ok(mut keys) = app.state::<AppState>().custom_shortcuts.lock() {
        *keys = entries.iter().map(|entry| entry.key.clone()).collect();
    }
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
    tracing::warn!("{reason}");
    tracing::warn!("Bind 'floter --toggle' as a custom shortcut in your compositor settings.");
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
#[allow(deprecated)]
pub fn run() {
    // Initialize tracing subscriber so users can set RUST_LOG=warn in production.
    // Without this, all eprintln! output floods macOS Console.app.
    let _ = tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .with_target(false)
        .with_level(true)
        .try_init();

    tracing::info!("floter starting up");

    // Before the builder, and therefore before anything initializes GTK: this
    // is the last point at which the renderer WebKitGTK will use can still be
    // chosen. See `linux_render` for why that choice cannot be made later.
    #[cfg(target_os = "linux")]
    linux_render::prepare(std::env::args_os());

    // The deep-link plugin must be registered before single-instance so that a
    // forwarded second-instance argument can be routed through the same
    // `floter://` parser the OS-delivered links use.
    let builder = tauri::Builder::default()
        .plugin(tauri_plugin_deep_link::init())
        .plugin(tauri_plugin_single_instance::init(
            |app, arguments, _working_directory| {
                if arguments.iter().any(|argument| argument == "--background") {
                    return;
                }
                // `floter clip` against a running instance opens the clipboard
                // page in place of the plain reveal.
                let wants_clip = arguments.iter().any(|argument| argument == "clip");
                // Any other external trigger is normalized to its `floter://`
                // URL and routed by the one parser (see `deep_link`). The CLI
                // spelling and the OS-delivered link therefore cannot diverge.
                let link = if wants_clip {
                    None
                } else {
                    deep_link::canonical_argument(&arguments)
                };
                let handle = app.clone();
                let _ = app.run_on_main_thread(move || {
                    if wants_clip {
                        plugin_pages::open_plugin_page(&handle, plugin_pages::CLIPBOARD_PLUGIN_ID);
                        return;
                    }
                    if let Some(url) = link {
                        deep_link::dispatch_url(&handle, &url, deep_link::Delivery::Live);
                        return;
                    }
                    if let Some(window) = handle.get_webview_window("main") {
                        let state = handle.state::<AppState>();
                        let _ = reveal_saved_mode(&window, &state);
                    }
                });
            },
        ));
    #[cfg(target_os = "macos")]
    let builder = builder.plugin(tauri_nspanel::init());

    let mut builder = builder
        .plugin(tauri_plugin_global_shortcut::Builder::new().build())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_dialog::init())
        // R7-10b: system notifications for background completions only. The
        // plugin is registered for its Rust API (`NotificationExt`); the
        // frontend never calls it, so the JS guest package is not a dependency.
        .plugin(tauri_plugin_notification::init())
        .manage(ApplicationState::new())
        .manage(TerminalState(Arc::new(Mutex::new(TerminalManager::new()))))
        .manage(AppState {
            window_visible: AtomicBool::new(false),
            terminal_mode: AtomicBool::new(false),
            // R123 · the constructor no longer reads the settings file. The
            // centering target is filled from the one startup read inside
            // `setup` below; until then it holds the same fallback the getter
            // uses for an unreadable slot, so a read that somehow never lands
            // degrades to the documented 600pt instead of a 0pt anchor.
            terminal_height: Mutex::new(TERMINAL_WINDOW_HEIGHT),
            tray_items: Mutex::new(None),
            toggle_shortcut: Mutex::new(String::new()),
            custom_shortcuts: Mutex::new(Vec::new()),
            pending_plugin_open: Mutex::new(None),
            pending_deep_link: Mutex::new(None),
            pending_deep_link_register: Mutex::new(None),
            pending_plugin_window_requests: Mutex::new(HashMap::new()),
            plugin_window_keys: Mutex::new(HashMap::new()),
            last_monitor: Mutex::new(None),
        });
    #[cfg(feature = "clipboard-history")]
    {
        builder = builder.manage(clipboard_history::ClipboardState::default());
    }
    builder.setup(|app| {
            // `floter clip` as the very first launch must land on the clipboard
            // page instead of the plain launcher. Stored for the frontend to
            // pick up — its event listeners may not exist yet this early.
            if std::env::args().skip(1).any(|argument| argument == "clip") {
                if let Ok(mut slot) = app.state::<AppState>().pending_plugin_open.lock() {
                    *slot = Some(plugin_pages::CLIPBOARD_PLUGIN_ID.to_string());
                }
            }
            // A cold start triggered by the scheme (or by its CLI spelling)
            // runs the very same router a live link does; the resolved request
            // is stored because the webview's listeners do not exist yet.
            //
            // The terminal spelling of `register` takes the terminal router, so
            // a cold `floter register rg` may still bind when the user already
            // said yes. Every other trigger — including a hand-typed
            // `floter://register…` URL — takes the link router and stops at the
            // offer.
            //
            // Order matters: `connect` reads `ExtensionState` (to stage a
            // remote manifest and to read the cache directory), so the state
            // has to be managed before the router runs. `open` needs nothing
            // but the window.
            let arguments = std::env::args().collect::<Vec<_>>();
            let cold_start = deep_link::canonical_argument(&arguments);
            let cold_start_is_terminal = deep_link::wants_register(&arguments);
            let extension_state = ExtensionState::new().map_err(std::io::Error::other)?;
            let _ = extension_state.app_handle.set(app.app_handle().clone());
            // Durable binding catch-up belongs to startup, not to any list poll:
            // `extensions_list` only reports a fingerprint divergence so a
            // rebuilt tool keeps rendering Connected; this pass is what writes
            // the refreshed fingerprint (and any state transition) back to
            // `tool-lock.json`. Idempotent, so an unchanged install writes
            // nothing; a failure is logged rather than aborting the launch.
            if let Err(error) = extension_state.reconcile_tool_bindings() {
                tracing::warn!("failed to reconcile tool bindings at startup: {error}");
            }
            app.manage(extension_state);
            if let Some(url) = cold_start {
                if cold_start_is_terminal {
                    deep_link::dispatch_terminal_url(
                        app.handle(),
                        &url,
                        deep_link::Delivery::ColdStart,
                    );
                } else {
                    deep_link::dispatch_url(app.handle(), &url, deep_link::Delivery::ColdStart);
                }
            }
            deep_link::register_scheme(app.handle());
            deep_link::listen_for_url_events(app.handle());
            // floter is tray-resident, and the non-activating NSPanel must not
            // promote the process or switch away from another app's fullscreen
            // Space when it takes key focus.
            #[cfg(target_os = "macos")]
            app.set_activation_policy(ActivationPolicy::Accessory);

            let settings = load_settings();
            // R123 · the startup path reads the file exactly once, so the
            // terminal height the collapsed panel centers against is installed
            // from the same value `saved_terminal_size()` used to compute at
            // build time — same normalization, placed after the read instead
            // of before it. Nothing reads this slot before `setup` returns:
            // every `reveal_saved_mode` / `default_position` caller is a
            // command, a tray/menu event, or a deep link, and all of those run
            // on the event loop `setup` precedes.
            if let Ok(mut current) = app.state::<AppState>().terminal_height.lock() {
                *current =
                    normalize_terminal_size(settings.terminal_width, settings.terminal_height).1;
            }
            if let Err(error) = ensure_launch_at_startup(settings.launch_at_startup) {
                tracing::error!("failed to reconcile launch-at-startup registration: {error}");
            }
            let (show_label, settings_label, reload_label, quit_label) =
                tray_labels(&settings.language);
            let show_item = MenuItem::with_id(app, "show", show_label, true, None::<&str>)?;
            let settings_item =
                MenuItem::with_id(app, "settings", settings_label, true, None::<&str>)?;
            let reload_item =
                MenuItem::with_id(app, "reload", reload_label, true, None::<&str>)?;
            #[cfg(target_os = "macos")]
            let quit_item = MenuItem::with_id(app, "quit", quit_label, true, Some("Cmd+Q"))?;
            #[cfg(not(target_os = "macos"))]
            let quit_item = MenuItem::with_id(app, "quit", quit_label, true, Some("Ctrl+Q"))?;
            let tray_menu = Menu::with_items(
                app,
                &[&show_item, &settings_item, &reload_item, &quit_item],
            )?;
            if let Ok(mut items) = app.state::<AppState>().tray_items.lock() {
                *items = Some(TrayMenuItems {
                    show: show_item.clone(),
                    settings: settings_item.clone(),
                    reload: reload_item.clone(),
                    quit: quit_item.clone(),
                });
            }
            let tray_icon = app
                .default_window_icon()
                .cloned()
                .ok_or("missing default window icon")?;
            TrayIconBuilder::with_id(tray_identity::TRAY_ICON_ID)
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
                    "settings" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let state = app.state::<AppState>();
                            let _ = reveal_saved_mode(&window, &state);
                        }
                        let _ = app.emit("floter://open-settings", ());
                    }
                    "reload" => {
                        if let Some(window) = app.get_webview_window("main") {
                            let state = app.state::<AppState>();
                            let _ = reveal_saved_mode(&window, &state);
                        }
                        let _ = app.emit("floter://reload-apps", ());
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

            // R7-10c: the setting decides the icon's visibility from the
            // first frame. The tray still exists as a resource when hidden,
            // which is what makes the switch reversible without a restart.
            // (The tray handle is looked up by id — the builder's return value
            // is owned here, but the registry keeps it addressable.)
            apply_tray_visibility(app.handle(), settings.show_menubar_icon);

            // R45: the tray has just written its icon under the id above; read
            // that back so the identity we registered is in the log (and a
            // silent non-registration is a warning rather than a mystery).
            #[cfg(target_os = "linux")]
            tray_identity::verify_registration();

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
            set_panel_resizable(&window, false)?;
            let shadow_window = window.clone();
            window.on_window_event(move |event| {
                match event {
                    tauri::WindowEvent::CloseRequested { api, .. } => api.prevent_close(),
                    // Every resize the panel goes through — the frontend sizing
                    // the launcher to its rows, an edge drag on the terminal,
                    // the settings panel opening. R67 · debounced, because the
                    // launcher's height walk resizes once per painted frame and
                    // a shadow recomputed per frame is the flicker the user saw.
                    #[cfg(target_os = "macos")]
                    tauri::WindowEvent::Resized(_) => refresh_macos_shadow_debounced(&shadow_window),
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
                tracing::warn!("failed to register global shortcut {shortcut}: {error}");
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
                tracing::info!("registered global shortcut: {shortcut}");
                // The grab was accepted by Xwayland, which is not the same as it
                // ever being delivered: the compositor keeps the key to itself.
                #[cfg(target_os = "linux")]
                if on_wayland() {
                    print_toggle_hint(
                        "Running under Wayland, where an X11 grab is accepted but never fires.",
                    );
                }
            }

            // Clipboard history monitor, only when enabled (the default).
            // R56 · there is no panel hotkey to register any more: the panel
            // is reached through launcher search and `floter clip`. Failures
            // inside are logged, never fatal.
            #[cfg(feature = "clipboard-history")]
            clipboard_history::initialize(app.handle(), settings.clipboard_history_enabled);

            // R55 · re-claim the user's custom global shortcuts. A key another
            // application has since taken is only logged here — the settings
            // page re-checks and reports it the next time it is opened; startup
            // must never block on one bad binding.
            let mut registered = Vec::new();
            for entry in commands::config::normalize_custom_shortcuts(&settings.custom_shortcuts) {
                match register_custom_shortcut(app.handle(), &entry.key, &entry.action) {
                    Ok(()) => registered.push(entry),
                    Err(error) => tracing::warn!(
                        "custom shortcut {} could not be registered at startup: {error}",
                        entry.key
                    ),
                }
            }
            set_registered_custom_shortcuts(app.handle(), &registered);

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            term_spawn,
            term_list_sessions,
            term_attach_existing,
            term_kill_session,
            term_input,
            term_resize,
            term_scroll,
            term_set_theme,
            term_set_cursor_style,
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
            resolve_dropped_files,
            term_close,
            get_settings,
            save_settings,
            set_launch_at_startup,
            app_version,
            get_shortcuts,
            reset_shortcuts,
            update_shortcut,
            set_custom_shortcuts,
            run_silent_command,
            suspend_shortcuts,
            resume_shortcuts,
            set_recording_flag,
            clipboard_write_text,
            clipboard_read_text,
            show_terminal,
            plugin_pages::builtin_plugins_list,
            plugin_pages::take_pending_plugin_page,
            deep_link::take_pending_deep_link,
            deep_link::take_pending_deep_link_register,
            hide_window,
            quit_app,
            show_input,
            detach_plugin_window,
            take_plugin_window_request,
            close_plugin_window,
            refocus_webview,
            start_drag,
            system_power,
            // R69 · the invoke row's detached spawn (bare argv, no shell).
            system_spawn_detached,
            extensions_list,
            extensions_export,
            extensions_import,
            extensions_install,
            extensions_create_custom,
            extensions_connect_tool,
            extensions_custom_get,
            extensions_custom_update,
            extensions_script_runtime_check,
            extensions_search_tools,
            extensions_tool_catalog,
            extensions_recommended_permissions,
            extensions_connect_recommended,
            extensions_reconnect_system,
            extensions_pick_local_manifest,
            extensions_pick_local_package,
            extensions_custom_export_script,
            extensions_local_manifest_review,
            extensions_uninstall,
            extensions_uninstall_componentized,
            extensions_enable,
            extensions_disable,
            extensions_repair,
            extensions_describe,
            extensions_diagnose,
            extensions_health,
            extensions_reprobe,
            extensions_reprobe_commands,
            extensions_launch,
            extensions_run,
            extensions_run_output,
            extensions_config_get,
            extensions_config_set,
            extensions_config_copy,
            extensions_config_export,
            catalog_search,
            catalog_complete,
            external_plugin_commands,
            external_plugin_run,
            extensions_cancel_operation,
            browser_data::browser_discover,
            browser_data::browser_default_profile,
            browser_data::browser_search_bookmarks,
            browser_data::browser_search_history,
            browser_data::browser_open_url,
            browser_data::tabs::browser_list_tabs,
            browser_data::tabs::browser_activate_tab,
            commands::config::browser_get_settings,
            commands::config::browser_set_settings,
            calculator_history::calculator_get_entries,
            calculator_history::calculator_add_entry,
            calculator_history::calculator_set_favorite,
            calculator_history::calculator_delete,
            calculator_history::calculator_clear_history,
            calculator_history::calculator_get_settings,
            calculator_history::calculator_set_settings,
            #[cfg(feature = "clipboard-history")]
            clipboard_history::clipboard_get_entries,
            #[cfg(feature = "clipboard-history")]
            clipboard_history::clipboard_set_favorite,
            #[cfg(feature = "clipboard-history")]
            clipboard_history::clipboard_delete,
            #[cfg(feature = "clipboard-history")]
            clipboard_history::clipboard_copy_entry,
            #[cfg(feature = "clipboard-history")]
            clipboard_history::clipboard_clear_history,
            #[cfg(feature = "clipboard-history")]
            clipboard_history::clipboard_thumbnail,
            #[cfg(feature = "clipboard-history")]
            clipboard_history::clipboard_get_settings,
            #[cfg(feature = "clipboard-history")]
            clipboard_history::clipboard_set_settings,
        ])
        .build(tauri::generate_context!())
        .expect("error while running tauri application")
        .run(|app, event| {
            match event {
                // Shut down sessions still owned by Floter. A broker session
                // handed to a system terminal has already left the manager and
                // stays alive.
                tauri::RunEvent::Exit => {
                    if let Ok(manager) = app.state::<TerminalState>().0.lock() {
                        manager.shutdown_all();
                    }
                    terminal::broker::shutdown_if_idle();
                    // The socket node outlives the process that made it, so the
                    // next start would have to reclaim it as stale.
                    #[cfg(target_os = "linux")]
                    ipc::cleanup();
                }
                // R7-10b: the notification-click callback.
                //
                // A click on one of the background-completion notifications
                // activates floter; for an Accessory app with no visible window
                // that is exactly `applicationShouldHandleReopen`, which Tauri
                // surfaces as `Reopen` (macOS only — see the module docs for why
                // the plugin's own `onAction` channel cannot be used on
                // desktop). The handler does one thing: run the app's existing
                // reveal path, the same one the tray, the global shortcut, the
                // deep-link router and the IPC socket use.
                #[cfg(target_os = "macos")]
                tauri::RunEvent::Reopen { .. } => notifications::reveal_after_activation(app),
                _ => {}
            }
        });
}

#[cfg(test)]
mod tray_visibility_tests {
    use super::{desired_tray_visibility, tray_labels};
    #[test]
    fn the_menu_bar_icon_follows_the_setting_exactly() {
        // R7-10c's red-line contract: the status item is visible iff the
        // setting is on. `apply_tray_visibility` is deliberately a one-liner
        // over this function, so an inverted application (`set_visible(!v)`)
        // is a failure here rather than something only a live menu bar shows.
        assert!(desired_tray_visibility(true));
        assert!(!desired_tray_visibility(false));
    }

    #[test]
    fn retitling_the_menu_never_mentions_the_icon_state() {
        // The other half of the round's red line: `apply_tray_language` may
        // only write labels. If a future edit made it call `set_visible`, the
        // language switch would resurrect a hidden icon. The labels it owns
        // are exactly the four menu items, in both languages.
        for language in ["en", "zh"] {
            let (show, settings, reload, quit) = tray_labels(language);
            assert!(!show.is_empty() && !settings.is_empty());
            assert!(!reload.is_empty() && !quit.is_empty());
        }
    }
}

#[cfg(test)]
mod interface_scale_height_tests {
    use super::{scaled_input_window_height, INPUT_WINDOW_HEIGHT, INPUT_WINDOW_WIDTH};

    /// R7-13c · the collapsed window's **fallback** height is a scale-1
    /// measurement (`INPUT_WINDOW_HEIGHT` counts the CSS input row and frame at
    /// `--ui-scale: 1`), and the native reset paths run before the frontend can
    /// measure the real card. So they must multiply the base by the step's
    /// factor or the window opens at the old step's height and is corrected a
    /// frame later — the exact blank-space bug the reset exists to prevent.
    ///
    /// The frontend goes the *other* way and re-measures (never multiplies),
    /// because the card it measures is already laid out at the step. These two
    /// tests are the two halves of that split.
    #[test]
    fn the_fallback_height_scales_with_the_interface_step() {
        assert_eq!(scaled_input_window_height("default"), INPUT_WINDOW_HEIGHT);
        assert_eq!(
            scaled_input_window_height("large"),
            INPUT_WINDOW_HEIGHT * 1.1
        );
        assert_eq!(
            scaled_input_window_height("small"),
            INPUT_WINDOW_HEIGHT * 0.9
        );
        assert_eq!(
            scaled_input_window_height("tiny"),
            INPUT_WINDOW_HEIGHT * 0.8
        );
        // R47 · a retired `larger` maps to its survivor (`large`, 1.1) rather
        // than to the generic fallback.
        assert_eq!(
            scaled_input_window_height("larger"),
            INPUT_WINDOW_HEIGHT * 1.1
        );
        // R47 · the shipped default is `small` (0.9), so an absent/unknown step
        // lands there — positive, never `0` (a collapsed window).
        for step in ["small", "unknown"] {
            assert!(scaled_input_window_height(step) > 0.0);
        }
    }

    /// The width is the window contract (R7-13a) and every interface step keeps
    /// it: the reference refuses to narrow the column too. A future edit that
    /// scaled the width would fail here.
    #[test]
    fn the_width_is_never_scaled_by_the_interface_step() {
        assert_eq!(INPUT_WINDOW_WIDTH, 720.0);
        // Guard against the shape rather than only the value: the width must
        // not appear in the scaled-height function's inputs.
        let source = include_str!("lib.rs");
        let function = source
            .split("fn scaled_input_window_height")
            .nth(1)
            .and_then(|rest| rest.split('}').next())
            .expect("scaled_input_window_height must exist");
        assert!(
            !function.contains("INPUT_WINDOW_WIDTH"),
            "the fallback *height* must not read the width constant"
        );
    }
}

#[cfg(test)]
mod detach_plugin_window_tests {
    use std::collections::HashMap;
    use std::sync::Mutex;

    use super::plugin_windows::PendingSlot;
    use super::{AppState, DetachRequest, PLUGIN_WINDOW_LABEL};

    /// R84's arm, built once. R90 · the tag aside, its fields are unchanged.
    fn external(command_id: &str) -> DetachRequest {
        DetachRequest::External {
            extension_id: "local.tool".into(),
            command_id: command_id.into(),
            command_label: "My Tool".into(),
            args: vec!["--flag".into()],
        }
    }

    /// R90's arm: one snapshot of text, no command.
    fn text(title: &str, body: &str) -> DetachRequest {
        DetachRequest::Text {
            title: title.into(),
            text: body.into(),
        }
    }

    /// R84/R90 · the same request the frontend's `validateDetachRequest`
    /// accepts is the one the backend's own gate lets through, and anything
    /// with an empty routing truth is refused on both sides. The text arm's
    /// body may be empty — a command that printed nothing is still content the
    /// user pinned — but its title may not be.
    #[test]
    fn a_request_without_its_routing_truth_is_refused() {
        assert!(external("run").valid());
        assert!(!external("").valid());
        assert!(!external("   ").valid());
        assert!(!DetachRequest::External {
            extension_id: String::new(),
            command_id: "run".into(),
            command_label: "My Tool".into(),
            args: vec![],
        }
        .valid());
        assert!(!DetachRequest::External {
            extension_id: "local.tool".into(),
            command_id: "run".into(),
            command_label: String::new(),
            args: vec![],
        }
        .valid());
        // R90 · the text arm's own gate.
        assert!(text("Note", "hello").valid());
        assert!(text("Note", "").valid());
        assert!(!text("", "hello").valid());
        assert!(!text("   ", "hello").valid());
    }

    /// R90 · the wire shape both sides share. The `kind` tag is the
    /// discriminator the frontend's `validateDetachRequest` reads; the fields
    /// keep the external arm's R84 camelCase names byte for byte (the tag
    /// aside) and the text arm's two keys. Round-tripped through the real
    /// serializer, so a missing tag, a wrong `kind`, or a renamed field fails
    /// here rather than at a window that opens onto nothing.
    #[test]
    fn both_arms_round_trip_through_the_wire_shape() {
        let external = DetachRequest::External {
            extension_id: "local.tool".into(),
            command_id: "run".into(),
            command_label: "My Tool".into(),
            args: vec!["--flag".into(), "value".into()],
        };
        let json = serde_json::to_value(&external).unwrap();
        assert_eq!(json["kind"], "external");
        assert_eq!(json["extensionId"], "local.tool");
        assert_eq!(json["commandId"], "run");
        assert_eq!(json["commandLabel"], "My Tool");
        assert_eq!(json["args"][0], "--flag");
        assert_eq!(json["args"][1], "value");
        assert_eq!(
            serde_json::from_value::<DetachRequest>(json).unwrap(),
            external
        );

        let text_arm = text("Note", "hello");
        let json = serde_json::to_value(&text_arm).unwrap();
        assert_eq!(json["kind"], "text");
        assert_eq!(json["title"], "Note");
        assert_eq!(json["text"], "hello");
        assert_eq!(
            serde_json::from_value::<DetachRequest>(json).unwrap(),
            text_arm
        );

        // The empty body is a legitimate arm and survives the wire.
        let empty = text("Empty", "");
        let json = serde_json::to_value(&empty).unwrap();
        assert_eq!(json["text"], "");
        assert_eq!(
            serde_json::from_value::<DetachRequest>(json).unwrap(),
            empty
        );

        // A payload with no tag — the pre-R90 shape — is not a request any
        // more, and neither is an unknown kind.
        assert!(serde_json::from_value::<DetachRequest>(serde_json::json!({
            "extensionId": "local.tool",
            "commandId": "run",
            "commandLabel": "My Tool",
            "args": [],
        }))
        .is_err());
        assert!(serde_json::from_value::<DetachRequest>(serde_json::json!({
            "kind": "nope",
            "title": "Note",
            "text": "hello",
        }))
        .is_err());
    }

    /// R90 · the window title both arms carry before the window exists: the
    /// external arm names its command, the text arm its own row.
    #[test]
    fn both_arms_name_their_window() {
        assert_eq!(external("run").window_title(), "My Tool");
        assert_eq!(text("Note", "hello").window_title(), "Note");
    }

    /// R84/R90 · the pull-slot contract; R91 · the slots are per label now. A
    /// parked request is handed over exactly once (a second read sees `None`),
    /// each label keeps its own request — the union included — and dropping a
    /// label's entry drops only that label's slot.
    #[test]
    fn the_pending_request_slots_hand_over_exactly_once_per_label() {
        let state = AppState {
            window_visible: std::sync::atomic::AtomicBool::new(false),
            terminal_mode: std::sync::atomic::AtomicBool::new(false),
            terminal_height: Mutex::new(0.0),
            tray_items: Mutex::new(None),
            toggle_shortcut: Mutex::new(String::new()),
            custom_shortcuts: Mutex::new(Vec::new()),
            pending_plugin_open: Mutex::new(None),
            pending_deep_link: Mutex::new(None),
            pending_deep_link_register: Mutex::new(None),
            pending_plugin_window_requests: Mutex::new(HashMap::new()),
            plugin_window_keys: Mutex::new(HashMap::new()),
            last_monitor: Mutex::new(None),
        };
        let first = external("run");
        let second = text("Note", "hello");
        {
            let mut slots = state.pending_plugin_window_requests.lock().unwrap();
            slots.insert(
                PLUGIN_WINDOW_LABEL.to_string(),
                PendingSlot {
                    request: first.clone(),
                    delivered: false,
                },
            );
            slots.insert(
                "plugin-detached-2".to_string(),
                PendingSlot {
                    request: second.clone(),
                    delivered: false,
                },
            );
        }
        {
            let mut slots = state.pending_plugin_window_requests.lock().unwrap();
            // Each label hands over its own request exactly once.
            let taken = slots.get_mut(PLUGIN_WINDOW_LABEL).unwrap().take();
            assert_eq!(taken.as_ref(), Some(&first));
            assert!(slots.get_mut(PLUGIN_WINDOW_LABEL).unwrap().take().is_none());
            let taken = slots.get_mut("plugin-detached-2").unwrap().take();
            assert_eq!(taken, Some(second));
            assert!(matches!(taken, Some(DetachRequest::Text { .. })));
            // Closing one label drops only that label's slot.
            slots.remove("plugin-detached-2");
            assert!(!slots.contains_key("plugin-detached-2"));
            assert!(slots.contains_key(PLUGIN_WINDOW_LABEL));
        }
    }

    /// R99 · an empty `AppState` for the cleanup tests, built exactly the way
    /// the pull-slot test above builds its own. The fields no cleanup test
    /// touches stay at the defaults the app's own constructor gives them.
    fn empty_state() -> AppState {
        AppState {
            window_visible: std::sync::atomic::AtomicBool::new(false),
            terminal_mode: std::sync::atomic::AtomicBool::new(false),
            terminal_height: Mutex::new(0.0),
            tray_items: Mutex::new(None),
            toggle_shortcut: Mutex::new(String::new()),
            custom_shortcuts: Mutex::new(Vec::new()),
            pending_plugin_open: Mutex::new(None),
            pending_deep_link: Mutex::new(None),
            pending_deep_link_register: Mutex::new(None),
            pending_plugin_window_requests: Mutex::new(HashMap::new()),
            plugin_window_keys: Mutex::new(HashMap::new()),
            last_monitor: Mutex::new(None),
        }
    }

    /// Park a label the way `detach_plugin_window` does: a content key for the
    /// geometry watcher and an undelivered slot for the page to pull.
    fn park_label(state: &AppState, label: &str, key: &str) {
        state
            .plugin_window_keys
            .lock()
            .unwrap()
            .insert(label.to_string(), key.to_string());
        state.pending_plugin_window_requests.lock().unwrap().insert(
            label.to_string(),
            PendingSlot {
                request: text("Note", "hello"),
                delivered: false,
            },
        );
    }

    /// R99 · a native close drops **both** in-memory traces of the label: the
    /// `plugin_window_keys` entry the geometry watcher resolves, and the
    /// `pending_plugin_window_requests` slot the page pulls from. Either one
    /// left behind leaks a pair per pin/close cycle over a long session.
    #[test]
    fn forgetting_a_label_drops_its_key_and_its_parked_slot() {
        let state = empty_state();
        park_label(&state, PLUGIN_WINDOW_LABEL, "local.tool\u{0}run");
        assert_eq!(
            state.plugin_window_key(PLUGIN_WINDOW_LABEL).as_deref(),
            Some("local.tool\u{0}run")
        );

        state.forget_plugin_window(PLUGIN_WINDOW_LABEL);

        assert!(state.plugin_window_keys.lock().unwrap().is_empty());
        assert!(state
            .pending_plugin_window_requests
            .lock()
            .unwrap()
            .is_empty());
    }

    /// R99 · the cleanup is idempotent. A window event handler cannot know how
    /// many times it will fire, so a label that is already gone (or was never
    /// parked) must be a silent no-op rather than a panic.
    #[test]
    fn forgetting_a_label_twice_is_a_no_op() {
        let state = empty_state();
        park_label(&state, PLUGIN_WINDOW_LABEL, "key");
        park_label(&state, "plugin-detached-2", "other-key");

        state.forget_plugin_window(PLUGIN_WINDOW_LABEL);
        state.forget_plugin_window(PLUGIN_WINDOW_LABEL);
        state.forget_plugin_window("plugin-detached-9");

        assert!(state.plugin_window_keys.lock().unwrap().len() == 1);
        assert!(state.pending_plugin_window_requests.lock().unwrap().len() == 1);
        assert_eq!(
            state.plugin_window_key("plugin-detached-2").as_deref(),
            Some("other-key")
        );
    }

    /// R99 · closing one window must not touch its siblings. The labels are
    /// independent instances (R91), so forgetting one is scoped to that label
    /// and nothing else — a map-wide `clear()` would pass a single-window test
    /// and silently break every other pinned window.
    #[test]
    fn forgetting_a_label_leaves_its_siblings_alone() {
        let state = empty_state();
        park_label(&state, PLUGIN_WINDOW_LABEL, "first");
        park_label(&state, "plugin-detached-2", "second");

        state.forget_plugin_window(PLUGIN_WINDOW_LABEL);

        assert_eq!(state.plugin_window_key(PLUGIN_WINDOW_LABEL), None);
        assert_eq!(
            state.plugin_window_key("plugin-detached-2").as_deref(),
            Some("second")
        );
        let slots = state.pending_plugin_window_requests.lock().unwrap();
        assert!(!slots.contains_key(PLUGIN_WINDOW_LABEL));
        assert!(slots.contains_key("plugin-detached-2"));
    }

    /// R148 · a failed `builder.build()` must roll back the slot/key pair the
    /// detach parked before it. The real failure needs a live Tauri runtime —
    /// no window can be built in a unit test — so this drives the exact
    /// rollback the failure branch calls, with the label the branch would pass.
    /// The pair goes, a sibling window's pair stays (the rollback is scoped to
    /// the label, not a map-wide clear), and a second call is a no-op.
    #[test]
    fn detach_failure_rolls_back_pending_slot() {
        let state = empty_state();
        park_label(&state, PLUGIN_WINDOW_LABEL, "local.tool\u{0}run");
        park_label(&state, "plugin-detached-2", "local.tool\u{0}other");

        super::rollback_pending_plugin_window(&state, "plugin-detached-2");

        assert!(!state
            .pending_plugin_window_requests
            .lock()
            .unwrap()
            .contains_key("plugin-detached-2"));
        assert!(!state
            .plugin_window_keys
            .lock()
            .unwrap()
            .contains_key("plugin-detached-2"));
        // The sibling is untouched.
        assert_eq!(
            state.plugin_window_key(PLUGIN_WINDOW_LABEL).as_deref(),
            Some("local.tool\u{0}run")
        );
        assert!(state
            .pending_plugin_window_requests
            .lock()
            .unwrap()
            .contains_key(PLUGIN_WINDOW_LABEL));
        // Idempotent: the event handler and a failed build can both want the
        // same label gone.
        super::rollback_pending_plugin_window(&state, "plugin-detached-2");
    }

    /// R99 · the on-disk geometry outlives the window. R85's contract is that
    /// pinning the same content again returns to where the user last left it,
    /// so the close path may drop the two in-memory entries and nothing else.
    ///
    /// The file is created at the exact path the geometry module resolves
    /// (`dirs::config_dir()/floter/…`), reached through the env var `dirs`
    /// reads — the same temp-dir isolation the geometry module's own tests get
    /// by injecting a path. A close path that deleted (or rewrote) the store
    /// fails here instead of only after a user's pinned window forgot where it
    /// was. Linux only: that is the platform where the XDG variable, and only
    /// it, moves `config_dir()`.
    #[cfg(target_os = "linux")]
    #[test]
    fn forgetting_a_label_leaves_the_geometry_file_alone() {
        // Mirrors `plugin_window_geometry::GEOMETRY_FILE`, which is private to
        // its module; a rename on one side is caught by the path no longer
        // existing, which fails the survival assertion below.
        const GEOMETRY_FILE_NAME: &str = "plugin-window-geometry.json";

        let dir = tempfile::tempdir().unwrap();
        let previous = std::env::var_os("XDG_CONFIG_HOME");
        std::env::set_var("XDG_CONFIG_HOME", dir.path());
        // Restore the variable on every exit path, panics included, so a
        // failing run cannot leak the temp dir into the rest of the suite.
        struct RestoreEnv(Option<std::ffi::OsString>);
        impl Drop for RestoreEnv {
            fn drop(&mut self) {
                match self.0.take() {
                    Some(value) => std::env::set_var("XDG_CONFIG_HOME", value),
                    None => std::env::remove_var("XDG_CONFIG_HOME"),
                }
            }
        }
        let _restore = RestoreEnv(previous);

        let config = dirs::config_dir().expect("a config dir");
        assert!(
            config.starts_with(dir.path()),
            "the test must resolve inside its own temp dir, got {config:?}"
        );
        let file = config.join("floter").join(GEOMETRY_FILE_NAME);
        std::fs::create_dir_all(file.parent().unwrap()).unwrap();
        let content = "{\"version\":2,\"windows\":{},\"default\":{\"width\":800,\"height\":600,\"x\":10,\"y\":20}}";
        std::fs::write(&file, content).unwrap();

        let state = empty_state();
        park_label(&state, PLUGIN_WINDOW_LABEL, "local.tool\u{0}run");
        state.forget_plugin_window(PLUGIN_WINDOW_LABEL);

        assert!(
            file.exists(),
            "the remembered geometry must survive the close (R85)"
        );
        assert_eq!(std::fs::read_to_string(&file).unwrap(), content);
    }

    /// R84 · the label is the branch three sides name (the Rust builder here,
    /// the capability file, the frontend's render switch in `main.tsx`). The
    /// builder's constant is pinned to the same literal the frontend's
    /// `PLUGIN_WINDOW_LABEL` test pins, so a one-sided rename cannot compile
    /// its half green.
    #[test]
    fn the_detached_label_is_the_literal_every_side_names() {
        assert_eq!(PLUGIN_WINDOW_LABEL, "plugin-detached");
    }
}
