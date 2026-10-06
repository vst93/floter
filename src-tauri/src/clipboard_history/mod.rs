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
//! surfaced in a terminal-styled panel summoned from launcher search or
//! `floter clip`. R56 removed the panel's global hotkey entirely: the panel
//! keeps its own invocation paths, and a settings-hide-only binding would have
//! been a configuration the UI no longer shows.

pub mod monitor;
pub mod search;
pub mod store;

use serde::{Deserialize, Serialize};
use std::borrow::Cow;
use std::sync::Mutex;

use tauri::{AppHandle, Manager};

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
    let (kept, dropped) = store::prune_entries(loaded, now_ms(), store::configured_max_items());
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
    // Same-frame invariant — `cache` was filled just above — but a sync
    // command must not carry an abort point: a panic here escapes the
    // webkit2gtk `extern "C"` trampoline and aborts the process instead of
    // unwinding (R111/R112, docs/AGENT-NOTES.md). Return an error instead.
    let entries = cache.as_mut().ok_or("History cache unavailable")?;
    store::update_entries(entries, f)
}

// ---- Tauri commands ------------------------------------------------------

/// R27 · the clipboard plugin's own settings, as its page reads them.
///
/// The page runs in a sandboxed iframe and cannot reach `get_settings` (which
/// carries the whole app settings object); this narrow pair reads and writes
/// exactly the plugin's capacity, through the same settings lock and atomic
/// write every other settings change uses.
#[tauri::command]
pub fn clipboard_get_settings() -> crate::commands::config::ClipboardPluginSettings {
    crate::commands::config::ClipboardPluginSettings {
        max_items: crate::commands::config::load_settings().clipboard_history_max_items,
    }
}

/// R27 · replace the clipboard plugin's settings.
///
/// Writing the capacity and truncating the live history happen together, in
/// that order: `prune_and_save` reads the freshly persisted number (see
/// `store::configured_max_items`), so lowering the setting drops the entries
/// that no longer fit **now** rather than on the next capture. The prune is
/// best-effort — a settings write that succeeded must not be reported as failed
/// because the retention pass could not reach the disk — but a *shrunk*
/// history is the point of the control, so its error is logged.
#[tauri::command]
pub fn clipboard_set_settings(
    app: AppHandle,
    settings: crate::commands::config::ClipboardPluginSettings,
) -> Result<crate::commands::config::ClipboardPluginSettings, String> {
    let max_items = crate::commands::config::write_clipboard_max_items(settings.max_items)?;
    if let Err(error) = mutate_history(&app, monitor::prune_and_save) {
        tracing::warn!("floter: clipboard capacity prune failed: {error}");
    }
    Ok(crate::commands::config::ClipboardPluginSettings { max_items })
}

/// R89 · how much of a text entry's content the **list** IPC carries.
///
/// The launcher's clipboard mode reads the whole history in one call, and every
/// entry could be a full 512 KB (`monitor::MAX_TEXT_BYTES`) — 300 of those is a
/// ~150 MB payload for a list that paints one line per row. The list therefore
/// ships only this prefix; the disk history, the monitor's in-memory copy, and
/// `clipboard_copy_entry` all keep the full text, so copying a row is unchanged.
/// The byte is the ceiling, not a guarantee: the prefix always ends on a UTF-8
/// character boundary (see [`text_prefix`]).
pub const LIST_TEXT_PREFIX_BYTES: usize = 8192;

/// R89 · the longest prefix of `text` that fits in `max_bytes` without splitting
/// a UTF-8 character.
///
/// `std`'s `floor_char_boundary` is not stable, so the walk back to a boundary is
/// written here rather than pulled in. A byte index that is already a boundary
/// (the ASCII case, and any multibyte text whose characters happen to line up)
/// is returned untouched.
pub fn text_prefix(text: &str, max_bytes: usize) -> &str {
    if text.len() <= max_bytes {
        return text;
    }
    let mut end = max_bytes;
    while end > 0 && !text.is_char_boundary(end) {
        end -= 1;
    }
    &text[..end]
}

/// R89 · the list's view of one entry: everything, with `text` cut to
/// [`LIST_TEXT_PREFIX_BYTES`]. Only the read path calls this — disk and the
/// monitor keep the full text, and `paths` is left alone (a path list is small
/// and every element is a search target).
pub fn entry_for_list(mut entry: ClipboardEntry) -> ClipboardEntry {
    if let Some(text) = entry.text.as_mut() {
        if text.len() > LIST_TEXT_PREFIX_BYTES {
            *text = text_prefix(text, LIST_TEXT_PREFIX_BYTES).to_string();
        }
    }
    entry
}

