//! R50 · built-in calculator history.
//!
//! The calculator plugin evaluates expressions in the webview (the evaluation
//! core is a JavaScript library — see `src/calculator.ts`), and asks this
//! module to record a successful result. The history lives under the app data
//! directory as `calculator-history/index.json`, written atomically
//! (tempfile + rename, the same discipline as `clipboard_history` and
//! `commands/config.rs`), and survives restarts. A corrupt or missing index
//! recovers as an empty history rather than taking the launcher down with it.
//!
//! ## Retention
//!
//! Two axes, both configurable (`CalculatorPluginSettings`): a capacity on
//! non-favorite entries and an age window. A **favorite is exempt from both** —
//! the user's 「收藏的记录不要清理，不管是超过条数还是时间」 — so a starred row
//! outlives a full history and an expired one alike.
//!
//! Pruning happens on **write**, not on read: a new entry, an un-favorite, a
//! delete and a settings change each trim the index in the same critical
//! section that persists it. The read path (`calculator_get_entries`) returns
//! exactly what is stored, so it stays a pure lookup with no surprise writes —
//! the shape the round's spec asked for.
//!
//! ## Dedupe
//!
//! Re-evaluating an expression promotes it to the top rather than stacking a
//! second copy: any stored entry with the same expression text is folded into
//! the new one, carrying the favorite flag forward (a starred calculation the
//! user re-runs is still starred). This is `clipboard_history::store::fold_capture`'s
//! rule, applied to expressions.

use serde::{Deserialize, Serialize};
use std::io::Write;
use std::path::PathBuf;
use std::sync::Mutex;

/// One recorded calculation.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct CalculatorEntry {
    pub id: String,
    /// The expression exactly as the user typed it (trimmed on entry).
    pub expression: String,
    /// The printed result, the string Enter copies in `result` mode.
    pub result: String,
    /// Unix timestamp in milliseconds.
    pub created_at: i64,
    /// Favorites are exempt from the capacity cap and the age window.
    pub favorite: bool,
}

static STORE_LOCK: Mutex<()> = Mutex::new(());

fn now_ms() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as i64
}

/// The history directory inside the existing app data dir, or `None` when the
/// platform cannot name one.
fn store_dir() -> Option<PathBuf> {
    dirs::config_dir().map(|dir| dir.join("floter").join("calculator-history"))
}

fn index_file() -> Option<PathBuf> {
    store_dir().map(|dir| dir.join("index.json"))
}

/// Load the index, recovering from a corrupt or missing file with an empty
/// history.
fn load_index() -> Vec<CalculatorEntry> {
    index_file()
        .and_then(|path| std::fs::read(path).ok())
        .and_then(|bytes| serde_json::from_slice(&bytes).ok())
        .unwrap_or_default()
}

/// Replace the index atomically. A crash mid-write leaves the previous index
/// intact.
fn save_index(entries: &[CalculatorEntry]) -> Result<(), String> {
    let Some(dir) = store_dir() else {
        return Err("No app data directory".to_string());
    };
    std::fs::create_dir_all(&dir).map_err(|error| error.to_string())?;
    let content = serde_json::to_vec_pretty(entries).map_err(|error| error.to_string())?;

    let mut temporary = tempfile::NamedTempFile::new_in(&dir).map_err(|error| error.to_string())?;
    temporary
        .write_all(&content)
        .and_then(|_| temporary.flush())
        .and_then(|_| temporary.as_file().sync_all())
        .map_err(|error| error.to_string())?;
    temporary
        .persist(dir.join("index.json"))
        .map_err(|error| error.to_string())?;
    Ok(())
}

/// Fold a fresh calculation into the history: replace every entry carrying the
/// same expression with the new top row, keeping the entry's favorite flag if
/// any previous copy was starred. Entries keep their relative order otherwise.
pub fn fold_entry(entries: &mut Vec<CalculatorEntry>, mut entry: CalculatorEntry) {
    let carries_favorite = entries
        .iter()
        .any(|existing| existing.expression == entry.expression && existing.favorite);
    entries.retain(|existing| existing.expression != entry.expression);
    entry.favorite = entry.favorite || carries_favorite;
    entries.insert(0, entry);
}

/// The age cutoff for a window of `days` at `now`; `i64::MIN` when the window
/// is off, so nothing is ever older than it.
fn retention_cutoff(now: i64, days: u32) -> i64 {
    if days == 0 {
        i64::MIN
    } else {
        now - (days as i64) * 24 * 60 * 60 * 1000
    }
}

