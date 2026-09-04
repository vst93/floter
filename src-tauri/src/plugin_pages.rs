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

/// Everything the host needs to render one plugin page.
pub struct PluginPageDescriptor {
    pub id: &'static str,
    /// i18n key for the human name, resolved by the frontend dictionaries.
    pub title_key: &'static str,
    /// i18n key for the one-line description.
    pub description_key: &'static str,
    /// Page asset path relative to the frontend root. Built-in pages ship in
    /// the bundled frontend dist; external integrations will point at a
    /// `page.html` inside their own directory instead.
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
    "clipboard_entry_statuses",
    "clipboard_read_file_preview",
];

/// The registry of built-in plugin pages.
static DESCRIPTORS: &[PluginPageDescriptor] = &[PluginPageDescriptor {
    id: CLIPBOARD_PLUGIN_ID,
    title_key: "settings.clipboardHistory",
    description_key: "settings.clipboardHistoryHint",
    page: "plugins/clipboard/index.html",
    allowed_commands: CLIPBOARD_COMMANDS,
}];

pub fn descriptor(id: &str) -> Option<&'static PluginPageDescriptor> {
    DESCRIPTORS.iter().find(|entry| entry.id == id)
}

fn all_descriptors() -> &'static [PluginPageDescriptor] {
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
    /// Whether the plugin declares an HTML page (all of them do today).
    pub has_page: bool,
    pub enabled: bool,
}

#[tauri::command]
pub fn builtin_plugins_list() -> Result<Vec<BuiltinPluginInfo>, String> {
    let settings = crate::commands::config::load_settings();
    Ok(all_descriptors()
        .iter()
        .map(|descriptor| BuiltinPluginInfo {
            id: descriptor.id.to_string(),
            title_key: descriptor.title_key.to_string(),
            description_key: descriptor.description_key.to_string(),
            has_page: true,
            // The persisted state of the clipboard base plugin lives in its
            // long-standing settings field; there is exactly one switch, and
            // this is where it reads from.
            enabled: match descriptor.id {
                CLIPBOARD_PLUGIN_ID => settings.clipboard_history_enabled,
                _ => false,
            },
        })
        .collect())
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
        "floter://plugin-page",
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
        "floter://plugin-page",
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
        assert_eq!(clipboard.page, "plugins/clipboard/index.html");
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

    #[test]
    fn every_descriptor_has_a_unique_id_and_a_root_relative_page() {
        let mut ids: Vec<&str> = Vec::new();
        for entry in all_descriptors() {
            assert!(!ids.contains(&entry.id), "duplicate plugin id {}", entry.id);
            ids.push(entry.id);
            assert!(
                entry.page.starts_with("plugins/"),
                "page must be sandboxed under its own directory"
            );
            assert!(!entry.allowed_commands.is_empty());
        }
        assert!(!ids.is_empty());
    }
}
