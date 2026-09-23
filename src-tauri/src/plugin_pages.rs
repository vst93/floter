//! Generic plugin HTML pages.
//!
//! A plugin (a built-in base plugin today, an external integration tomorrow)
//! may declare an HTML page. Opening it swaps the panel's whole canvas — the
//! same shell and window geometry the terminal page uses — for that page,
//! rendered inside a sandboxed iframe. Exactly one plugin page is open at a
//! time; Esc / Cmd+W / Ctrl+W / re-invoke closes back onto the remembered
//! surface, and any PTY underneath keeps running untouched.
//!
//! Pages never touch Tauri APIs themselves. They talk to the host through a
//! minimal postMessage bridge; the host performs `invoke()` on the page's
//! behalf and enforces a per-plugin command allowlist (see
//! [`PluginPageDescriptor::allowed_commands`]). For built-in plugins the
//! allowlist is this static registry; for external integrations it will come
//! from their descriptor (`page.html` in the integration dir, commands listed
//! beside it) through the same shape — the mechanism is deliberately not
//! clipboard-specific.
//!
//! Why an iframe rather than injecting the HTML into the app document:
//! external plugin HTML is less trusted than our own. External pages use a
//! sandboxed opaque origin — no DOM access to the host app, no Tauri IPC
//! surface at all. The trusted built-in clipboard page is the one exception:
//! WebKit needs same-origin enabled there to load its bundled stylesheet. Its
//! only host capability remains the allowlisted bridge.

use serde::Serialize;
use tauri::{AppHandle, Emitter, Manager};

use crate::AppState;

/// Stable id of the built-in clipboard base plugin, the first user of this
/// mechanism.
pub const CLIPBOARD_PLUGIN_ID: &str = "builtin.clipboard";

/// Stable id of the built-in browser plugin, the second user of this
/// mechanism.
pub const BROWSER_PLUGIN_ID: &str = "builtin.browser";

/// Everything the host needs to render one plugin page.
pub struct PluginPageDescriptor {
    pub id: &'static str,
    /// i18n key for the human name, resolved by the frontend dictionaries.
    pub title_key: &'static str,
    /// i18n key for the one-line description.
    pub description_key: &'static str,
    /// Page asset path relative to the frontend root, or `""` when the plugin
    /// has no page. R33 retired every built-in page onto the launcher's generic
    /// configuration overlay, so the built-in slots are empty; a future
    /// external integration would point this at a `page.html` in its own dir.
    pub page: &'static str,
    /// The only commands the bridge will invoke on this page's behalf.
    pub allowed_commands: &'static [&'static str],
}

const CLIPBOARD_COMMANDS: &[&str] = &[
    "clipboard_get_entries",
    "clipboard_set_favorite",
    "clipboard_delete",
    "clipboard_copy_entry",
    "clipboard_clear_history",
    "clipboard_read_image",
    // R38 · the row-icon thumbnail the launcher's inline mode reads.
    "clipboard_thumbnail",
    "clipboard_entry_statuses",
    "clipboard_read_file_preview",
    // R27 · the plugin's own settings card writes through this narrow pair,
    // exactly as the browser page's card does through `browser_set_settings`.
    "clipboard_get_settings",
    "clipboard_set_settings",
];

/// R26-B · the browser plugin's own page. Its settings card writes through
/// `browser_get_settings`/`browser_set_settings`, which touch only the plugin's
/// settings block; the three read commands and the two tab commands are the
/// same ones the launcher's inline mode uses.
const BROWSER_COMMANDS: &[&str] = &[
    "browser_discover",
    "browser_search_bookmarks",
    "browser_search_history",
    "browser_list_tabs",
    "browser_activate_tab",
    "browser_open_url",
    "browser_get_settings",
    "browser_set_settings",
];

