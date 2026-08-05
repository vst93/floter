use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::io::Write;
use std::sync::Mutex;
use tauri::Manager;
use tauri_plugin_global_shortcut::GlobalShortcutExt;

/// Action ids for the configurable shortcuts. They are the keys of the
/// `shortcuts` map both on disk and in the frontend, so the two sides stay in
/// step through these constants rather than through scattered string literals.
pub const TOGGLE_WINDOW: &str = "toggle_window";
pub const NEW_COMMAND: &str = "new_command";
pub const OPEN_EXTERNAL_TERMINAL: &str = "open_external_terminal";
pub const COPY_SELECTION: &str = "copy_selection";
pub const PASTE: &str = "paste";
pub const OPEN_SETTINGS: &str = "open_settings";
pub const SELECT_RESULT: &str = "select_result";

const DEFAULT_TERMINAL_WIDTH: f64 = 860.0;
const DEFAULT_TERMINAL_HEIGHT: f64 = 600.0;
const MIN_TERMINAL_WIDTH: f64 = 640.0;
const MIN_TERMINAL_HEIGHT: f64 = 360.0;
const MAX_TERMINAL_WIDTH: f64 = 2_560.0;
const MAX_TERMINAL_HEIGHT: f64 = 1_800.0;
const DEFAULT_MAIN_OPACITY: u8 = 94;
const DEFAULT_TERMINAL_OPACITY: u8 = 92;
const MIN_WINDOW_OPACITY: u8 = 10;
const MAX_WINDOW_OPACITY: u8 = 100;
const MIN_FONT_SIZE: u32 = 8;
const MAX_FONT_SIZE: u32 = 48;

static SETTINGS_LOCK: Mutex<()> = Mutex::new(());

const SHORTCUT_ACTIONS: [&str; 7] = [
    TOGGLE_WINDOW,
    NEW_COMMAND,
    OPEN_EXTERNAL_TERMINAL,
    COPY_SELECTION,
    PASTE,
    OPEN_SETTINGS,
    SELECT_RESULT,
];

/// Shortcut fallback for the window toggle, which is registered with the OS and
/// therefore must not collide with the platform's own bindings.
pub const DEFAULT_TOGGLE_WINDOW: &str = "Ctrl+Space";

/// The modifier apps use for their own commands: Cmd on macOS, Ctrl elsewhere.
#[cfg(target_os = "macos")]
const APP_MODIFIER: &str = "Cmd";
#[cfg(not(target_os = "macos"))]
const APP_MODIFIER: &str = "Ctrl";

/// Missing keys fall back to `Default`, so settings files written by older
/// builds keep working when new fields are introduced.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(default)]
pub struct AppSettings {
    pub hotkey: String,
    pub hide_on_blur: bool,
    pub launch_at_startup: bool,
    /// UI theme: "dark" | "light" | "auto". Only the frontend reads it — "auto"
    /// is resolved there against the system appearance.
    pub theme: String,
    pub font_size: u32,
    pub font_family: String,
    /// Default cursor shape: "beam" | "block" | "underline".
    pub cursor_shape: String,
    /// UI language: "en" | "zh".
    pub language: String,
    /// Last user-selected terminal window dimensions, in logical pixels.
    pub terminal_width: f64,
    pub terminal_height: f64,
    /// Background opacity percentages for the launcher/settings surface and
    /// terminal canvas. Values are clamped before persistence.
    pub main_opacity: u8,
    pub terminal_opacity: u8,
    /// Action id -> shortcut string ("Cmd+W", "Ctrl+Shift+Space").
    pub shortcuts: HashMap<String, String>,
}

impl Default for AppSettings {
    fn default() -> Self {
        Self {
            hotkey: DEFAULT_TOGGLE_WINDOW.to_string(),
            hide_on_blur: true,
            launch_at_startup: false,
            theme: "auto".to_string(),
            font_size: 14,
            font_family: "monospace".to_string(),
            cursor_shape: "beam".to_string(),
            language: "en".to_string(),
            terminal_width: DEFAULT_TERMINAL_WIDTH,
            terminal_height: DEFAULT_TERMINAL_HEIGHT,
            main_opacity: DEFAULT_MAIN_OPACITY,
            terminal_opacity: DEFAULT_TERMINAL_OPACITY,
            shortcuts: default_shortcuts(),
        }
    }
}

