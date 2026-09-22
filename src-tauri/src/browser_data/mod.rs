//! Built-in browser plugin: bookmarks and history search.
//!
//! Chromium-family browsers (Chrome, Edge, Brave, Chromium) store their
//! bookmarks and history in the same place and the same format on every
//! platform, so one reader serves all of them. This module owns:
//!
//! * **discovery** — which browsers and profiles exist ([`discover`]);
//! * **bookmarks** — the JSON tree, flattened ([`bookmarks`]);
//! * **history** — the locked SQLite database, copied and queried ([`history`]).
//!
//! Everything here is soft-failing by design. A missing browser, a corrupt
//! bookmarks file, a history database the browser is holding open: none of them
//! may panic or take the launcher down. Each reader returns either data or an
//! error *string* the caller can surface; the launcher treats the error as an
//! empty result.
//!
//! This is the read half of the plugin. The launcher's inline result mode and
//! the plugin's own settings page consume these commands; the live tabs of a
//! running browser are the third surface ([`tabs`]), read through AppleScript
//! on macOS and the DevTools Protocol elsewhere.

pub mod bookmarks;
pub mod discover;
pub mod history;
pub mod tabs;

use serde::Serialize;
use std::path::PathBuf;

use discover::BrowserProfileInfo;

/// Seconds between the Unix epoch (1970-01-01) and Chromium's (1601-01-01).
/// Chromium stores timestamps as microseconds since 1601 in both `Bookmarks`
/// (`date_added`) and `History` (`last_visit_time`).
pub const CHROMIUM_EPOCH_OFFSET_SECONDS: i64 = 11_644_473_600;

/// Convert a Chromium timestamp (microseconds since 1601) to Unix seconds.
pub fn chromium_time_to_unix(micros: i64) -> i64 {
    micros / 1_000_000 - CHROMIUM_EPOCH_OFFSET_SECONDS
}

/// "Now" in Chromium's microsecond-since-1601 form, for the history recency
/// filter. The comparison stays in the column's own units so no row has to be
/// converted to filter it.
pub fn chromium_now_micros() -> i64 {
    let unix = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|duration| duration.as_secs() as i64)
        .unwrap_or(0);
    (unix + CHROMIUM_EPOCH_OFFSET_SECONDS) * 1_000_000
}

/// One search result — a bookmark or a history entry.
///
/// The two kinds share a shape so the frontend can render them with one row
/// component: `folder_path`/`date_added` are bookmark-only, and
/// `visit_count`/`last_visit` are history-only. Absent fields are omitted from
/// the JSON rather than sent as `null`, so a bookmark row never carries a
/// meaningless `visit_count: 0`.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct BrowserItem {
    pub id: String,
    pub title: String,
    pub url: String,
    /// The `profile_key` this row came from, so "open" does not have to
    /// re-resolve the target.
    pub profile_key: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub folder_path: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub date_added: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub visit_count: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_visit: Option<i64>,
}

impl BrowserItem {
    fn from_bookmark(profile_key: &str, bookmark: bookmarks::BrowserBookmark) -> Self {
        Self {
            id: format!("bookmark:{}", bookmark.id),
            title: if bookmark.title.trim().is_empty() {
                bookmark.url.clone()
            } else {
                bookmark.title
            },
            url: bookmark.url,
            profile_key: profile_key.to_string(),
            folder_path: Some(bookmark.folder_path),
            date_added: bookmark.date_added,
            visit_count: None,
            last_visit: None,
        }
    }

    fn from_history(profile_key: &str, entry: history::BrowserHistoryEntry) -> Self {
        Self {
            id: format!("history:{}", entry.id),
            title: if entry.title.trim().is_empty() {
                entry.url.clone()
            } else {
                entry.title
            },
            url: entry.url,
            profile_key: profile_key.to_string(),
            folder_path: None,
            date_added: None,
            visit_count: Some(entry.visit_count),
            last_visit: Some(entry.last_visit),
        }
    }
}

