use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, HashMap};
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

// R42 · the terminal's appearance axes. The string domains match
// `terminal/terminal-appearance.ts`; Rust cannot import TypeScript, so the two
// tables are pinned against each other by `tests/terminal-settings.test.ts`.
// The line height is a multiple of the font size; the padding is one of three
// named steps whose pixel values live in the frontend (the backend only has to
// keep the id legal).
const MIN_LINE_HEIGHT: f64 = 1.0;
const MAX_LINE_HEIGHT: f64 = 2.0;
const DEFAULT_LINE_HEIGHT: f64 = 1.4;
const DEFAULT_TERMINAL_PADDING: &str = "regular";
const TERMINAL_PADDINGS: [&str; 3] = ["compact", "regular", "relaxed"];
const DEFAULT_TERMINAL_THEME: &str = "inherit";
const TERMINAL_THEMES: [&str; 3] = ["inherit", "contrast", "paper"];

/// R7-13c · the three interface-size steps and the `--ui-scale` multiplier each
/// one writes. The string domain matches `UiScale` in `src/ui-scale.ts`; the
/// numbers match `UI_SCALE_FACTORS` there. Rust cannot import TypeScript, so the
/// two tables are pinned against each other by
/// `tests/ui-scale-steps.test.ts`, exactly as `INPUT_WINDOW_WIDTH` is.
///
/// The value the *native* reset path needs is the multiplier: the launcher's
/// fallback height (`INPUT_WINDOW_HEIGHT`, a scale-1 measurement) is multiplied
/// by it before the window is re-homed, so the pre-frontend frame is already
/// the right size for the chosen step. Every other consumer is CSS.
pub const UI_SCALE_STEPS: [(&str, f64); 3] = [("default", 1.0), ("large", 1.1), ("larger", 1.25)];

/// The shipped step when the key is absent or names something unknown. Every
/// build before R7-13c shipped this step, so an older or hand-edited settings
/// file keeps the scale the user never chose — `default`.
pub const DEFAULT_UI_SCALE: &str = "default";

/// Serde default for `ui_scale`. Named rather than a bare `#[serde(default)]`
/// so an absent key lands on the shipped step instead of on `String::default()`
/// (`""`): the same reason `show_menubar_icon` has `default_true`.
pub fn default_ui_scale() -> String {
    DEFAULT_UI_SCALE.to_string()
}

/// Map a stored step onto its multiplier. Any unknown value (an empty string, a
/// typo, a number) falls back to `default` rather than to whichever entry sorts
/// first: a scale the user did not pick must not be applied.
pub fn ui_scale_factor(step: &str) -> f64 {
    UI_SCALE_STEPS
        .iter()
        .find(|(id, _)| *id == step)
        .map(|(_, factor)| *factor)
        .unwrap_or(1.0)
}

/// Serde default for the switches that ship on. Named rather than a closure so
/// the migration is a function the tests can call: a settings file written
/// before the key existed has to come back as `true`, which is what keeps
/// R7-10c from silently hiding every existing user's menu bar icon.
pub fn default_true() -> bool {
    true
}

/// R42 · the terminal's line-height multiple. `#[serde(default = ...)]` keeps a
/// settings file written before this key existing at the shipped 1.4 rather
/// than at `f64::default()` (`0.0`, which would collapse every row).
pub fn default_terminal_line_height() -> f64 {
    DEFAULT_LINE_HEIGHT
}

/// R42 · the terminal's padding step. `regular` is the 3px every earlier build
/// shipped, so a pre-round file renders pixel-identically.
pub fn default_terminal_padding() -> String {
    DEFAULT_TERMINAL_PADDING.to_string()
}

/// R42 · the terminal palette. `inherit` keeps the shipped behaviour (the
/// canvas reads the `--terminal-*` tokens for the app's theme).
pub fn default_terminal_theme() -> String {
    DEFAULT_TERMINAL_THEME.to_string()
}

/// R35 · the page-residency window: how long a surface survives a dismissal
/// before a summon returns to the search box. The number matches
/// `DEFAULT_RESIDENCY_SECONDS` / `RESIDENCY_MAX_SECONDS` in
/// `src/surface-residency.ts`; the two tables are pinned against each other by
/// `tests/surface-residency.test.ts`, the same arrangement `UI_SCALE_STEPS`
/// uses. The frontend owns the clock — Rust only stores and clamps the number.
///
/// R41 · the ceiling is no longer the last preset (30s) but the custom
/// domain's own maximum, and [`SURFACE_RESIDENCY_NEVER_SECONDS`] is a sentinel
/// that passes through untouched. The field stays `u32` so every settings file
/// the earlier rounds wrote keeps deserializing.
pub const DEFAULT_SURFACE_RESIDENCY_SECONDS: u32 = 10;
/// R41 · the custom ceiling (one day). Matches
/// `RESIDENCY_CUSTOM_MAX_SECONDS` in `src/surface-residency.ts`.
pub const MAX_SURFACE_RESIDENCY_SECONDS: u32 = 86_400;
/// R41 · "never": the surface survives any automatic dismissal until the user
/// leaves it explicitly. The top of `u32`, so the persisted field can stay
/// `u32` and the frontend's `RESIDENCY_NEVER_SECONDS` is the same number.
/// The normalizer exempts it from the custom ceiling — a sentinel that got
/// clamped to a day would silently stop being "never".
pub const SURFACE_RESIDENCY_NEVER_SECONDS: u32 = u32::MAX;

/// Serde default for `surface_residency_seconds`. Named rather than a bare
/// `#[serde(default)]` so a settings file written before the key existed lands
/// on the shipped ten seconds, not on `u32::default()` (`0`, the disabled
/// state — which would silently turn the round's feature off for every
/// existing user).
pub fn default_surface_residency_seconds() -> u32 {
    DEFAULT_SURFACE_RESIDENCY_SECONDS
}

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

/// R26-A: the built-in browser plugin's own settings.
///
/// The plugin ships working out of the box, so every field has a default that
/// means "decide for me": `target = "auto"` picks the browser with data, and
/// `custom_base_dir = None` leaves discovery to the platform table.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(default)]
pub struct BrowserPluginSettings {
    /// R26-D · whether the plugin is switched on. The browser plugin used to be
    /// unconditionally available (its only trigger was a typed word), so it had
    /// no switch; the user asked for one so it behaves like the clipboard base
    /// plugin. `true` is the shipped state, and the explicit `default_true` is
    /// what makes a settings file written before this key existed (R26-A/B)
    /// deserialize to the behaviour it shipped with rather than to
    /// `bool::default()`.
    #[serde(default = "default_true")]
    pub enabled: bool,
    /// Which browser the launcher searches: `"auto"` or a browser id
    /// (`"chrome"`, `"edge"`, `"brave"`, `"chromium"`, `"custom"`).
    pub target: String,
    /// An extra directory to discover as a `custom` browser, for an install
    /// the platform table does not know about.
    pub custom_base_dir: Option<String>,
    /// How far back history search looks, in days. `0` disables the filter.
    pub history_days: u32,
    /// R26-B: whether to read live tabs from the browser's DevTools debug
    /// endpoint. Off by default — the endpoint only exists when the browser was
    /// started with `--remote-debugging-port`, so this is an explicit opt-in,
    /// never something the plugin turns on behind the user's back. Ignored on
    /// macOS, where tabs come from AppleScript instead.
    pub cdp_enabled: bool,
    /// R26-B: the port the debug endpoint listens on. The browser's own default
    /// is 9222; the setting exists because a second browser instance has to
    /// pick another one.
    pub cdp_port: u16,
    /// R27 · how bookmark and history results are ordered: `"relevance"` (the
    /// ranking the launcher has always used), `"recent"`, `"alphabetical"` or
    /// `"visits"`. An unknown value normalizes back to `"relevance"`, so a
    /// hand-edited file cannot leave the list unsorted.
    pub sort_order: String,
    /// R32 · which fields the launcher's browser search matches: `"all"`
    /// (title or URL, the shipped behaviour), `"title"` or `"url"`. The
    /// launcher applies the value in memory; the backend only persists and
    /// normalizes it. A settings file written before this key existed
    /// deserializes to `"all"` through the container's `#[serde(default)]`.
    pub search_fields: String,
}

