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

/// Total wall-clock budget for the connect-time `--help` probe. The probe
/// itself already has its own timeout; this outer bound keeps connects snappy
/// even when a binary hangs before the inner timeout fires.
const HELP_DERIVE_TIMEOUT: Duration = Duration::from_secs(3);

/// Run `executable --help` once (any exit code counts) and parse option
/// definitions from its output, preferring stdout and falling back to stderr
/// when stdout is empty. Returns an empty vector on every failure: help
/// parsing is purely additive and must never block a connection.
pub async fn probe_derive_arguments(executable: &Path) -> Vec<ArgumentDescriptor> {
    let probe = CapabilityProbe::custom("help-args", ["--help"]).expect_exit_code(None);
    let derived = tokio::time::timeout(HELP_DERIVE_TIMEOUT, async move {
        let result = probe.probe(executable).await.ok()?;
        let text = if result.stdout.trim().is_empty() {
            result.stderr.as_str()
        } else {
            result.stdout.as_str()
        };
        (!text.trim().is_empty()).then(|| derive_arguments(text))
    })
    .await;
    match derived {
        Ok(Some(arguments)) => arguments,
        // Timed out, could not execute, or produced no readable help text.
        _ => Vec::new(),
    }
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
