//! Best-effort `--help` output parsing used to seed static argument hints.
//!
//! When a local tool is connected, Floter runs its executable once with
//! `--help` and feeds the captured output to [`derive_arguments`]. The parser
//! is deliberately conservative and generic: it recognizes option-definition
//! lines across common CLI styles (clap, argparse, cobra/Go flag, getopt) and
//! silently ignores everything it does not understand. A connected tool must
//! never fail to connect, nor suggest wrong parameters, because of this step —
//! garbage in, empty (or partial) out.

use crate::extensions::capability_probe::CapabilityProbe;
use crate::extensions::provider::{ArgumentDescriptor, ArgumentKind};
use serde_json::Value;
use std::path::Path;
use std::time::Duration;

/// Upper bound on derived arguments so pathological help output cannot flood
/// launcher completions.
const MAX_ARGUMENTS: usize = 40;

/// Upper bound on derived subcommands (mirrors the `MAX_ARGUMENTS` spirit:
/// root command + at most this many per-subcommand descriptor commands).
const MAX_SUBCOMMANDS: usize = 40;

/// How many subcommands get their own second-level help probe at connect
/// time. Tools with huge plugin lists must not turn one connection into a
/// process storm; entries beyond this budget simply ship without flags.
const MAX_SUBCOMMAND_PROBES: usize = 12;

/// Total wall-clock budget for the whole second-level probing phase (`--help`
/// plus a `-h` retry per subcommand). Per-probe failures and timeouts yield
/// no arguments for that subcommand and never block the connection.
const SUBCOMMAND_PROBE_TIMEOUT: Duration = Duration::from_secs(4);

/// Total wall-clock budget for the connect-time `--help` probe. The probe
/// itself already has its own timeout; this outer bound keeps connects snappy
/// even when a binary hangs before the inner timeout fires.
const HELP_DERIVE_TIMEOUT: Duration = Duration::from_secs(3);

/// One subcommand/plugin entry extracted from a listing-style `--help`
/// output (v-style plugin rows, cobra `Available Commands:` sections).
#[derive(Debug, Clone)]
pub struct DerivedSubcommand {
    /// Sanitized name: lowercase ASCII alphanumerics plus `-`/`_`, starting
    /// with a letter or digit — the same charset generated commands use.
    pub name: String,
    /// Aliases captured from an `(aliases: a, b)` group when present.
    pub aliases: Vec<String>,
    /// Description from the indented follow-up line (v style) or the same-line
    /// remainder after a wide gap (cobra style). Empty when none was found.
    pub description: String,
    /// Flags parsed from the subcommand's own help output; empty unless
    /// [`probe_derive`] managed to probe it within budget.
    pub arguments: Vec<ArgumentDescriptor>,
}

/// Full connect-time derivation: root-level argument hints plus one entry per
/// detected subcommand (each carrying its own probed flags when available).
#[derive(Debug, Clone, Default)]
pub struct HelpDerivation {
    pub root_arguments: Vec<ArgumentDescriptor>,
    pub subcommands: Vec<DerivedSubcommand>,
}

/// Capture the help text produced by `executable args…` (any exit code
/// counts), preferring stdout and falling back to stderr when stdout is
/// empty. Returns `None` when nothing readable came back in time.
async fn probe_help_text(executable: &Path, args: &[&str]) -> Option<String> {
    let probe = CapabilityProbe::custom("help-args", args.iter().copied()).expect_exit_code(None);
    let result = tokio::time::timeout(HELP_DERIVE_TIMEOUT, probe.probe(executable))
        .await
        .ok()?
        .ok()?;
    let text = if result.stdout.trim().is_empty() {
        result.stderr.as_str()
    } else {
        result.stdout.as_str()
    };
    (!text.trim().is_empty()).then(|| text.to_string())
}

/// Run `executable --help` once and derive everything Floter understands from
/// it: root option definitions and, when the output turns out to be a
/// subcommand/plugin listing instead of an option listing, one entry per
/// listed subcommand — each probed once for its own help (`<sub> --help`,
/// retried with `<sub> -h` when empty) so its real flags can be suggested.
/// Best-effort by contract: any failure degrades silently (root-only or no
/// flags at all) and never blocks a connection.
pub async fn probe_derive(executable: &Path) -> HelpDerivation {
    let Some(root_text) = probe_help_text(executable, &["--help"]).await else {
        return HelpDerivation::default();
    };
    let root_arguments = derive_arguments(&root_text);
    let candidates = derive_subcommands(&root_text);
    let subcommands = probe_subcommand_help(executable, &candidates).await;
    HelpDerivation {
        root_arguments,
        subcommands,
    }
}