/// The registry of built-in plugin pages.
static DESCRIPTORS: &[PluginPageDescriptor] = &[
    PluginPageDescriptor {
        id: CLIPBOARD_PLUGIN_ID,
        title_key: "settings.clipboardHistory",
        description_key: "settings.clipboardHistoryHint",
        // R33 · the built-in iframe pages are retired. The descriptor keeps
        // its empty page slot (and its whole allowlist) so the registry shape
        // external plugin pages will use is already here; nothing opens it.
        page: "",
        allowed_commands: CLIPBOARD_COMMANDS,
    },
    PluginPageDescriptor {
        id: BROWSER_PLUGIN_ID,
        title_key: "settings.browser",
        description_key: "settings.browserHint",
        page: "",
        allowed_commands: BROWSER_COMMANDS,
    },
];

pub fn descriptor(id: &str) -> Option<&'static PluginPageDescriptor> {
    DESCRIPTORS.iter().find(|entry| entry.id == id)
}

fn all_descriptors() -> &'static [PluginPageDescriptor] {
    DESCRIPTORS
}

/// Every registered page, for callers that have to cover the whole registry
/// rather than resolve one id — the notification copy table is checked against
/// it so a plugin page cannot exist without a bilingual name.
#[cfg(test)]
pub(crate) fn descriptors() -> &'static [PluginPageDescriptor] {
    DESCRIPTORS
}

/// Wire shape returned to the frontend.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PluginPageInfo {
    pub id: String,
    pub title_key: String,
    pub page: String,
    pub allowed_commands: Vec<String>,
}

impl From<&'static PluginPageDescriptor> for PluginPageInfo {
    fn from(descriptor: &'static PluginPageDescriptor) -> Self {
        Self {
            id: descriptor.id.to_string(),
            title_key: descriptor.title_key.to_string(),
            page: descriptor.page.to_string(),
            allowed_commands: descriptor
                .allowed_commands
                .iter()
                .map(|command| (*command).to_string())
                .collect(),
        }
    }
}

#[tauri::command]
pub fn plugin_page_descriptor(id: String) -> Result<PluginPageInfo, String> {
    descriptor(&id)
        .map(PluginPageInfo::from)
        .ok_or_else(|| format!("Unknown plugin page: {id}"))
}

/// A registered base plugin as the extensions ecosystem shows it. `enabled`
/// is the plugin's persisted state — for now one honest match arm per builtin
/// plugin reading its settings field; external integrations would read their
/// lock entry here instead.
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BuiltinPluginInfo {
    pub id: String,
    pub title_key: String,
    pub description_key: String,
    /// R33 · whether the plugin declares an HTML page. Both built-ins were
    /// retired onto the launcher's generic configuration overlay, so this is
    /// false for every descriptor today; the field stays for external pages.
    pub has_page: bool,
    pub enabled: bool,
}

#[tauri::command]
pub fn builtin_plugins_list() -> Result<Vec<BuiltinPluginInfo>, String> {
    Ok(builtin_plugin_infos(&crate::commands::config::load_settings()))
}

/// The registry projected through one settings snapshot.
///
/// Split out from the command so a test can render the list from
/// `AppSettings::default()` without reading (or depending on) the machine's own
/// config file. Every descriptor becomes a row — that is the whole point: a
/// plugin registered in `DESCRIPTORS` can never be missing from the settings
/// panel's list, which is exactly how `builtin.browser` went missing on the
/// frontend side (see `BUILTIN_BASE_PLUGINS` in `src/plugin-pages.ts`).
pub fn builtin_plugin_infos(
    settings: &crate::commands::config::AppSettings,
) -> Vec<BuiltinPluginInfo> {
    all_descriptors()
        .iter()
        .map(|descriptor| BuiltinPluginInfo {
            id: descriptor.id.to_string(),
            title_key: descriptor.title_key.to_string(),
            description_key: descriptor.description_key.to_string(),
            has_page: false,
            // The persisted state of the clipboard base plugin lives in its
            // long-standing settings field; there is exactly one switch, and
            // this is where it reads from.
            enabled: match descriptor.id {
                CLIPBOARD_PLUGIN_ID => settings.clipboard_history_enabled,
                // R26-D · the browser plugin now has a real switch: its
                // persisted state lives in the plugin's own settings block,
                // exactly as the clipboard one lives in its long-standing
                // field. Disabled, the launcher entry, the result list, the
                // settings page and the notification entry all soft-close.
                BROWSER_PLUGIN_ID => settings.browser_plugin.enabled,
                _ => false,
            },
        })
        .collect()
}