/// Default page size for a search. The launcher shows at most eight rows; a
/// settings-page list can ask for more, but nothing needs an unbounded scan.
const DEFAULT_LIMIT: usize = 50;
const MAX_LIMIT: usize = 500;

/// Where a profile's `Bookmarks` and `History` files live.
struct ProfileFiles {
    bookmarks: PathBuf,
    history: PathBuf,
}

/// Resolve a `profile_key` to its files.
///
/// The base directory comes from discovery rather than from a fresh lookup,
/// because a custom base dir may point either at a browser's base directory or
/// at a single profile inside it — discovery already resolved which shape it is
/// and recorded the base dir it found the profile under. An unknown browser id,
/// a malformed key or a profile that no longer exists is an error string, not a
/// panic.
fn profile_files(profile_key: &str) -> Result<ProfileFiles, String> {
    let settings = crate::commands::config::load_settings();
    let custom = settings
        .browser_plugin
        .custom_base_dir
        .as_deref()
        .map(PathBuf::from);
    profile_files_with(profile_key, custom.as_deref())
}

/// [`profile_files`] with the custom base dir supplied by the caller, so a test
/// can point at a fixture tree instead of the machine's real config.
fn profile_files_with(
    profile_key: &str,
    custom_base_dir: Option<&std::path::Path>,
) -> Result<ProfileFiles, String> {
    let (browser_id, profile_dir_name) = discover::parse_profile_key(profile_key)
        .ok_or_else(|| format!("invalid profile key: {profile_key}"))?;
    let profiles = discover::discover_profiles_cached(custom_base_dir);
    let profile = profiles
        .iter()
        .find(|profile| {
            profile.browser_id == browser_id && profile.profile_dir_name == profile_dir_name
        })
        .ok_or_else(|| format!("unknown browser profile: {profile_key}"))?;
    let profile_dir = PathBuf::from(&profile.base_dir).join(&profile.profile_dir_name);
    Ok(ProfileFiles {
        bookmarks: profile_dir.join("Bookmarks"),
        history: profile_dir.join("History"),
    })
}

fn normalize_limit(limit: Option<usize>) -> usize {
    limit.unwrap_or(DEFAULT_LIMIT).clamp(1, MAX_LIMIT)
}

/// Every browser profile the plugin can read, `Default` profile first.
///
/// Cached until a base directory or a `Local State` file changes, so calling
/// this from the launcher on every summon is a hash and a clone, not a scan.
#[tauri::command]
pub fn browser_discover() -> Vec<BrowserProfileInfo> {
    let settings = crate::commands::config::load_settings();
    let custom = settings
        .browser_plugin
        .custom_base_dir
        .as_deref()
        .map(PathBuf::from);
    discover::discover_profiles_cached(custom.as_deref())
}

/// The profile the launcher searches when the user has not picked one.
///
/// `browser_plugin.target` names a browser id (`"chrome"`, …) or `"auto"`. In
/// auto mode the first profile that actually has data wins — a browser that is
/// installed but never opened has an empty `Default` and would otherwise shadow
/// the one the user really browses with.
#[tauri::command]
pub fn browser_default_profile() -> Option<String> {
    let settings = crate::commands::config::load_settings();
    let target = settings.browser_plugin.target.as_str();
    let custom = settings
        .browser_plugin
        .custom_base_dir
        .as_deref()
        .map(PathBuf::from);
    let profiles = discover::discover_profiles_cached(custom.as_deref());

    if !target.is_empty() && target != "auto" {
        if let Some(profile) = profiles.iter().find(|profile| profile.browser_id == target) {
            return Some(profile.profile_key.clone());
        }
    }
    profiles
        .iter()
        .find(|profile| profile.has_history)
        .or_else(|| profiles.iter().find(|profile| profile.has_bookmarks))
        .or_else(|| profiles.first())
        .map(|profile| profile.profile_key.clone())
}