/// Thin wrapper kept for callers that only need root-level hints.
pub async fn probe_derive_arguments(executable: &Path) -> Vec<ArgumentDescriptor> {
    probe_derive(executable).await.root_arguments
}

/// Probe up to [`MAX_SUBCOMMAND_PROBES`] subcommands concurrently within one
/// total budget and attach each probe's derived flags to its entry. Results
/// are returned in the candidates' original order regardless of which probe
/// finished first; failed probes contribute empty flag lists.
async fn probe_subcommand_help(
    executable: &Path,
    candidates: &[DerivedSubcommand],
) -> Vec<DerivedSubcommand> {
    let selected: Vec<&DerivedSubcommand> = candidates.iter().take(MAX_SUBCOMMAND_PROBES).collect();
    if selected.is_empty() {
        return Vec::new();
    }
    let mut set = tokio::task::JoinSet::new();
    for candidate in &selected {
        let executable = executable.to_path_buf();
        let name = candidate.name.clone();
        set.spawn(async move {
            let arguments = tokio::time::timeout(SUBCOMMAND_PROBE_TIMEOUT, async {
                let mut text = probe_help_text(&executable, &[name.as_str(), "--help"]).await;
                if text.as_deref().is_none_or(|text| text.trim().is_empty()) {
                    // Some tools only answer the short form (v plugins do).
                    text = probe_help_text(&executable, &[name.as_str(), "-h"]).await;
                }
                text.map(|text| derive_arguments(&text)).unwrap_or_default()
            })
            .await;
            (name, arguments.unwrap_or_default())
        });
    }
    let mut probed = std::collections::HashMap::new();
    while let Some(joined) = set.join_next().await {
        if let Ok((name, arguments)) = joined {
            probed.insert(name, arguments);
        }
    }
    selected
        .into_iter()
        .map(|candidate| DerivedSubcommand {
            arguments: probed.remove(&candidate.name).unwrap_or_default(),
            ..candidate.clone()
        })
        .collect()
}

/// Extract subcommand entries from a listing-style help output.
///
/// Two best-effort shapes are recognized:
/// - v-style plugin rows anywhere in the output: leading non-word glyphs
///   (emoji, punctuation) are stripped, the first token becomes the candidate
///   name, a following version-looking token is ignored, and `(aliases: a,
///   b)` groups are captured; the description comes from the next indented
///   line.
/// - cobra/go-style rows inside `Commands:` / `Available Commands:` /
///   `Subcommands:` sections: `name` + wide gap + description on the same
///   line.
///
/// Flag lines, usage banners, URLs, and obvious non-commands (`help`,
/// `version`, `completion`, `man`) never become entries; names failing the
/// generated-command charset are skipped; results dedupe by name, keep
/// first-seen order, and cap at [`MAX_SUBCOMMANDS`].
pub fn derive_subcommands(help_output: &str) -> Vec<DerivedSubcommand> {
    const NON_COMMANDS: [&str; 4] = ["help", "version", "completion", "man"];
    let mut subcommands: Vec<DerivedSubcommand> = Vec::new();
    let mut seen: std::collections::HashSet<String> = std::collections::HashSet::new();
    // Index of the most recent v-style row still awaiting its indented
    // description line.
    let mut pending: Option<usize> = None;
    // True while inside a Commands/Available Commands/Subcommands section
    // (cobra-style listings), enabling same-line row recognition there.
    let mut in_command_section = false;

    for line in help_output.lines() {
        let trimmed = line.trim();
        if trimmed.is_empty() {
            continue;
        }
        let indented = line.starts_with([' ', '\t']);
        if is_section_header(trimmed) {
            let lowered = trimmed.to_ascii_lowercase();
            in_command_section = lowered.contains("command") || lowered.contains("plugin");
            pending = None;
            continue;
        }
        // Usage banners and URLs never introduce an entry.
        if skipped_line(trimmed) {
            continue;
        }
        // Flag definitions are never subcommands.
        if trimmed.starts_with('-') {
            continue;
        }
        // Indented follow-up line describing the previous v-style row.
        if indented && !in_command_section {
            if let Some(index) = pending {
                if subcommands[index].description.is_empty() {
                    subcommands[index].description = trimmed.to_string();
                }
                pending = None;
            }
            continue;
        }
        if subcommands.len() >= MAX_SUBCOMMANDS {
            break;
        }

        let (remainder, aliases) = extract_aliases(trimmed);
        let parsed = if in_command_section && indented {
            command_section_row(&remainder)
        } else if !indented {
            listing_row(&remainder)
        } else {
            None
        };
        let Some((name, description)) = parsed else {
            continue;
        };
        if NON_COMMANDS.contains(&name.as_str()) || !seen.insert(name.clone()) {
            continue;
        }
        pending = Some(subcommands.len());
        subcommands.push(DerivedSubcommand {
            name,
            aliases,
            description,
            arguments: Vec::new(),
        });
    }
    subcommands
}