impl Default for BrowserPluginSettings {
    fn default() -> Self {
        Self {
            enabled: true,
            target: "auto".to_string(),
            custom_base_dir: None,
            history_days: 30,
            cdp_enabled: false,
            cdp_port: crate::browser_data::tabs::DEFAULT_CDP_PORT,
            sort_order: DEFAULT_BROWSER_SORT_ORDER.to_string(),
            search_fields: DEFAULT_BROWSER_SEARCH_FIELDS.to_string(),
        }
    }
}

/// The shipped sort order, and the value every other spelling falls back to.
pub const DEFAULT_BROWSER_SORT_ORDER: &str = "relevance";

/// R32 · the shipped search-field setting, and the fallback for anything else.
pub const DEFAULT_BROWSER_SEARCH_FIELDS: &str = "all";

/// R32 · the three fields the browser search can match, in the order the
/// settings radio lists them. `all` is the OR of the other two.
pub const BROWSER_SEARCH_FIELDS: [&str; 3] = ["all", "title", "url"];

/// Accept one of [`BROWSER_SEARCH_FIELDS`]; anything else is `all`.
pub fn normalize_browser_search_fields(value: &str) -> String {
    let value = value.trim().to_lowercase();
    if BROWSER_SEARCH_FIELDS.contains(&value.as_str()) {
        value
    } else {
        DEFAULT_BROWSER_SEARCH_FIELDS.to_string()
    }
}

/// R27 · the four sort orders the browser plugin offers, in the order its
/// settings card lists them. `relevance` is the launcher's own ranking (the
/// backend's match score); the other three are explicit orderings a bookmark
/// tool is expected to offer.
pub const BROWSER_SORT_ORDERS: [&str; 4] =
    ["relevance", "recent", "alphabetical", "visits"];

/// Accept one of {@link BROWSER_SORT_ORDERS}; anything else is `relevance`.
pub fn normalize_browser_sort_order(value: &str) -> String {
    let value = value.trim().to_lowercase();
    if BROWSER_SORT_ORDERS.contains(&value.as_str()) {
        value
    } else {
        DEFAULT_BROWSER_SORT_ORDER.to_string()
    }
}

/// R27 · the clipboard plugin's own settings block, as its page reads and
/// writes it.
///
/// One field today (the capacity). A struct rather than a bare number for the
/// same reason the browser plugin has one: the page's narrow command pair needs
/// a shape to grow into, and a JSON object is the shape the settings file
/// already uses for the browser block. Deliberately **not** `rename_all` — the
/// browser block and the rest of `AppSettings` are snake_case over the wire, and
/// a second convention for one field would be the surprise.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(default)]
pub struct ClipboardPluginSettings {
    pub max_items: u32,
}

impl Default for ClipboardPluginSettings {
    fn default() -> Self {
        Self {
            max_items: DEFAULT_CLIPBOARD_MAX_ITEMS,
        }
    }
}

/// R27 · persist the clipboard capacity and return the stored (normalized)
/// value.
///
/// A function rather than a command so the clipboard module — which owns the
/// history cache and is feature-gated — can write this one field without
/// reaching into the settings lock or duplicating the normalization.
#[cfg(feature = "clipboard-history")]
pub fn write_clipboard_max_items(max_items: u32) -> Result<u32, String> {
    let _guard = settings_lock()?;
    let mut settings = load_settings();
    settings.clipboard_history_max_items = max_items;
    let settings = normalize_settings(settings);
    write_settings(&settings)?;
    Ok(settings.clipboard_history_max_items)
}