/// R27 · order bookmark/history results for one of the plugin's four sort
/// orders.
///
/// `relevance` is deliberately a no-op: the order the caller already produced
/// *is* the launcher's ranking (the bookmarks bar's own order for bookmarks,
/// newest-first for history), and re-deriving a match score here would be a
/// second, silently different ranking. The other three are explicit orderings
/// a bookmark tool is expected to offer; `visits` falls back to recency where a
/// row has no visit count (every bookmark does not).
pub fn sort_browser_items(items: &mut [BrowserItem], order: &str) {
    match order {
        "alphabetical" => items.sort_by(|a, b| {
            a.title
                .to_lowercase()
                .cmp(&b.title.to_lowercase())
                .then_with(|| a.url.cmp(&b.url))
        }),
        "recent" => items.sort_by(|a, b| {
            let stamp = |item: &BrowserItem| item.last_visit.or(item.date_added).unwrap_or(0);
            stamp(b).cmp(&stamp(a))
        }),
        "visits" => items.sort_by(|a, b| {
            b.visit_count
                .unwrap_or(0)
                .cmp(&a.visit_count.unwrap_or(0))
                .then_with(|| b.last_visit.unwrap_or(0).cmp(&a.last_visit.unwrap_or(0)))
        }),
        _ => {}
    }
}

/// The order to apply and how many rows to fetch before ordering.
///
/// A non-relevance order has to see more than the caller's limit, or it would
/// only reorder the newest N rows and quietly mis-sort the rest; `MAX_LIMIT` is
/// the same bound every other query uses.
fn sort_order_and_fetch_limit(limit: Option<usize>, order: Option<&str>) -> (String, usize) {
    let order = match order {
        Some(value) => crate::commands::config::normalize_browser_sort_order(value),
        None => crate::commands::config::load_settings().browser_plugin.sort_order,
    };
    let fetch = if order == crate::commands::config::DEFAULT_BROWSER_SORT_ORDER {
        normalize_limit(limit)
    } else {
        MAX_LIMIT
    };
    (order, fetch)
}

/// Search one profile's bookmarks by title or URL.
///
/// An empty query returns the bookmarks in file order (the bar first, since
/// Chromium writes `bookmark_bar` first), which is the launcher's default view.
/// R27 · `sort_order` overrides the stored setting for one call; omitted, the
/// user's `browser_plugin.sort_order` applies.
#[tauri::command]
pub fn browser_search_bookmarks(
    profile_key: String,
    query: String,
    limit: Option<usize>,
    sort_order: Option<String>,
) -> Result<Vec<BrowserItem>, String> {
    let files = profile_files(&profile_key)?;
    let needle = query.trim().to_lowercase();
    let parsed = bookmarks::parse_bookmarks_file(&files.bookmarks)?;
    let (order, _fetch) = sort_order_and_fetch_limit(limit, sort_order.as_deref());
    let mut items: Vec<BrowserItem> = parsed
        .into_iter()
        .filter(|bookmark| bookmarks::matches_query(bookmark, &needle))
        .map(|bookmark| BrowserItem::from_bookmark(&profile_key, bookmark))
        .collect();
    sort_browser_items(&mut items, &order);
    items.truncate(normalize_limit(limit));
    Ok(items)
}

/// Search one profile's history by title or URL, newest first by default.
///
/// `days` limits the window (0 disables it). When omitted, the user's
/// `browser_plugin.history_days` setting applies. R27 · `sort_order` overrides
/// the stored ordering for one call; omitted, `browser_plugin.sort_order`
/// applies.
#[tauri::command]
pub fn browser_search_history(
    profile_key: String,
    query: String,
    limit: Option<usize>,
    days: Option<u32>,
    sort_order: Option<String>,
) -> Result<Vec<BrowserItem>, String> {
    let files = profile_files(&profile_key)?;
    let days = days.unwrap_or_else(|| crate::commands::config::load_settings().browser_plugin.history_days);
    let (order, fetch) = sort_order_and_fetch_limit(limit, sort_order.as_deref());
    let entries = history::query_history_file(&files.history, &query, fetch, days)?;
    let mut items: Vec<BrowserItem> = entries
        .into_iter()
        .map(|entry| BrowserItem::from_history(&profile_key, entry))
        .collect();
    sort_browser_items(&mut items, &order);
    items.truncate(normalize_limit(limit));
    Ok(items)
}

