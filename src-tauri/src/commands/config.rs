use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::io::Write;
use std::path::Path;
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
pub const PIN_TERMINAL: &str = "pin_terminal";

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

const SETTINGS_FILE_NAME: &str = "settings.json";
const SETTINGS_BACKUP_FILE_NAME: &str = "settings.json.backup";

const SHORTCUT_ACTIONS: [&str; 8] = [
    TOGGLE_WINDOW,
    NEW_COMMAND,
    OPEN_EXTERNAL_TERMINAL,
    COPY_SELECTION,
    PASTE,
    OPEN_SETTINGS,
    SELECT_RESULT,
    PIN_TERMINAL,
];

/// Shortcut fallback for the window toggle, which is registered with the OS and
/// therefore must not collide with the platform's own bindings.
pub const DEFAULT_TOGGLE_WINDOW: &str = "Ctrl+Space";

/// Action id for the clipboard panel hotkey, which is stored as its own
/// settings field rather than in the shortcuts map (it is registered and
/// rebound by `clipboard_history`, not by the shortcut plumbing here).
pub const CLIPBOARD_PANEL: &str = "clipboard_panel";
/// The clipboard panel ships with NO global hotkey: nothing is registered on
/// startup and the panel stays reachable through launcher search and
/// `floter clip`. Users may bind one in Shortcuts settings; an empty string
/// here always means "no hotkey".
pub const DEFAULT_CLIPBOARD_HOTKEY: &str = "";

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
    /// Whether system-command discovery appears in launcher search results.
    /// Off by default; provider-connected tools are always searchable.
    pub show_commands_in_search: bool,
    /// Whether the built-in clipboard history monitor runs (default on).
    pub clipboard_history_enabled: bool,
    /// Global hotkey that summons the clipboard panel.
    pub clipboard_history_hotkey: String,
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
            show_commands_in_search: false,
            clipboard_history_enabled: true,
            clipboard_history_hotkey: DEFAULT_CLIPBOARD_HOTKEY.to_string(),
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
        (
            PIN_TERMINAL,
            if cfg!(target_os = "macos") {
                "Cmd+Shift+P".to_string()
            } else {
                "Ctrl+Shift+P".to_string()
            },
        ),
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
    let Some(config_dir) = dirs::config_dir().map(|directory| directory.join("floter")) else {
        return AppSettings::default();
    };
    load_settings_from(&config_dir)
}

fn load_settings_from(config_dir: &Path) -> AppSettings {
    read_settings(&config_dir.join(SETTINGS_FILE_NAME))
        .or_else(|| read_settings(&config_dir.join(SETTINGS_BACKUP_FILE_NAME)))
        .unwrap_or_default()
}

fn read_settings(path: &Path) -> Option<AppSettings> {
    let bytes = std::fs::read(path).ok()?;
    serde_json::from_slice(&bytes).ok()
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
    // A hand-edited or unparseable clipboard hotkey falls back to NO hotkey
    // rather than silently registering something unexpected. A bare key
    // without any modifier would swallow ordinary typing system-wide, so it
    // does not count as valid either. An empty string is the legitimate
    // disabled state.
    settings.clipboard_history_hotkey =
        normalize_shortcut(CLIPBOARD_PANEL, &settings.clipboard_history_hotkey)
            .filter(|normalized| normalized.contains('+'))
            .unwrap_or_default();
    settings
}

/// Merge the settings owned by the frontend with fields persisted by dedicated
/// commands. Those commands can run independently, so accepting their stale
/// values from a full frontend snapshot would reintroduce an older terminal
/// size or shortcut map.
fn merge_frontend_settings(mut submitted: AppSettings, stored: &AppSettings) -> AppSettings {
    submitted.terminal_width = stored.terminal_width;
    submitted.terminal_height = stored.terminal_height;
    submitted.shortcuts = resolved_shortcuts(stored);
    submitted.hotkey = stored.hotkey.clone();
    // The clipboard hotkey is owned by its dedicated command; a stale frontend
    // snapshot must not resurrect an older binding.
    submitted.clipboard_history_hotkey = stored.clipboard_history_hotkey.clone();
    normalize_settings(submitted)
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
    write_settings_to(&floter_dir, settings)
}