/// Missing keys fall back to `Default`, so settings files written by older
/// builds keep working when new fields are introduced.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(default)]
pub struct AppSettings {
    pub hotkey: String,
    pub hide_on_blur: bool,
    /// R35 · how many seconds a surface (a plugin mode, its configuration
    /// overlay, the settings panel, the terminal view) survives after the panel
    /// is dismissed, before a summon returns to the search box. `0` disables
    /// the window and restores the pre-R35 behaviour. The frontend owns the
    /// clock; this is only the number it reads. `#[serde(default = ...)]` keeps
    /// every settings file written before this key existed deserializing to the
    /// shipped ten seconds rather than to `u32::default()` (`0` — which would
    /// silently turn the feature off).
    #[serde(default = "default_surface_residency_seconds")]
    pub surface_residency_seconds: u32,
    pub launch_at_startup: bool,
    /// UI theme: "dark" | "light" | "auto". Only the frontend reads it — "auto"
    /// is resolved there against the system appearance.
    pub theme: String,
    pub font_size: u32,
    pub font_family: String,
    /// Default cursor shape: "beam" | "block" | "underline".
    pub cursor_shape: String,
    /// R42 · the terminal's line-height multiple (1.0–2.0, default 1.4). The
    /// renderer clamps it too; this is the persisted, hand-edit-proof copy.
    #[serde(default = "default_terminal_line_height")]
    pub terminal_line_height: f64,
    /// R42 · the canvas's inner inset as a named step: "compact" | "regular"
    /// | "relaxed". `regular` is the 3px every earlier build shipped.
    #[serde(default = "default_terminal_padding")]
    pub terminal_padding: String,
    /// R42 · the user's cursor-blink veto (default on, the shipped behaviour).
    /// The program's own request still arrives on the wire; this is the user's
    /// override, applied by the canvas renderer.
    #[serde(default = "default_true")]
    pub terminal_cursor_blink: bool,
    /// R42 · the canvas palette: "inherit" | "contrast" | "paper". A canvas
    /// colour mapping only — the R30 base.css material/glass formulas and every
    /// global token are untouched.
    #[serde(default = "default_terminal_theme")]
    pub terminal_theme: String,
    /// R42 · whether the scrollbar overlay may be drawn (default on).
    #[serde(default = "default_true")]
    pub terminal_scrollbar: bool,
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
    /// R27 · how many non-favorite clipboard entries the history keeps. The
    /// long-standing constant was 300; the clipboard plugin's own settings page
    /// exposes it (10–500). Favorites are exempt from the cap and from the
    /// retention window, exactly as before.
    pub clipboard_history_max_items: u32,
    /// Application path -> launch count, ranking the launcher's empty-query
    /// recent list. Owned by the frontend; no dedicated command persists it.
    pub launch_counts: HashMap<String, u32>,
    /// Settings page the user last had open, restored on the next launch.
    /// Owned by the frontend, which validates the value before saving.
    pub last_settings_page: String,
    /// R7-10c: whether the macOS menu bar status item / Windows+Linux tray
    /// icon is shown (default on). Off hides the icon and nothing else — the
    /// global hotkey, the settings page and the deep-link router are all
    /// independent summon paths and stay intact.
    ///
    /// The explicit `default_true` is what makes an older settings file (no
    /// key at all) deserialize to the behaviour it shipped with, rather than
    /// to `bool::default()`.
    #[serde(default = "default_true")]
    pub show_menubar_icon: bool,
    /// R7-13c · the interface-size step: "default" | "large" | "larger".
    ///
    /// This is the one settings field the CSS `--ui-scale` knob reads: the
    /// frontend writes the step's multiplier onto the document root, and every
    /// box dimension and type step in the stylesheet derives from it. The
    /// native side reads it only to scale the launcher's fallback height (see
    /// [`ui_scale_factor`] and `INPUT_WINDOW_HEIGHT` in `lib.rs`).
    ///
    /// `#[serde(default = "default_ui_scale")]` is what keeps every settings file
    /// written before this round deserializing to `default` — the step those
    /// builds shipped.
    #[serde(default = "default_ui_scale")]
    pub ui_scale: String,
    /// R7-11: user-defined per-command aliases for launcher search, keyed by
    /// the catalog command name (`"git"` -> `"gfm"`). The key is the command
    /// name, so a second write for the same command overwrites the first while
    /// two *commands* sharing one alias value are resolved by
    /// [`resolve_command_aliases`] (first command in ascending name order locks
    /// the alias, later duplicates are inert). `#[serde(default)]` keeps every
    /// settings file written before this key existed deserializing to an empty
    /// map.
    #[serde(default)]
    pub command_aliases: HashMap<String, String>,
    /// R26-A: the built-in browser plugin (bookmarks / history search).
    ///
    /// `#[serde(default)]` keeps every settings file written before this key
    /// existed deserializing to the shipped behaviour (auto-detect, no custom
    /// directory, 30-day history window).
    #[serde(default)]
    pub browser_plugin: BrowserPluginSettings,
    /// R39 · the per-command switches of external (non-built-in) plugins:
    /// `extensionId -> commandId -> enabled`.
    ///
    /// An external plugin's commands are declared by the extension's own
    /// provider descriptor (`CommandDescriptor`, see
    /// `extensions::provider`). Each one gets a switch in the integrations
    /// panel; the launcher's plugin mode for that command exists only while
    /// the switch is on, and the mode's Enter runs the command through the
    /// same execution plan the catalog already builds (`provider::execution_plan`
    /// — no shell, no new allowlist).
    ///
    /// **Absence means off.** A command with no entry here has never been
    /// enabled by the user, so the plugin is not summonable: the user's own
    /// words were 「打开后就可以允许在搜索框内呼出插件」. `#[serde(default)]`
    /// keeps every settings file written before this key existing
    /// deserializing to an empty map — which is exactly the pre-round
    /// behaviour of "no external plugin mode exists".
    #[serde(default)]
    pub plugin_command_switches: BTreeMap<String, BTreeMap<String, bool>>,
}

impl Default for AppSettings {
    fn default() -> Self {
        Self {
            hotkey: DEFAULT_TOGGLE_WINDOW.to_string(),
            // R41 · the shipped value is still `true`, except under Hyprland,
            // where the user asked for it to default off (a tiled desktop's
            // focus hand-off makes blur-hide read as the panel vanishing for no
            // reason). Only a *fresh* settings file is born from this; an
            // existing key always wins. See `crate::hyprland`.
            hide_on_blur: crate::hyprland::default_hide_on_blur(),
            surface_residency_seconds: DEFAULT_SURFACE_RESIDENCY_SECONDS,
            launch_at_startup: false,
            theme: "auto".to_string(),
            font_size: 14,
            font_family: "monospace".to_string(),
            cursor_shape: "beam".to_string(),
            terminal_line_height: DEFAULT_LINE_HEIGHT,
            terminal_padding: DEFAULT_TERMINAL_PADDING.to_string(),
            terminal_cursor_blink: true,
            terminal_theme: DEFAULT_TERMINAL_THEME.to_string(),
            terminal_scrollbar: true,
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
            clipboard_history_max_items: DEFAULT_CLIPBOARD_MAX_ITEMS,
            launch_counts: HashMap::new(),
            last_settings_page: "general".to_string(),
            show_menubar_icon: default_true(),
            ui_scale: DEFAULT_UI_SCALE.to_string(),
            command_aliases: HashMap::new(),
            browser_plugin: BrowserPluginSettings::default(),
            plugin_command_switches: BTreeMap::new(),
        }
    }
}

/// Resolve the user's command aliases into the shape search consumes: one
/// effective alias per command, with cross-command conflict resolution applied.
///
/// Two commands may be given the same alias (the settings map is keyed by
/// command name, so nothing else prevents it). The policy is **first command
/// locks the alias**: commands are visited in ascending name order and the
/// first one to claim an alias keeps it; a later command whose alias collides is
/// inert for that alias (its command name still matches normally). This is
/// deterministic regardless of the `HashMap` iteration order, and — unlike
/// silently dropping the alias in the settings file — it never rewrites what the
/// user typed.
///
/// Empty (or whitespace-only) aliases are ignored, as is an alias that only
/// differs in case from one already claimed.
pub fn resolve_command_aliases(aliases: &HashMap<String, String>) -> HashMap<String, String> {
    let mut commands: Vec<&String> = aliases
        .keys()
        .filter(|name| !name.trim().is_empty())
        .collect();
    commands.sort();
    let mut claimed: std::collections::HashSet<String> = std::collections::HashSet::new();
    let mut resolved = HashMap::new();
    for command in commands {
        let alias = aliases[command].trim();
        if alias.is_empty() {
            continue;
        }
        if claimed.insert(alias.to_ascii_lowercase()) {
            resolved.insert(command.clone(), alias.to_string());
        }
    }
    resolved
}

