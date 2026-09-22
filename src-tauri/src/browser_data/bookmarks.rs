//! Chromium `Bookmarks` parser.
//!
//! `Bookmarks` is a plain JSON document with no lock and no journal, so it can
//! be read straight off disk while the browser runs. The tree is
//! `roots.{bookmark_bar,other,synced}`, each a node whose `type` is either
//! `folder` (with `children`) or `url` (with `url` and `date_added`).
//!
//! `date_added` is a *string* of microseconds since 1601-01-01 (Chromium's
//! Windows-epoch convention), not a Unix timestamp — see
//! [`crate::browser_data::chromium_time_to_unix`].

use serde::Serialize;
use std::path::Path;

/// One bookmark, flattened out of the tree.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct BrowserBookmark {
    /// Chromium's `guid` when present, else a positional id — either way stable
    /// for a given file, which is all the frontend needs it for.
    pub id: String,
    pub title: String,
    pub url: String,
    /// `"Bookmarks bar / Work / Tools"`, empty for a bookmark directly under a
    /// root.
    pub folder_path: String,
    /// Unix seconds, when the file carried a parseable `date_added`.
    pub date_added: Option<i64>,
}

/// Read and flatten a `Bookmarks` file. Any failure — missing file, corrupt
/// JSON — is returned as a message; the caller turns it into an empty result so
/// a broken browser profile never breaks the launcher.
pub fn parse_bookmarks_file(path: &Path) -> Result<Vec<BrowserBookmark>, String> {
    let bytes = std::fs::read(path).map_err(|error| format!("{path:?}: {error}"))?;
    parse_bookmarks_json(&bytes)
}

/// Flatten a `Bookmarks` document.
pub fn parse_bookmarks_json(bytes: &[u8]) -> Result<Vec<BrowserBookmark>, String> {
    let value: serde_json::Value =
        serde_json::from_slice(bytes).map_err(|error| format!("invalid bookmarks JSON: {error}"))?;
    let mut out = Vec::new();
    if let Some(roots) = value.get("roots").and_then(|roots| roots.as_object()) {
        for (_root_key, root) in roots {
            // The root's own name is added by `walk_node`'s folder branch, so
            // start it from an empty path — otherwise "Bookmarks bar" would
            // appear twice in every path under it.
            walk_node(root, "", &mut out);
        }
    }
    Ok(out)
}

fn walk_node(node: &serde_json::Value, folder_path: &str, out: &mut Vec<BrowserBookmark>) {
    let kind = node.get("type").and_then(|kind| kind.as_str()).unwrap_or("");
    match kind {
        "folder" => {
            let name = node.get("name").and_then(|name| name.as_str()).unwrap_or("");
            let child_path = if folder_path.is_empty() {
                name.to_string()
            } else if name.is_empty() {
                folder_path.to_string()
            } else {
                format!("{folder_path} / {name}")
            };
            if let Some(children) = node.get("children").and_then(|children| children.as_array()) {
                for child in children {
                    walk_node(child, &child_path, out);
                }
            }
        }
        "url" => {
            let Some(url) = node.get("url").and_then(|url| url.as_str()) else {
                return;
            };
            let title = node
                .get("name")
                .and_then(|name| name.as_str())
                .unwrap_or_default();
            let date_added = node
                .get("date_added")
                .and_then(|value| value.as_str())
                .and_then(|value| value.parse::<i64>().ok())
                .map(crate::browser_data::chromium_time_to_unix);
            let id = node
                .get("guid")
                .and_then(|guid| guid.as_str())
                .filter(|guid| !guid.is_empty())
                .map(str::to_string)
                .unwrap_or_else(|| format!("{}|{}|{}", url, title, out.len()));
            out.push(BrowserBookmark {
                id,
                title: title.to_string(),
                url: url.to_string(),
                folder_path: folder_path.to_string(),
                date_added,
            });
        }
        _ => {}
    }
}

