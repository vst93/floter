//! Built-in clipboard history.
//!
//! When enabled in settings (default on), a background monitor captures every
//! system-wide copy of text, images, or file lists for as long as floter runs.
//!
//! File copies (`kind: "files"`) store **references only**: the original
//! absolute path strings plus a hash derived from their canonicalized forms.
//! File contents are never copied into the history. Capture relies on
//! arboard's native file-list support, which version 3.6 covers everywhere we
//! ship: Windows CF_HDROP, macOS NSFilenamesPboardType, and Linux X11/Wayland
//! via `text/uri-list`. No text-sniffing heuristic is involved. Entries are
//! stored locally under the app data directory, survive restarts, and are
//! surfaced in a terminal-styled panel summoned from launcher search,
//! `floter clip`, or an optional global hotkey the user binds in Shortcuts
//! settings (disabled by default).

pub mod monitor;
pub mod store;

use serde::{Deserialize, Serialize};
use std::borrow::Cow;
use std::collections::HashMap;
use std::sync::Mutex;

use tauri::{AppHandle, Manager};
use tauri_plugin_global_shortcut::{GlobalShortcutExt, ShortcutState};

use crate::AppState;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct ClipboardEntry {
    pub id: String,
    /// "text" | "image" | "files"
    pub kind: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    /// Absolute paths referenced by a "files" entry — one entry holds every
    /// item of a single copy operation. Paths only; never file contents.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub paths: Option<Vec<String>>,
    /// File name inside the history's `images/` directory; image entries only.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub image_file: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub width: Option<u32>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub height: Option<u32>,
    /// Content identity used to skip unchanged polls and to dedupe re-copies
    /// across the whole history.
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
    #[cfg(target_os = "linux")]
    clipboard_owner: Mutex<Option<arboard::Clipboard>>,
}