/// Apply the retention policy: drop non-favorite entries older than the window,
/// then the oldest non-favorites beyond the capacity. Favorites are exempt from
/// both. Entries keep their input order (newest first); the cap is applied by
/// age, not by position, so a hand-edited out-of-order index still prunes the
/// truly oldest. Returns `(kept, dropped)`.
pub fn prune_entries(
    entries: Vec<CalculatorEntry>,
    now: i64,
    max_items: usize,
    retention_days: u32,
) -> (Vec<CalculatorEntry>, Vec<CalculatorEntry>) {
    let cutoff = retention_cutoff(now, retention_days);
    let mut fresh = Vec::new();
    let mut dropped = Vec::new();
    for entry in entries {
        if !entry.favorite && entry.created_at < cutoff {
            dropped.push(entry);
        } else {
            fresh.push(entry);
        }
    }

    let mut non_favorite_indices: Vec<usize> = fresh
        .iter()
        .enumerate()
        .filter(|(_, entry)| !entry.favorite)
        .map(|(index, _)| index)
        .collect();
    non_favorite_indices.sort_by_key(|&index| fresh[index].created_at);
    let excess = non_favorite_indices.len().saturating_sub(max_items);
    let doomed: std::collections::HashSet<usize> =
        non_favorite_indices.into_iter().take(excess).collect();

    let mut kept = Vec::with_capacity(fresh.len());
    for (index, entry) in fresh.into_iter().enumerate() {
        if doomed.contains(&index) {
            dropped.push(entry);
        } else {
            kept.push(entry);
        }
    }
    (kept, dropped)
}

/// Serialize a read-modify-write of the index: lock, load, apply the change,
/// prune with the current settings, save. Every write path funnels through here
/// so retention is applied once per write and never on read.
fn update(
    f: impl FnOnce(&mut Vec<CalculatorEntry>) -> Result<(), String>,
) -> Result<Vec<CalculatorEntry>, String> {
    let _guard = STORE_LOCK
        .lock()
        .map_err(|_| "Calculator store lock poisoned".to_string())?;
    let mut entries = load_index();
    f(&mut entries)?;
    let settings = crate::commands::config::load_settings().calculator_plugin;
    let (kept, _dropped) = prune_entries(
        entries,
        now_ms(),
        settings.max_items as usize,
        settings.retention_days,
    );
    save_index(&kept)?;
    Ok(kept)
}

// ---- Tauri commands ------------------------------------------------------

#[tauri::command]
pub fn calculator_get_entries() -> Result<Vec<CalculatorEntry>, String> {
    let _guard = STORE_LOCK
        .lock()
        .map_err(|_| "Calculator store lock poisoned".to_string())?;
    Ok(load_index())
}

/// Record a successful calculation and return the pruned history.
#[tauri::command]
pub fn calculator_add_entry(
    expression: String,
    result: String,
) -> Result<Vec<CalculatorEntry>, String> {
    let expression = expression.trim().to_string();
    if expression.is_empty() {
        return Err("Empty expression".to_string());
    }
    let entry = CalculatorEntry {
        id: uuid::Uuid::new_v4().to_string(),
        expression,
        result,
        created_at: now_ms(),
        favorite: false,
    };
    update(|entries| {
        fold_entry(entries, entry);
        Ok(())
    })
}

#[tauri::command]
pub fn calculator_set_favorite(id: String, favorite: bool) -> Result<Vec<CalculatorEntry>, String> {
    update(|entries| {
        entries
            .iter_mut()
            .find(|entry| entry.id == id)
            .map(|entry| entry.favorite = favorite)
            .ok_or_else(|| format!("Unknown calculator entry: {id}"))
    })
}

#[tauri::command]
pub fn calculator_delete(id: String) -> Result<Vec<CalculatorEntry>, String> {
    update(|entries| {
        let position = entries
            .iter()
            .position(|entry| entry.id == id)
            .ok_or_else(|| format!("Unknown calculator entry: {id}"))?;
        entries.remove(position);
        Ok(())
    })
}

/// Clear the history, keeping favorites — the same rule R38's clipboard clear
/// uses. Favorites are the user's deliberate keep.
#[tauri::command]
pub fn calculator_clear_history() -> Result<(), String> {
    let _guard = STORE_LOCK
        .lock()
        .map_err(|_| "Calculator store lock poisoned".to_string())?;
    let entries = load_index();
    let kept: Vec<CalculatorEntry> = entries.into_iter().filter(|entry| entry.favorite).collect();
    save_index(&kept)
}