/// Platform defaults for every configurable shortcut.
///
/// `select_result` holds the binding for the *first* result; the digits 2-9 and
/// 0 reuse its modifiers, so recording `Cmd+1` rebinds the whole 1-9-and-0
/// range (R36 added `0` as the family's tenth key).
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
    // R42 · the terminal's appearance. A non-finite or out-of-range line height
    // falls back to the shipped 1.4 (the renderer would clamp anyway, but the
    // stored value has to be sane too); an unknown padding step or palette
    // falls back to the shipped one rather than to whichever variant happens
    // to sort first.
    settings.terminal_line_height = if settings.terminal_line_height.is_finite() {
        settings
            .terminal_line_height
            .clamp(MIN_LINE_HEIGHT, MAX_LINE_HEIGHT)
    } else {
        DEFAULT_LINE_HEIGHT
    };
    settings.terminal_padding = if TERMINAL_PADDINGS.contains(&settings.terminal_padding.as_str()) {
        settings.terminal_padding
    } else {
        DEFAULT_TERMINAL_PADDING.to_string()
    };
    settings.terminal_theme = if TERMINAL_THEMES.contains(&settings.terminal_theme.as_str()) {
        settings.terminal_theme
    } else {
        DEFAULT_TERMINAL_THEME.to_string()
    };
    (settings.terminal_width, settings.terminal_height) =
        normalize_terminal_size(settings.terminal_width, settings.terminal_height);
    settings.main_opacity = normalize_window_opacity(settings.main_opacity, DEFAULT_MAIN_OPACITY);
    settings.terminal_opacity =
        normalize_window_opacity(settings.terminal_opacity, DEFAULT_TERMINAL_OPACITY);
    settings.glass_step = normalize_glass_step(&settings.glass_step);
    // R35 · the page-residency window. `0` is a legitimate value (the window is
    // off), so unlike the opacity sliders this does not map `0` to a default;
    // it only trims the hand-edited upper end. The floor is `0` by type.
    // R41 · the ceiling is the custom domain's day, and the "never" sentinel is
    // exempt so it survives the clamp.
    if settings.surface_residency_seconds != SURFACE_RESIDENCY_NEVER_SECONDS {
        settings.surface_residency_seconds = settings
            .surface_residency_seconds
            .min(MAX_SURFACE_RESIDENCY_SECONDS);
    }
    // R7-13c: an unknown or hand-edited step falls back to the shipped one
    // rather than to whichever variant happens to sort first — a scale the user
    // never chose must not be applied to the whole interface.
    settings.ui_scale = if UI_SCALE_STEPS
        .iter()
        .any(|(id, _)| *id == settings.ui_scale)
    {
        settings.ui_scale
    } else {
        DEFAULT_UI_SCALE.to_string()
    };
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
    // R27 · the clipboard capacity. The floor is what makes the setting a
    // *capacity* rather than an off switch (the switch is
    // `clipboard_history_enabled`); the ceiling keeps a hand-edited file from
    // asking the monitor to hold an unbounded index in memory.
    settings.clipboard_history_max_items = settings
        .clipboard_history_max_items
        .clamp(MIN_CLIPBOARD_MAX_ITEMS, MAX_CLIPBOARD_MAX_ITEMS);
    // R27 · the browser plugin's result ordering. An unknown value falls back to
    // the launcher's own ranking, never to an unsorted list.
    settings.browser_plugin.sort_order =
        normalize_browser_sort_order(&settings.browser_plugin.sort_order);
    // R32 · the launcher's search-field setting. An unknown value falls back to
    // matching title and URL, never to an empty result set.
    settings.browser_plugin.search_fields =
        normalize_browser_search_fields(&settings.browser_plugin.search_fields);
    // R26-A: the browser plugin's target is either `auto` or a browser id the
    // discovery table knows. An unknown value (a browser the user uninstalled,
    // a hand-edited file) falls back to `auto` rather than to a dead target.
    settings.browser_plugin.target = normalize_browser_target(&settings.browser_plugin.target);
    // An empty custom directory is the same as none; a path is kept verbatim
    // and only trimmed.
    settings.browser_plugin.custom_base_dir = settings
        .browser_plugin
        .custom_base_dir
        .map(|dir| dir.trim().to_string())
        .filter(|dir| !dir.is_empty());
    settings.browser_plugin.history_days = settings.browser_plugin.history_days.min(MAX_HISTORY_DAYS);
    // R26-B: a debug port of 0 is not a port; fall back to the browser's own
    // default rather than writing a value the connect call can never use.
    if settings.browser_plugin.cdp_port == 0 {
        settings.browser_plugin.cdp_port = crate::browser_data::tabs::DEFAULT_CDP_PORT;
    }
    // R39 · the external plugins' per-command switches. See
    // [`normalize_plugin_command_switches`].
    settings.plugin_command_switches =
        normalize_plugin_command_switches(settings.plugin_command_switches);
    settings
}

/// R39 · normalize the per-command switch map.
///
/// The keys are ids, so an empty or whitespace-only id can never match a
/// declared command and is dropped; an inner map left empty is dropped too, so
/// a plugin the user never touched has no entry at all — which is the same
/// state as "every command off", the meaning of absence (see
/// [`AppSettings::plugin_command_switches`]). Values are booleans and are kept
/// verbatim: `false` is a deliberate "this one is off" and must not be pruned
/// into a re-enabled default.
fn normalize_plugin_command_switches(
    switches: BTreeMap<String, BTreeMap<String, bool>>,
) -> BTreeMap<String, BTreeMap<String, bool>> {
    let mut normalized = BTreeMap::new();
    for (extension, commands) in switches {
        let extension = extension.trim();
        if extension.is_empty() {
            continue;
        }
        let mut kept = BTreeMap::new();
        for (command, enabled) in commands {
            let command = command.trim();
            if command.is_empty() {
                continue;
            }
            kept.insert(command.to_string(), enabled);
        }
        if !kept.is_empty() {
            normalized.insert(extension.to_string(), kept);
        }
    }
    normalized
}

/// The largest history window the plugin will honour (ten years). A larger
/// value is a typo, not a request.
const MAX_HISTORY_DAYS: u32 = 3650;

/// R27 · the clipboard capacity the plugin's settings page offers. The default
/// is the constant the store shipped with (300 non-favorite entries); the range
/// is wide enough to be a preference and narrow enough to be a bound.
pub const DEFAULT_CLIPBOARD_MAX_ITEMS: u32 = 300;
pub const MIN_CLIPBOARD_MAX_ITEMS: u32 = 10;
pub const MAX_CLIPBOARD_MAX_ITEMS: u32 = 500;