/// Parse one v-style listing row into `(name, description)`. The description
/// is always empty here — v-style rows take theirs from the next indented
/// line, and any same-line remainder is metadata (author, homepage).
fn listing_row(remainder: &str) -> Option<(String, String)> {
    let rest = remainder.trim_start_matches(|character: char| !character.is_ascii_alphanumeric());
    if rest.is_empty() {
        return None;
    }
    let token_end = rest.find(char::is_whitespace).unwrap_or(rest.len());
    // Strict charset: unscoped prose lines must not slip through as entries,
    // so mixed-case tokens ("Run …", "Version: …") are rejected outright.
    let name = sanitize_subcommand_name(&rest[..token_end], true)?;
    let after = rest[token_end..].trim_start();
    // Ignore a version-looking token directly after the name (`jv 1.0.0`).
    let after = match after.split_whitespace().next() {
        Some(token) if is_version_token(token) => after[token.len()..].trim_start(),
        _ => after,
    };
    // Banner/usage shapes (`demo - Gadgets…`, `demo [options]`) are not
    // entries even though their first token looks like a valid name.
    if after.starts_with('-') || after.starts_with('[') || after.starts_with('<') {
        return None;
    }
    Some((name, String::new()))
}

/// Parse one cobra/go-style row (`  get    Get something`) into
/// `(name, same-line description)`. Requires the wide gap that separates the
/// definition column from the description column.
fn command_section_row(line: &str) -> Option<(String, String)> {
    let rest = line.trim_start_matches(|character: char| !character.is_ascii_alphanumeric());
    if rest.is_empty() {
        return None;
    }
    let token_end = rest.find(char::is_whitespace).unwrap_or(rest.len());
    let name = sanitize_subcommand_name(&rest[..token_end], false)?;
    let after = &rest[token_end..];
    let gap = after.len() - after.trim_start().len();
    (gap >= 2).then(|| (name, after.trim().to_string()))
}

/// Validate and canonicalize a candidate name against the generated-command
/// charset: lowercase ASCII alphanumerics plus `-`/`_`, starting with a
/// letter or digit. Trailing list punctuation (`,`, `:`, `.`, `;`) is
/// stripped first; when `strict` is set, uppercase letters are rejected
/// instead of folded (used for unscoped lines where mixed case usually means
/// prose rather than a command name).
fn sanitize_subcommand_name(raw: &str, strict: bool) -> Option<String> {
    let raw = raw.trim_end_matches([',', ':', '.', ';']);
    if raw.is_empty() {
        return None;
    }
    let mut name = String::with_capacity(raw.len());
    for character in raw.chars() {
        if character.is_ascii_uppercase() {
            if strict {
                return None;
            }
            name.push(character.to_ascii_lowercase());
        } else if character.is_ascii_lowercase()
            || character.is_ascii_digit()
            || matches!(character, '-' | '_')
        {
            name.push(character);
        } else {
            return None;
        }
    }
    let first = name.chars().next()?;
    first.is_ascii_alphanumeric().then_some(name)
}

/// Version-looking token: digits and dots with at least one dot (`1.0.0`,
/// `0.2`). Single numbers stay untouched so numeric subcommand names work.
fn is_version_token(token: &str) -> bool {
    !token.is_empty()
        && token.contains('.')
        && token
            .bytes()
            .all(|byte| byte.is_ascii_digit() || byte == b'.')
}

