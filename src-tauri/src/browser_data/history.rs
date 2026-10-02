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
/// the last `days` days are returned. No `tokens` returns the most recently
/// visited URLs, which is the launcher's empty-state view. `tokens` are the
/// launcher's AND rule (see [`query_history_database`]) and `search_field`
/// narrows where each one may land.
pub fn query_history_file(
    source: &Path,
    tokens: &[String],
    search_field: &str,
    limit: usize,
    days: u32,
) -> Result<Vec<BrowserHistoryEntry>, String> {
    if !source.is_file() {
        return Err(format!("no history file at {source:?}"));
    }
    // R72 · the copy is cached per source, keyed on the source's own signature —
    // its length and mtime, plus the `-wal`\'s, since that is where the browser
    // writes commits between checkpoints. The user\'s report: 「书签插件是不是很吃
    // 内存，打开后突出出现明显的短暂卡顿」. It is not memory, it is this copy: a
    // History database is tens to hundreds of megabytes (34 MB on the machine the
    // report came from) and every fetch — opening the plugin, every chip switch —
    // copied it into a temp directory before it could run one query. One copy per
    // *signature* is enough: the browser holds the write lock, so the bytes only
    // change when it checkpoints or writes, and either moves the mtime.
    let signature = history_signature(source);
    if let Ok(guard) = HISTORY_COPY.lock() {
        if let Some(entry) = guard.as_ref() {
            if entry.source == source && entry.signature == signature {
                let copy = entry.copy.clone();
                drop(guard);
                return query_history_database(&copy, tokens, search_field, limit, days);
            }
        }
    }
    let temp = tempfile::tempdir().map_err(|error| format!("no temp dir: {error}"))?;
    let copy = temp.path().join("History");
    copy_database(source, &copy)?;
    // The `TempDir` rides in the cache so the copy it made stays on disk; a
    // later signature replaces it and the old directory deletes itself.
    if let Ok(mut cache) = HISTORY_COPY.lock() {
        *cache = Some(HistoryCopy {
            source: source.to_path_buf(),
            signature,
            copy: copy.clone(),
            _temp: temp,
        });
    }
    query_history_database(&copy, tokens, search_field, limit, days)
}

/// One source\'s `(length, mtime)` pair — the main database and its `-wal`.
type HistorySignature = (
    (u64, Option<std::time::SystemTime>),
    (u64, Option<std::time::SystemTime>),
);

fn history_signature(source: &Path) -> HistorySignature {
    let stat = |path: PathBuf| {
        std::fs::metadata(path)
            .map(|meta| (meta.len(), meta.modified().ok()))
            .unwrap_or((0, None))
    };
    let wal = PathBuf::from(format!("{}-wal", source.display()));
    (stat(source.to_path_buf()), stat(wal))
}

/// The one cached copy. A single slot: the launcher reads one profile\'s history
/// at a time, so an LRU would only ever hold one live entry anyway.
static HISTORY_COPY: std::sync::Mutex<Option<HistoryCopy>> = std::sync::Mutex::new(None);

struct HistoryCopy {
    source: PathBuf,
    signature: HistorySignature,
    copy: PathBuf,
    /// Held for the cache\'s lifetime: dropping a `TempDir` deletes the copy.
    _temp: tempfile::TempDir,
}