impl Default for ClipboardState {
    fn default() -> Self {
        Self {
            entries: Mutex::new(None),
            monitor: Mutex::new(None),
            #[cfg(target_os = "linux")]
            clipboard_owner: Mutex::new(None),
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
        if let Err(error) = store::save_index(&paths, &kept) {
            tracing::warn!("floter: clipboard retention save failed: {error}");
        } else {
            store::remove_orphan_images(&paths, &kept);
        }
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
    store::update_entries(entries, f)
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
        .filter(|entry| match (&entry.text, &entry.paths) {
            // Text entries match on content; image entries carrying a caption
            // (a simultaneous image+text copy stores one image entry whose
            // `text` is the caption) match on it too. The arm only decides
            // *whether* a row is kept — the kind is never touched here, so a
            // captioned image still renders as an image row.
            (Some(text), _) => text.to_lowercase().contains(&needle),
            // Files entries answer to any stored path; a basename is a
            // substring of its own full path, so both come free.
            (None, Some(paths)) => paths
                .iter()
                .any(|path| path.to_lowercase().contains(&needle)),
            (None, None) => ["image", "img", "图片"]
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
        let removed = store::take_non_favorites(entries);
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
            // An entry captured with a caption restores as IMAGE ONLY: arboard
            // cannot set pixels and text in one atomic write, and a two-step
            // write would hand the clipboard to another app mid-way. The
            // caption survives in history; only the pixels go back to the
            // system clipboard.
            clipboard
                .set_image(arboard::ImageData {
                    width: width as usize,
                    height: height as usize,
                    bytes: Cow::Owned(rgba),
                })
                .map_err(|error| error.to_string())
        }
        "files" => {
            let stored = entry
                .paths
                .clone()
                .filter(|paths| !paths.is_empty())
                .ok_or("Files entry has no paths")?;
            // A failed native file-list write must not claim that references
            // were restored after replacing them with a different data type.
            clipboard
                .set()
                .file_list(&stored)
                .map_err(|error| error.to_string())
        }
        other => Err(format!("Unknown clipboard entry kind: {other}")),
    }?;
    // X11 serves the selection from this handle. Dropping the last owner
    // loses the restored data on desktops without a clipboard manager.
    #[cfg(target_os = "linux")]
    {
        let state = app.state::<ClipboardState>();
        let mut owner = state
            .clipboard_owner
            .lock()
            .map_err(|_| "Clipboard owner poisoned".to_string())?;
        *owner = Some(clipboard);
    }
    Ok(())
}

/// Existence map for files entries: `true` = every stored path still exists.
/// Called once per panel load; each check is a bare `metadata()` stat, cheap
/// enough to run for the whole visible history.
#[tauri::command]
pub fn clipboard_entry_statuses(
    app: AppHandle,
    ids: Vec<String>,
) -> Result<HashMap<String, bool>, String> {
    let entries = read_history(&app)?;
    let mut statuses = HashMap::with_capacity(ids.len());
    for id in ids {
        let complete = entries
            .iter()
            .find(|entry| entry.id == id)
            .and_then(|entry| entry.paths.as_ref())
            .map(|paths| {
                !paths.is_empty() && paths.iter().all(|path| std::fs::metadata(path).is_ok())
            })
            // Non-files entries have nothing to go missing.
            .unwrap_or(true);
        statuses.insert(id, complete);
    }
    Ok(statuses)
}

/// Extensions whose bytes the webview can render directly in a row thumbnail.
pub const PREVIEW_IMAGE_EXTENSIONS: [&str; 6] = ["png", "jpg", "jpeg", "gif", "webp", "bmp"];
/// Hard ceiling for preview reads (10 MB).
pub const MAX_PREVIEW_BYTES: u64 = 10 * 1024 * 1024;

/// Whether `path` names a file of a previewable raster-image format, judged by
/// extension alone. Aware of both separators so a Windows-style stored path
/// behaves identically to a POSIX one.
pub fn is_image_file_path(path: &str) -> bool {
    let name = match path.rfind(|c| c == '/' || c == '\\') {
        Some(index) => &path[index + 1..],
        None => path,
    };
    let Some(dot) = name.rfind('.') else {
        return false;
    };
    if dot + 1 >= name.len() {
        return false;
    }
    let extension = name[dot + 1..].to_ascii_lowercase();
    PREVIEW_IMAGE_EXTENSIONS.contains(&extension.as_str())
}

/// Whether a files entry qualifies for an in-panel pixel preview: exactly one
/// path whose extension names a renderable format. Size is gated at read time
/// in [`clipboard_read_file_preview`] — stat-ing here would defeat the point
/// of a pure predicate.
pub fn is_files_preview_candidate(paths: Option<&[String]>) -> bool {
    match paths {
        Some([single]) => is_image_file_path(single),
        _ => false,
    }
}

/// Bytes of the single image file a files entry points at, for rendering the
/// row thumbnail straight from disk. Strictly gated: known id, exactly one
/// path, an existing regular file, an image extension, and the size cap.
/// Stored paths are our own data, but they still refuse NUL bytes.
#[tauri::command]
pub fn clipboard_read_file_preview(app: AppHandle, id: String) -> Result<Vec<u8>, String> {
    let entry = read_history(&app)?
        .into_iter()
        .find(|entry| entry.id == id)
        .ok_or_else(|| format!("Unknown clipboard entry: {id}"))?;
    let paths = entry.paths.as_deref().ok_or("Entry has no file paths")?;
    if !is_files_preview_candidate(Some(paths)) {
        return Err("Entry has no previewable image file".to_string());
    }
    let path = &paths[0];
    if path.contains('\0') {
        return Err("Invalid file path".to_string());
    }
    let metadata = std::fs::metadata(path).map_err(|error| error.to_string())?;
    if !metadata.is_file() {
        return Err("Not a regular file".to_string());
    }
    if metadata.len() > MAX_PREVIEW_BYTES {
        return Err("File too large for preview".to_string());
    }
    std::fs::read(path).map_err(|error| error.to_string())
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
/// flip its clipboard page. This is now a thin alias over the generic plugin-
/// page toggle — the hotkey is just another invocation of `floter clip`, and
/// only differs in that pressing it again while the page is up means "hide".
fn toggle_panel(app: &AppHandle) {
    crate::plugin_pages::toggle_plugin_page(app, crate::plugin_pages::CLIPBOARD_PLUGIN_ID);
}

// ---- Lifecycle ------------------------------------------------------------

/// Called once from the setup hook: start the monitor and register the hotkey
/// only when the setting says so. An empty hotkey means "no global shortcut"
/// — nothing is registered and the panel stays reachable through launcher
/// search and `floter clip`. Failures are logged, never fatal — a system
/// that refuses the hotkey still gets the monitor.
pub fn initialize(app: &AppHandle, enabled: bool, hotkey: &str) {
    if !enabled {
        return;
    }
    monitor::start(app);
    if hotkey.trim().is_empty() {
        return;
    }
    if let Err(error) = register_panel_shortcut(app, hotkey) {
        eprintln!("floter: clipboard panel shortcut registration failed: {error}");
    }
}

/// Reconcile runtime state (monitor + hotkey) with the settings after any
/// change. Both branches are idempotent, so callers need not diff first. An
/// empty hotkey always ends up with no shortcut registered.
pub fn sync_runtime(app: &AppHandle, enabled: bool, hotkey: &str) {
    if enabled {
        monitor::start(app);
        if hotkey.trim().is_empty() {
            unregister_panel_shortcut(app);
            return;
        }
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
            paths: None,
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

    #[test]
    fn files_entry_round_trips_and_old_rows_without_paths_still_load() {
        // An index written before files existed: no `paths` key at all.
        let legacy = r#"{
            "id": "f",
            "kind": "files",
            "hash": "h",
            "created_at": 1,
            "favorite": false
        }"#;
        let parsed: ClipboardEntry = serde_json::from_str(legacy).expect("deserialize");
        assert_eq!(parsed.kind, "files");
        assert!(parsed.paths.is_none());

        let entry = ClipboardEntry {
            id: "f2".to_string(),
            kind: "files".to_string(),
            text: None,
            paths: Some(vec!["/tmp/a".to_string(), "/tmp/b".to_string()]),
            image_file: None,
            width: None,
            height: None,
            hash: "h".to_string(),
            created_at: 2,
            favorite: false,
        };
        let serialized = serde_json::to_string(&entry).expect("serialize");
        assert!(serialized.contains("/tmp/a"));
        assert!(!serialized.contains("image_file"));
        let back: ClipboardEntry = serde_json::from_str(&serialized).expect("deserialize");
        assert_eq!(back, entry);
    }

    #[test]
    fn preview_candidates_need_exactly_one_image_extension_path() {
        let one = vec!["/x/pic.PNG".to_string()];
        assert!(is_files_preview_candidate(Some(one.as_slice())));

        let two = vec!["/x/pic.png".to_string(), "/y/pic.jpg".to_string()];
        assert!(!is_files_preview_candidate(Some(two.as_slice())));

        let text = vec!["/x/notes.txt".to_string()];
        assert!(!is_files_preview_candidate(Some(text.as_slice())));
        assert!(!is_files_preview_candidate(None));
    }

    #[test]
    fn image_extension_check_understands_both_path_styles() {
        assert!(is_image_file_path("/a/b/c.png"));
        assert!(is_image_file_path("C:\\a\\b\\c.JPG"));
        // A dot earlier in the path does not make an extension.
        assert!(is_image_file_path("/a/b.d/c.webp"));
        assert!(!is_image_file_path("/a/b/c.txt"));
        assert!(!is_image_file_path("/a/b/c"));
        assert!(!is_image_file_path("/a/b/c."));
    }
}
