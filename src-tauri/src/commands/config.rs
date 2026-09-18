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
/// R8: the single pre-R8 "glass strength" slider became two orthogonal
/// controls. `main_opacity` is now the **window transparency** percentage
/// (how solid the shell's tint is, and the only field a native window-alpha
/// path may ever read); the material/effect itself is the five-step
/// `glass_step`.
const DEFAULT_MAIN_OPACITY: u8 = 47;
const DEFAULT_TERMINAL_OPACITY: u8 = 46;
const MIN_WINDOW_OPACITY: u8 = 10;
const MAX_WINDOW_OPACITY: u8 = 100;

/// Key that marks a settings file as post-R8. Its absence is what tells
/// `read_settings` to run the legacy split migration exactly once.
const GLASS_STEP_KEY: &str = "glass_step";
const DEFAULT_GLASS_STEP: &str = "regular";
/// The effect-step domain. GLASS-REAXIS pointed the UI stops at the *effect*
/// axis (blur / saturation / control-lens quality) and restored the dual
/// transparency sliders; GLASS-3STOP then collapsed the five stops to three
/// and decoupled the frame's alpha from the step entirely. The struct field,
/// its default and the pre-R8 migration are all unchanged — only the set of
/// accepted step ids changed, from five (`low`/`mid`/`high`/`deep`/`jelly`)
/// to three (`frosted`/`regular`/`liquid`).
const GLASS_STEPS: [&str; 3] = ["frosted", "regular", "liquid"];
/// The pre-GLASS-3STOP five-stop vocabulary, and the three-stop step each id
/// becomes on read. The two thin stops collapse onto `frosted`/`regular`; every
/// heavy stop (`high`/`deep`/`jelly`) collapses onto `liquid`, so an upgrading
/// user who was on the heaviest material is not demoted to the balanced one.
const LEGACY_GLASS_STEPS: [(&str, &str); 5] = [
    ("low", "frosted"),
    ("mid", "regular"),
    ("high", "liquid"),
    ("deep", "liquid"),
    ("jelly", "liquid"),
];
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
    /// Window transparency percentages for the launcher/settings surface and
    /// the terminal canvas. These control how solid the shell's tint is — the
    /// readability control — and nothing else. Values are clamped before
    /// persistence.
    pub main_opacity: u8,
    pub terminal_opacity: u8,
    /// Liquid-glass effect step: "frosted" | "regular" | "liquid".
    /// This is the effect control (frosted glass → Apple's liquid glass →
    /// liquid at full strength): it drives the blur radius, the saturation
    /// boost, the haze layer and the control-lens quality — never the frame's
    /// alpha. It is deliberately *not* read by any native window path — see
    /// the `glass_step_is_not_a_native_alpha_input` test.
    pub glass_step: String,
    /// Action id -> shortcut string ("Cmd+W", "Ctrl+Shift+Space").
    pub shortcuts: HashMap<String, String>,
    /// Whether system-command discovery appears in launcher search results.
    /// Off by default; provider-connected tools are always searchable.
    pub show_commands_in_search: bool,
    /// Whether the launcher's empty query offers the most-launched applications.
    /// On by default; off leaves the list empty until something is typed.
    pub show_recent_in_launcher: bool,
    /// Whether the built-in clipboard history monitor runs (default on).
    pub clipboard_history_enabled: bool,
    /// Global hotkey that summons the clipboard panel.
    pub clipboard_history_hotkey: String,
    /// Application path -> launch count, ranking the launcher's empty-query
    /// recent list. Owned by the frontend; no dedicated command persists it.
    pub launch_counts: HashMap<String, u32>,
    /// Settings page the user last had open, restored on the next launch.
    /// Owned by the frontend, which validates the value before saving.
    pub last_settings_page: String,
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
            glass_step: DEFAULT_GLASS_STEP.to_string(),
            shortcuts: default_shortcuts(),
            show_commands_in_search: false,
            show_recent_in_launcher: true,
            clipboard_history_enabled: true,
            clipboard_history_hotkey: DEFAULT_CLIPBOARD_HOTKEY.to_string(),
            launch_counts: HashMap::new(),
            last_settings_page: "general".to_string(),
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
    let value: serde_json::Value = serde_json::from_slice(&bytes).ok()?;
    // A file written before R8 has no `glass_step` key: its single
    // "glass strength" number has to be split across the two controls that
    // replaced it. The marker is the key itself rather than a version field,
    // so the migration is invisible to everything downstream and the round's
    // `serde(default)` compatibility guarantee stays intact.
    let legacy = value.get(GLASS_STEP_KEY).is_none();
    let mut settings: AppSettings = serde_json::from_value(value).ok()?;
    if legacy {
        migrate_legacy_glass_strength(&mut settings);
    }
    // GLASS-3STOP renamed the step vocabulary (five ids → three); a file
    // written by a build that shipped the old ids still has to open on the
    // material it implied. This is idempotent, so re-running it through
    // `normalize_settings` on the command path is harmless.
    settings.glass_step = normalize_glass_step(&settings.glass_step);
    Some(settings)
}