/// Platform defaults for every configurable shortcut.
///
/// `select_result` holds the binding for the *first* result; the digits 2-9
/// reuse its modifiers, so recording `Cmd+1` rebinds the whole 1-9 range.
pub fn default_shortcuts() -> HashMap<String, String> {
    [
        (TOGGLE_WINDOW, DEFAULT_TOGGLE_WINDOW.to_string()),
        (NEW_COMMAND, format!("{APP_MODIFIER}+W")),
        (OPEN_EXTERNAL_TERMINAL, format!("{APP_MODIFIER}+N")),
        (
            COPY_SELECTION,
            if cfg!(target_os = "macos") {
                "Cmd+C".to_string()
            } else {
                "Ctrl+Shift+C".to_string()
            },
        ),
        (
            PASTE,
            if cfg!(target_os = "macos") {
                "Cmd+V".to_string()
            } else {
                "Ctrl+Shift+V".to_string()
            },
        ),
        (OPEN_SETTINGS, format!("{APP_MODIFIER}+Comma")),
        (SELECT_RESULT, format!("{APP_MODIFIER}+1")),
    ]
    .into_iter()
    .map(|(action, shortcut)| (action.to_string(), shortcut))
    .collect()
}

/// The stored shortcuts with every missing action filled in, which is the shape
/// both the frontend and the global-shortcut registration expect.
pub fn resolved_shortcuts(settings: &AppSettings) -> HashMap<String, String> {
    let mut shortcuts = default_shortcuts();
    for action in SHORTCUT_ACTIONS {
        if let Some(shortcut) = settings.shortcuts.get(action) {
            if let Some(shortcut) = normalize_shortcut(action, shortcut) {
                insert_shortcut_if_available(&mut shortcuts, action, shortcut);
            }
        }
    }
    // A non-default hotkey customized before the shortcuts map existed still
    // applies. Modern saves keep both fields synchronized, so this only wins
    // when deserialization filled a missing map with platform defaults.
    if settings.hotkey.trim() != DEFAULT_TOGGLE_WINDOW {
        if let Some(hotkey) = normalize_shortcut(TOGGLE_WINDOW, &settings.hotkey) {
            insert_shortcut_if_available(&mut shortcuts, TOGGLE_WINDOW, hotkey);
        }
    }
    shortcuts
}

fn insert_shortcut_if_available(
    shortcuts: &mut HashMap<String, String>,
    action: &str,
    candidate: String,
) {
    let available = shortcuts.iter().all(|(existing_action, existing)| {
        existing_action == action
            || !shortcut_conflicts(action, &candidate, existing_action, existing)
    });
    if available {
        shortcuts.insert(action.to_string(), candidate);
    }
}

/// Load settings from disk, falling back to defaults when missing or invalid.
pub fn load_settings() -> AppSettings {
    let Some(config_path) = dirs::config_dir().map(|d| d.join("floter").join("settings.json"))
    else {
        return AppSettings::default();
    };
    if let Ok(content) = std::fs::read_to_string(&config_path) {
        if let Ok(settings) = serde_json::from_str::<AppSettings>(&content) {
            return settings;
        }
    }
    AppSettings::default()
}

/// The persisted terminal size, normalized defensively so a hand-edited
/// settings file cannot create an unusable off-screen panel.
pub fn saved_terminal_size() -> (f64, f64) {
    let settings = load_settings();
    normalize_terminal_size(settings.terminal_width, settings.terminal_height)
}

pub fn save_terminal_size(width: f64, height: f64) -> Result<(f64, f64), String> {
    let (width, height) = normalize_terminal_size(width, height);
    let _guard = settings_lock()?;
    let mut settings = load_settings();
    settings.terminal_width = width;
    settings.terminal_height = height;
    write_settings(&settings)?;
    Ok((width, height))
}

fn normalize_window_opacity(value: u8, default: u8) -> u8 {
    if value == 0 {
        default
    } else {
        value.clamp(MIN_WINDOW_OPACITY, MAX_WINDOW_OPACITY)
    }
}

fn normalize_terminal_size(width: f64, height: f64) -> (f64, f64) {
    let width = if width.is_finite() {
        width.clamp(MIN_TERMINAL_WIDTH, MAX_TERMINAL_WIDTH)
    } else {
        DEFAULT_TERMINAL_WIDTH
    };
    let height = if height.is_finite() {
        height.clamp(MIN_TERMINAL_HEIGHT, MAX_TERMINAL_HEIGHT)
    } else {
        DEFAULT_TERMINAL_HEIGHT
    };
    (width, height)
}

