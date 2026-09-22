//! Chromium `History` reader.
//!
//! `History` is a SQLite database, and the browser holds a write lock on it for
//! as long as it runs. Opening it in place therefore fails or — worse — can
//! read a half-written page. The standard answer, and the one used here, is to
//! copy the file (plus its `-wal`/`-shm` sidecars, which hold the commits not
//! yet checkpointed) into a temp directory, query the copy read-only, and let
//! the temp directory delete itself on the way out.
//!
//! The copy lives in `TMPDIR` via `tempfile`, so a read never leaves a stray
//! file behind even if the process is interrupted mid-query.

use rusqlite::{Connection, OpenFlags};
use serde::Serialize;
use std::path::{Path, PathBuf};

use super::chromium_time_to_unix;

/// One `urls` row.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct BrowserHistoryEntry {
    pub id: i64,
    pub title: String,
    pub url: String,
    pub visit_count: u32,
    /// Unix seconds.
    pub last_visit: i64,
}

/// Copy `source` (and any `-wal`/`-shm` sidecars) into `destination`.
///
/// Split out so the copy path is testable without a live browser: the sidecar
/// handling is the part that is easy to get wrong and invisible when it is.
fn copy_database(source: &Path, destination: &Path) -> Result<(), String> {
    std::fs::copy(source, destination)
        .map_err(|error| format!("could not copy {source:?}: {error}"))?;
    for suffix in ["-wal", "-shm"] {
        let sidecar = PathBuf::from(format!("{}{suffix}", source.display()));
        if sidecar.is_file() {
            let target = PathBuf::from(format!("{}{suffix}", destination.display()));
            // A sidecar that vanishes between the check and the copy is not a
            // failure: the main file alone is still a readable database.
            let _ = std::fs::copy(&sidecar, &target);
        }
    }
    Ok(())
}

/// Escape a user query for a `LIKE` pattern.
///
/// `%` and `_` are wildcards in `LIKE`, so a user searching for `100%` would
/// otherwise match far more than they typed. `\` is the escape character and so
/// has to escape itself first.
fn escape_like(query: &str) -> String {
    query
        .replace('\\', "\\\\")
        .replace('%', "\\%")
        .replace('_', "\\_")
}

/// Query a `History` file.
///
/// `days` of `0` disables the recency filter; otherwise only rows visited within
/// the last `days` days are returned. An empty `query` returns the most recently
/// visited URLs, which is the launcher's empty-state view.
pub fn query_history_file(
    source: &Path,
    query: &str,
    limit: usize,
    days: u32,
) -> Result<Vec<BrowserHistoryEntry>, String> {
    if !source.is_file() {
        return Err(format!("no history file at {source:?}"));
    }
    let temp = tempfile::tempdir().map_err(|error| format!("no temp dir: {error}"))?;
    let copy = temp.path().join("History");
    copy_database(source, &copy)?;
    query_history_database(&copy, query, limit, days)
}

/// Query an already-copied database. Kept separate from
/// [`query_history_file`] so the SQL can be tested against a fixture database
/// without a browser's lock in the way.
pub fn query_history_database(
    database: &Path,
    query: &str,
    limit: usize,
    days: u32,
) -> Result<Vec<BrowserHistoryEntry>, String> {
    let connection = Connection::open_with_flags(database, OpenFlags::SQLITE_OPEN_READ_ONLY)
        .map_err(|error| format!("could not open history: {error}"))?;

    // A fixed statement with the two optional filters expressed as
    // "parameter is empty / zero, so this clause passes" rather than a
    // dynamically built WHERE. The empty-query and no-recency-filter cases
    // then take the same code path as the filtered ones.
    const SQL: &str = "SELECT id, url, title, visit_count, last_visit_time FROM urls \
         WHERE (:needle = '' OR url LIKE :needle ESCAPE '\\' OR title LIKE :needle ESCAPE '\\') \
           AND (:days = 0 OR last_visit_time >= :cutoff) \
         ORDER BY last_visit_time DESC LIMIT :limit";

    let mut statement = connection
        .prepare(SQL)
        .map_err(|error| format!("could not read history: {error}"))?;

    let trimmed = query.trim();
    let needle = if trimmed.is_empty() {
        String::new()
    } else {
        format!("%{}%", escape_like(&trimmed.to_lowercase()))
    };
    let cutoff = super::chromium_now_micros() - i64::from(days) * 86_400 * 1_000_000;
    let limit = limit.min(i64::MAX as usize) as i64;

    let rows = statement
        .query_map(
            rusqlite::named_params! {
                ":needle": needle,
                ":days": i64::from(days),
                ":cutoff": cutoff,
                ":limit": limit,
            },
            |row| {
                let last_visit_time: i64 = row.get(4)?;
                Ok(BrowserHistoryEntry {
                    id: row.get(0)?,
                    url: row.get(1)?,
                    title: row.get::<_, Option<String>>(2)?.unwrap_or_default(),
                    visit_count: row.get::<_, Option<i64>>(3)?.unwrap_or(0).max(0) as u32,
                    last_visit: chromium_time_to_unix(last_visit_time),
                })
            },
        )
        .map_err(|error| format!("could not query history: {error}"))?;

    let mut entries = Vec::new();
    for row in rows {
        entries.push(row.map_err(|error| format!("could not read history row: {error}"))?);
    }
    Ok(entries)
}