/// Split a pre-R8 single "glass strength" value across the two controls.
///
/// The old slider was labelled *glass strength* but drove three things at
/// once: the shell tint's alpha, the blur radius (inversely) and the
/// saturation. R8 separates the two axes that actually compose the material:
/// the **material step** (blur, saturation, variant) and the **window
/// transparency** (the fill). A faithful split therefore spends the old value
/// on both axes at half weight — `transparency = old / 2`, and the step is the
/// old value's own tertile — so an old 25 (thin) lands at frosted + 13% and an
/// old 100 (maxed) at liquid + 50%. Neither new control is parked at an
/// extreme, which is precisely why the single axis was split in the first
/// place.
///
/// The division by two is integer and rounds to nearest, so 25 → 13 and
/// 94 → 47 rather than silently truncating the user's intent downward. The
/// tertile now names the three-stop vocabulary GLASS-3STOP shipped.
fn migrate_legacy_glass_strength(settings: &mut AppSettings) {
    let legacy = settings.main_opacity as u32;
    settings.main_opacity = legacy.div_ceil(2) as u8;
    settings.terminal_opacity = (settings.terminal_opacity as u32).div_ceil(2) as u8;
    settings.glass_step = match legacy {
        0..=33 => "frosted",
        34..=66 => "regular",
        _ => "liquid",
    }
    .to_string();
}