fn write_settings_to(config_dir: &Path, settings: &AppSettings) -> Result<(), String> {
    std::fs::create_dir_all(config_dir).map_err(|error| error.to_string())?;
    let content = serde_json::to_vec_pretty(settings).map_err(|error| error.to_string())?;

    // Make a complete, durable recovery copy before replacing the canonical
    // file. If the second rename is interrupted, startup can still recover the
    // exact snapshot the user asked us to save.
    write_settings_file(&config_dir.join(SETTINGS_BACKUP_FILE_NAME), &content)?;
    crate::extensions::lock::sync_directory(config_dir).map_err(|error| error.to_string())?;

    write_settings_file(&config_dir.join(SETTINGS_FILE_NAME), &content)?;
    crate::extensions::lock::sync_directory(config_dir).map_err(|error| error.to_string())
}

fn write_settings_file(path: &Path, content: &[u8]) -> Result<(), String> {
    let parent = path.parent().ok_or("Invalid settings path")?;
    let mut temporary =
        tempfile::NamedTempFile::new_in(parent).map_err(|error| error.to_string())?;
    temporary
        .write_all(content)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| error.to_string())?;
    temporary
        .persist(path)
        .map(|_| ())
        .map_err(|error| error.to_string())
}

#[tauri::command]
pub fn get_settings() -> Result<AppSettings, String> {
    Ok(normalize_settings(load_settings()))
}

