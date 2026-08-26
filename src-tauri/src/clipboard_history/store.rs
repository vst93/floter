//! Persistence for the clipboard history.
//!
//! The history lives under the app data directory as
//! `clipboard-history/index.json` plus one PNG per image entry under
//! `clipboard-history/images/`. Every index write is atomic (tempfile +
//! rename, mirroring `commands/config.rs`), and a corrupt or missing index is
//! recovered from as an empty history rather than a crash — losing captured
//! entries to corruption is bad; taking the whole panel down with it is worse.

use super::ClipboardEntry;
use sha2::{Digest, Sha256};
use std::collections::HashSet;
use std::io::Write;
use std::path::PathBuf;
use std::sync::{Mutex, MutexGuard};

/// Newest N non-favorite entries are kept. Favorites are exempt from both
/// this cap and the age cap below.
pub const MAX_NON_FAVORITE_ENTRIES: usize = 200;
/// Non-favorite entries older than this are dropped (30 days, in ms).
pub const RETENTION_MS: i64 = 30 * 24 * 60 * 60 * 1000;

static STORE_LOCK: Mutex<()> = Mutex::new(());

const DIR_NAME: &str = "clipboard-history";
const INDEX_FILE_NAME: &str = "index.json";
const IMAGES_DIR_NAME: &str = "images";

/// Serializes every read-modify-write of the index between the monitor task
/// and the Tauri commands.
pub fn store_lock() -> Result<MutexGuard<'static, ()>, String> {
    STORE_LOCK
        .lock()
        .map_err(|_| "History lock poisoned".to_string())
}

/// Where the history lives on disk, rooted at the existing app data dir.
#[derive(Debug, Clone)]
pub struct StorePaths {
    pub root: PathBuf,
}

impl StorePaths {
    pub fn index_file(&self) -> PathBuf {
        self.root.join(INDEX_FILE_NAME)
    }

    pub fn images_dir(&self) -> PathBuf {
        self.root.join(IMAGES_DIR_NAME)
    }
}

/// Paths in the real app data dir, or `None` when the platform cannot name
/// one (which on every supported platform means a broken installation).
pub fn app_store_paths() -> Option<StorePaths> {
    dirs::config_dir().map(|dir| StorePaths {
        root: dir.join("floter").join(DIR_NAME),
    })
}

/// Stable identity of a piece of clipboard content, used both to skip polls
/// that saw nothing new and to dedupe re-copies across the whole history.
pub fn content_hash(bytes: &[u8]) -> String {
    let digest = Sha256::digest(bytes);
    let mut hex = String::with_capacity(digest.len() * 2);
    for byte in digest {
        hex.push_str(&format!("{byte:02x}"));
    }
    hex
}

/// Load the index, recovering from a corrupt or missing file with an empty
/// history — same philosophy as settings recovery in `commands/config.rs`.
pub fn load_index(paths: &StorePaths) -> Vec<ClipboardEntry> {
    std::fs::read(paths.index_file())
        .ok()
        .and_then(|bytes| serde_json::from_slice(&bytes).ok())
        .unwrap_or_default()
}

/// Replace the index atomically. A crash mid-write leaves the previous index
/// intact; at worst an orphaned image file, which [`remove_orphan_images`]
/// cleans up on the next pass.
pub fn save_index(paths: &StorePaths, entries: &[ClipboardEntry]) -> Result<(), String> {
    std::fs::create_dir_all(&paths.root).map_err(|error| error.to_string())?;
    std::fs::create_dir_all(paths.images_dir()).map_err(|error| error.to_string())?;
    let content = serde_json::to_vec_pretty(entries).map_err(|error| error.to_string())?;

    let mut temporary =
        tempfile::NamedTempFile::new_in(&paths.root).map_err(|error| error.to_string())?;
    temporary
        .write_all(&content)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| error.to_string())?;
    temporary
        .persist(paths.index_file())
        .map(|_| ())
        .map_err(|error| error.to_string())?;

    crate::extensions::lock::sync_directory(&paths.root).map_err(|error| error.to_string())
}