/// Accept `auto`, the `custom` slot, or any id in the discovery table.
pub fn normalize_browser_target(target: &str) -> String {
    let target = target.trim();
    if target.is_empty() || target == "auto" {
        return "auto".to_string();
    }
    if target == "custom" {
        return "custom".to_string();
    }
    let known = crate::browser_data::discover::candidate_browsers()
        .iter()
        .any(|candidate| candidate.browser_id == target);
    if known {
        target.to_string()
    } else {
        "auto".to_string()
    }
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
    // R27 · the clipboard capacity is owned by the clipboard plugin's own
    // settings page (through `clipboard_set_settings`), the same way the
    // browser plugin's data fields are owned by its page. A whole-app save
    // submits the frontend's snapshot of these, which is stale the moment the
    // plugin page writes — so the stored values win.
    submitted.clipboard_history_max_items = stored.clipboard_history_max_items;
    // R27 · the browser block's *data* fields are the page's; only the on/off
    // switch is the settings panel's. Taking the stored block and re-applying
    // the submitted switch keeps both owners honest: a whole-app save can no
    // longer revert a sort order or a directory the page just wrote, and the
    // panel's toggle still lands.
    let browser_enabled = submitted.browser_plugin.enabled;
    submitted.browser_plugin = stored.browser_plugin.clone();
    submitted.browser_plugin.enabled = browser_enabled;
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
            "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9"
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
    // R7-10c: the icon switch is applied on every save. This is separate from
    // the retitle above so a language change cannot resurrect a hidden icon —
    // `apply_tray_language` only ever writes menu labels.
    crate::apply_tray_visibility(&app, settings.show_menubar_icon);
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

/// R26-B: read the built-in browser plugin's own settings.
///
/// The plugin's page runs inside a sandboxed iframe and cannot reach the app's
/// `get_settings`/`save_settings` pair — those carry the whole app settings
/// object, which the page has no business rewriting. This narrow pair reads and
/// writes exactly the `browser_plugin` block, through the same settings lock and
/// the same atomic write every other settings change uses.
#[tauri::command]
pub fn browser_get_settings() -> BrowserPluginSettings {
    load_settings().browser_plugin
}

/// R26-B: replace the browser plugin's settings and return the normalized
/// result.
///
/// `normalize_browser_target` and the rest of `normalize_settings` still have
/// the last word, so an unknown browser id or a blank directory comes back as
/// the value that was actually stored. Changing `custom_base_dir` invalidates
/// the discovery cache explicitly: the cache is keyed by a directory signature
/// that does include the custom path, but a path that does not exist yet hashes
/// to nothing, so the explicit clear is what makes "point it at a new folder"
/// take effect on the very next scan rather than the next restart.
#[tauri::command]
pub fn browser_set_settings(settings: BrowserPluginSettings) -> Result<BrowserPluginSettings, String> {
    let _guard = settings_lock()?;
    let mut stored = load_settings();
    // The page's settings card owns the five data fields, not the on/off
    // switch: it sends no `enabled`, so serde would default it back to `true`
    // and a save from the card would silently re-enable a plugin the user had
    // switched off. Carry the stored flag across instead.
    let mut next = settings;
    next.enabled = stored.browser_plugin.enabled;
    stored.browser_plugin = next;
    let stored = normalize_settings(stored);
    write_settings(&stored)?;
    crate::browser_data::discover::clear_discovery_cache();
    Ok(stored.browser_plugin)
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
    fn an_older_settings_file_gets_the_shipped_browser_plugin_defaults() {
        // R26-A: a file written before the plugin existed has no
        // `browser_plugin` key at all. `#[serde(default)]` on the field must
        // produce the shipped behaviour — auto-detect, no custom dir, a
        // 30-day window — rather than an error or a zeroed struct.
        let settings: AppSettings =
            serde_json::from_str("{\"theme\":\"dark\"}").expect("old settings deserialize");
        assert_eq!(settings.browser_plugin, BrowserPluginSettings::default());
        assert_eq!(settings.browser_plugin.target, "auto");
        assert_eq!(settings.browser_plugin.custom_base_dir, None);
        assert_eq!(settings.browser_plugin.history_days, 30);
    }

    /// R26-D · the browser plugin's on/off switch. A file written by R26-A/B has
    /// no `enabled` key: it must deserialize to `true` (the behaviour those
    /// builds shipped), and an explicit `false` must survive.
    #[test]
    fn the_browser_plugin_switch_defaults_on_and_round_trips_off() {
        let shipped: AppSettings = serde_json::from_str(
            "{\"browser_plugin\":{\"target\":\"edge\",\"history_days\":9}}",
        )
        .expect("R26-A settings deserialize");
        assert!(
            shipped.browser_plugin.enabled,
            "a settings file without the switch deserializes to the shipped state (on)"
        );

        let off: AppSettings =
            serde_json::from_str("{\"browser_plugin\":{\"enabled\":false}}")
                .expect("disabled settings deserialize");
        assert!(!off.browser_plugin.enabled, "an explicit off is honoured");
    }

    /// R32 · the browser search-field setting normalizes to one of three values,
    /// and a settings file written before the key existed deserializes to `all`.
    #[test]
    fn the_browser_search_fields_normalize_and_default_to_all() {
        assert_eq!(DEFAULT_BROWSER_SEARCH_FIELDS, "all");
        assert_eq!(normalize_browser_search_fields("all"), "all");
        assert_eq!(normalize_browser_search_fields("TITLE"), "title");
        assert_eq!(normalize_browser_search_fields(" url "), "url");
        assert_eq!(normalize_browser_search_fields("body"), "all");
        assert_eq!(normalize_browser_search_fields(""), "all");
        assert_eq!(AppSettings::default().browser_plugin.search_fields, "all");

        // A file without the key (every build before this round) keeps the
        // shipped behaviour rather than matching nothing.
        let legacy: AppSettings =
            serde_json::from_str("{\"browser_plugin\":{\"target\":\"edge\"}}")
                .expect("legacy settings deserialize");
        assert_eq!(legacy.browser_plugin.search_fields, "all");

        // A hand-edited value is normalized on save.
        let normalized = normalize_settings(AppSettings {
            browser_plugin: BrowserPluginSettings {
                search_fields: "BODY".into(),
                ..BrowserPluginSettings::default()
            },
            ..AppSettings::default()
        });
        assert_eq!(normalized.browser_plugin.search_fields, "all");
    }

    /// R35/R41 · the page-residency window. It ships at ten seconds, `0` is a
    /// legitimate "off" (so it must not be mapped to a default the way the
    /// opacity clamp maps a zero), the custom ceiling is one day, the "never"
    /// sentinel passes through untouched, and a settings file written before the
    /// key existed deserializes to the shipped ten rather than to
    /// `u32::default()` (`0`, the disabled state).
    #[test]
    fn the_surface_residency_ships_at_ten_clamps_at_a_day_and_keeps_never() {
        assert_eq!(DEFAULT_SURFACE_RESIDENCY_SECONDS, 10);
        assert_eq!(MAX_SURFACE_RESIDENCY_SECONDS, 86_400);
        assert_eq!(SURFACE_RESIDENCY_NEVER_SECONDS, u32::MAX);
        assert_eq!(default_surface_residency_seconds(), 10);
        assert_eq!(
            AppSettings::default().surface_residency_seconds,
            DEFAULT_SURFACE_RESIDENCY_SECONDS,
        );

        // A file without the key (every build before this round) keeps the
        // shipped window rather than silently disabling it.
        let legacy: AppSettings =
            serde_json::from_str("{\"hide_on_blur\":true}").expect("legacy settings deserialize");
        assert_eq!(legacy.surface_residency_seconds, 10);

        // `0` survives normalization: it is the off switch, not a missing
        // value.
        let off = normalize_settings(AppSettings {
            surface_residency_seconds: 0,
            ..AppSettings::default()
        });
        assert_eq!(off.surface_residency_seconds, 0);

        // A preset and a custom value inside the day both survive.
        for kept in [30, 120, 3_600, MAX_SURFACE_RESIDENCY_SECONDS] {
            let normalized = normalize_settings(AppSettings {
                surface_residency_seconds: kept,
                ..AppSettings::default()
            });
            assert_eq!(normalized.surface_residency_seconds, kept);
        }

        // The upper end is trimmed; the floor is `0` by type.
        let trimmed = normalize_settings(AppSettings {
            surface_residency_seconds: MAX_SURFACE_RESIDENCY_SECONDS + 1,
            ..AppSettings::default()
        });
        assert_eq!(
            trimmed.surface_residency_seconds,
            MAX_SURFACE_RESIDENCY_SECONDS
        );

        // The "never" sentinel is exempt from the ceiling: it is not a long
        // duration, it is a different state.
        let never = normalize_settings(AppSettings {
            surface_residency_seconds: SURFACE_RESIDENCY_NEVER_SECONDS,
            ..AppSettings::default()
        });
        assert_eq!(
            never.surface_residency_seconds,
            SURFACE_RESIDENCY_NEVER_SECONDS
        );
    }

    #[test]
    fn the_browser_target_normalizes_to_a_known_id_or_auto() {
        assert_eq!(normalize_browser_target(""), "auto");
        assert_eq!(normalize_browser_target("  auto "), "auto");
        assert_eq!(normalize_browser_target("chrome"), "chrome");
        assert_eq!(normalize_browser_target("edge"), "edge");
        assert_eq!(normalize_browser_target("custom"), "custom");
        // A browser this table does not know (Firefox stores its data in a
        // different format entirely) must not become a dead target.
        assert_eq!(normalize_browser_target("firefox"), "auto");
    }

    #[test]
    fn the_history_window_is_clamped_and_an_empty_custom_dir_is_dropped() {
        let settings = normalize_settings(AppSettings {
            browser_plugin: BrowserPluginSettings {
                enabled: true,
                target: "firefox".into(),
                custom_base_dir: Some("   ".into()),
                history_days: u32::MAX,
                // R26-B: 0 is not a port; the normalizer restores the default.
                cdp_enabled: true,
                cdp_port: 0,
                // R27 · an unknown ordering normalizes back to the launcher's
                // own ranking rather than leaving the list unsorted.
                sort_order: "sideways".into(),
                // R32 · an unknown search field normalizes to `all`.
                search_fields: "body".into(),
            },
            ..AppSettings::default()
        });
        assert_eq!(settings.browser_plugin.target, "auto");
        assert_eq!(settings.browser_plugin.custom_base_dir, None);
        assert_eq!(settings.browser_plugin.history_days, MAX_HISTORY_DAYS);
        assert_eq!(
            settings.browser_plugin.cdp_port,
            crate::browser_data::tabs::DEFAULT_CDP_PORT
        );
        assert!(settings.browser_plugin.cdp_enabled);
        assert_eq!(settings.browser_plugin.sort_order, "relevance");
        assert_eq!(settings.browser_plugin.search_fields, "all");

        let kept = normalize_settings(AppSettings {
            browser_plugin: BrowserPluginSettings {
                enabled: true,
                target: "auto".into(),
                custom_base_dir: Some(" /opt/browser ".into()),
                history_days: 7,
                cdp_enabled: false,
                cdp_port: 9333,
                sort_order: "recent".into(),
                search_fields: "url".into(),
            },
            ..AppSettings::default()
        });
        assert_eq!(
            kept.browser_plugin.custom_base_dir.as_deref(),
            Some("/opt/browser")
        );
        assert_eq!(kept.browser_plugin.history_days, 7);
        assert_eq!(kept.browser_plugin.cdp_port, 9333);
        assert!(!kept.browser_plugin.cdp_enabled);
        assert_eq!(kept.browser_plugin.sort_order, "recent");
        assert_eq!(kept.browser_plugin.search_fields, "url");
    }

    /// R27 · the clipboard capacity is clamped on save, ships at the value the
    /// store always had, and — like the browser plugin's data fields — is owned
    /// by the plugin's own page, so a whole-app settings save cannot reset it.
    #[test]
    fn the_clipboard_capacity_is_clamped_and_owned_by_the_plugin_page() {
        let clamp = |value: u32| {
            normalize_settings(AppSettings {
                clipboard_history_max_items: value,
                ..AppSettings::default()
            })
            .clipboard_history_max_items
        };
        assert_eq!(clamp(0), MIN_CLIPBOARD_MAX_ITEMS);
        assert_eq!(clamp(10), 10);
        assert_eq!(clamp(500), 500);
        assert_eq!(clamp(u32::MAX), MAX_CLIPBOARD_MAX_ITEMS);
        // The shipped default is the capacity the store always had.
        assert_eq!(DEFAULT_CLIPBOARD_MAX_ITEMS, 300);
        assert_eq!(AppSettings::default().clipboard_history_max_items, 300);

        // The stored snapshot is the authority for both plugin-owned blocks…
        let mut stored = normalize_settings(AppSettings {
            clipboard_history_max_items: 50,
            ..AppSettings::default()
        });
        stored.browser_plugin.sort_order = "recent".into();
        // …and the frontend's snapshot is stale on both, but carries the one
        // field the settings *panel* owns: the browser plugin's switch.
        let submitted = AppSettings {
            clipboard_history_max_items: 300,
            browser_plugin: BrowserPluginSettings {
                enabled: false,
                ..BrowserPluginSettings::default()
            },
            ..AppSettings::default()
        };
        let merged = merge_frontend_settings(submitted, &stored);
        assert_eq!(
            merged.clipboard_history_max_items, 50,
            "a whole-app save must not reset the plugin page's capacity"
        );
        assert_eq!(
            merged.browser_plugin.sort_order, "recent",
            "a whole-app save must not reset the plugin page's sort order"
        );
        assert!(
            !merged.browser_plugin.enabled,
            "the settings panel's own switch still lands"
        );
    }

    /// R26-B · the settings round trip the plugin page depends on: what
    /// `browser_set_settings` writes is exactly what `browser_get_settings`
    /// reads back, and the R26-A block keeps its meaning across the round trip.
    #[test]
    fn the_browser_plugin_settings_survive_a_write_and_read() {
        let directory = tempfile::tempdir().expect("temp dir");
        let settings = normalize_settings(AppSettings {
            browser_plugin: BrowserPluginSettings {
                enabled: true,
                target: "brave".into(),
                custom_base_dir: Some("/data/brave".into()),
                history_days: 14,
                cdp_enabled: true,
                cdp_port: 9333,
                sort_order: "alphabetical".into(),
                search_fields: "title".into(),
            },
            ..AppSettings::default()
        });
        write_settings_to(directory.path(), &settings).expect("write settings");
        let read_back = read_settings(&directory.path().join(SETTINGS_FILE_NAME))
            .expect("read settings back");
        assert_eq!(read_back.browser_plugin, settings.browser_plugin);
        assert_eq!(read_back.browser_plugin.target, "brave");
        assert_eq!(
            read_back.browser_plugin.custom_base_dir.as_deref(),
            Some("/data/brave")
        );
        assert_eq!(read_back.browser_plugin.history_days, 14);
        assert!(read_back.browser_plugin.cdp_enabled);
        assert_eq!(read_back.browser_plugin.cdp_port, 9333);
        // R27 · the sort order is part of the round trip like every other field.
        assert_eq!(read_back.browser_plugin.sort_order, "alphabetical");
    }

    #[test]
    fn older_browser_settings_get_the_debug_port_defaults() {
        // A settings file written by R26-A has a `browser_plugin` block without
        // the R26-B keys. The plugin must open with tab capture off and the
        // browser's own port, not with a zeroed port it could never connect to.
        let settings: AppSettings = serde_json::from_str(
            "{\"browser_plugin\":{\"target\":\"chrome\",\"history_days\":9}}",
        )
        .expect("R26-A settings deserialize");
        assert_eq!(settings.browser_plugin.target, "chrome");
        assert_eq!(settings.browser_plugin.history_days, 9);
        assert!(!settings.browser_plugin.cdp_enabled);
        assert_eq!(
            settings.browser_plugin.cdp_port,
            crate::browser_data::tabs::DEFAULT_CDP_PORT
        );
    }

    #[test]
    fn older_settings_keep_the_menu_bar_icon() {
        // R7-10c migration: a file written before the switch existed has no
        // `show_menubar_icon` key. `bool::default()` would be `false` and would
        // hide the icon of every existing user on upgrade, so the field carries
        // an explicit `default_true` — and this test is the lock on it.
        let settings: AppSettings =
            serde_json::from_str("{\"theme\":\"dark\"}").expect("old settings deserialize");
        assert!(settings.show_menubar_icon);
        assert!(default_true());
    }

    /// R7-13c · the interface-size step's migration. A file written before the
    /// round has no `ui_scale` key; `String::default()` would be `""`, which
    /// names no shipped step. The field's bare `#[serde(default)]` plus the
    /// default struct value must land on the shipped step (`default`), so an
    /// upgrading user is not silently rescaled.
    #[test]
    fn older_settings_keep_the_default_interface_scale() {
        let settings: AppSettings =
            serde_json::from_str("{\"theme\":\"dark\"}").expect("old settings deserialize");
        assert_eq!(settings.ui_scale, DEFAULT_UI_SCALE);
        assert_eq!(AppSettings::default().ui_scale, DEFAULT_UI_SCALE);
        // An explicit choice survives the round-trip intact.
        let stored = normalize_settings(AppSettings {
            ui_scale: "larger".into(),
            ..AppSettings::default()
        });
        assert_eq!(stored.ui_scale, "larger");
    }

    /// A hand-edited or unknown step is normalized to the shipped one, not
    /// passed through: the CSS knob is written from this value, so an unknown
    /// id would leave the interface at whatever `--ui-scale` last held.
    #[test]
    fn an_unknown_interface_scale_falls_back_to_default() {
        for unknown in ["", "huge", "1.1", "LARGE", "standard"] {
            let settings = normalize_settings(AppSettings {
                ui_scale: unknown.into(),
                ..AppSettings::default()
            });
            assert_eq!(
                settings.ui_scale, DEFAULT_UI_SCALE,
                "{unknown:?} is not a shipped step"
            );
        }
        // The three shipped ids pass through untouched.
        for step in ["default", "large", "larger"] {
            let settings = normalize_settings(AppSettings {
                ui_scale: step.into(),
                ..AppSettings::default()
            });
            assert_eq!(settings.ui_scale, step);
        }
    }

    /// R7-13c · the step -> multiplier table. The native fallback height (see
    /// `scaled_input_window_height` in `lib.rs`) multiplies a scale-1 constant
    /// by this factor, so the table is load-bearing on the native side and must
    /// match `UI_SCALE_FACTORS` in `src/ui-scale.ts` (pinned by
    /// `tests/ui-scale-steps.test.ts`).
    #[test]
    fn the_interface_scale_factors_are_the_shipped_steps() {
        assert_eq!(ui_scale_factor("default"), 1.0);
        assert_eq!(ui_scale_factor("large"), 1.1);
        assert_eq!(ui_scale_factor("larger"), 1.25);
        // An unknown step is the default factor, never `0` (a zero-height
        // window) and never a panic.
        assert_eq!(ui_scale_factor(""), 1.0);
        assert_eq!(ui_scale_factor("nonsense"), 1.0);
    }

    #[test]
    fn the_interface_size_field_is_written_back_verbatim() {
        // The field rides the ordinary settings write channel; this pins that
        // `merge_frontend_settings` does not reset it (it is a frontend-owned
        // field, like `theme`).
        let stored = AppSettings::default();
        let submitted = AppSettings {
            ui_scale: "large".into(),
            ..AppSettings::default()
        };
        assert_eq!(
            merge_frontend_settings(submitted, &stored).ui_scale,
            "large"
        );
    }

    #[test]
    fn an_explicitly_hidden_menu_bar_icon_round_trips() {
        // The other half of the migration: once the user turns the switch off,
        // the value survives a write/read cycle as `false` rather than being
        // re-defaulted back to `true`.
        let directory = tempfile::tempdir().expect("settings directory");
        let settings = AppSettings {
            show_menubar_icon: false,
            ..AppSettings::default()
        };
        write_settings_to(directory.path(), &settings).expect("write settings");

        let reloaded = load_settings_from(directory.path());
        assert!(!reloaded.show_menubar_icon);
    }

    #[test]
    fn the_frontend_snapshot_owns_the_menu_bar_icon_switch() {
        // The switch is read and written by the settings page, so its value
        // must come from the submitted snapshot rather than the stored one —
        // `merge_frontend_settings` only overrides the fields a dedicated
        // command owns.
        let stored = AppSettings::default();
        let submitted = AppSettings {
            show_menubar_icon: false,
            ..AppSettings::default()
        };
        let merged = merge_frontend_settings(submitted, &stored);
        assert!(!merged.show_menubar_icon);
    }

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

    /// R42 · the terminal's appearance axes. A hand-edited or out-of-range
    /// value lands on the shipped one rather than on a variant that would
    /// silently change how the canvas paints; the number is clamped, the two
    /// enums are validated.
    #[test]
    fn normalize_settings_guards_the_terminal_appearance() {
        let settings = normalize_settings(AppSettings {
            terminal_line_height: 9.0,
            terminal_padding: "enormous".into(),
            terminal_theme: "solarized".into(),
            ..AppSettings::default()
        });
        assert_eq!(settings.terminal_line_height, MAX_LINE_HEIGHT);
        assert_eq!(settings.terminal_padding, DEFAULT_TERMINAL_PADDING);
        assert_eq!(settings.terminal_theme, DEFAULT_TERMINAL_THEME);

        // Below the floor clamps up, and a non-finite value falls back.
        let low = normalize_settings(AppSettings {
            terminal_line_height: 0.1,
            ..AppSettings::default()
        });
        assert_eq!(low.terminal_line_height, MIN_LINE_HEIGHT);
        let nan = normalize_settings(AppSettings {
            terminal_line_height: f64::NAN,
            ..AppSettings::default()
        });
        assert_eq!(nan.terminal_line_height, DEFAULT_LINE_HEIGHT);

        // The legitimate ids survive untouched.
        let kept = normalize_settings(AppSettings {
            terminal_line_height: 1.65,
            terminal_padding: "relaxed".into(),
            terminal_theme: "paper".into(),
            terminal_cursor_blink: false,
            terminal_scrollbar: false,
            ..AppSettings::default()
        });
        assert_eq!(kept.terminal_line_height, 1.65);
        assert_eq!(kept.terminal_padding, "relaxed");
        assert_eq!(kept.terminal_theme, "paper");
        assert!(!kept.terminal_cursor_blink);
        assert!(!kept.terminal_scrollbar);
    }

    /// R42 · a settings file written before the keys existed must come back as
    /// the shipped behaviour: a blinking cursor, a visible scrollbar, the
    /// inherited palette and the 1.4 line height — never `Default::default()`,
    /// which would turn the first two off and collapse the rows.
    #[test]
    fn pre_round_files_get_the_shipped_terminal_appearance() {
        let mut value = serde_json::to_value(AppSettings::default()).unwrap();
        let object = value.as_object_mut().unwrap();
        for key in [
            "terminal_line_height",
            "terminal_padding",
            "terminal_cursor_blink",
            "terminal_theme",
            "terminal_scrollbar",
        ] {
            object.remove(key);
        }
        let recovered: AppSettings = serde_json::from_value(value).unwrap();
        assert_eq!(recovered.terminal_line_height, DEFAULT_LINE_HEIGHT);
        assert_eq!(recovered.terminal_padding, DEFAULT_TERMINAL_PADDING);
        assert!(recovered.terminal_cursor_blink);
        assert_eq!(recovered.terminal_theme, DEFAULT_TERMINAL_THEME);
        assert!(recovered.terminal_scrollbar);
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
        // R36 · `0` is the family's tenth key, so it normalizes to the head too.
        assert_eq!(
            normalize_shortcut(SELECT_RESULT, "Alt+0").as_deref(),
            Some("Alt+1")
        );
        assert_eq!(normalize_shortcut(SELECT_RESULT, "F1"), None);
    }

    #[test]
    fn the_result_family_includes_the_ten_key() {
        // R36 · the launcher's numbered slots run `1`-`9` plus `0`, so a `0`
        // binding under the family's modifiers is a conflict like any other
        // digit — otherwise `⌘0` could be handed to a second action while
        // `⌘1` owns the family.
        assert!(shortcut_conflicts(
            SELECT_RESULT,
            "Ctrl+1",
            NEW_COMMAND,
            "Ctrl+0"
        ));
        assert!(shortcut_conflicts(
            NEW_COMMAND,
            "Ctrl+0",
            SELECT_RESULT,
            "Ctrl+1"
        ));
        assert!(!shortcut_conflicts(
            SELECT_RESULT,
            "Ctrl+1",
            NEW_COMMAND,
            "Ctrl+Space"
        ));
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

    // ── R7-11 · command aliases ────────────────────────────────────────────

    #[test]
    fn older_settings_start_with_no_command_aliases() {
        // Migration lock: a settings file written before the key existed (no
        // `command_aliases` at all) must deserialize to an empty map, not fail.
        let settings: AppSettings =
            serde_json::from_str("{\"theme\":\"dark\"}").expect("old settings deserialize");
        assert!(settings.command_aliases.is_empty());
        let empty: AppSettings = serde_json::from_str("{}").expect("settings deserialize");
        assert!(empty.command_aliases.is_empty());
    }

    #[test]
    fn command_aliases_round_trip_through_disk() {
        let directory = tempfile::tempdir().expect("settings directory");
        let settings = AppSettings {
            command_aliases: HashMap::from([
                ("git".to_string(), "gfm".to_string()),
                ("kubectl".to_string(), "k".to_string()),
            ]),
            ..AppSettings::default()
        };
        write_settings_to(directory.path(), &settings).expect("write settings");
        let reloaded = load_settings_from(directory.path());
        assert_eq!(
            reloaded.command_aliases.get("git").map(String::as_str),
            Some("gfm")
        );
        assert_eq!(
            reloaded.command_aliases.get("kubectl").map(String::as_str),
            Some("k")
        );
    }

    #[test]
    fn the_command_name_is_the_unique_key_of_the_alias_map() {
        // Keyed by command name: a second write for the same command overwrites
        // the first rather than accumulating a second alias for it.
        let mut aliases = HashMap::new();
        aliases.insert("git".to_string(), "g".to_string());
        aliases.insert("git".to_string(), "gfm".to_string());
        assert_eq!(aliases.len(), 1);
        assert_eq!(aliases.get("git").map(String::as_str), Some("gfm"));

        let settings = AppSettings {
            command_aliases: aliases,
            ..AppSettings::default()
        };
        assert_eq!(settings.command_aliases.len(), 1);
    }

    #[test]
    fn a_shared_alias_is_locked_by_the_first_command_in_name_order() {
        // Two commands may be given the same alias. The policy is first command
        // (ascending name order) locks the alias; the later duplicate is inert,
        // so the winner never depends on `HashMap` iteration order.
        let raw = HashMap::from([
            ("zeta".to_string(), "shared".to_string()),
            ("alpha".to_string(), "shared".to_string()),
        ]);
        let resolved = resolve_command_aliases(&raw);
        assert_eq!(resolved.len(), 1);
        assert_eq!(resolved.get("alpha").map(String::as_str), Some("shared"));
        assert!(!resolved.contains_key("zeta"));
    }

    #[test]
    fn blank_aliases_are_dropped_and_casing_does_not_double_claim() {
        let raw = HashMap::from([
            ("git".to_string(), "  ".to_string()),
            ("kubectl".to_string(), "K".to_string()),
            ("kn".to_string(), "k".to_string()),
        ]);
        let resolved = resolve_command_aliases(&raw);
        assert!(
            !resolved.contains_key("git"),
            "a blank alias is a removal, not an entry"
        );
        assert_eq!(resolved.get("kn").map(String::as_str), Some("k"));
        assert!(!resolved.contains_key("kubectl"));
    }

    #[test]
    fn the_frontend_snapshot_owns_the_command_aliases() {
        // The alias editor writes through `save_settings`; a stale full snapshot
        // must not resurrect an older map, and a fresh one must land.
        let stored = AppSettings::default();
        let submitted = AppSettings {
            command_aliases: HashMap::from([("git".to_string(), "gfm".to_string())]),
            ..AppSettings::default()
        };
        let merged = merge_frontend_settings(submitted, &stored);
        assert_eq!(
            merged.command_aliases.get("git").map(String::as_str),
            Some("gfm")
        );
    }

    // ── R39 · external plugin command switches ─────────────────────────────

    #[test]
    fn older_settings_start_with_no_plugin_command_switches() {
        // Migration lock: a settings file written before the key existed (no
        // `plugin_command_switches` at all) must deserialize to an empty map,
        // which is the "every command off" state — no external plugin mode.
        let settings: AppSettings =
            serde_json::from_str("{\"theme\":\"dark\"}").expect("old settings deserialize");
        assert!(settings.plugin_command_switches.is_empty());
        let empty: AppSettings = serde_json::from_str("{}").expect("settings deserialize");
        assert!(empty.plugin_command_switches.is_empty());
    }

    #[test]
    fn plugin_command_switches_round_trip_through_disk() {
        let directory = tempfile::tempdir().expect("settings directory");
        let settings = AppSettings {
            plugin_command_switches: BTreeMap::from([(
                "local.tool".to_string(),
                BTreeMap::from([
                    ("search".to_string(), true),
                    ("index".to_string(), false),
                ]),
            )]),
            ..AppSettings::default()
        };
        write_settings_to(directory.path(), &settings).expect("write settings");
        let reloaded = load_settings_from(directory.path());
        let tool = reloaded
            .plugin_command_switches
            .get("local.tool")
            .expect("the plugin's block survives");
        assert_eq!(tool.get("search"), Some(&true));
        // An explicit `false` is a decision and must survive normalization and a
        // disk round trip; pruning it would silently re-enable the command.
        assert_eq!(tool.get("index"), Some(&false));
    }

    #[test]
    fn plugin_command_switches_drop_empty_keys_and_keep_explicit_off() {
        let settings = normalize_settings(AppSettings {
            plugin_command_switches: BTreeMap::from([
                (
                    "  ".to_string(),
                    BTreeMap::from([("search".to_string(), true)]),
                ),
                (
                    "local.tool".to_string(),
                    BTreeMap::from([
                        ("".to_string(), true),
                        ("  index  ".to_string(), false),
                    ]),
                ),
                (
                    "local.empty".to_string(),
                    BTreeMap::from([("   ".to_string(), true)]),
                ),
            ]),
            ..AppSettings::default()
        });
        // A blank plugin id can never match a declared command.
        assert!(!settings.plugin_command_switches.contains_key(""));
        // An inner map left empty is dropped, so the plugin has no block at all
        // (the same state as "never touched").
        assert!(!settings.plugin_command_switches.contains_key("local.empty"));
        let tool = settings
            .plugin_command_switches
            .get("local.tool")
            .expect("the kept block");
        assert_eq!(tool.len(), 1, "the blank command id is dropped");
        assert_eq!(tool.get("index"), Some(&false));
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