/// Open a URL in the browser the row came from.
///
/// `profile_key` of `"default"` (or an unknown/empty one) opens the system's
/// default browser. On macOS the browser id maps to its `.app` name and the
/// URL is handed to `open -a`; if that app is not installed, `open` falls back
/// to the default handler. Windows hands the URL to `cmd /c start` — the
/// shell's own association lookup — and Linux to `xdg-open`. Only the URL ever
/// reaches the shell as an argument; it is never interpolated into a command
/// string.
#[tauri::command]
pub fn browser_open_url(profile_key: String, url: String) -> Result<(), String> {
    let url = url.trim();
    if url.is_empty() {
        return Err("empty url".to_string());
    }
    let browser_id = discover::parse_profile_key(&profile_key)
        .map(|(browser_id, _)| browser_id)
        .unwrap_or_default();
    open_url_with(&browser_id, url)
}

#[cfg(target_os = "macos")]
fn open_url_with(browser_id: &str, url: &str) -> Result<(), String> {
    if let Some(app) = macos_app_name(browser_id) {
        if let Ok(status) = std::process::Command::new("open")
            .arg("-a")
            .arg(app)
            .arg(url)
            .status()
        {
            if status.success() {
                return Ok(());
            }
        }
    }
    std::process::Command::new("open")
        .arg(url)
        .status()
        .map(|_| ())
        .map_err(|error| format!("could not open url: {error}"))
}

/// Map a browser id to its macOS application bundle name.
///
/// The Edge channels are separate app bundles, not one bundle with a flag, so
/// each id names its own bundle — the same suffix the data directory uses.
#[cfg(target_os = "macos")]
fn macos_app_name(browser_id: &str) -> Option<&'static str> {
    match browser_id {
        "chrome" => Some("Google Chrome"),
        "edge" => Some("Microsoft Edge"),
        "edge-beta" => Some("Microsoft Edge Beta"),
        "edge-dev" => Some("Microsoft Edge Dev"),
        "edge-canary" => Some("Microsoft Edge Canary"),
        "brave" => Some("Brave Browser"),
        "chromium" => Some("Chromium"),
        _ => None,
    }
}

#[cfg(target_os = "windows")]
fn open_url_with(_browser_id: &str, url: &str) -> Result<(), String> {
    // `start` is a `cmd` builtin, not an executable; the empty argument is the
    // window-title slot, which `start` would otherwise take the URL for.
    std::process::Command::new("cmd")
        .args(["/c", "start", "", url])
        .status()
        .map(|_| ())
        .map_err(|error| format!("could not open url: {error}"))
}

#[cfg(target_os = "linux")]
fn open_url_with(_browser_id: &str, url: &str) -> Result<(), String> {
    std::process::Command::new("xdg-open")
        .arg(url)
        .status()
        .map(|_| ())
        .map_err(|error| format!("could not open url: {error}"))
}