fn normalize_settings(mut settings: AppSettings) -> AppSettings {
    settings.theme = match settings.theme.as_str() {
        "dark" | "light" | "auto" => settings.theme,
        _ => AppSettings::default().theme,
    };
    settings.language = match settings.language.as_str() {
        "en" | "zh" => settings.language,
        _ => AppSettings::default().language,
    };
    settings.cursor_shape = match settings.cursor_shape.as_str() {
        "beam" | "block" | "underline" => settings.cursor_shape,
        _ => AppSettings::default().cursor_shape,
    };
    settings.font_size = settings.font_size.clamp(MIN_FONT_SIZE, MAX_FONT_SIZE);
    if settings.font_family.trim().is_empty() {
        settings.font_family = AppSettings::default().font_family;
    } else {
        settings.font_family = settings.font_family.trim().to_string();
    }
    (settings.terminal_width, settings.terminal_height) =
        normalize_terminal_size(settings.terminal_width, settings.terminal_height);
    settings.main_opacity = normalize_window_opacity(settings.main_opacity, DEFAULT_MAIN_OPACITY);
    settings.terminal_opacity =
        normalize_window_opacity(settings.terminal_opacity, DEFAULT_TERMINAL_OPACITY);
    settings.shortcuts = resolved_shortcuts(&settings);
    settings.hotkey = settings
        .shortcuts
        .get(TOGGLE_WINDOW)
        .cloned()
        .unwrap_or_else(|| DEFAULT_TOGGLE_WINDOW.to_string());
    settings
}

fn settings_lock() -> Result<std::sync::MutexGuard<'static, ()>, String> {
    SETTINGS_LOCK
        .lock()
        .map_err(|_| "Settings lock is poisoned".to_string())
}

fn modifier_name(value: &str) -> Option<&'static str> {
    match value.to_ascii_lowercase().as_str() {
        "ctrl" | "control" => Some("Ctrl"),
        "alt" | "option" => Some("Alt"),
        "shift" => Some("Shift"),
        "cmd" | "command" | "meta" | "super" | "win" => Some(if cfg!(target_os = "macos") {
            "Cmd"
        } else {
            "Super"
        }),
        "commandorcontrol" | "cmdorctrl" => Some(if cfg!(target_os = "macos") {
            "Cmd"
        } else {
            "Ctrl"
        }),
        _ => None,
    }
}

fn normalize_shortcut(action: &str, value: &str) -> Option<String> {
    let mut modifiers = Vec::new();
    let mut key = None;
    for part in value.split('+').map(str::trim) {
        if part.is_empty() {
            return None;
        }
        if let Some(modifier) = modifier_name(part) {
            if modifiers.contains(&modifier) {
                return None;
            }
            modifiers.push(modifier);
        } else if key.replace(part).is_some() {
            return None;
        }
    }
    let mut key = key?;
    if action == SELECT_RESULT {
        if modifiers.is_empty() {
            return None;
        }
        key = "1";
    }
    modifiers.push(key);
    Some(modifiers.join("+"))
}

fn shortcut_conflicts(
    candidate_action: &str,
    candidate: &str,
    existing_action: &str,
    existing: &str,
) -> bool {
    if candidate.eq_ignore_ascii_case(existing) {
        return true;
    }
    let result_and_other = if candidate_action == SELECT_RESULT {
        Some((candidate, existing))
    } else if existing_action == SELECT_RESULT {
        Some((existing, candidate))
    } else {
        None
    };
    let Some((result, other)) = result_and_other else {
        return false;
    };
    let Some((result_modifiers, _)) = result.rsplit_once('+') else {
        return false;
    };
    let Some((other_modifiers, other_key)) = other.rsplit_once('+') else {
        return false;
    };
    result_modifiers.eq_ignore_ascii_case(other_modifiers)
        && matches!(
            other_key,
            "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"
        )
}