/// Split a trailing `(aliases: a, b)` group off a listing row. Returns the
/// remaining text plus the collected, validated alias list.
fn extract_aliases(trimmed: &str) -> (String, Vec<String>) {
    let lowered = trimmed.to_ascii_lowercase();
    let Some(marker) = lowered.find("(aliases") else {
        return (trimmed.to_string(), Vec::new());
    };
    let Some(offset) = trimmed[marker..].find(')') else {
        return (trimmed.to_string(), Vec::new());
    };
    let inner = trimmed[marker + "(aliases".len()..marker + offset]
        .trim()
        .trim_start_matches(':')
        .trim();
    let mut seen: std::collections::HashSet<String> = std::collections::HashSet::new();
    let aliases = inner
        .split(',')
        .map(str::trim)
        .filter(|alias| {
            !alias.is_empty()
                && !alias
                    .chars()
                    .any(|character| character.is_whitespace() || matches!(character, '(' | ')'))
                && seen.insert(alias.to_string())
        })
        .map(String::from)
        .collect();
    (
        format!("{}{}", &trimmed[..marker], &trimmed[marker + offset + 1..]),
        aliases,
    )
}

/// Parse raw `--help` output into ordered argument descriptors.
///
/// Line-based by design: each line is classified independently, with one
/// narrow exception — an indented plain-text line following an option whose
/// description is still empty is treated as its description (Go `flag` style).
pub fn derive_arguments(help_output: &str) -> Vec<ArgumentDescriptor> {
    let mut arguments: Vec<ArgumentDescriptor> = Vec::new();
    let mut seen_keys: std::collections::HashSet<String> = std::collections::HashSet::new();
    for line in help_output.lines() {
        let trimmed = line.trim();
        if trimmed.is_empty() || skipped_line(trimmed) {
            continue;
        }
        // Continuation line (Go flag style): indented prose describing the
        // previous option. Guarded so separators, extra flags, and headers
        // are never glued onto an unrelated argument.
        if line.starts_with([' ', '\t'])
            && !trimmed.starts_with('-')
            && arguments
                .last()
                .is_some_and(|last| last.description.is_empty())
        {
            if let Some(last) = arguments.last_mut() {
                last.description = continuation_description(trimmed);
            }
            continue;
        }
        let Some(raw) = parse_option_line(line) else {
            continue;
        };
        if raw.names.is_empty()
            || raw
                .names
                .iter()
                .any(|name| matches!(name.as_str(), "-h" | "--help" | "-V" | "--version"))
        {
            continue;
        }
        // Dedupe by canonical long name (or first flag when no long form
        // exists) so the same option reached through wrapped or repeated
        // listings is only suggested once.
        let key = raw
            .names
            .iter()
            .find(|name| name.starts_with("--"))
            .cloned()
            .unwrap_or_else(|| raw.names[0].clone());
        if !seen_keys.insert(key) {
            continue;
        }
        let takes_value = raw.value_hint.is_some();
        let kind = argument_kind(raw.value_hint.as_deref(), takes_value);
        arguments.push(ArgumentDescriptor {
            names: raw.names,
            kind,
            description: raw.description,
            takes_value,
            required: false,
            repeatable: false,
            values: Vec::new(),
            value_hint: raw.value_hint,
        });
        if arguments.len() >= MAX_ARGUMENTS {
            break;
        }
    }
    arguments
}

/// Serialize parsed descriptors into the JSON shape demanded by the
/// provider-description schema. Mirrors [`ArgumentDescriptor`]'s own
/// serialization except that a missing `valueHint` is omitted entirely —
/// the schema rejects explicit `null`s and provider.rs owns that struct,
/// so the cleanup happens here.
pub fn to_json_array(arguments: &[ArgumentDescriptor]) -> Vec<Value> {
    arguments
        .iter()
        .map(|argument| {
            let Ok(mut value) = serde_json::to_value(argument) else {
                return Value::Null;
            };
            if value.get("valueHint").is_some_and(Value::is_null) {
                if let Some(object) = value.as_object_mut() {
                    object.remove("valueHint");
                }
            }
            value
        })
        .collect()
}

/// Lines that are never option definitions even though they may sit inside
/// the options section: usage banners (English and Chinese) and URLs.
fn skipped_line(trimmed: &str) -> bool {
    let lowered = trimmed.to_ascii_lowercase();
    lowered.starts_with("usage:")
        || trimmed.starts_with("用法：")
        || trimmed.starts_with("用法:")
        || trimmed.contains("http://")
        || trimmed.contains("https://")
        || is_section_header(trimmed)
}