/// Apply the retention policy: drop non-favorites older than the retention
/// window, then keep only the newest [`MAX_NON_FAVORITE_ENTRIES`] of what is
/// left. Favorites never expire and are never counted against the cap.
///
/// Entries are kept in their input order (newest first, as stored); returns
/// `(kept, dropped)` so callers can delete the image files of dropped image
/// entries.
pub fn prune_entries(
    entries: Vec<ClipboardEntry>,
    now_ms: i64,
) -> (Vec<ClipboardEntry>, Vec<ClipboardEntry>) {
    let cutoff = now_ms - RETENTION_MS;
    let mut kept = Vec::new();
    let mut dropped = Vec::new();
    for entry in entries {
        if !entry.favorite && entry.created_at < cutoff {
            dropped.push(entry);
        } else {
            kept.push(entry);
        }
    }

    // Oldest non-favorites beyond the cap go, regardless of position in the
    // list. Sorting by timestamp rather than trusting list order keeps this
    // correct even if a hand-edited index is out of order.
    let mut non_favorite_indices: Vec<usize> = kept
        .iter()
        .enumerate()
        .filter(|(_, entry)| !entry.favorite)
        .map(|(index, _)| index)
        .collect();
    non_favorite_indices.sort_by_key(|&index| kept[index].created_at);
    let excess = non_favorite_indices
        .len()
        .saturating_sub(MAX_NON_FAVORITE_ENTRIES);
    let doomed: HashSet<usize> = non_favorite_indices.into_iter().take(excess).collect();

    let mut survivors = Vec::with_capacity(kept.len());
    for (index, entry) in kept.into_iter().enumerate() {
        if doomed.contains(&index) {
            dropped.push(entry);
        } else {
            survivors.push(entry);
        }
    }
    (survivors, dropped)
}

/// Fold a fresh capture into the history per full-history dedupe.
///
/// - Content already sitting on top (any favorite state) is a no-op: `false`
///   is returned and the history is untouched — the copy is where the user
///   expects it, so refreshing its timestamp buys nothing.
/// - Otherwise every *non-favorite* entry carrying the same hash — anywhere in
///   the history, not just consecutively — is removed and the capture lands on
///   top with its fresh timestamp: re-copying something promotes it.
/// - A favorited entry holding the same content survives untouched and its
///   flag carries onto the new top entry. Favorites are never silently
///   dropped by a re-copy; they are only ever removed explicitly.
///
/// Hash comparison is content-based per kind, so this applies to text, image,
/// and files captures alike; hashes of different kinds do not collide.
pub fn fold_capture(entries: &mut Vec<ClipboardEntry>, mut capture: ClipboardEntry) -> bool {
    if entries
        .first()
        .is_some_and(|newest| newest.hash == capture.hash)
    {
        return false;
    }
    let carries_favorite = entries
        .iter()
        .any(|entry| entry.hash == capture.hash && entry.favorite);
    entries.retain(|entry| !(entry.hash == capture.hash && !entry.favorite));
    capture.favorite = carries_favorite;
    entries.insert(0, capture);
    true
}

/// Identity of a multi-file copy: sha256 over the sorted, `\n`-joined
/// canonical paths. Sorting makes the same selection re-copied in a different
/// order dedupe to one entry; joining keeps `a/b` + `c` distinct from
/// `a` + `b/c`. Callers pass canonical paths; stored entries keep originals.
pub fn files_hash(canonical_paths: &[String]) -> String {
    let mut sorted: Vec<&str> = canonical_paths.iter().map(String::as_str).collect();
    sorted.sort_unstable();
    content_hash(sorted.join("\n").as_bytes())
}

pub fn write_image(paths: &StorePaths, id: &str, png_bytes: &[u8]) -> Result<(), String> {
    std::fs::create_dir_all(paths.images_dir()).map_err(|error| error.to_string())?;
    std::fs::write(paths.images_dir().join(format!("{id}.png")), png_bytes)
        .map_err(|error| error.to_string())
}

pub fn read_image(paths: &StorePaths, file_name: &str) -> Result<Vec<u8>, String> {
    // `file_name` comes from our own index, but it round-trips through JSON,
    // so refuse anything that is not a plain file name before joining it.
    if file_name.contains('/') || file_name.contains('\\') || file_name.contains("..") {
        return Err("Invalid image file name".to_string());
    }
    std::fs::read(paths.images_dir().join(file_name)).map_err(|error| error.to_string())
}

pub fn delete_image(paths: &StorePaths, file_name: &str) {
    if file_name.is_empty() || file_name.contains('/') || file_name.contains("..") {
        return;
    }
    let _ = std::fs::remove_file(paths.images_dir().join(file_name));
}

/// Delete every PNG under `images/` no live entry references. Returns how
/// many files were removed.
pub fn remove_orphan_images(paths: &StorePaths, entries: &[ClipboardEntry]) -> usize {
    let referenced: HashSet<&str> = entries
        .iter()
        .filter_map(|entry| entry.image_file.as_deref())
        .collect();
    let Ok(dirents) = std::fs::read_dir(paths.images_dir()) else {
        return 0;
    };
    let mut removed = 0;
    for dirent in dirents.flatten() {
        let name = dirent.file_name();
        let Some(name) = name.to_str() else { continue };
        if !name.ends_with(".png") || referenced.contains(name) {
            continue;
        }
        if std::fs::remove_file(dirent.path()).is_ok() {
            removed += 1;
        }
    }
    removed
}