/// Case-insensitive substring match over the title and the URL. An empty query
/// matches everything, which is what the "bookmarks bar" default view needs.
pub fn matches_query(bookmark: &BrowserBookmark, needle: &str) -> bool {
    if needle.is_empty() {
        return true;
    }
    let needle = needle.to_lowercase();
    bookmark.title.to_lowercase().contains(&needle) || bookmark.url.to_lowercase().contains(&needle)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// 2023-01-01T00:00:00Z in Chromium's microseconds-since-1601 form.
    const CHROMIUM_2023: &str = "13317004800000000";

    const FIXTURE: &str = r#"{
      "checksum": "abc",
      "roots": {
        "bookmark_bar": {
          "children": [
            { "date_added": "13317004800000000", "guid": "aaa", "id": "1", "name": "Rust", "type": "url", "url": "https://www.rust-lang.org/" },
            {
              "children": [
                { "date_added": "13317004800000000", "guid": "bbb", "id": "3", "name": "Docs", "type": "url", "url": "https://doc.rust-lang.org/" }
              ],
              "date_added": "13317004800000000",
              "guid": "folder",
              "id": "2",
              "name": "Work",
              "type": "folder"
            }
          ],
          "date_added": "0",
          "date_modified": "0",
          "id": "0",
          "name": "Bookmarks bar",
          "type": "folder"
        },
        "other": {
          "children": [
            { "date_added": "13317004800000000", "guid": "ccc", "id": "5", "name": "", "type": "url", "url": "https://example.com/" }
          ],
          "id": "4",
          "name": "Other bookmarks",
          "type": "folder"
        },
        "synced": { "children": [], "id": "6", "name": "Mobile bookmarks", "type": "folder" }
      },
      "version": 1
    }"#;

    #[test]
    fn bookmarks_are_flattened_with_folder_paths() {
        let bookmarks = parse_bookmarks_json(FIXTURE.as_bytes()).unwrap();
        assert_eq!(bookmarks.len(), 3);

        let rust = bookmarks
            .iter()
            .find(|bookmark| bookmark.title == "Rust")
            .unwrap();
        assert_eq!(rust.url, "https://www.rust-lang.org/");
        assert_eq!(rust.folder_path, "Bookmarks bar");
        assert_eq!(rust.id, "aaa");

        let docs = bookmarks
            .iter()
            .find(|bookmark| bookmark.title == "Docs")
            .unwrap();
        assert_eq!(docs.folder_path, "Bookmarks bar / Work");

        let unnamed = bookmarks
            .iter()
            .find(|bookmark| bookmark.url == "https://example.com/")
            .unwrap();
        assert_eq!(unnamed.folder_path, "Other bookmarks");
    }

    #[test]
    fn date_added_is_converted_from_the_chromium_epoch() {
        let bookmarks = parse_bookmarks_json(FIXTURE.as_bytes()).unwrap();
        let rust = bookmarks
            .iter()
            .find(|bookmark| bookmark.title == "Rust")
            .unwrap();
        // 13317004800000000 µs since 1601 → 1672531200 (2023-01-01T00:00:00Z).
        assert_eq!(rust.date_added, Some(1_672_531_200));
        assert_eq!(CHROMIUM_2023.parse::<i64>().unwrap() / 1_000_000 - 11_644_473_600, 1_672_531_200);
    }

    #[test]
    fn a_missing_or_corrupt_file_is_a_soft_failure() {
        let temp = tempfile::tempdir().unwrap();
        assert!(parse_bookmarks_file(&temp.path().join("nope")).is_err());
        assert!(parse_bookmarks_json(b"{ not json").is_err());
    }

    #[test]
    fn an_empty_document_yields_no_bookmarks() {
        assert!(parse_bookmarks_json(b"{}").unwrap().is_empty());
        assert!(parse_bookmarks_json(br#"{"roots":{}}"#).unwrap().is_empty());
    }

    #[test]
    fn query_matching_is_case_insensitive_over_title_and_url() {
        let bookmarks = parse_bookmarks_json(FIXTURE.as_bytes()).unwrap();
        assert!(matches_query(&bookmarks[0], ""));
        assert!(matches_query(&bookmarks[0], "rust"));
        assert!(matches_query(&bookmarks[0], "RUST-LANG"));
        assert!(!matches_query(&bookmarks[0], "python"));
    }
}