fn write_settings(settings: &AppSettings) -> Result<(), String> {
    let config_dir = dirs::config_dir().ok_or("Cannot find config directory")?;
    let floter_dir = config_dir.join("floter");
    std::fs::create_dir_all(&floter_dir).map_err(|e| e.to_string())?;
    let config_path = floter_dir.join("settings.json");
    let content = serde_json::to_vec_pretty(settings).map_err(|e| e.to_string())?;
    let mut temporary = tempfile::NamedTempFile::new_in(&floter_dir).map_err(|e| e.to_string())?;
    temporary.write_all(&content).map_err(|e| e.to_string())?;
    temporary.flush().map_err(|e| e.to_string())?;
    temporary.as_file().sync_all().map_err(|e| e.to_string())?;
    temporary
        .persist(config_path)
        .map(|_| ())
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub fn get_settings() -> Result<AppSettings, String> {
    Ok(normalize_settings(load_settings()))
}

#[tauri::command]
pub fn save_settings(app: tauri::AppHandle, settings: AppSettings) -> Result<(), String> {
    let _guard = settings_lock()?;
    let mut settings = settings;
    // Shortcut changes have their own command because the global binding and
    // disk must change transactionally. A delayed general settings save must
    // never restore an older shortcut map.
    let stored = load_settings();
    settings.shortcuts = resolved_shortcuts(&stored);
    settings.hotkey = stored.hotkey;
    let settings = normalize_settings(settings);
    write_settings(&settings)?;
    crate::apply_tray_language(&app, &settings.language);
    Ok(())
}

#[tauri::command]
pub fn get_shortcuts() -> Result<HashMap<String, String>, String> {
    Ok(resolved_shortcuts(&load_settings()))
}

/// Restore every shortcut to its platform default. Rebind the global toggle
/// first so a system conflict leaves the existing settings untouched.
#[tauri::command]
pub fn reset_shortcuts(app: tauri::AppHandle) -> Result<HashMap<String, String>, String> {
    let _guard = settings_lock()?;
    let shortcuts = default_shortcuts();
    let toggle = shortcuts
        .get(TOGGLE_WINDOW)
        .cloned()
        .unwrap_or_else(|| DEFAULT_TOGGLE_WINDOW.to_string());
    crate::rebind_toggle_shortcut(&app, &toggle)?;

    let mut settings = load_settings();
    settings.hotkey = toggle;
    settings.shortcuts = shortcuts.clone();
    if let Err(error) = write_settings(&settings) {
        let previous = resolved_shortcuts(&load_settings())
            .get(TOGGLE_WINDOW)
            .cloned()
            .unwrap_or_else(|| DEFAULT_TOGGLE_WINDOW.to_string());
        let _ = crate::rebind_toggle_shortcut(&app, &previous);
        return Err(error);
    }
    Ok(shortcuts)
}

/// Rebind one action and persist it.
///
/// The window toggle is owned by the OS, so it is re-registered *before* the
/// change is written: if the combination is already taken by another app the
/// old binding stays in place and the error travels back to the UI.
#[tauri::command]
pub fn update_shortcut(
    app: tauri::AppHandle,
    action: String,
    shortcut: String,
) -> Result<(), String> {
    let _guard = settings_lock()?;
    let shortcut =
        normalize_shortcut(&action, &shortcut).ok_or_else(|| "Invalid shortcut".to_string())?;

    let mut settings = load_settings();
    let mut shortcuts = resolved_shortcuts(&settings);
    let Some(previous) = shortcuts.get(&action).cloned() else {
        return Err(format!("Unknown shortcut action: {action}"));
    };
    if previous == shortcut {
        return Ok(());
    }

    if shortcuts.iter().any(|(existing_action, existing)| {
        existing_action != &action
            && shortcut_conflicts(&action, &shortcut, existing_action, existing)
    }) {
        return Err("Shortcut conflicts with another action".to_string());
    }

    if action == TOGGLE_WINDOW {
        crate::rebind_toggle_shortcut(&app, &shortcut)?;
        // Keep the legacy field in step so both readers agree.
        settings.hotkey = shortcut.clone();
    }

    shortcuts.insert(action.clone(), shortcut.clone());
    settings.shortcuts = shortcuts;
    if let Err(error) = write_settings(&normalize_settings(settings)) {
        if action == TOGGLE_WINDOW {
            let _ = crate::rebind_toggle_shortcut(&app, &previous);
        }
        return Err(error);
    }
    Ok(())
}

/// Build version injected at compile time.
///
/// `CARGO_PKG_VERSION` comes from `Cargo.toml`, which the release workflow
/// rewrites before tagging. In `tauri dev` the crate version is whatever is
/// in the working copy and debug assertions are on, so we show "DEV" instead
/// of a meaningless 0.0.0-something that has nothing to do with a release.
#[tauri::command]
pub fn app_version() -> String {
    if cfg!(debug_assertions) {
        "DEV".to_string()
    } else {
        env!("CARGO_PKG_VERSION").to_string()
    }
}

/// Temporarily unregister all global shortcuts so the shortcut recorder can
/// capture the current binding without the app toggling itself.
#[tauri::command]
pub fn suspend_shortcuts(app: tauri::AppHandle) -> Result<(), String> {
    app.global_shortcut()
        .unregister_all()
        .map_err(|e| e.to_string())
}

/// Re-register the toggle shortcut from saved settings after recording ends.
#[tauri::command]
pub fn resume_shortcuts(app: tauri::AppHandle) -> Result<(), String> {
    let settings = load_settings();
    let shortcuts = resolved_shortcuts(&settings);
    let toggle = shortcuts
        .get(TOGGLE_WINDOW)
        .cloned()
        .unwrap_or_else(|| DEFAULT_TOGGLE_WINDOW.to_string());
    let active = app
        .state::<crate::AppState>()
        .toggle_shortcut
        .lock()
        .map(|value| value.clone())
        .unwrap_or_default();
    if active.eq_ignore_ascii_case(&toggle) && app.global_shortcut().is_registered(toggle.as_str())
    {
        return Ok(());
    }
    if !active.is_empty() && app.global_shortcut().is_registered(active.as_str()) {
        let _ = app.global_shortcut().unregister(active.as_str());
    }
    crate::register_toggle_shortcut(&app, &toggle)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn older_settings_do_not_enable_autostart() {
        let settings: AppSettings = serde_json::from_str("{}").expect("settings deserialize");
        assert!(!settings.launch_at_startup);
    }

    #[test]
    fn terminal_size_is_finite_and_within_usable_bounds() {
        assert_eq!(
            normalize_terminal_size(f64::NAN, f64::INFINITY),
            (DEFAULT_TERMINAL_WIDTH, DEFAULT_TERMINAL_HEIGHT)
        );
        assert_eq!(
            normalize_terminal_size(1.0, 9_999.0),
            (MIN_TERMINAL_WIDTH, MAX_TERMINAL_HEIGHT)
        );
    }

    #[test]
    fn window_opacity_keeps_surfaces_legible() {
        assert_eq!(
            normalize_window_opacity(0, DEFAULT_MAIN_OPACITY),
            DEFAULT_MAIN_OPACITY
        );
        assert_eq!(
            normalize_window_opacity(1, DEFAULT_MAIN_OPACITY),
            MIN_WINDOW_OPACITY
        );
        assert_eq!(
            normalize_window_opacity(255, DEFAULT_MAIN_OPACITY),
            MAX_WINDOW_OPACITY
        );
    }

    #[test]
    fn result_shortcuts_require_modifiers_and_always_store_one() {
        assert_eq!(
            normalize_shortcut(SELECT_RESULT, "Ctrl+K").as_deref(),
            Some("Ctrl+1")
        );
        assert_eq!(
            normalize_shortcut(SELECT_RESULT, "Alt+9").as_deref(),
            Some("Alt+1")
        );
        assert_eq!(normalize_shortcut(SELECT_RESULT, "F1"), None);
    }

    #[test]
    fn result_family_conflicts_with_every_number_using_its_modifiers() {
        assert!(shortcut_conflicts(
            SELECT_RESULT,
            "Ctrl+1",
            NEW_COMMAND,
            "Ctrl+7"
        ));
        assert!(!shortcut_conflicts(
            SELECT_RESULT,
            "Ctrl+1",
            NEW_COMMAND,
            "Alt+7"
        ));
        assert!(!shortcut_conflicts(
            SELECT_RESULT,
            "Ctrl+1",
            NEW_COMMAND,
            "Ctrl+W"
        ));
    }

    #[test]
    fn normalizes_all_persisted_setting_ranges() {
        let settings = normalize_settings(AppSettings {
            theme: "unknown".into(),
            language: "xx".into(),
            cursor_shape: "square".into(),
            font_size: 1_000,
            font_family: "  ".into(),
            terminal_width: f64::NAN,
            terminal_height: f64::INFINITY,
            ..AppSettings::default()
        });
        assert_eq!(settings.theme, "auto");
        assert_eq!(settings.language, "en");
        assert_eq!(settings.cursor_shape, "beam");
        assert_eq!(settings.font_size, MAX_FONT_SIZE);
        assert_eq!(settings.font_family, "monospace");
        assert_eq!(settings.terminal_width, DEFAULT_TERMINAL_WIDTH);
        assert_eq!(settings.terminal_height, DEFAULT_TERMINAL_HEIGHT);
    }
}