/// Query an already-copied database. Kept separate from
/// [`query_history_file`] so the SQL can be tested against a fixture database
/// without a browser's lock in the way.
///
/// `tokens` is the launcher's own AND rule (`plugins/search.ts`), pushed down
/// into SQL: every token must be found, a token is a case-insensitive substring,
/// and `search_field` (`all` / `title` / `url`) narrows which column it may hit.
/// R32 kept that rule in the launcher's memory and fetched a fixed 500 rows; a
/// match outside that window was unreachable. See the WHERE note below.
pub fn query_history_database(
    database: &Path,
    tokens: &[String],
    search_field: &str,
    limit: usize,
    days: u32,
) -> Result<Vec<BrowserHistoryEntry>, String> {
    let connection = Connection::open_with_flags(database, OpenFlags::SQLITE_OPEN_READ_ONLY)
        .map_err(|error| format!("could not open history: {error}"))?;

    // R75 · the WHERE is built per token. Through R32 this was a fixed statement
    // with one `:needle` substring — deliberately, so the empty-query and the
    // filtered cases shared one code path — while the launcher applied the real
    // AND rule in memory over the 500 rows `LIMIT` had already chosen. That is
    // the bug this round fixes: `LIMIT` runs *after* the filters, so with the
    // filter pushed down a token the user typed can reach a row older than the
    // newest 500. The fixed statement is gone because avoiding a dynamic WHERE
    // was its whole point, and the dynamic WHERE is now the point.
    //
    // Only the clause *shape* is built from the field and the token count —
    // never from the token text. Every token is a bound parameter, so a token
    // containing `'`, `%`, `_` or a SQL fragment is a literal pattern, not code.
    let field = match search_field {
        "title" => "title",
        "url" => "url",
        _ => "all",
    };
    let mut clauses: Vec<&str> = Vec::new();
    let mut values: Vec<Box<dyn rusqlite::ToSql>> = Vec::new();
    for token in tokens {
        let token = token.trim().to_lowercase();
        if token.is_empty() {
            continue;
        }
        let pattern = format!("%{}%", escape_like(&token));
        match field {
            "title" => clauses.push("title LIKE ? ESCAPE '\\'"),
            "url" => clauses.push("url LIKE ? ESCAPE '\\'"),
            _ => clauses.push("(url LIKE ? ESCAPE '\\' OR title LIKE ? ESCAPE '\\')"),
        }
        values.push(Box::new(pattern.clone()));
        // `all` tests the one pattern against both columns, so it binds twice.
        if field == "all" {
            values.push(Box::new(pattern));
        }
    }
    // No tokens is no filter; `1` keeps the days/cutoff/limit parameters in the
    // same positions whether or not the needle contributed a clause.
    let token_clause = if clauses.is_empty() {
        "1".to_string()
    } else {
        clauses.join(" AND ")
    };
    let sql = format!(
        "SELECT id, url, title, visit_count, last_visit_time FROM urls \
         WHERE ({token_clause}) \
           AND (? = 0 OR last_visit_time >= ?) \
         ORDER BY last_visit_time DESC LIMIT ?"
    );

    let cutoff = super::chromium_now_micros() - i64::from(days) * 86_400 * 1_000_000;
    values.push(Box::new(i64::from(days)));
    values.push(Box::new(cutoff));
    values.push(Box::new(limit.min(i64::MAX as usize) as i64));

    let mut statement = connection
        .prepare(&sql)
        .map_err(|error| format!("could not read history: {error}"))?;

    let rows = statement
        .query_map(rusqlite::params_from_iter(values), |row| {
            let last_visit_time: i64 = row.get(4)?;
            Ok(BrowserHistoryEntry {
                id: row.get(0)?,
                url: row.get(1)?,
                title: row.get::<_, Option<String>>(2)?.unwrap_or_default(),
                visit_count: row.get::<_, Option<i64>>(3)?.unwrap_or(0).max(0) as u32,
                last_visit: chromium_time_to_unix(last_visit_time),
            })
        })
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
            (
                1,
                "https://www.rust-lang.org/",
                "Rust Programming Language",
                12,
                base,
            ),
            (
                2,
                "https://doc.rust-lang.org/book/",
                "The Rust Book",
                3,
                base - 3_600_000_000,
            ),
            (
                3,
                "https://example.com/",
                "Example Domain",
                1,
                base - 86_400_000_000,
            ),
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
        let entries = query_history_database(&database, &[], "all", 10, 0).unwrap();
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

        let by_title =
            query_history_database(&database, &["book".to_string()], "all", 10, 0).unwrap();
        assert_eq!(by_title.len(), 1);
        assert_eq!(by_title[0].title, "The Rust Book");

        let by_url =
            query_history_database(&database, &["example.com".to_string()], "all", 10, 0).unwrap();
        assert_eq!(by_url.len(), 1);
        assert_eq!(by_url[0].id, 3);
    }

    /// R75 · the launcher's AND rule, in SQL: every token must be found, a token
    /// is a substring, and a token may land in either column under `all`.
    #[test]
    fn every_token_must_match_and_each_is_a_substring() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);

        // Both tokens land in row 2's title.
        let both = query_history_database(
            &database,
            &["rust".to_string(), "book".to_string()],
            "all",
            10,
            0,
        )
        .unwrap();
        assert_eq!(both.len(), 1);
        assert_eq!(both[0].id, 2);

        // One token in the title, the other in the URL: still an AND across the
        // row's fields, exactly like `matchesTokens`.
        let split = query_history_database(
            &database,
            &["example".to_string(), "domain".to_string()],
            "all",
            10,
            0,
        )
        .unwrap();
        assert_eq!(split.len(), 1);
        assert_eq!(split[0].id, 3);

        // A token no row carries drops the whole result, not just its clause.
        assert!(query_history_database(
            &database,
            &["rust".to_string(), "python".to_string()],
            "all",
            10,
            0,
        )
        .unwrap()
        .is_empty());

        // Whitespace-only and empty tokens are no filter, not an AND with "".
        assert_eq!(
            query_history_database(&database, &["  ".to_string(), "".to_string()], "all", 10, 0)
                .unwrap()
                .len(),
            3,
        );
    }

    /// R75 · `search_field` chooses which column a token may hit: `url` and
    /// `title` each see one column, `all` sees both.
    #[test]
    fn the_search_field_narrows_where_a_token_may_land() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);

        // `example.com` is in row 3's URL, and its title ("Example Domain")
        // does not carry the dotted host.
        assert_eq!(
            query_history_database(&database, &["example.com".to_string()], "url", 10, 0)
                .unwrap()
                .len(),
            1,
        );
        assert!(
            query_history_database(&database, &["example.com".to_string()], "title", 10, 0)
                .unwrap()
                .is_empty()
        );
        assert_eq!(
            query_history_database(&database, &["example.com".to_string()], "all", 10, 0)
                .unwrap()
                .len(),
            1,
        );

        // `domain` is in row 3's title only.
        assert_eq!(
            query_history_database(&database, &["domain".to_string()], "title", 10, 0)
                .unwrap()
                .len(),
            1,
        );
        assert!(
            query_history_database(&database, &["domain".to_string()], "url", 10, 0)
                .unwrap()
                .is_empty()
        );
    }

    /// R75 · a token is data, never SQL. A quote-and-DROP attempt matches
    /// nothing and leaves the table intact — the proof that the pattern went in
    /// as a bound parameter.
    #[test]
    fn a_sql_fragment_in_a_token_is_a_literal_pattern() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        let attack = "x'; DROP TABLE urls; --".to_string();
        assert!(query_history_database(&database, &[attack], "all", 10, 0)
            .unwrap()
            .is_empty());
        // The table survived, so the next query still reads all three rows.
        assert_eq!(
            query_history_database(&database, &[], "all", 10, 0)
                .unwrap()
                .len(),
            3,
        );
    }

    /// R75 · the row this round exists for: a match older than the newest
    /// `LIMIT` rows. The filter runs before `LIMIT`, so the 501st-oldest row is
    /// reachable; the old fixed-query shape returned the newest 500 and then
    /// filtered them in memory, which could never see it.
    #[test]
    fn a_match_older_than_the_fetch_limit_is_reachable() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        let connection = Connection::open(&database).unwrap();
        connection
            .execute_batch(
                "CREATE TABLE urls (
                    id INTEGER PRIMARY KEY,
                    url LONGVARCHAR,
                    title LONGVARCHAR,
                    visit_count INTEGER DEFAULT 0 NOT NULL,
                    last_visit_time INTEGER NOT NULL
                );",
            )
            .unwrap();
        // 510 rows, oldest first: row 9 is the 10th oldest, i.e. outside the
        // newest 500 (rows 10..=509). Only it carries the needle.
        let base = 13_317_004_800_000_000_i64;
        for index in 0..510_i64 {
            let (url, title) = if index == 9 {
                (
                    "https://buried.example/".to_string(),
                    "Buried Entry".to_string(),
                )
            } else {
                (
                    format!("https://site{index}.example/{index}"),
                    format!("Row {index}"),
                )
            };
            connection
                .execute(
                    "INSERT INTO urls (id, url, title, visit_count, last_visit_time) \
                     VALUES (?1, ?2, ?3, 1, ?4)",
                    rusqlite::params![index, url, title, base + index],
                )
                .unwrap();
        }
        drop(connection);

        // The unfiltered view is still the newest `limit` rows and does not
        // contain the buried row.
        let newest = query_history_database(&database, &[], "all", 500, 0).unwrap();
        assert_eq!(newest.len(), 500);
        assert!(newest.iter().all(|entry| entry.id != 9));

        let buried =
            query_history_database(&database, &["buried".to_string()], "all", 500, 0).unwrap();
        assert_eq!(
            buried.len(),
            1,
            "a match beyond the newest 500 must be found"
        );
        assert_eq!(buried[0].id, 9);
        assert_eq!(buried[0].title, "Buried Entry");
    }

    #[test]
    fn like_wildcards_in_the_query_are_escaped() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        // `%` would otherwise match every row.
        assert!(
            query_history_database(&database, &["%".to_string()], "all", 10, 0)
                .unwrap()
                .is_empty()
        );
        assert!(
            query_history_database(&database, &["_".to_string()], "all", 10, 0)
                .unwrap()
                .is_empty()
        );
    }

    #[test]
    fn the_limit_is_honoured() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        assert_eq!(
            query_history_database(&database, &[], "all", 2, 0)
                .unwrap()
                .len(),
            2
        );
    }

    #[test]
    fn the_recency_filter_uses_the_chromium_epoch() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        // The fixture rows are from 2023; a one-day window from *now* must drop
        // all of them (this also proves the cutoff is not compared in Unix
        // seconds against a 1601-epoch column).
        let recent = query_history_database(&database, &[], "all", 10, 1).unwrap();
        assert!(recent.is_empty());
    }

    #[test]
    fn a_missing_file_is_a_soft_failure() {
        let temp = tempfile::tempdir().unwrap();
        let error =
            query_history_file(&temp.path().join("History"), &[], "all", 10, 30).unwrap_err();
        assert!(error.contains("no history file"));
    }

    #[test]
    fn a_corrupt_database_is_a_soft_failure() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        std::fs::write(&database, b"not a database at all").unwrap();
        assert!(query_history_database(&database, &[], "all", 10, 0).is_err());
    }

    #[test]
    fn query_history_file_copies_into_a_temp_directory() {
        let temp = tempfile::tempdir().unwrap();
        let database = temp.path().join("History");
        fixture_database(&database);
        // A `-wal` sidecar is copied alongside the main file.
        std::fs::write(temp.path().join("History-wal"), b"").unwrap();
        let entries = query_history_file(&database, &["rust".to_string()], "all", 10, 0).unwrap();
        assert_eq!(entries.len(), 2);
        // The source file is untouched.
        assert!(database.is_file());
    }
}