/// The plugin page a cold start should open, consumed once by the frontend
/// (`floter clip` on a fresh launch). Stored rather than emitted because the
/// webview may not have mounted its listeners yet during setup.
#[tauri::command]
pub(crate) fn take_pending_plugin_page(state: tauri::State<'_, AppState>) -> Option<String> {
    state
        .pending_plugin_open
        .lock()
        .ok()
        .and_then(|mut slot| slot.take())
}

/// Event payload announcing a plugin-page request to the frontend.
#[derive(Debug, Clone, Serialize)]
pub struct PluginPageEvent {
    pub id: String,
    /// True when the trigger is the toggle hotkey, which means "close" if the
    /// very same page is already showing; CLI invocations always open.
    pub toggle: bool,
}

/// Open a plugin page over whatever surface is showing, revealing the hidden
/// panel first. The single internal path shared by the `floter clip` CLI, the
/// global hotkey and the launcher entry.
pub fn open_plugin_page(app: &AppHandle, id: &str) {
    let Some(window) = app.get_webview_window("main") else {
        return;
    };
    let state = app.state::<AppState>();
    if !state
        .window_visible
        .load(std::sync::atomic::Ordering::SeqCst)
    {
        let _ = crate::reveal_saved_mode(&window, &state);
    }
    let _ = window.emit(
        "floter://plugin-config",
        PluginPageEvent {
            id: id.to_string(),
            toggle: false,
        },
    );
}

