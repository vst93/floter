//! Built-in clipboard history.
//!
//! When enabled in settings (default on), a background monitor captures every
//! system-wide copy of text or images for as long as floter runs. Entries are
//! stored locally under the app data directory, survive restarts, and are
//! surfaced in a terminal-styled panel summoned by a dedicated global hotkey
//! (`Alt+V` by default, rebindable in settings).

pub mod monitor;
pub mod store;

use serde::{Deserialize, Serialize};
use std::borrow::Cow;
use std::sync::Mutex;

use tauri::{AppHandle, Emitter, Manager};
use tauri_plugin_global_shortcut::{GlobalShortcutExt, ShortcutState};

use crate::AppState;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ClipboardEntry {
    pub id: String,
    /// "text" | "image"
    pub kind: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    /// File name inside the history's `images/` directory; image entries only.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image_file: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub width: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub height: Option<u32>,
    /// Content identity used to skip unchanged polls and consecutive copies.
    pub hash: String,
    /// Unix timestamp in milliseconds.
    pub created_at: i64,
    /// Favorites are exempt from pruning: neither the 200-entry cap nor the
    /// 30-day expiry ever applies to them.
    pub favorite: bool,
}

/// Managed state: an in-memory mirror of the index plus the live monitor's
/// cancellation handle.
pub struct ClipboardState {
    entries: Mutex<Option<Vec<ClipboardEntry>>>,
    monitor: Mutex<Option<monitor::MonitorHandle>>,
}

impl Default for ClipboardState {
    fn default() -> Self {
        Self {
            entries: Mutex::new(None),
            monitor: Mutex::new(None),
        }
    }
}

fn now_ms() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as i64
}

/// Load the index into memory once per process, applying the retention policy
/// on the way in so a long-shelved install prunes before showing anything.
fn ensure_loaded(entries: &mut Vec<ClipboardEntry>) -> Result<(), String> {
    let paths = store::app_store_paths().ok_or("No app data directory")?;
    let loaded = store::load_index(&paths);
    let (kept, dropped) = store::prune_entries(loaded, now_ms());
    if !dropped.is_empty() {
        let _ = store::save_index(&paths, &kept);
        for entry in &dropped {
            if let Some(file) = &entry.image_file {
                store::delete_image(&paths, file);
            }
        }
        store::remove_orphan_images(&paths, &kept);
    }
    *entries = kept;
    Ok(())
}

/// Read the current history without writing anything back.
fn read_history(app: &AppHandle) -> Result<Vec<ClipboardEntry>, String> {
    let _guard = store::store_lock()?;
    let state = app.state::<ClipboardState>();
    let mut cache = state
        .entries
        .lock()
        .map_err(|_| "History cache poisoned".to_string())?;
    if cache.is_none() {
        let mut loaded = Vec::new();
        ensure_loaded(&mut loaded)?;
        *cache = Some(loaded);
    }
    Ok(cache.as_ref().cloned().unwrap_or_default())
}

/// Serialize one read-modify-write cycle against both the in-memory mirror
/// and the on-disk index. `f` mutates the vector; persistence happens here so
/// no caller can forget it.
fn mutate_history<T>(
    app: &AppHandle,
    f: impl FnOnce(&mut Vec<ClipboardEntry>) -> Result<T, String>,
) -> Result<T, String> {
    let _guard = store::store_lock()?;
    let state = app.state::<ClipboardState>();
    let mut cache = state
        .entries
        .lock()
        .map_err(|_| "History cache poisoned".to_string())?;
    if cache.is_none() {
        let mut loaded = Vec::new();
        ensure_loaded(&mut loaded)?;
        *cache = Some(loaded);
    }
    let entries = cache.as_mut().expect("just initialized");
    f(entries)
}

// ---- Tauri commands ------------------------------------------------------

#[tauri::command]
pub fn clipboard_get_entries(
    app: AppHandle,
    filter: Option<String>,
) -> Result<Vec<ClipboardEntry>, String> {
    let needle = filter.unwrap_or_default().trim().to_lowercase();
    let entries = read_history(&app)?;
    if needle.is_empty() {
        return Ok(entries);
    }
    Ok(entries
        .into_iter()
        .filter(|entry| match &entry.text {
            Some(text) => text.to_lowercase().contains(&needle),
            None => ["image", "img", "图片"]
                .iter()
                .any(|word| word.contains(&needle)),
        })
        .collect())
}