/// Section headers such as `Options:`, `Flags:`, `Available Commands:`.
/// Structurally these are already rejected (they never start with `-`),
/// but checking explicitly documents intent and keeps the classifier honest.
fn is_section_header(trimmed: &str) -> bool {
    trimmed.ends_with(':')
        && trimmed.chars().all(|character| {
            character.is_alphanumeric() || character.is_whitespace() || character == ':'
        })
}

struct RawOption {
    names: Vec<String>,
    value_hint: Option<String>,
    description: String,
}

/// Parse one candidate line into flags plus optional value placeholder and
/// description. Returns `None` for anything that does not begin with a
/// syntactically plausible flag, which structurally excludes usage lines,
/// section headers, subcommand lists, and free-form examples.
fn parse_option_line(line: &str) -> Option<RawOption> {
    let trimmed = line.trim_start();
    if !trimmed.starts_with('-') {
        return None;
    }

    // Tokenize with byte offsets and record the whitespace gap preceding each
    // token; a gap of 2+ columns separates the definition from its
    // description in virtually every help formatter.
    let mut tokens: Vec<(usize, &str)> = Vec::new();
    let mut gaps: Vec<usize> = Vec::new();
    let mut cursor = 0usize;
    while cursor < trimmed.len() {
        let leading_ws = trimmed[cursor..].len() - trimmed[cursor..].trim_start().len();
        let start = cursor + leading_ws;
        if start >= trimmed.len() {
            break;
        }
        let end = trimmed[start..]
            .find(char::is_whitespace)
            .map_or(trimmed.len(), |offset| start + offset);
        tokens.push((start, &trimmed[start..end]));
        gaps.push(leading_ws);
        cursor = end;
    }

    let mut names: Vec<String> = Vec::new();
    let mut value_hint: Option<String> = None;
    let mut description_start: Option<usize> = None;

    for (index, &(start, token)) in tokens.iter().enumerate() {
        if names.is_empty() {
            // Leading cluster: every token must be a flag (possibly with an
            // inline `=` value); anything else disqualifies the line.
            let Some((flag, inline)) = flag_token(token) else {
                return None;
            };
            names.push(flag);
            if let Some(inline) = inline {
                if let Some(value) = inline_value(inline) {
                    value_hint.get_or_insert(value);
                }
            }
            continue;
        }
        let wide_gap = gaps[index] >= 2;
        if let Some((flag, inline)) = flag_token(token) {
            if wide_gap {
                // Next row of a multi-column layout, not an alias.
                description_start = Some(start);
                break;
            }
            names.push(flag);
            if let Some(inline) = inline.and_then(inline_value) {
                value_hint.get_or_insert(inline);
            }
            continue;
        }
        // Placeholders may carry an alias-list comma too (`-c COUNT, ...`).
        let bare = token.strip_suffix(',').unwrap_or(token);
        if let Some(value) = placeholder_value(bare) {
            value_hint.get_or_insert(value);
            continue;
        }
        // Unknown token: the description begins here, whether or not the
        // formatter used a wide gap (fallback per spec).
        description_start = Some(start);
        break;
    }

    let description = description_start
        .and_then(|start| trimmed.get(start..))
        .map(str::trim)
        .unwrap_or_default()
        .to_string();

    (!names.is_empty()).then(|| RawOption {
        names,
        value_hint,
        description,
    })
}

/// Recognize a flag token (`-o,` `--output` `--format=json`) and return its
/// canonical spelling plus any inline value text.
fn flag_token(token: &str) -> Option<(String, Option<&str>)> {
    let (raw, inline) = match token.split_once('=') {
        Some((left, right)) => (left, Some(right)),
        None => (token, None),
    };
    let long = raw.starts_with("--");
    let body = if long {
        &raw[2..]
    } else {
        raw.strip_prefix('-')?
    };
    // Alias lists separate flags with commas (`-o, --output`).
    let body = body.strip_suffix(',').unwrap_or(body);
    let mut characters = body.chars();
    let first = characters.next()?;
    if !first.is_ascii_alphabetic()
        || !body
            .chars()
            .all(|character| character.is_ascii_alphanumeric() || matches!(character, '-' | '_'))
    {
        return None;
    }
    let dashes = if long { "--" } else { "-" };
    Some((format!("{dashes}{body}"), inline))
}