#[tauri::command]
pub fn save_settings(app: tauri::AppHandle, settings: AppSettings) -> Result<(), String> {
    let _guard = settings_lock()?;
    let stored = load_settings();
    let settings = merge_frontend_settings(settings, &stored);
    write_settings(&settings)?;
    crate::apply_tray_language(&app, &settings.language);
    // Keep the monitor and its global hotkey in step with the switch. Both
    // branches are idempotent, so this is safe on every settings save.
    crate::clipboard_history::sync_runtime(
        &app,
        settings.clipboard_history_enabled,
        &settings.clipboard_history_hotkey,
    );
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

/// Rebind the clipboard panel's global hotkey and persist it.
///
/// The same contract as `update_shortcut` for the window toggle: the new
/// combination is claimed from the OS before anything is written, so a
/// conflict leaves both the file and the live registration untouched. An
/// empty (or whitespace-only) string unregisters the hotkey and persists the
/// disabled state — that is how the Shortcuts settings page turns it off.
#[tauri::command]
pub fn update_clipboard_hotkey(app: tauri::AppHandle, hotkey: String) -> Result<(), String> {
    let _guard = settings_lock()?;
    let mut settings = load_settings();
    let enabled = settings.clipboard_history_enabled;

    if hotkey.trim().is_empty() {
        let previous = settings.clipboard_history_hotkey.clone();
        if previous.is_empty() {
            return Ok(());
        }
        if enabled {
            crate::clipboard_history::unregister_panel_shortcut(&app);
        }
        settings.clipboard_history_hotkey = String::new();
        if let Err(error) = write_settings(&normalize_settings(settings)) {
            if enabled {
                let _ = crate::clipboard_history::rebind_panel_shortcut(&app, &previous);
            }
            return Err(error);
        }
        return Ok(());
    }

    let normalized = normalize_shortcut(CLIPBOARD_PANEL, &hotkey)
        .ok_or_else(|| "Invalid shortcut".to_string())?;

    if resolved_shortcuts(&settings)
        .values()
        .any(|existing| existing.eq_ignore_ascii_case(&normalized))
    {
        return Err("Shortcut conflicts with another action".to_string());
    }
    if settings.clipboard_history_hotkey == normalized {
        return Ok(());
    }

    let previous = settings.clipboard_history_hotkey.clone();
    if enabled {
        crate::clipboard_history::rebind_panel_shortcut(&app, &normalized)?;
    }
    settings.clipboard_history_hotkey = normalized;
    if let Err(error) = write_settings(&normalize_settings(settings)) {
        if enabled {
            let _ = crate::clipboard_history::rebind_panel_shortcut(&app, &previous);
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
    let toggle_already_live = active.eq_ignore_ascii_case(&toggle)
        && app.global_shortcut().is_registered(toggle.as_str());
    if !toggle_already_live {
        if !active.is_empty() && app.global_shortcut().is_registered(active.as_str()) {
            let _ = app.global_shortcut().unregister(active.as_str());
        }
        crate::register_toggle_shortcut(&app, &toggle)?;
    }
    // Recording suspends every global shortcut; put the clipboard panel's back
    // exactly as the toggle's is above.
    crate::clipboard_history::sync_runtime(
        &app,
        settings.clipboard_history_enabled,
        &settings.clipboard_history_hotkey,
    );
    Ok(())
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
    fn older_settings_keep_command_discovery_hidden() {
        let settings: AppSettings = serde_json::from_str("{}").expect("settings deserialize");
        assert!(!settings.show_commands_in_search);
    }

    #[test]
    fn older_settings_enable_clipboard_history_without_a_hotkey() {
        let settings: AppSettings = serde_json::from_str("{}").expect("settings deserialize");
        assert!(settings.clipboard_history_enabled);
        // The clipboard panel ships with no global hotkey at all — startup
        // must not register anything, and an empty string means disabled.
        assert_eq!(settings.clipboard_history_hotkey, DEFAULT_CLIPBOARD_HOTKEY);
        assert_eq!(DEFAULT_CLIPBOARD_HOTKEY, "");
    }

    #[test]
    fn an_unparseable_clipboard_hotkey_falls_back_to_no_hotkey() {
        let settings = normalize_settings(AppSettings {
            clipboard_history_hotkey: "not a shortcut at all".into(),
            ..AppSettings::default()
        });
        assert_eq!(settings.clipboard_history_hotkey, "");
    }

    #[test]
    fn frontend_snapshots_do_not_resurrect_a_stale_clipboard_hotkey() {
        let stored = AppSettings {
            clipboard_history_hotkey: "Ctrl+Alt+B".into(),
            ..AppSettings::default()
        };
        let merged = merge_frontend_settings(AppSettings::default(), &stored);
        assert_eq!(merged.clipboard_history_hotkey, "Ctrl+Alt+B");
    }

    #[test]
    fn settings_write_keeps_a_parseable_recovery_copy() {
        let directory = tempfile::tempdir().expect("settings directory");
        let settings = AppSettings {
            theme: "light".into(),
            language: "zh".into(),
            ..AppSettings::default()
        };

        write_settings_to(directory.path(), &settings).expect("write settings");

        let primary = read_settings(&directory.path().join(SETTINGS_FILE_NAME));
        let backup = read_settings(&directory.path().join(SETTINGS_BACKUP_FILE_NAME));
        assert_eq!(
            primary.as_ref().map(|value| value.theme.as_str()),
            Some("light")
        );
        assert_eq!(
            backup.as_ref().map(|value| value.language.as_str()),
            Some("zh")
        );
    }

    #[test]
    fn invalid_primary_settings_recover_from_the_backup() {
        let directory = tempfile::tempdir().expect("settings directory");
        let settings = AppSettings {
            font_size: 22,
            ..AppSettings::default()
        };
        write_settings_to(directory.path(), &settings).expect("write settings");
        std::fs::write(directory.path().join(SETTINGS_FILE_NAME), b"{truncated")
            .expect("corrupt primary settings");

        let recovered = load_settings_from(directory.path());

        assert_eq!(recovered.font_size, 22);
    }

    #[test]
    fn valid_primary_settings_take_precedence_over_a_stale_backup() {
        let directory = tempfile::tempdir().expect("settings directory");
        let stale = AppSettings {
            theme: "dark".into(),
            ..AppSettings::default()
        };
        let current = AppSettings {
            theme: "light".into(),
            ..AppSettings::default()
        };
        write_settings_to(directory.path(), &stale).expect("write stale settings");
        let current_bytes = serde_json::to_vec_pretty(&current).expect("serialize settings");
        write_settings_file(&directory.path().join(SETTINGS_FILE_NAME), &current_bytes)
            .expect("write current settings");

        assert_eq!(load_settings_from(directory.path()).theme, "light");
    }

    #[test]
    fn frontend_settings_preserve_fields_owned_by_dedicated_commands() {
        let mut stored = AppSettings {
            terminal_width: 1_240.0,
            terminal_height: 760.0,
            hotkey: "Ctrl+Alt+Space".into(),
            ..AppSettings::default()
        };
        stored
            .shortcuts
            .insert(TOGGLE_WINDOW.into(), "Ctrl+Alt+Space".into());

        let submitted = AppSettings {
            theme: "light".into(),
            ..AppSettings::default()
        };
        let merged = merge_frontend_settings(submitted, &stored);

        assert_eq!(merged.theme, "light");
        assert_eq!(merged.terminal_width, 1_240.0);
        assert_eq!(merged.terminal_height, 760.0);
        assert_eq!(merged.hotkey, "Ctrl+Alt+Space");
        assert_eq!(merged.shortcuts[TOGGLE_WINDOW], "Ctrl+Alt+Space");
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