#[cfg(test)]
mod tests {
    use super::*;

    fn text_entry(id: &str, created_at: i64, favorite: bool, hash: &str) -> ClipboardEntry {
        ClipboardEntry {
            id: id.to_string(),
            kind: "text".to_string(),
            text: Some(id.to_string()),
            paths: None,
            image_file: None,
            width: None,
            height: None,
            hash: hash.to_string(),
            created_at,
            favorite,
        }
    }

    #[test]
    fn entries_older_than_thirty_days_are_pruned_unless_favorite() {
        let now = 1_700_000_000_000;
        let entries = vec![
            text_entry("old-fav", now - RETENTION_MS - 1, true, "h1"),
            text_entry("old", now - RETENTION_MS - 1, false, "h2"),
            text_entry("fresh", now - 1000, false, "h3"),
        ];

        let (kept, dropped) = prune_entries(entries, now);

        assert_eq!(kept.len(), 2);
        assert_eq!(kept[0].id, "old-fav");
        assert_eq!(kept[1].id, "fresh");
        assert_eq!(dropped.len(), 1);
        assert_eq!(dropped[0].id, "old");
    }

    #[test]
    fn count_cap_keeps_the_newest_non_favorites_only() {
        let now = 1_700_000_000_000;
        // MAX + favorites + one ancient favorite: only plain non-favorites
        // beyond the cap may go.
        let mut entries = vec![text_entry("fav", now, true, "hf")];
        for index in 0..MAX_NON_FAVORITE_ENTRIES {
            let created = now - (MAX_NON_FAVORITE_ENTRIES - index) as i64 * 1000;
            entries.push(text_entry(&format!("e{index}"), created, false, "h"));
        }
        entries.push(text_entry("oldest", now - RETENTION_MS, false, "hold"));

        let (kept, dropped) = prune_entries(entries, now);

        assert!(kept.iter().all(|entry| entry.id != "oldest"));
        assert_eq!(
            kept.iter().filter(|entry| !entry.favorite).count(),
            MAX_NON_FAVORITE_ENTRIES
        );
        assert!(kept.iter().any(|entry| entry.id == "fav"));
        assert_eq!(dropped.len(), 1);
        assert_eq!(dropped[0].id, "oldest");
    }

    #[test]
    fn mixed_pruning_applies_age_first_then_count() {
        let now = 1_700_000_000_000;
        let mut entries: Vec<ClipboardEntry> = (0..MAX_NON_FAVORITE_ENTRIES + 5)
            .map(|index| {
                let created = if index < 3 {
                    // Three ancient non-favorites: dropped by age…
                    now - RETENTION_MS - (4 - index as i64) * 1000
                } else {
                    // …the rest by recency order under the cap.
                    now - (index as i64) * 1000
                };
                text_entry(&format!("e{index}"), created, false, "h")
            })
            .collect();
        entries.insert(0, text_entry("fav-ancient", 0, true, "hfa"));

        let (kept, dropped) = prune_entries(entries, now);

        // Three fall to the age cap, two more (the oldest survivors) to the
        // count cap.
        assert_eq!(dropped.len(), 5);
        assert!(dropped.iter().all(|entry| entry.id.starts_with('e')));
        assert_eq!(
            kept.iter().filter(|entry| !entry.favorite).count(),
            MAX_NON_FAVORITE_ENTRIES
        );
        assert!(kept.iter().any(|entry| entry.id == "fav-ancient"));
    }

    #[test]
    fn identical_content_hashes_equal_across_kinds() {
        assert_eq!(content_hash(b"hello"), content_hash(b"hello"));
        assert_ne!(content_hash(b"hello"), content_hash(b"world"));
    }

    #[test]
    fn duplicate_at_top_is_a_no_op_regardless_of_favorite_state() {
        let now = 1_700_000_000_000;
        for favorite in [false, true] {
            let mut entries = vec![text_entry("top", now, favorite, "h")];
            let capture = text_entry("fresh", now + 5, false, "h");

            assert!(!fold_capture(&mut entries, capture));

            assert_eq!(entries.len(), 1);
            assert_eq!(entries[0].id, "top");
            assert_eq!(entries[0].created_at, now);
            assert_eq!(entries[0].favorite, favorite);
        }
    }