/// Detect a value placeholder and return its hint text: bracketed forms
/// (`<FILE>`, `[N]`, `(DIR)`) or a bare uppercase/type token (`N`, `COUNT`,
/// `int`) following the flags.
fn placeholder_value(token: &str) -> Option<String> {
    if token.len() > 24 || token.is_empty() {
        return None;
    }
    let bracketed = token
        .strip_prefix('<')
        .and_then(|rest| rest.strip_suffix('>'))
        .or_else(|| {
            token
                .strip_prefix('[')
                .and_then(|rest| rest.strip_suffix(']'))
        })
        .or_else(|| {
            token
                .strip_prefix('(')
                .and_then(|rest| rest.strip_suffix(')'))
        });
    if let Some(inner) = bracketed {
        return (!inner.is_empty() && !inner.chars().any(char::is_whitespace))
            .then(|| inner.to_string());
    }
    if token
        .chars()
        .all(|character| character.is_ascii_uppercase() || character.is_ascii_digit())
        && token
            .chars()
            .any(|character| character.is_ascii_alphabetic())
    {
        return Some(token.to_string());
    }
    // Lowercase type words emitted by Go `flag` and similar formatters.
    const TYPE_WORDS: [&str; 8] = [
        "int", "integer", "uint", "string", "float", "number", "duration", "bool",
    ];
    (TYPE_WORDS.contains(&token)).then(|| token.to_string())
}

/// Inline value after `=`: bracketed forms or any non-empty remainder.
fn inline_value(text: &str) -> Option<String> {
    placeholder_value(text).or((!text.is_empty()).then(|| text.to_string()))
}

/// Map a value hint onto a descriptor kind. Flags stay `Flag`; hints naming
/// files/directories upgrade to the richer kinds so completions can offer
/// paths, everything else degrades to plain strings.
fn argument_kind(value_hint: Option<&str>, takes_value: bool) -> ArgumentKind {
    if !takes_value {
        return ArgumentKind::Flag;
    }
    let hint = value_hint.unwrap_or_default().to_ascii_lowercase();
    if hint.contains("dir") {
        ArgumentKind::Directory
    } else if hint.contains("file") || hint.contains("path") {
        ArgumentKind::Path
    } else {
        ArgumentKind::String
    }
}

