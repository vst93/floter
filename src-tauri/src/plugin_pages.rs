//! The built-in base-plugin registry and the plugin-surface event path.
//!
//! R33 retired the built-in iframe plugin pages and R76 deleted their source,
//! so there is no HTML page layer left: a plugin's whole surface is the
//! launcher's inline mode plus the schema-driven configuration overlay. R96
//! deleted the last of the page machinery — the descriptor's `page` slot, its
//! per-plugin command allowlist and the has-page flag on the wire — because
//! nothing could produce or consume them any more.
//!
//! What this module still owns:
//!
//! * the stable ids and the descriptor table the settings panel renders
//!   ([`DESCRIPTORS`] → [`builtin_plugin_infos`]);
//! * [`open_plugin_page`], the one internal path the console's `clip` command
//!   (and the cold-start hand-off) uses to reveal the panel and emit
//!   `floter://plugin-config`, which the launcher's `PluginConfigOverlay`
//!   answers.

use serde::Serialize;
use tauri::{AppHandle, Emitter, Manager};

use crate::AppState;

/// Stable id of the built-in clipboard base plugin.
pub const CLIPBOARD_PLUGIN_ID: &str = "builtin.clipboard";

/// Stable id of the built-in browser plugin.
pub const BROWSER_PLUGIN_ID: &str = "builtin.browser";

/// R50 · stable id of the built-in calculator plugin.
///
/// R51 · it is a descriptor like the other two built-ins (the settings panel's
/// "Base plugins" list and the launcher's search read this registry). It has no
/// page and no on/off switch; its whole surface is the launcher mode and its
/// configuration overlay.
pub const CALCULATOR_PLUGIN_ID: &str = "builtin.calculator";

/// One row of the built-in plugin registry: the identity and the two i18n keys
/// the settings panel renders. The command allowlist and the page slot that
/// used to live here were deleted in R96 with the retired page layer.
pub struct PluginPageDescriptor {
    pub id: &'static str,
    /// i18n key for the human name, resolved by the frontend dictionaries.
    pub title_key: &'static str,
    /// i18n key for the one-line description.
    pub description_key: &'static str,
}

/// The registry of built-in plugins.
static DESCRIPTORS: &[PluginPageDescriptor] = &[
    PluginPageDescriptor {
        id: CLIPBOARD_PLUGIN_ID,
        title_key: "settings.clipboardHistory",
        description_key: "settings.clipboardHistoryHint",
    },
    PluginPageDescriptor {
        id: BROWSER_PLUGIN_ID,
        title_key: "settings.browser",
        description_key: "settings.browserHint",
    },
    PluginPageDescriptor {
        id: CALCULATOR_PLUGIN_ID,
        title_key: "settings.calculator",
        description_key: "settings.calculatorHint",
    },
];

pub fn descriptor(id: &str) -> Option<&'static PluginPageDescriptor> {
    DESCRIPTORS.iter().find(|entry| entry.id == id)
}

fn all_descriptors() -> &'static [PluginPageDescriptor] {
    DESCRIPTORS
}

/// Every registered plugin, for callers that have to cover the whole registry
/// rather than resolve one id — the notification copy table is checked against
/// it so a plugin cannot exist without a bilingual name.
#[cfg(test)]
pub(crate) fn descriptors() -> &'static [PluginPageDescriptor] {
    DESCRIPTORS
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
    pub enabled: bool,
}

#[tauri::command]
pub fn builtin_plugins_list() -> Result<Vec<BuiltinPluginInfo>, String> {
    Ok(builtin_plugin_infos(
        &crate::commands::config::load_settings(),
    ))
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
                // R51 · the calculator has no persisted switch: it is always
                // available (the settings row is `toggleable: false` and draws
                // no switch at all). Reported `true` rather than falling into
                // the `_ => false` arm, which would describe an on plugin as
                // off to any consumer that read the field without the row's
                // toggleable flag beside it.
                CALCULATOR_PLUGIN_ID => true,
                _ => false,
            },
        })
        .collect()
}

/// The plugin a cold start should open, consumed once by the frontend
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

/// Event payload announcing a plugin request to the frontend.
#[derive(Debug, Clone, Serialize)]
pub struct PluginPageEvent {
    pub id: String,
    /// True when the trigger is the toggle hotkey, which means "close" if the
    /// very same plugin is already showing; CLI invocations always open.
    pub toggle: bool,
}

/// Open a plugin's surface over whatever is showing, revealing the hidden panel
/// first. The single internal path shared by the `floter clip` CLI, the global
/// hotkey and the launcher entry.
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn the_registry_contains_the_clipboard_descriptor() {
        let clipboard = descriptor(CLIPBOARD_PLUGIN_ID).expect("clipboard descriptor");
        assert_eq!(clipboard.title_key, "settings.clipboardHistory");
        assert_eq!(clipboard.description_key, "settings.clipboardHistoryHint");
        // An unknown plugin has no descriptor.
        assert!(descriptor("builtin.nope").is_none());
        assert!(descriptor("../../etc/passwd").is_none());
    }

    #[test]
    fn the_registry_contains_the_browser_descriptor() {
        let browser = descriptor(BROWSER_PLUGIN_ID).expect("browser descriptor");
        assert_eq!(browser.title_key, "settings.browser");
        assert_eq!(browser.description_key, "settings.browserHint");
    }

    #[test]
    fn the_registry_contains_the_calculator_descriptor() {
        // R51 · the calculator is a descriptor like the other two built-ins
        // (see `BUILTIN_BASE_PLUGINS` in `src/plugin-pages.ts`).
        let calculator = descriptor(CALCULATOR_PLUGIN_ID).expect("calculator descriptor");
        assert_eq!(calculator.title_key, "settings.calculator");
        assert_eq!(calculator.description_key, "settings.calculatorHint");
    }

    #[test]
    fn every_descriptor_has_a_unique_id() {
        let mut ids: Vec<&str> = Vec::new();
        for entry in all_descriptors() {
            assert!(!ids.contains(&entry.id), "duplicate plugin id {}", entry.id);
            ids.push(entry.id);
        }
        assert!(!ids.is_empty());
    }

    /// R51 · the calculator row is in the list the settings panel renders, and
    /// it reports itself available: it has no persisted switch, so the frontend
    /// renders no switch (`toggleable: false`) and the backend must not describe
    /// an always-on plugin as off through the `_ => false` default.
    #[test]
    fn the_builtin_plugin_list_carries_the_calculator_descriptor() {
        let infos = builtin_plugin_infos(&crate::commands::config::AppSettings::default());
        let calculator = infos
            .iter()
            .find(|info| info.id == CALCULATOR_PLUGIN_ID)
            .expect("calculator row");
        assert!(calculator.enabled, "the calculator is always available");
        assert_eq!(calculator.title_key, "settings.calculator");
        assert_eq!(calculator.description_key, "settings.calculatorHint");
        // One row per descriptor, never a hand-picked subset.
        assert_eq!(infos.len(), all_descriptors().len());
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
        assert!(browser.enabled, "the browser plugin ships switched on");
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