#[tauri::command]
pub fn clipboard_set_favorite(app: AppHandle, id: String, favorite: bool) -> Result<(), String> {
    mutate_history(&app, |entries| {
        entries
            .iter_mut()
            .find(|entry| entry.id == id)
            .map(|entry| entry.favorite = favorite)
            .ok_or_else(|| format!("Unknown clipboard entry: {id}"))?;
        // A favorite flag change never affects retention, but keeping the
        // index write in one place means this stays true even if that does.
        monitor::prune_and_save(entries)
    })
}

#[tauri::command]
pub fn clipboard_delete(app: AppHandle, id: String) -> Result<(), String> {
    let paths = store::app_store_paths().ok_or("No app data directory")?;
    mutate_history(&app, |entries| {
        let position = entries
            .iter()
            .position(|entry| entry.id == id)
            .ok_or_else(|| format!("Unknown clipboard entry: {id}"))?;
        let removed = entries.remove(position);
        store::save_index(&paths, entries)?;
        if let Some(file) = &removed.image_file {
            store::delete_image(&paths, file);
        }
        Ok(())
    })
}

#[tauri::command]
pub fn clipboard_clear_history(app: AppHandle) -> Result<(), String> {
    let paths = store::app_store_paths().ok_or("No app data directory")?;
    mutate_history(&app, |entries| {
        let removed: Vec<ClipboardEntry> =
            entries.drain(..).filter(|entry| !entry.favorite).collect();
        store::save_index(&paths, entries)?;
        for entry in &removed {
            if let Some(file) = &entry.image_file {
                store::delete_image(&paths, file);
            }
        }
        Ok(())
    })
}

/// Restore one entry onto the system clipboard.
///
/// Clipboard access can fail because another application holds it open; that
/// surfaces to the caller as an error string, never a panic.
#[tauri::command]
pub fn clipboard_copy_entry(app: AppHandle, id: String) -> Result<(), String> {
    let paths = store::app_store_paths().ok_or("No app data directory")?;
    let entry = read_history(&app)?
        .into_iter()
        .find(|entry| entry.id == id)
        .ok_or_else(|| format!("Unknown clipboard entry: {id}"))?;

    let mut clipboard = arboard::Clipboard::new().map_err(|error| error.to_string())?;
    match entry.kind.as_str() {
        "text" => {
            let text = entry.text.clone().ok_or("Text entry has no content")?;
            clipboard.set_text(text).map_err(|error| error.to_string())
        }
        "image" => {
            let file = entry.image_file.clone().ok_or("Image entry has no file")?;
            let bytes = store::read_image(&paths, &file)?;
            let (width, height, rgba) = monitor::decode_png(&bytes)?;
            clipboard
                .set_image(arboard::ImageData {
                    width: width as usize,
                    height: height as usize,
                    bytes: Cow::Owned(rgba),
                })
                .map_err(|error| error.to_string())
        }
        other => Err(format!("Unknown clipboard entry kind: {other}")),
    }
}

/// PNG bytes of a stored image entry, for thumbnail rendering in the panel.
#[tauri::command]
pub fn clipboard_read_image(app: AppHandle, id: String) -> Result<Vec<u8>, String> {
    let paths = store::app_store_paths().ok_or("No app data directory")?;
    let entry = read_history(&app)?
        .into_iter()
        .find(|entry| entry.id == id)
        .ok_or_else(|| format!("Unknown clipboard entry: {id}"))?;
    let file = entry.image_file.ok_or("Entry is not an image")?;
    store::read_image(&paths, &file)
}

// ---- Global shortcut plumbing --------------------------------------------

/// Register `shortcut` with the OS as the clipboard panel toggle.
pub fn register_panel_shortcut(app: &AppHandle, shortcut: &str) -> Result<(), String> {
    app.global_shortcut()
        .on_shortcut(shortcut, move |app_handle, _shortcut, event| {
            if event.state == ShortcutState::Pressed {
                toggle_panel(app_handle);
            }
        })
        .map_err(|e| e.to_string())?;

    if let Ok(mut active) = app.state::<AppState>().clipboard_shortcut.lock() {
        *active = shortcut.to_string();
    }
    Ok(())
}

