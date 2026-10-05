//! R89 · the clipboard list's token search, over the **full** text.
//!
//! The launcher's clipboard mode used to receive the whole history and filter it
//! in memory. R89 caps what that list IPC carries (see
//! `LIST_TEXT_PREFIX_BYTES` in the parent module), so an in-memory filter can no
//! longer see a match that lives past an entry's prefix. This module is the
//! backend's answer to that: the same AND rule the frontend runs
//! (`plugins/search.ts` + `clipboardEntrySearchFields`), executed here against
//! the untruncated text read off disk, so a query whose only match is deep in a
//! long entry still has a path to a row.
//!
//! It is deliberately the *same rule*, not a similar one. The node contract
//! suite (`tests/r89-clipboard-search-contract.test.ts`) hard-codes the same
//! fixture vectors this module's tests do, and one of those node tests reads
//! this file back to assert the two fixtures have not drifted. A token rule that
//! only agrees in one language is a bug waiting for the next round.
//!
//! Pure: no Tauri, no disk, no globals. The parent module hands it a needle and
//! the already-read entries.

use super::ClipboardEntry;

/// Split a needle into its AND tokens: lowercased, whitespace-separated,
/// empties dropped. `""`, `"   "` and `"  a  "` are `[]` and `["a"]` — the
/// Rust twin of `searchTokens` in `src/plugins/search.ts`.
pub fn search_tokens(needle: &str) -> Vec<String> {
    needle.split_whitespace().map(str::to_lowercase).collect()
}

/// The strings one clipboard entry can be searched through — the Rust twin of
/// `clipboardEntrySearchFields` in `src/clipboard-history.ts`.
///
/// A files entry answers to its stored paths and **only** its paths (its `text`
/// is a preview, not a search target); a text entry answers to its content; an
/// image entry answers to a stored caption *and* to its own name in either UI
/// language. Filtering only decides which rows are kept — the kind is never
/// touched, so a captioned image still renders as an image.
pub fn entry_search_fields(entry: &ClipboardEntry) -> Vec<&str> {
    let mut fields: Vec<&str> = Vec::new();
    if entry.kind == "files" {
        if let Some(paths) = entry.paths.as_ref() {
            fields.extend(paths.iter().map(String::as_str));
        }
    } else if let Some(text) = entry.text.as_ref() {
        fields.push(text.as_str());
    }
    if entry.kind == "image" {
        fields.extend(["image", "img", "图片"]);
    }
    fields
}

/// Whether every token is found in at least one of the fields. Tokens may hit
/// **different** fields (a captioned image answering to its caption and to
/// `图片`), and nullish or empty fields are dropped so a missing field cannot
/// accidentally satisfy a token. With no tokens the answer is `true`: an empty
/// query matches everything.
pub fn matches_tokens(tokens: &[String], fields: &[&str]) -> bool {
    if tokens.is_empty() {
        return true;
    }
    let haystacks: Vec<String> = fields
        .iter()
        .filter(|field| !field.is_empty())
        .map(|field| field.to_lowercase())
        .collect();
    tokens.iter().all(|token| {
        haystacks
            .iter()
            .any(|haystack| haystack.contains(token.as_str()))
    })
}

/// Whether one entry survives a query's tokens under the shared rule.
pub fn entry_matches(entry: &ClipboardEntry, tokens: &[String]) -> bool {
    matches_tokens(tokens, &entry_search_fields(entry))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn entry(kind: &str, text: Option<&str>, paths: &[&str]) -> ClipboardEntry {
        ClipboardEntry {
            id: "id".to_string(),
            kind: kind.to_string(),
            text: text.map(str::to_string),
            paths: if paths.is_empty() {
                None
            } else {
                Some(paths.iter().map(|path| path.to_string()).collect())
            },
            image_file: None,
            width: None,
            height: None,
            hash: "h".to_string(),
            created_at: 0,
            favorite: false,
        }
    }

    /// The shared contract fixture. These vectors are byte-for-byte the ones in
    /// `tests/r89-clipboard-search-contract.test.ts` (the node suite reads this
    /// file and asserts the names still line up); changing one side without the
    /// other is a red suite on purpose.
    macro_rules! case {
        ($name:expr, $kind:expr, $text:expr, $paths:expr, $query:expr, $expected:expr) => {
            ($name, $kind, $text, $paths, $query, $expected)
        };
    }

    /// One fixture vector: name, kind, text, paths, query, expected. A named
    /// alias rather than an inline tuple, which clippy rightly calls complex.
    type Vector = (
        &'static str,
        &'static str,
        Option<&'static str>,
        &'static [&'static str],
        &'static str,
        bool,
    );

    #[test]
    fn the_shared_contract_vectors_hold() {
        // One vector per line, deliberately: the node suite reads these names
        // back, and a wrapped fixture is unreadable. rustfmt is told to leave it.
        #[rustfmt::skip]
        let cases: &[Vector] = &[
            case!("empty query matches everything", "text", Some("hello"), &[], "", true),
            case!("whitespace-only query matches everything", "text", Some("hello"), &[], "   ", true),
            case!("single token hits the text field", "text", Some("hello world"), &[], "hello", true),
            case!("AND: two tokens in one text field", "text", Some("hello world"), &[], "world hello", true),
            case!("AND: one missing token rejects", "text", Some("hello world"), &[], "hello missing", false),
            case!("case-insensitive hit", "text", Some("Rust Async"), &[], "rUsT aSYNC", true),
            case!("CJK token", "text", Some("剪贴板历史"), &[], "剪贴板", true),
            case!("CJK two tokens AND", "text", Some("剪贴板历史"), &[], "历史 剪贴板", true),
            case!("files entry matches a path token", "files", None, &["/tmp/report.pdf"], "report", true),
            case!("files entry matches a full path token", "files", None, &["/tmp/report.pdf"], "/tmp/report.pdf", true),
            case!("files entry ignores a stray text field", "files", Some("hidden"), &["/tmp/a.txt"], "hidden", false),
            case!("files entry misses an absent path token", "files", None, &["/tmp/a.txt"], "zzz", false),
            case!("bare image answers to its name", "image", None, &[], "图片", true),
            case!("bare image answers to img", "image", None, &[], "IMG", true),
            case!("captioned image matches its caption", "image", Some("diagram"), &[], "diagram", true),
            case!("captioned image: tokens may hit different fields", "image", Some("diagram"), &[], "diagram 图片", true),
            case!("text entry does not answer to the image word", "text", Some("diagram"), &[], "diagram 图片", false),
            case!("non-files text entry matches its text", "text", Some("hidden"), &[], "hidden", true),
        ];
        for &(name, kind, text, paths, query, expected) in cases {
            let entry = entry(kind, text, paths);
            let tokens = search_tokens(query);
            assert_eq!(
                entry_matches(&entry, &tokens),
                expected,
                "shared vector drifted: {name}"
            );
        }
    }

    #[test]
    fn tokens_split_like_the_frontend_rule() {
        assert!(search_tokens("").is_empty());
        assert!(search_tokens("   ").is_empty());
        assert_eq!(search_tokens("  a  "), vec!["a".to_string()]);
        assert_eq!(
            search_tokens("A  b"),
            vec!["a".to_string(), "b".to_string()]
        );
    }

    #[test]
    fn a_missing_field_cannot_satisfy_a_token() {
        // An image entry with no caption and no name words in the query: only
        // the two name words are searchable, and neither is "ghost".
        let entry = entry("image", None, &[]);
        assert!(!entry_matches(&entry, &search_tokens("ghost")));
        assert!(entry_matches(&entry, &search_tokens("img")));
    }
}