#[tauri::command]
pub fn calculator_get_settings() -> crate::commands::config::CalculatorPluginSettings {
    crate::commands::config::load_settings().calculator_plugin
}

/// Persist the plugin's settings and prune the live history with the freshly
/// stored numbers, so shrinking the capacity or the window takes effect now.
#[tauri::command]
pub fn calculator_set_settings(
    settings: crate::commands::config::CalculatorPluginSettings,
) -> Result<crate::commands::config::CalculatorPluginSettings, String> {
    let stored = crate::commands::config::write_calculator_settings(settings)?;
    let _guard = STORE_LOCK
        .lock()
        .map_err(|_| "Calculator store lock poisoned".to_string())?;
    let entries = load_index();
    let (kept, _dropped) = prune_entries(
        entries,
        now_ms(),
        stored.max_items as usize,
        stored.retention_days,
    );
    save_index(&kept)?;
    Ok(stored)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn entry(id: &str, expression: &str, created_at: i64, favorite: bool) -> CalculatorEntry {
        CalculatorEntry {
            id: id.to_string(),
            expression: expression.to_string(),
            result: "1".to_string(),
            created_at,
            favorite,
        }
    }

    const DAY: i64 = 24 * 60 * 60 * 1000;

    #[test]
    fn the_age_window_drops_only_non_favorites() {
        let now = 100 * DAY;
        let entries = vec![
            entry("old-fav", "1+1", now - 40 * DAY, true),
            entry("old", "1+2", now - 40 * DAY, false),
            entry("fresh", "1+3", now - 2 * DAY, false),
        ];
        let (kept, dropped) = prune_entries(entries, now, 100, 30);
        assert_eq!(
            kept.iter().map(|e| e.id.as_str()).collect::<Vec<_>>(),
            vec!["old-fav", "fresh"]
        );
        assert_eq!(dropped.len(), 1);
        assert_eq!(dropped[0].id, "old");
    }

    #[test]
    fn the_capacity_drops_the_oldest_non_favorites_and_spares_favorites() {
        let now = 10 * DAY;
        let mut entries = vec![entry("fav", "9", 0, true)];
        for index in 0..5 {
            entries.push(entry(
                &format!("e{index}"),
                &format!("{index}"),
                (index as i64) * 1000,
                false,
            ));
        }
        let (kept, dropped) = prune_entries(entries, now, 2, 0);
        // The two newest non-favorites plus the favorite survive.
        let ids: Vec<&str> = kept.iter().map(|e| e.id.as_str()).collect();
        assert!(ids.contains(&"fav"));
        assert_eq!(ids.len(), 3);
        assert_eq!(dropped.len(), 3);
    }

    #[test]
    fn a_full_and_expired_history_keeps_favorites_and_drops_the_rest() {
        let now = 100 * DAY;
        let entries = vec![
            entry("fav-old", "1", now - 40 * DAY, true),
            entry("plain-old", "2", now - 40 * DAY, false),
            entry("plain-a", "3", now - 1000, false),
            entry("plain-b", "4", now - 900, false),
            entry("plain-c", "5", now - 800, false),
        ];
        let (kept, dropped) = prune_entries(entries, now, 1, 30);
        assert_eq!(
            kept.iter().map(|e| e.id.as_str()).collect::<Vec<_>>(),
            vec!["fav-old", "plain-c"]
        );
        assert_eq!(dropped.len(), 3);
    }

    #[test]
    fn re_evaluating_an_expression_promotes_it_and_carries_the_favorite() {
        let mut entries = vec![
            entry("newer", "2+2", 200, false),
            entry("target", "1+1", 100, true),
        ];
        fold_entry(&mut entries, entry("fresh", "1+1", 300, false));
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[0].id, "fresh");
        assert!(
            entries[0].favorite,
            "the carried favorite survives the fold"
        );
        assert!(entries
            .iter()
            .all(|e| e.expression != "1+1" || e.id == "fresh"));
    }

    #[test]
    fn the_age_window_off_keeps_everything() {
        let now = 1000 * DAY;
        let entries = vec![entry("ancient", "1", 0, false)];
        let (kept, dropped) = prune_entries(entries, now, 100, 0);
        assert_eq!(kept.len(), 1);
        assert!(dropped.is_empty());
    }
}