pub fn unregister_panel_shortcut(app: &AppHandle) {
    let active = app
        .state::<AppState>()
        .clipboard_shortcut
        .lock()
        .ok()
        .map(|value| value.clone())
        .unwrap_or_default();
    if !active.is_empty() {
        let _ = app.global_shortcut().unregister(active.as_str());
    }
    if let Ok(mut slot) = app.state::<AppState>().clipboard_shortcut.lock() {
        slot.clear();
    }
}

/// Move the panel hotkey to `next`, restoring the previous binding when the
/// OS refuses the new one — same contract as [`crate::rebind_toggle_shortcut`].
pub fn rebind_panel_shortcut(app: &AppHandle, next: &str) -> Result<(), String> {
    let previous = app
        .state::<AppState>()
        .clipboard_shortcut
        .lock()
        .ok()
        .map(|value| value.clone())
        .unwrap_or_default();

    if !previous.is_empty() {
        let _ = app.global_shortcut().unregister(previous.as_str());
    }
    if let Err(error) = register_panel_shortcut(app, next) {
        if !previous.is_empty() {
            let _ = register_panel_shortcut(app, previous.as_str());
        }
        return Err(error);
    }
    Ok(())
}

/// Register the hotkey unless it already matches what is live.
fn ensure_panel_shortcut(app: &AppHandle, shortcut: &str) -> Result<(), String> {
    let active = app
        .state::<AppState>()
        .clipboard_shortcut
        .lock()
        .ok()
        .map(|value| value.clone())
        .unwrap_or_default();
    if active.eq_ignore_ascii_case(shortcut) && app.global_shortcut().is_registered(shortcut) {
        return Ok(());
    }
    if !active.is_empty() && app.global_shortcut().is_registered(active.as_str()) {
        let _ = app.global_shortcut().unregister(active.as_str());
    }
    register_panel_shortcut(app, shortcut)
}

/// Show the window (revealing it first when hidden) and tell the frontend to
/// flip its clipboard panel; pressing the hotkey again closes it.
fn toggle_panel(app: &AppHandle) {
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
    let _ = window.emit("floter://clipboard", ());
}

// ---- Lifecycle ------------------------------------------------------------

/// Called once from the setup hook: start the monitor and register the hotkey
/// only when the setting says so. Failures are logged, never fatal — a system
/// that refuses the hotkey still gets the monitor, and the panel remains
/// reachable through the launcher.
pub fn initialize(app: &AppHandle, enabled: bool, hotkey: &str) {
    if !enabled {
        return;
    }
    monitor::start(app);
    if let Err(error) = register_panel_shortcut(app, hotkey) {
        eprintln!("floter: clipboard panel shortcut registration failed: {error}");
    }
}

/// Reconcile runtime state (monitor + hotkey) with the settings after any
/// change. Both branches are idempotent, so callers need not diff first.
pub fn sync_runtime(app: &AppHandle, enabled: bool, hotkey: &str) {
    if enabled {
        monitor::start(app);
        if let Err(error) = ensure_panel_shortcut(app, hotkey) {
            eprintln!("floter: clipboard panel shortcut registration failed: {error}");
        }
    } else {
        monitor::stop(app);
        unregister_panel_shortcut(app);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn entry(hash: &str, created_at: i64) -> ClipboardEntry {
        ClipboardEntry {
            id: uuid::Uuid::new_v4().to_string(),
            kind: "text".to_string(),
            text: Some(format!("content-{hash}")),
            image_file: None,
            width: None,
            height: None,
            hash: hash.to_string(),
            created_at,
            favorite: false,
        }
    }

    #[test]
    fn entry_json_round_trips_with_all_optional_fields_absent() {
        let original = entry("abc", 123);
        let json = serde_json::to_string(&original).expect("serialize");
        assert!(!json.contains("image_file"));
        assert!(!json.contains("width"));

        let parsed: ClipboardEntry = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(parsed, original);
    }

    #[test]
    fn old_index_entries_without_optional_fields_deserialize() {
        // Simulates an index written before a schema tweak: bare required
        // fields only.
        let json = r#"{
            "id": "x",
            "kind": "text",
            "text": "hi",
            "hash": "h",
            "created_at": 5,
            "favorite": false
        }"#;
        let parsed: ClipboardEntry = serde_json::from_str(json).expect("deserialize");
        assert_eq!(parsed.kind, "text");
        assert_eq!(parsed.text.as_deref(), Some("hi"));
    }

    #[test]
    fn now_ms_is_a_plausible_unix_timestamp() {
        assert!(now_ms() > 1_600_000_000_000);
    }
}