/// The hotkey path: summon when hidden or showing something else, hide when
/// the very same page is already up. The payload records whether the window
/// was visible so the frontend can tell those apart.
pub fn toggle_plugin_page(app: &AppHandle, id: &str) {
    let Some(window) = app.get_webview_window("main") else {
        return;
    };
    let state = app.state::<AppState>();
    let was_visible = state
        .window_visible
        .load(std::sync::atomic::Ordering::SeqCst);
    if !was_visible {
        let _ = crate::reveal_saved_mode(&window, &state);
    }
    let _ = window.emit(
        "floter://plugin-config",
        PluginPageEvent {
            id: id.to_string(),
            toggle: was_visible,
        },
    );
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_registry_contains_the_clipboard_page_with_its_commands() {
        let clipboard = descriptor(CLIPBOARD_PLUGIN_ID).expect("clipboard page");
        assert_eq!(clipboard.page, "", "the clipboard page is retired (R33)");
        assert!(clipboard
            .allowed_commands
            .contains(&"clipboard_get_entries"));
        assert!(clipboard
            .allowed_commands
            .contains(&"clipboard_read_file_preview"));
        // An unknown plugin has no page and no permissions.
        assert!(descriptor("builtin.nope").is_none());
        assert!(descriptor("../../etc/passwd").is_none());
    }

    /// R27 · the clipboard page's own settings card writes through a narrow
    /// pair, exactly as the browser page's does — and the pair is on the
    /// clipboard page's allowlist, not the browser's.
    #[test]
    fn the_clipboard_page_can_read_and_write_its_own_settings() {
        let clipboard = descriptor(CLIPBOARD_PLUGIN_ID).expect("clipboard page");
        for command in ["clipboard_get_settings", "clipboard_set_settings"] {
            assert!(
                clipboard.allowed_commands.contains(&command),
                "{command} must be allowlisted for the clipboard page"
            );
        }
        let browser = descriptor(BROWSER_PLUGIN_ID).expect("browser page");
        assert!(
            !browser.allowed_commands.contains(&"clipboard_set_settings"),
            "a page's allowlist is its whole capability surface"
        );
    }

    #[test]
    fn the_registry_contains_the_browser_page_with_its_commands() {
        let browser = descriptor(BROWSER_PLUGIN_ID).expect("browser page");
        assert_eq!(browser.page, "", "the browser page is retired (R33)");
        for command in [
            "browser_search_bookmarks",
            "browser_search_history",
            "browser_list_tabs",
            "browser_activate_tab",
            "browser_open_url",
            "browser_get_settings",
            "browser_set_settings",
        ] {
            assert!(
                browser.allowed_commands.contains(&command),
                "{command} must be allowlisted for the browser page"
            );
        }
    }

    #[test]
    fn the_two_builtin_pages_share_no_command() {
        // A page's allowlist is its whole capability surface, so the browser
        // page must not inherit a clipboard command (or the reverse) by being
        // listed twice.
        let clipboard = descriptor(CLIPBOARD_PLUGIN_ID).expect("clipboard page");
        let browser = descriptor(BROWSER_PLUGIN_ID).expect("browser page");
        for command in browser.allowed_commands {
            assert!(
                !clipboard.allowed_commands.contains(command),
                "{command} is allowlisted for both pages"
            );
        }
    }

    #[test]
    fn every_descriptor_has_a_unique_id_and_no_registered_page() {
        let mut ids: Vec<&str> = Vec::new();
        for entry in all_descriptors() {
            assert!(!ids.contains(&entry.id), "duplicate plugin id {}", entry.id);
            ids.push(entry.id);
            // R33 · the built-in iframe pages are retired: a registered path
            // would be a page nothing is allowed to open. A future external
            // page may set one; today the slot is empty for everyone.
            assert!(entry.page.is_empty(), "no built-in page may be registered");
            assert!(!entry.allowed_commands.is_empty());
        }
        assert!(!ids.is_empty());
    }

    /// R26-C · the list the settings panel renders carries every descriptor,
    /// browser included. The frontend's own guard lives in
    /// `tests/plugin-pages.test.ts`; this one pins the backend half.
    #[test]
    fn the_builtin_plugin_list_carries_the_browser_descriptor() {
        let settings = crate::commands::config::AppSettings::default();
        let infos = builtin_plugin_infos(&settings);
        let ids: Vec<&str> = infos.iter().map(|info| info.id.as_str()).collect();
        assert!(ids.contains(&CLIPBOARD_PLUGIN_ID));
        assert!(
            ids.contains(&BROWSER_PLUGIN_ID),
            "the base-plugin list must contain builtin.browser"
        );
        // One row per descriptor, never a hand-picked subset.
        assert_eq!(infos.len(), all_descriptors().len());
        let browser = infos
            .iter()
            .find(|info| info.id == BROWSER_PLUGIN_ID)
            .expect("browser row");
        assert!(
            browser.enabled,
            "the browser plugin ships switched on"
        );
        assert!(
            !browser.has_page,
            "R33 · the built-in pages are retired; the row opens the overlay instead"
        );
        assert_eq!(browser.title_key, "settings.browser");
        assert_eq!(browser.description_key, "settings.browserHint");
    }

    /// R26-D · the browser row's switch reads the plugin's own persisted
    /// `enabled`, and switching it off is visible in the very list the settings
    /// panel renders — the switch is not a frontend-only illusion.
    #[test]
    fn the_browser_row_reflects_the_plugins_own_switch() {
        let mut settings = crate::commands::config::AppSettings::default();
        settings.browser_plugin.enabled = false;
        let infos = builtin_plugin_infos(&settings);
        let browser = infos
            .iter()
            .find(|info| info.id == BROWSER_PLUGIN_ID)
            .expect("browser row");
        assert!(!browser.enabled, "the row follows browser_plugin.enabled");
        // The clipboard switch is an independent field: disabling the browser
        // must not touch it.
        let clipboard = infos
            .iter()
            .find(|info| info.id == CLIPBOARD_PLUGIN_ID)
            .expect("clipboard row");
        assert!(clipboard.enabled);
    }
}