#[cfg(test)]
mod tests {
    use super::*;
    use rusqlite::Connection;

    /// The `urls` table as Chromium writes it (the columns this reader uses).
    fn fixture_database(path: &Path) {
        let connection = Connection::open(path).unwrap();
        connection
            .execute_batch(
                "CREATE TABLE urls (
                    id INTEGER PRIMARY KEY,
                    url LONGVARCHAR,
                    title LONGVARCHAR,
                    visit_count INTEGER DEFAULT 0 NOT NULL,
                    typed_count INTEGER DEFAULT 0 NOT NULL,
                    last_visit_time INTEGER NOT NULL,
                    hidden INTEGER DEFAULT 0 NOT NULL
                );",
            )
            .unwrap();
        // Microseconds since 1601. 13317004800000000 == 2023-01-01T00:00:00Z.
        let base = 13_317_004_800_000_000_i64;
        let rows = [
            (1, "https://www.rust-lang.org/", "Rust Programming Language", 12, base),
            (2, "https://doc.rust-lang.org/book/", "The Rust Book", 3, base - 3_600_000_000),
            (3, "https://example.com/", "Example Domain", 1, base - 86_400_000_000),
        ];
        for (id, url, title, visits, last_visit) in rows {
            connection
                .execute(
                    "INSERT INTO urls (id, url, title, visit_count, last_visit_time) \
                     VALUES (?1, ?2, ?3, ?4, ?5)",
                    rusqlite::params![id, url, title, visits, last_visit],
                )
                .unwrap();
        }
    }

    #[test]
    fn empty_query_returns_the_most_recent_visits() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        let entries = query_history_database(&database, "", 10, 0).unwrap();
        assert_eq!(entries.len(), 3);
        assert_eq!(entries[0].url, "https://www.rust-lang.org/");
        assert_eq!(entries[0].visit_count, 12);
        assert_eq!(entries[0].last_visit, 1_672_531_200);
        assert!(entries[0].last_visit > entries[2].last_visit);
    }

    #[test]
    fn query_filters_on_title_and_url() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);

        let by_title = query_history_database(&database, "book", 10, 0).unwrap();
        assert_eq!(by_title.len(), 1);
        assert_eq!(by_title[0].title, "The Rust Book");

        let by_url = query_history_database(&database, "example.com", 10, 0).unwrap();
        assert_eq!(by_url.len(), 1);
        assert_eq!(by_url[0].id, 3);
    }

    #[test]
    fn like_wildcards_in_the_query_are_escaped() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        // `%` would otherwise match every row.
        assert!(query_history_database(&database, "%", 10, 0).unwrap().is_empty());
        assert!(query_history_database(&database, "_", 10, 0).unwrap().is_empty());
    }

    #[test]
    fn the_limit_is_honoured() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        assert_eq!(query_history_database(&database, "", 2, 0).unwrap().len(), 2);
    }

    #[test]
    fn the_recency_filter_uses_the_chromium_epoch() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        // The fixture rows are from 2023; a one-day window from *now* must drop
        // all of them (this also proves the cutoff is not compared in Unix
        // seconds against a 1601-epoch column).
        let recent = query_history_database(&database, "", 10, 1).unwrap();
        assert!(recent.is_empty());
    }

    #[test]
    fn a_missing_file_is_a_soft_failure() {
        let temp = tempfile::tempdir().unwrap();
        let error = query_history_file(&temp.path().join("History"), "", 10, 30).unwrap_err();
        assert!(error.contains("no history file"));
    }

    #[test]
    fn a_corrupt_database_is_a_soft_failure() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        std::fs::write(&database, b"not a database at all").unwrap();
        assert!(query_history_database(&database, "", 10, 0).is_err());
    }

    #[test]
    fn query_history_file_copies_into_a_temp_directory() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        // A `-wal` sidecar is copied alongside the main file.
        std::fs::write(temp.path().join("History-wal"), b"").unwrap();
        let entries = query_history_file(&database, "rust", 10, 0).unwrap();
        assert_eq!(entries.len(), 2);
        // The source file is untouched.
        assert!(database.is_file());
    }
}