/// Clamp the effect step to the shipped variants, migrating a pre-GLASS-3STOP
/// id on the way.
///
/// GLASS-3STOP renamed the step vocabulary from five ids to three. A file
/// written by a build that shipped the old ids still has to open on the
/// material it implied, so the old values are mapped rather than rejected:
/// `low → frosted`, `mid → regular`, and every heavy stop (`high` / `deep` /
/// `jelly`) → `liquid`, so an upgrading user who was on the heaviest material
/// is not demoted. An unknown value (hand-edited file, a future step this
/// build does not know) falls back to the balanced Regular step rather than to
/// a random one, and a warning is logged so the fallback is not silent.
fn normalize_glass_step(value: &str) -> String {
    let trimmed = value.trim().to_ascii_lowercase();
    if GLASS_STEPS.contains(&trimmed.as_str()) {
        return trimmed;
    }
    if let Some((_, mapped)) = LEGACY_GLASS_STEPS.iter().find(|(old, _)| *old == trimmed) {
        return (*mapped).to_string();
    }
    eprintln!("floter: unknown glass_step {value:?}; falling back to {DEFAULT_GLASS_STEP:?}");
    DEFAULT_GLASS_STEP.to_string()
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
    settings.glass_step = normalize_glass_step(&settings.glass_step);
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
    #[cfg(feature = "clipboard-history")]
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
        #[cfg(feature = "clipboard-history")]
        if enabled {
            crate::clipboard_history::unregister_panel_shortcut(&app);
        }
        settings.clipboard_history_hotkey = String::new();
        #[cfg(feature = "clipboard-history")]
        if let Err(error) = write_settings(&normalize_settings(settings)) {
            if enabled {
                let _ = crate::clipboard_history::rebind_panel_shortcut(&app, &previous);
            }
            return Err(error);
        }
        #[cfg(not(feature = "clipboard-history"))]
        if let Err(error) = write_settings(&normalize_settings(settings)) {
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
    #[cfg(feature = "clipboard-history")]
    if enabled {
        crate::clipboard_history::rebind_panel_shortcut(&app, &normalized)?;
    }
    settings.clipboard_history_hotkey = normalized;
    #[cfg(feature = "clipboard-history")]
    if let Err(error) = write_settings(&normalize_settings(settings)) {
        if enabled {
            let _ = crate::clipboard_history::rebind_panel_shortcut(&app, &previous);
        }
        return Err(error);
    }
    #[cfg(not(feature = "clipboard-history"))]
    if let Err(error) = write_settings(&normalize_settings(settings)) {
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
    #[cfg(feature = "clipboard-history")]
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
    fn older_settings_start_with_no_launch_counts() {
        let settings: AppSettings = serde_json::from_str("{}").expect("settings deserialize");
        assert!(settings.launch_counts.is_empty());
    }

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
    fn older_settings_keep_the_launcher_recent_list() {
        let settings: AppSettings = serde_json::from_str("{}").expect("settings deserialize");
        assert!(settings.show_recent_in_launcher);
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

    // ── R8: the glass-strength split ──────────────────────────────────────

    /// A pre-R8 file has `main_opacity` but no `glass_step`. Reading it has to
    /// split the old single "glass strength" number across the two controls
    /// that replaced it, and it must do so exactly once: the second read sees
    /// the marker key and leaves the values alone.
    #[test]
    fn legacy_glass_strength_splits_across_both_controls() {
        let legacy = r#"{ "main_opacity": 100, "terminal_opacity": 80 }"#;
        let settings: AppSettings = serde_json::from_str(legacy).expect("legacy deserialize");
        // No default is applied by `serde` for the *migration*: the field is
        // present in the struct but the marker is what decides.
        assert_eq!(settings.main_opacity as u32, 100);
        let mut settings = settings;
        // The marker's absence is what the loader keys on; here the migration
        // is invoked directly to pin the arithmetic.
        settings.main_opacity = 100;
        settings.terminal_opacity = 80;
        migrate_legacy_glass_strength(&mut settings);
        // ÷2, rounded to nearest: the old value was one axis driving two, so
        // each new axis gets half the intent rather than the whole of it.
        assert_eq!(settings.main_opacity, 50);
        assert_eq!(settings.terminal_opacity, 40);
        assert_eq!(settings.glass_step, "liquid");
    }

    #[test]
    fn legacy_split_rounds_to_nearest_and_picks_the_tertile() {
        let cases = [
            (10u8, 5u8, "frosted"),
            (25, 13, "frosted"),
            (33, 17, "frosted"),
            (34, 17, "regular"),
            (50, 25, "regular"),
            (66, 33, "regular"),
            (67, 34, "liquid"),
            (94, 47, "liquid"),
            (100, 50, "liquid"),
        ];
        for (legacy, expected_opacity, expected_step) in cases {
            let mut settings = AppSettings::default();
            settings.main_opacity = legacy;
            migrate_legacy_glass_strength(&mut settings);
            assert_eq!(
                settings.main_opacity, expected_opacity,
                "legacy {legacy} must halve to {expected_opacity}"
            );
            assert_eq!(
                settings.glass_step, expected_step,
                "legacy {legacy} must land on the {expected_step} step"
            );
        }
    }

    /// The migration is keyed on the marker's *absence*, so it must not run
    /// twice. A post-R8 value that happened to look legacy would otherwise be
    /// halved on every launch.
    #[test]
    fn the_legacy_migration_runs_at_most_once() {
        let config_dir = tempfile::tempdir().expect("tempdir");
        let legacy = r#"{ "main_opacity": 100, "terminal_opacity": 80 }"#;
        std::fs::write(config_dir.path().join(SETTINGS_FILE_NAME), legacy).expect("write legacy");
        let first = load_settings_from(config_dir.path());
        assert_eq!(first.main_opacity, 50);
        assert_eq!(first.glass_step, "liquid");

        // A save writes the marker, and the next read must leave it alone.
        write_settings_to(config_dir.path(), &first).expect("write migrated");
        let second = load_settings_from(config_dir.path());
        assert_eq!(
            second.main_opacity, 50,
            "a migrated file must not halve again"
        );
        assert_eq!(second.glass_step, "liquid");
    }

    /// A fresh settings file starts on the balanced step, and the two
    /// defaults are the halved equivalents of the pre-R8 94/92 — so an
    /// upgrading user who never opens settings sees the *same* window
    /// solidity their old file implied, not a jump to a new one.
    #[test]
    fn defaults_are_the_split_equivalent_of_the_pre_r8_slider() {
        let defaults = AppSettings::default();
        assert_eq!(
            defaults.main_opacity,
            (94u32 / 2) as u8,
            "the default transparency must be the old 94 split in half"
        );
        assert_eq!(defaults.terminal_opacity, (92u32 / 2) as u8);
        assert_eq!(defaults.glass_step, "regular");
    }

    /// A pre-GLASS-3STOP file carries one of the old five ids. Reading it has
    /// to migrate the id onto the three-stop vocabulary: the two thin stops
    /// land on `frosted`/`regular`, and *every* heavy stop lands on `liquid`
    /// so an upgrading user on the heaviest material is not demoted.
    #[test]
    fn the_old_five_stop_ids_migrate_onto_three() {
        for (old, expected) in [
            ("low", "frosted"),
            ("mid", "regular"),
            ("high", "liquid"),
            ("deep", "liquid"),
            ("jelly", "liquid"),
        ] {
            assert_eq!(
                normalize_glass_step(old),
                expected,
                "the pre-GLASS-3STOP id {old:?} must migrate to {expected:?}"
            );
            // The migration is case- and whitespace-insensitive, like the
            // normalizer it lives in.
            assert_eq!(
                normalize_glass_step(&format!("  {}  ", old.to_uppercase())),
                expected
            );
        }
    }

    /// The migration runs at *read* time, not only when the command layer
    /// normalizes: a settings file that still holds an old id must come back
    /// from `load_settings_from` already migrated, or the first render would
    /// write the dead id onto <html>.
    #[test]
    fn reading_a_file_with_an_old_step_migrates_it() {
        let config_dir = tempfile::tempdir().expect("tempdir");
        for (old, expected) in [
            ("low", "frosted"),
            ("mid", "regular"),
            ("high", "liquid"),
            ("deep", "liquid"),
            ("jelly", "liquid"),
        ] {
            let file = format!(r#"{{ "glass_step": "{old}" }}"#);
            std::fs::write(config_dir.path().join(SETTINGS_FILE_NAME), file).expect("write");
            let settings = load_settings_from(config_dir.path());
            assert_eq!(
                settings.glass_step, expected,
                "a stored {old:?} must read back as {expected:?}"
            );
        }
    }

    /// An unknown step is normalized to Regular rather than to whichever
    /// variant happens to sort first: a wrong-but-balanced material beats an
    /// unintended frosted panel over a photo. The old ids are *not* unknown —
    /// they migrate — so they are checked in their own test above.
    #[test]
    fn an_unknown_glass_step_falls_back_to_regular() {
        assert_eq!(normalize_glass_step("frosted"), "frosted");
        assert_eq!(normalize_glass_step("REGULAR"), "regular");
        assert_eq!(normalize_glass_step(" liquid "), "liquid");
        for unknown in ["", "clear", "balanced", "strong", "999", "ultra"] {
            assert_eq!(
                normalize_glass_step(unknown),
                DEFAULT_GLASS_STEP,
                "{unknown:?} is not a shipped step"
            );
        }
    }

    /// The whole point of the split, asserted structurally: the material step
    /// is not an alpha and no native window path may read it. If this test
    /// ever fails, the two controls have been re-coupled.
    #[test]
    fn glass_step_is_not_a_native_alpha_input() {
        // The step is a string from the shipped stop list, never a number between the
        // transparency bounds — a "step" that could be clamped to [10,100]
        // would be the old slider wearing a new name.
        assert!(
            normalize_glass_step("0").parse::<u8>().is_err(),
            "the step must not be numeric: a number here would be an opacity"
        );
        assert!(
            !GLASS_STEPS.contains(&"0"),
            "the step domain is the three effect steps and nothing else"
        );
        // `glass_step` is absent from every alpha application: the only fields
        // the window path may read are the two opacity percentages. This is a
        // *line-level whitelist* rather than a "contains neither" disjunction:
        // every line of the code section (comments and the test module excluded)
        // that mentions the step must be one of the four places the step is
        // legitimately touched — its field declaration, its default, the
        // legacy-split migration, or its normalizer. Anything else, and in
        // particular a line that also feeds a native alpha call, fails here.
        // The old shape `!contains("glass_step") || !contains("set_alpha")`
        // was vacuously true because `set_alpha` occurred nowhere in the file,
        // so it guarded nothing.
        let source = include_str!("config.rs");
        let code: String = source
            .lines()
            .filter(|line| {
                !line.trim_start().starts_with("//") && !line.trim_start().starts_with("///")
            })
            .collect::<Vec<_>>()
            .join("\n");
        let code_before_tests = code
            .split("mod tests")
            .next()
            .expect("the module has a code section");

        // The shapes a `glass_step` line is allowed to have. Each is a
        // declaration, a default, or an assignment through the normalizer —
        // never a call argument.
        let allowed = [
            r#"const GLASS_STEP_KEY: &str = "glass_step";"#,
            "pub glass_step: String",
            "glass_step: DEFAULT_GLASS_STEP.to_string()",
            "settings.glass_step = match legacy",
            "fn normalize_glass_step(value: &str) -> String",
            "settings.glass_step = normalize_glass_step(&settings.glass_step)",
            // The unknown-value warning names the field it could not read; it
            // is a log line, not an alpha application.
            "floter: unknown glass_step",
        ];
        // Tokens that identify a native alpha/vibrancy/window application. If
        // one ever shares a line with the step, the two controls have been
        // re-coupled at the exact seam this round separates.
        let alpha_calls = [
            "set_alpha",
            "with_alpha",
            "set_opacity",
            "vibrancy",
            "apply_alpha",
            "window_alpha",
        ];

        let mut seen = 0usize;
        for (index, line) in code_before_tests.lines().enumerate() {
            if !line.contains("glass_step") {
                continue;
            }
            seen += 1;
            assert!(
                allowed.iter().any(|shape| line.contains(shape)),
                "line {} reads the material step outside its declaration/migration/normalizer: \n  {}",
                index + 1,
                line.trim()
            );
            for call in alpha_calls {
                assert!(
                    !line.contains(call),
                    "line {} feeds the material step into a native alpha call ({call}): \n  {}",
                    index + 1,
                    line.trim()
                );
            }
        }
        // A guard against the whitelist silently going vacuous: the step must
        // still be a real, referenced field. `normalize_glass_step` is its one
        // true consumer in the code section.
        assert!(
            seen >= 4,
            "expected the step in its field, its default, its migration and its normalizer; saw {seen}"
        );
    }

    #[test]
    fn normalize_settings_clamps_the_step_and_both_transparencies() {
        let settings = normalize_settings(AppSettings {
            main_opacity: 0,
            terminal_opacity: 0,
            glass_step: "ultra".into(),
            ..AppSettings::default()
        });
        assert_eq!(settings.main_opacity, DEFAULT_MAIN_OPACITY);
        assert_eq!(settings.terminal_opacity, DEFAULT_TERMINAL_OPACITY);
        assert_eq!(settings.glass_step, DEFAULT_GLASS_STEP);
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