#[cfg(not(any(target_os = "macos", target_os = "windows", target_os = "linux")))]
fn open_url_with(_browser_id: &str, url: &str) -> Result<(), String> {
    Err(format!("opening {url} is not supported on this platform"))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn write(path: std::path::PathBuf, contents: &str) {
        std::fs::create_dir_all(path.parent().unwrap()).unwrap();
        std::fs::write(path, contents).unwrap();
    }

    #[test]
    fn a_custom_profile_key_resolves_to_its_files() {
        let temp = tempfile::tempdir().unwrap();
        let base = temp.path().join("Weird Browser");
        write(base.join("Default").join("Preferences"), "{}");
        write(base.join("Default").join("Bookmarks"), "{}");

        // The custom dir is the browser's base directory.
        let files = profile_files_with("custom/Default", Some(&base)).unwrap();
        assert_eq!(files.bookmarks, base.join("Default").join("Bookmarks"));
        assert_eq!(files.history, base.join("Default").join("History"));

        // The custom dir is itself the profile: the base dir must be its
        // parent, or the resolved path would repeat the profile folder.
        let profile = base.join("Profile 7");
        write(profile.join("Preferences"), "{}");
        let files = profile_files_with("custom/Profile 7", Some(&profile)).unwrap();
        assert_eq!(files.bookmarks, profile.join("Bookmarks"));
    }

    #[test]
    fn chromium_timestamps_convert_to_unix_seconds() {
        // 1601-01-01T00:00:00Z is the epoch itself.
        assert_eq!(chromium_time_to_unix(0), -CHROMIUM_EPOCH_OFFSET_SECONDS);
        // 2023-01-01T00:00:00Z.
        assert_eq!(chromium_time_to_unix(13_317_004_800_000_000), 1_672_531_200);
    }

    #[test]
    fn now_in_chromium_micros_is_a_plausible_wall_clock() {
        let now = chromium_now_micros();
        // Between 2020 and 2100, in the 1601 epoch — a wrong offset would be
        // off by centuries and fail this immediately.
        let unix = chromium_time_to_unix(now);
        assert!(unix > 1_577_836_800, "unix={unix}");
        assert!(unix < 4_102_444_800, "unix={unix}");
    }

    #[test]
    fn a_bookmark_becomes_an_item_with_a_folder_and_no_visit_count() {
        let bookmark = bookmarks::BrowserBookmark {
            id: "guid-1".into(),
            title: "Rust".into(),
            url: "https://www.rust-lang.org/".into(),
            folder_path: "Bookmarks bar".into(),
            date_added: Some(1_672_531_200),
        };
        let item = BrowserItem::from_bookmark("chrome/Default", bookmark);
        assert_eq!(item.id, "bookmark:guid-1");
        assert_eq!(item.title, "Rust");
        assert_eq!(item.profile_key, "chrome/Default");
        assert_eq!(item.folder_path.as_deref(), Some("Bookmarks bar"));
        assert_eq!(item.date_added, Some(1_672_531_200));
        assert_eq!(item.visit_count, None);
        assert_eq!(item.last_visit, None);
    }

    #[test]
    fn a_history_row_becomes_an_item_with_a_visit_count() {
        let entry = history::BrowserHistoryEntry {
            id: 7,
            title: "Example".into(),
            url: "https://example.com/".into(),
            visit_count: 3,
            last_visit: 1_672_531_200,
        };
        let item = BrowserItem::from_history("edge/Profile 1", entry);
        assert_eq!(item.id, "history:7");
        assert_eq!(item.profile_key, "edge/Profile 1");
        assert_eq!(item.visit_count, Some(3));
        assert_eq!(item.last_visit, Some(1_672_531_200));
        assert_eq!(item.folder_path, None);
        assert_eq!(item.date_added, None);
    }

    #[test]
    fn an_untitled_row_falls_back_to_its_url() {
        let bookmark = bookmarks::BrowserBookmark {
            id: "x".into(),
            title: "   ".into(),
            url: "https://example.com/".into(),
            folder_path: String::new(),
            date_added: None,
        };
        assert_eq!(
            BrowserItem::from_bookmark("chrome/Default", bookmark).title,
            "https://example.com/"
        );
    }

    #[test]
    fn the_limit_is_clamped_to_a_sane_range() {
        assert_eq!(normalize_limit(None), DEFAULT_LIMIT);
        assert_eq!(normalize_limit(Some(0)), 1);
        assert_eq!(normalize_limit(Some(10)), 10);
        assert_eq!(normalize_limit(Some(usize::MAX)), MAX_LIMIT);
    }

    #[test]
    fn an_unknown_profile_key_is_an_error_string_not_a_panic() {
        assert!(profile_files("chrome").is_err());
        assert!(profile_files("nope/Default").is_err());
    }

    #[test]
    fn opening_an_empty_url_is_rejected() {
        assert!(browser_open_url("default".into(), "   ".into()).is_err());
    }

    #[test]
    fn a_default_profile_key_never_reaches_the_browser_mapping() {
        // `default` has no browser id in the table, so the opener falls back to
        // the system handler rather than erroring.
        assert!(discover::parse_profile_key("default").is_none());
    }

    /// R27 · the four sort orders the plugin's settings card offers. Each one is
    /// pinned against the *fact* it orders by, not against an incidental list
    /// order — a sort that happens to look right on one fixture is the kind of
    /// bug this round exists to avoid.
    #[test]
    fn the_result_orders_sort_by_the_fact_they_name() {
        let item = |title: &str, url: &str, added: Option<i64>, visits: Option<u32>, last: Option<i64>| BrowserItem {
            id: url.to_string(),
            title: title.to_string(),
            url: url.to_string(),
            profile_key: "chrome/Default".to_string(),
            folder_path: None,
            date_added: added,
            visit_count: visits,
            last_visit: last,
        };
        let fixture = || {
            vec![
                item("Zeta", "https://z.example", Some(300), Some(2), Some(100)),
                item("alpha", "https://a.example", Some(100), Some(9), Some(900)),
                item("Beta", "https://b.example", Some(200), Some(5), Some(500)),
            ]
        };

        // `relevance` is a no-op: the caller's order *is* the ranking.
        let mut items = fixture();
        sort_browser_items(&mut items, "relevance");
        assert_eq!(items[0].title, "Zeta", "relevance leaves the input order alone");

        // Alphabetical is case-insensitive, so `alpha` leads.
        let mut items = fixture();
        sort_browser_items(&mut items, "alphabetical");
        assert_eq!(items[0].title, "alpha");
        assert_eq!(items[2].title, "Zeta");

        // Most recent: the greatest timestamp (last visit, else date added).
        let mut items = fixture();
        sort_browser_items(&mut items, "recent");
        assert_eq!(items[0].title, "alpha");
        assert_eq!(items[2].title, "Zeta");

        // Most visited: the greatest visit count. A bookmark with no count
        // sorts last rather than being dropped.
        let mut items = fixture();
        items.push(item("Bookmark", "https://c.example", Some(400), None, None));
        sort_browser_items(&mut items, "visits");
        assert_eq!(items[0].title, "alpha");
        assert_eq!(items[3].title, "Bookmark");

        // An unknown order is a no-op, exactly like relevance.
        let mut items = fixture();
        sort_browser_items(&mut items, "sideways");
        assert_eq!(items[0].title, "Zeta");
    }

    /// R27 · a non-relevance order must see more than the caller's limit, or it
    /// would only reorder the newest N and quietly mis-sort the rest.
    #[test]
    fn a_non_relevance_order_fetches_beyond_the_callers_limit() {
        let (order, fetch) = sort_order_and_fetch_limit(Some(24), Some("alphabetical"));
        assert_eq!(order, "alphabetical");
        assert_eq!(fetch, MAX_LIMIT);
        // …while the default order keeps the caller's own limit.
        let (order, fetch) = sort_order_and_fetch_limit(Some(24), Some("relevance"));
        assert_eq!(order, "relevance");
        assert_eq!(fetch, 24);
        // An unknown order normalizes to relevance, and therefore to the
        // caller's limit — it is not a hidden wide query.
        let (order, fetch) = sort_order_and_fetch_limit(Some(24), Some("sideways"));
        assert_eq!(order, crate::commands::config::DEFAULT_BROWSER_SORT_ORDER);
        assert_eq!(fetch, 24);
    }
}