/// Continuation-line description cleanup: drop Go-style `(default …)`
/// trailers, which describe defaults rather than purpose.
fn continuation_description(trimmed: &str) -> String {
    let mut text = trimmed;
    if let Some(index) = text.find("(default") {
        if text.ends_with(')') {
            text = text[..index].trim_end();
        }
    }
    text.to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn names(arguments: &[ArgumentDescriptor]) -> Vec<Vec<String>> {
        arguments.iter().map(|a| a.names.clone()).collect()
    }

    #[test]
    fn parses_clap_style_block() {
        let help = "\
Usage: demo [OPTIONS]

Options:
  -o, --output <FILE>    Write result to FILE
      --verbose          Enable verbose logging
  -j, --threads N        number of threads
  -C, --dir <DIR>        Working directory
";
        let arguments = derive_arguments(help);
        assert_eq!(
            names(&arguments),
            vec![
                vec!["-o", "--output"],
                vec!["--verbose"],
                vec!["-j", "--threads"],
                vec!["-C", "--dir"],
            ]
        );
        assert!(arguments[0].takes_value);
        assert_eq!(arguments[0].value_hint.as_deref(), Some("FILE"));
        assert_eq!(arguments[0].kind, ArgumentKind::Path);
        assert_eq!(arguments[0].description, "Write result to FILE");
        assert!(!arguments[1].takes_value);
        assert_eq!(arguments[1].kind, ArgumentKind::Flag);
        assert_eq!(arguments[1].description, "Enable verbose logging");
        assert!(arguments[2].takes_value);
        assert_eq!(arguments[2].value_hint.as_deref(), Some("N"));
        assert_eq!(arguments[3].kind, ArgumentKind::Directory);
    }

    #[test]
    fn parses_argparse_style_block() {
        let help = "\
usage: prog [-h] [--count COUNT] [--mode MODE] source

options:
  -h, --help            show this help message and exit
  -c COUNT, --count COUNT
                        number of items to process
  --mode MODE           operation mode
";
        let arguments = derive_arguments(help);
        assert_eq!(
            names(&arguments),
            vec![vec!["-c", "--count"], vec!["--mode"]]
        );
        assert!(arguments[0].takes_value);
        assert_eq!(arguments[0].description, "number of items to process");
        assert_eq!(arguments[1].description, "operation mode");
    }

    #[test]
    fn parses_go_flag_style_continuation_lines() {
        let help = "\
Usage of demo:
  -j int
    	number of threads (default 4)
  -verbose
    	enable verbose logging
";
        let arguments = derive_arguments(help);
        assert_eq!(names(&arguments), vec![vec!["-j"], vec!["-verbose"]]);
        assert!(arguments[0].takes_value);
        assert_eq!(arguments[0].description, "number of threads");
        assert!(!arguments[1].takes_value);
        assert_eq!(arguments[1].description, "enable verbose logging");
    }

    #[test]
    fn skips_chinese_usage_lines_urls_and_headers() {
        let help = "\
用法：demo [选项]

选项：
  -o, --output <FILE>    输出文件
  文档见 https://example.com/docs
";
        let arguments = derive_arguments(help);
        assert_eq!(names(&arguments), vec![vec!["-o", "--output"]]);
        assert_eq!(arguments[0].description, "输出文件");
    }

    #[test]
    fn excludes_help_and_version_options() {
        let help = "\
Options:
  -h, --help       Print help
  -V, --version    Print version
  -v, --verbose    Verbose output
";
        let arguments = derive_arguments(help);
        assert_eq!(names(&arguments), vec![vec!["-v", "--verbose"]]);
    }

    #[test]
    fn dedupes_repeated_definitions_by_canonical_name() {
        let help = "\
Options:
  -o, --output <FILE>    Write result to FILE
  --output <FILE>        Aliased listing of the same option
  --input <FILE>         Read from FILE
";
        let arguments = derive_arguments(help);
        assert_eq!(
            names(&arguments),
            vec![vec!["-o", "--output"], vec!["--input"]]
        );
    }

    #[test]
    fn detects_value_placeholders_across_styles() {
        assert_eq!(placeholder_value("<FILE>").as_deref(), Some("FILE"));
        assert_eq!(placeholder_value("[COUNT]").as_deref(), Some("COUNT"));
        assert_eq!(placeholder_value("(DIR)").as_deref(), Some("DIR"));
        assert_eq!(placeholder_value("N").as_deref(), Some("N"));
        assert_eq!(placeholder_value("int").as_deref(), Some("int"));
        assert!(placeholder_value("enable").is_none());
        assert!(placeholder_value("--flag").is_none());
    }

    #[test]
    fn flags_without_values_take_no_value() {
        let arguments = derive_arguments("--quiet                   suppress output");
        assert_eq!(arguments.len(), 1);
        assert!(!arguments[0].takes_value);
        assert_eq!(arguments[0].value_hint, None);
    }

    #[test]
    fn caps_results_and_never_panics_on_garbage() {
        let mut help = String::from("Options:\n");
        for index in 0..60 {
            help.push_str(&format!("  --opt-{index}            option {index}\n"));
        }
        let arguments = derive_arguments(&help);
        assert_eq!(arguments.len(), MAX_ARGUMENTS);

        for garbage in [
            "",
            "\u{fffd}\u{0} \n",
            "---- ==== <<<>>>",
            "-",
            "random text",
        ] {
            assert!(derive_arguments(garbage).is_empty(), "{garbage:?}");
        }
    }

    #[test]
    fn parses_v_style_plugin_rows_with_aliases_and_descriptions() {
        let help = "\
v - Gadgets under the terminal
Version: dev  🏠 https://github.com/vst93/v

Available Plugins
==================================================
📦 json2excel 0.0.1 👤 vst  (aliases: j2e)
  convert json data to excel file

📦 jv 1.0.0 👤 vst
  JSON Viewer & Formatter - format, compress, escape, ...
📦 codec 0.3 (aliases: cc, enc)

Run v <command> -h for detailed help.
";
        let subcommands = derive_subcommands(help);
        assert_eq!(
            subcommands
                .iter()
                .map(|sub| sub.name.as_str())
                .collect::<Vec<_>>(),
            vec!["json2excel", "jv", "codec"]
        );
        assert_eq!(subcommands[0].aliases, ["j2e"]);
        assert_eq!(
            subcommands[0].description,
            "convert json data to excel file"
        );
        assert_eq!(
            subcommands[1].description,
            "JSON Viewer & Formatter - format, compress, escape, ..."
        );
        assert_eq!(subcommands[2].aliases, ["cc", "enc"]);
        assert!(subcommands[2].description.is_empty());
    }

    /// Indented rows inside a Commands/Plugins section follow the cobra shape
    /// and require the wide definition-column gap; an indented single-spaced
    /// row matches neither documented listing style and must yield no entry.
    /// (This is why live fixtures must print v-style plugin rows unindented.)
    #[test]
    fn rejects_indented_section_row_without_wide_gap() {
        let help = "\
Available Plugins
  alpha 1.0.0 (aliases: al)
    First gadget
";
        assert!(derive_subcommands(help).is_empty());
        // The same row unindented is the documented v-style listing shape.
        let help = "\
Available Plugins
alpha 1.0.0 (aliases: al)
    First gadget
";
        let subcommands = derive_subcommands(help);
        assert_eq!(subcommands.len(), 1);
        assert_eq!(subcommands[0].name, "alpha");
        assert_eq!(subcommands[0].aliases, ["al"]);
    }

    #[test]
    fn parses_cobra_available_commands_sections() {
        let help = "\
Usage:
  mycli [command]

Available Commands:
  get         Get something from somewhere
  set         Set something useful
  completion  Generate the autocompletion script
  help        Help about any command

Flags:
  -h, --help   help for mycli

Use \"mycli [command] --help\" for more information.
";
        let subcommands = derive_subcommands(help);
        assert_eq!(
            subcommands
                .iter()
                .map(|sub| (sub.name.as_str(), sub.description.as_str()))
                .collect::<Vec<_>>(),
            vec![
                ("get", "Get something from somewhere"),
                ("set", "Set something useful")
            ]
        );
    }

    #[test]
    fn flag_lines_urls_and_banners_never_become_subcommands() {
        let help = "\
Options:
  -f         Format JSON
  -sort      Sort keys
Docs: https://example.com/docs
mytool - A banner description
usage: mytool [options]
";
        assert!(derive_subcommands(help).is_empty());
    }

    #[test]
    fn invalid_or_prose_names_are_skipped() {
        let help = "\
🚀 Bad-Name 1.0
@@@ !!!
Run demo <command> -h now
ok-name 1.2.3
  indented prose line without a section
";
        let names = derive_subcommands(help)
            .into_iter()
            .map(|sub| sub.name)
            .collect::<Vec<_>>();
        // Only the valid lowercase row survives; "Bad-Name", "Run", and the
        // glyph-only row are rejected by the strict charset check.
        assert_eq!(names, vec!["ok-name"]);
    }

    #[test]
    fn dedupes_subcommands_and_caps_the_result() {
        let mut help = String::from("Available Commands:\n");
        help.push_str("  dup        First occurrence\n");
        help.push_str("  dup        Second occurrence\n");
        for index in 0..MAX_SUBCOMMANDS + 5 {
            help.push_str(&format!("  cmd-{index}       Command {index}\n"));
        }
        let subcommands = derive_subcommands(&help);
        assert_eq!(subcommands.len(), MAX_SUBCOMMANDS);
        assert_eq!(subcommands[0].name, "dup");
        assert_eq!(subcommands[0].description, "First occurrence");
        assert_eq!(subcommands[1].name, "cmd-0");
    }

    #[test]
    fn version_tokens_are_ignored_but_numeric_names_survive() {
        assert!(is_version_token("0.0.1"));
        assert!(is_version_token("1.10"));
        assert!(!is_version_token("dev"));
        assert!(!is_version_token("42"));
        let subcommands = derive_subcommands("📦 7zip 1.0.0 👤 vst\n  zip tool\n");
        assert_eq!(subcommands.len(), 1);
        assert_eq!(subcommands[0].name, "7zip");
        assert_eq!(subcommands[0].description, "zip tool");
    }

    #[test]
    fn to_json_array_omits_null_value_hints() {
        let arguments =
            derive_arguments("-o, --output <FILE>    file\n--verbose              be loud");
        let json = to_json_array(&arguments);
        assert_eq!(json.len(), 2);
        assert!(json[0].get("valueHint").is_some());
        assert!(json[1].get("valueHint").is_none());
        assert_eq!(json[1]["kind"], "flag");
    }
}