    #[test]
    fn duplicate_older_elsewhere_moves_to_top_with_fresh_timestamp() {
        let now = 1_700_000_000_000;
        let mut entries = vec![
            text_entry("newer", now - 1000, false, "h2"),
            text_entry("older", now - 60_000, false, "h1"),
        ];

        assert!(fold_capture(
            &mut entries,
            text_entry("fresh", now, false, "h1")
        ));

        assert_eq!(entries.len(), 2);
        assert_eq!(entries[0].id, "fresh");
        assert_eq!(entries[0].created_at, now);
        assert_eq!(entries[1].id, "newer");
        assert!(!entries.iter().any(|entry| entry.id == "older"));
    }

    #[test]
    fn favorited_duplicate_survives_and_carries_its_flag_to_the_top() {
        let now = 1_700_000_000_000;
        let mut entries = vec![
            text_entry("other", now - 500, false, "h2"),
            text_entry("starred", now - 60_000, true, "h1"),
        ];

        assert!(fold_capture(
            &mut entries,
            text_entry("fresh", now, false, "h1")
        ));

        // The starred entry stays put; the fresh copy lands on top carrying
        // the favorite flag.
        assert_eq!(entries.len(), 3);
        assert_eq!(entries[0].id, "fresh");
        assert_eq!(entries[0].favorite, true);
        assert!(entries.iter().any(|entry| entry.id == "starred"));
    }

    #[test]
    fn a_hash_match_only_touches_entries_of_the_same_content() {
        let now = 1_700_000_000_000;
        let mut entries = vec![
            text_entry("text", now - 1000, false, "htext"),
            text_entry("image", now - 2000, false, "himage"),
            text_entry("files", now - 3000, false, "hfiles"),
        ];

        assert!(fold_capture(
            &mut entries,
            text_entry("fresh", now, false, "himage")
        ));

        assert_eq!(entries.len(), 3);
        assert_eq!(entries[0].id, "fresh");
        assert!(entries.iter().any(|entry| entry.id == "text"));
        assert!(entries.iter().any(|entry| entry.id == "files"));
        assert!(!entries.iter().any(|entry| entry.id == "image"));
    }

    #[test]
    fn files_hash_is_order_insensitive_but_content_sensitive() {
        let ab = vec!["/a".to_string(), "/b".to_string()];
        let ba = vec!["/b".to_string(), "/a".to_string()];
        assert_eq!(files_hash(&ab), files_hash(&ba));

        let ac = vec!["/a".to_string(), "/c".to_string()];
        assert_ne!(files_hash(&ab), files_hash(&ac));

        // Joining (not concatenating) keeps path-boundary collisions apart.
        let split = vec!["/a/b".to_string(), "/c".to_string()];
        let merged = vec!["/a".to_string(), "/b/c".to_string()];
        assert_ne!(files_hash(&split), files_hash(&merged));
    }

    #[test]
    fn index_round_trips_through_disk() {
        let directory = tempfile::tempdir().expect("temp dir");
        let paths = StorePaths {
            root: directory.path().to_path_buf(),
        };
        let entries = vec![
            text_entry("a", 123, false, "ha"),
            ClipboardEntry {
                id: "img".to_string(),
                kind: "image".to_string(),
                text: None,
                paths: None,
                image_file: Some("img.png".to_string()),
                width: Some(4),
                height: Some(6),
                hash: "hi".to_string(),
                created_at: 456,
                favorite: true,
            },
        ];

        save_index(&paths, &entries).expect("save index");

        assert_eq!(load_index(&paths), entries);
    }

    #[test]
    fn a_corrupt_index_recovers_as_empty_rather_than_crashing() {
        let directory = tempfile::tempdir().expect("temp dir");
        let paths = StorePaths {
            root: directory.path().to_path_buf(),
        };
        std::fs::create_dir_all(directory.path()).expect("create dir");
        std::fs::write(paths.index_file(), b"{ not json").expect("write garbage");

        assert_eq!(load_index(&paths), Vec::new());

        // And saving over it repairs the store.
        save_index(&paths, &[text_entry("a", 1, false, "h")]).expect("save");
        assert_eq!(load_index(&paths).len(), 1);
    }

    #[test]
    fn orphan_images_are_removed_referenced_ones_survive() {
        let directory = tempfile::tempdir().expect("temp dir");
        let paths = StorePaths {
            root: directory.path().to_path_buf(),
        };
        write_image(&paths, "live", b"png").expect("write live image");
        write_image(&paths, "dead", b"png").expect("write dead image");

        let entries = vec![ClipboardEntry {
            id: "live".to_string(),
            kind: "image".to_string(),
            text: None,
            paths: None,
            image_file: Some("live.png".to_string()),
            width: None,
            height: None,
            hash: "h".to_string(),
            created_at: 1,
            favorite: false,
        }];

        assert_eq!(remove_orphan_images(&paths, &entries), 1);
        assert!(paths.images_dir().join("live.png").exists());
        assert!(!paths.images_dir().join("dead.png").exists());
    }
}