/// R89 · read the history for the list.
///
/// `filter` is the launcher's clipboard needle: whitespace-split AND tokens,
/// each of which must hit one of the entry's search fields — the same rule the
/// frontend applies in memory (`search::entry_matches`, the Rust twin of
/// `plugins/search.ts` + `clipboardEntrySearchFields`). It is applied to the
/// **full** text read off disk, so a match past an entry's prefix is still
/// reachable; the rows that come back are then cut to the list prefix. `None` (or
/// a blank string) is no filter at all.
#[tauri::command]
pub fn clipboard_get_entries(
    app: AppHandle,
    filter: Option<String>,
) -> Result<Vec<ClipboardEntry>, String> {
    let tokens = search::search_tokens(filter.as_deref().unwrap_or_default());
    let entries = read_history(&app)?;
    if tokens.is_empty() {
        return Ok(entries.into_iter().map(entry_for_list).collect());
    }
    Ok(entries
        .into_iter()
        .filter(|entry| search::entry_matches(entry, &tokens))
        .map(entry_for_list)
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
    let result = mutate_history(&app, |entries| {
        let removed = store::take_non_favorites(entries);
        store::save_index(&paths, entries)?;
        for entry in &removed {
            if let Some(file) = &entry.image_file {
                store::delete_image(&paths, file);
            }
        }
        Ok(())
    });
    // R7-10b: clearing history is the clipboard plugin page's one long action,
    // and a plugin page is the surface a user is most likely to close the panel
    // over (Esc returns to the launcher, the toggle hides the panel). The
    // frontend keeps its own toast for the on-screen case; the notification
    // rule in `notifications` decides whether this one is raised at all.
    crate::notifications::notify_completion(
        &app,
        &crate::notifications::Subject::Plugin(crate::plugin_pages::CLIPBOARD_PLUGIN_ID),
        crate::notifications::CompletionAction::ClearHistory,
        crate::notifications::outcome_of(&result),
    );
    result
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

/// R38 · the default ceiling for a row thumbnail, in pixels on the long side.
/// The launcher's icon plate is 28u, so 32 gives the browser a hair of source
/// to scale without handing it a full-resolution capture.
pub const THUMBNAIL_MAX_SIDE: u32 = 32;
/// R38 · bounds on a caller-supplied thumbnail size. A tiny floor still
/// produces a legible glyph; the ceiling keeps a bad caller from asking the
/// decoder to do real work under the name "thumbnail".
pub const THUMBNAIL_MIN_SIDE: u32 = 8;
pub const THUMBNAIL_MAX_REQUEST: u32 = 128;

/// The long side a thumbnail request resolves to: the caller's number clamped
/// into the accepted range, or the shipped default when none was given. A
/// malformed size can therefore never ask the decoder for a full-resolution
/// pass under the name "thumbnail".
pub fn thumbnail_side(requested: Option<u32>) -> u32 {
    requested
        .unwrap_or(THUMBNAIL_MAX_SIDE)
        .clamp(THUMBNAIL_MIN_SIDE, THUMBNAIL_MAX_REQUEST)
}

/// R38 · a small PNG `data:` URL for one image entry's row icon.
///
/// The history stores full-resolution PNGs on disk (`image_file`); a launcher
/// row wants a 32px glyph, and handing the webview the whole capture so a
/// 28u `<img>` can shrink it is exactly the per-row decode the round forbids.
/// So the backend decodes the stored PNG **once**, box-downscales it to the
/// requested side and re-encodes a thumbnail-sized PNG as a data URL. Nothing
/// is written to disk: the caller memoizes the string for the session and the
/// thumbnail is recomputed on demand next launch. Only the image kind is
/// served — a file list keeps its folder glyph.
#[tauri::command]
pub fn clipboard_thumbnail(
    app: AppHandle,
    id: String,
    size: Option<u32>,
) -> Result<String, String> {
    use base64::Engine as _;
    let paths = store::app_store_paths().ok_or("No app data directory")?;
    let entry = read_history(&app)?
        .into_iter()
        .find(|entry| entry.id == id)
        .ok_or_else(|| format!("Unknown clipboard entry: {id}"))?;
    let file = entry.image_file.ok_or("Entry is not an image")?;
    let bytes = store::read_image(&paths, &file)?;
    let (width, height, rgba) = monitor::decode_png(&bytes)?;
    let side = thumbnail_side(size);
    let (small_width, small_height, small) = monitor::downscale_rgba(width, height, &rgba, side);
    let png = monitor::encode_png(small_width, small_height, &small)?;
    Ok(format!(
        "data:image/png;base64,{}",
        base64::engine::general_purpose::STANDARD.encode(png)
    ))
}

// ---- Lifecycle ------------------------------------------------------------

/// Called once from the setup hook: start the monitor when the setting says
/// so. The panel has no global hotkey of its own — it stays reachable through
/// launcher search and `floter clip`, the two invocation paths every platform
/// can honour.
pub fn initialize(app: &AppHandle, enabled: bool) {
    if !enabled {
        return;
    }
    monitor::start(app);
}

/// Reconcile the monitor with the settings after any change. Both branches are
/// idempotent, so callers need not diff first.
pub fn sync_runtime(app: &AppHandle, enabled: bool) {
    if enabled {
        monitor::start(app);
    } else {
        monitor::stop(app);
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

    /// R38 · the thumbnail size is bounded on both ends: no size means the
    /// shipped icon default, a tiny request is lifted to the floor, and an
    /// absurd one is capped so "thumbnail" can never become a full decode.
    #[test]
    fn thumbnail_side_is_bounded_on_both_ends() {
        assert_eq!(thumbnail_side(None), THUMBNAIL_MAX_SIDE);
        assert_eq!(thumbnail_side(Some(32)), 32);
        assert_eq!(thumbnail_side(Some(1)), THUMBNAIL_MIN_SIDE);
        assert_eq!(thumbnail_side(Some(0)), THUMBNAIL_MIN_SIDE);
        assert_eq!(thumbnail_side(Some(u32::MAX)), THUMBNAIL_MAX_REQUEST);
    }

    // ── R89 · the list text prefix ────────────────────────────────────────

    /// The prefix is a byte ceiling that never lands inside a character. Each
    /// case puts the 8192-byte line just before, on, and inside a 3-byte CJK
    /// character; the result must stay a valid `&str` (the compiler enforces it)
    /// and must be the last boundary at or before the ceiling.
    #[test]
    fn the_list_prefix_never_splits_a_utf8_character() {
        // '剪' is 3 bytes (E5 89 AA).
        let cjk = '剪';
        let max = LIST_TEXT_PREFIX_BYTES;

        // The ceiling falls one byte *past* the last character: back up to its
        // start (8191 a's, then the character).
        let past = format!("{}{cjk}", "a".repeat(max - 1));
        assert_eq!(past.len(), max + 2);
        let cut = text_prefix(&past, max);
        assert_eq!(cut.len(), max - 1);
        assert!(past.is_char_boundary(cut.len()));
        assert_eq!(cut, "a".repeat(max - 1));

        // The ceiling falls *inside* the character (its second byte): the cut
        // backs all the way up to its start, two bytes short.
        let inside = format!("{}{cjk}", "a".repeat(max - 2));
        let cut = text_prefix(&inside, max);
        assert_eq!(cut.len(), max - 2);
        assert!(inside.is_char_boundary(cut.len()));

        // The ceiling falls exactly on a boundary: nothing is given back.
        let on_boundary = format!("{}{cjk}", "a".repeat(max));
        assert_eq!(text_prefix(&on_boundary, max).len(), max);

        // A text that already fits is returned whole, byte for byte.
        let short = "a".repeat(max);
        assert_eq!(text_prefix(&short, max), short);
        assert_eq!(text_prefix("", max), "");
    }

    /// The read path applies that prefix to `text` only, and leaves a short text
    /// and the `paths` field untouched.
    #[test]
    fn entry_for_list_truncates_text_and_leaves_paths_alone() {
        let long = "a".repeat(LIST_TEXT_PREFIX_BYTES + 10);
        let cut = entry_for_list(ClipboardEntry {
            text: Some(long.clone()),
            ..entry("h", 1)
        });
        assert_eq!(
            cut.text.as_deref().map(str::len),
            Some(LIST_TEXT_PREFIX_BYTES)
        );
        assert_eq!(cut.text.as_deref(), Some(&long[..LIST_TEXT_PREFIX_BYTES]));

        let short = entry_for_list(entry("h", 1));
        assert_eq!(short.text.as_deref(), Some("content-h"));

        let files = entry_for_list(ClipboardEntry {
            kind: "files".to_string(),
            text: Some(long.clone()),
            paths: Some(vec!["/tmp/".to_string() + &long]),
            ..entry("h", 1)
        });
        assert_eq!(
            files.text.as_deref().map(str::len),
            Some(LIST_TEXT_PREFIX_BYTES)
        );
        assert_eq!(
            files.paths.as_ref().map(|paths| paths[0].len()),
            Some(5 + long.len())
        );
    }
}
