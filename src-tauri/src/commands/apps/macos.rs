//! macOS application discovery: `.app` bundles read through `Info.plist`.

use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::Duration;
use tauri::AppHandle;

use super::{
    cached_icon_is_fresh, icon_cache_path, mark_icon_cached, paths_signature, LocalApplication,
};

pub fn roots() -> Vec<PathBuf> {
    let mut roots = vec![
        PathBuf::from("/Applications"),
        PathBuf::from("/System/Applications"),
        PathBuf::from("/System/Applications/Utilities"),
    ];

    if let Some(home) = dirs::home_dir() {
        roots.push(home.join("Applications"));
    }

    roots
}

pub fn scan(roots: &[PathBuf]) -> Vec<LocalApplication> {
    let mut apps = Vec::new();
    let mut seen = HashSet::new();

    for dir in roots {
        collect_apps(dir, 0, &mut seen, &mut apps);
    }

    apps
}

pub fn source_signature(roots: &[PathBuf]) -> u64 {
    let mut sources = roots.to_vec();
    for root in roots {
        collect_signature_paths(root, 0, &mut sources);
    }
    paths_signature(sources)
}

pub fn signature_check_interval() -> Duration {
    Duration::from_secs(30)
}

pub fn max_cache_age() -> Option<Duration> {
    None
}

pub fn open(path: &Path) -> Result<(), String> {
    Command::new("open")
        .arg(path)
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(())
}

fn collect_apps(
    dir: &Path,
    depth: usize,
    seen: &mut HashSet<String>,
    apps: &mut Vec<LocalApplication>,
) {
    if depth > 2 || !dir.is_dir() {
        return;
    }

    let entries = match fs::read_dir(dir) {
        Ok(entries) => entries,
        Err(_) => return,
    };

    for entry in entries.flatten() {
        let path = entry.path();
        if is_app_bundle(&path) {
            let canonical = path.to_string_lossy().to_string();
            if seen.insert(canonical.clone()) {
                let metadata = app_metadata(&path);
                apps.push(LocalApplication {
                    name: metadata.name,
                    localized_name: metadata.localized_name,
                    path: canonical,
                    icon_path: metadata.icon_path,
                    comment: None,
                    // Filled in by `list_applications` once the scan is done, so
                    // the pinyin lookup lives in one place rather than in every
                    // scanner.
                    initials: String::new(),
                    aliases: metadata.aliases,
                });
            }
            continue;
        }

        if depth < 2 && path.is_dir() && !path_is_bundle(&path) {
            collect_apps(&path, depth + 1, seen, apps);
        }
    }
}

fn collect_signature_paths(dir: &Path, depth: usize, paths: &mut Vec<PathBuf>) {
    if depth > 2 || !dir.is_dir() {
        return;
    }
    let Ok(entries) = fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if is_app_bundle(&path) {
            paths.push(path.join("Contents/Info.plist"));
        } else if depth < 2 && path.is_dir() && !path_is_bundle(&path) {
            paths.push(path.clone());
            collect_signature_paths(&path, depth + 1, paths);
        }
    }
}

fn is_app_bundle(path: &Path) -> bool {
    path.extension()
        .and_then(|ext| ext.to_str())
        .is_some_and(|ext| ext.eq_ignore_ascii_case("app"))
}

fn path_is_bundle(path: &Path) -> bool {
    path.extension()
        .and_then(|ext| ext.to_str())
        .is_some_and(|extension| {
            matches!(
                extension.to_ascii_lowercase().as_str(),
                "app" | "bundle" | "framework" | "plugin" | "prefpane" | "appex"
            )
        })
}

/// What a scan needs out of one `.app` bundle.
struct AppMetadata {
    /// The name a Latin keyboard can type, which is what the launcher shows as
    /// the subtitle when there is a localized name above it.
    name: String,
    /// The name in the user's language, when the bundle ships one that differs.
    localized_name: Option<String>,
    icon_path: Option<String>,
    aliases: Vec<String>,
}

/// Read a bundle's names, icon and search aliases out of its `Info.plist`.
///
/// Which of the two names is which matters: the launcher titles a result with
/// the localized name and subtitles it with the other one, so `name` has to be
/// the Latin spelling wherever the bundle has one. That is not always what
/// `Info.plist` says at the top level — an application published for a Chinese
/// audience puts its Chinese name there and its English one in `en.lproj` —
/// hence the English localization is preferred over the bundle's own answer.
///
/// Applications that ship no Latin name at all (企业微信 is the usual example)
/// are still reachable through the aliases: every bundle has an identifier and
/// an executable, and both are Latin by construction.
fn app_metadata(path: &Path) -> AppMetadata {
    let fallback = path
        .file_stem()
        .and_then(|name| name.to_str())
        .unwrap_or("Application")
        .to_string();

    let info_path = path.join("Contents").join("Info.plist");
    let info = plist::Value::from_file(&info_path).ok();
    let bundle_name = info
        .as_ref()
        .and_then(|value| plist_value_first_string(value, &["CFBundleDisplayName", "CFBundleName"]))
        .unwrap_or_else(|| fallback.clone());
    let icon_name = info
        .as_ref()
        .and_then(|value| plist_value_first_string(value, &["CFBundleIconFile"]));

    let (name, localized_name) = resolve_names(
        bundle_name.clone(),
        localized_app_name(path, &ENGLISH_LOCALES),
        localized_app_name(path, &CHINESE_LOCALES),
    );

    let identifier = info
        .as_ref()
        .and_then(|value| plist_value_first_string(value, &["CFBundleIdentifier"]));
    let executable = info
        .as_ref()
        .and_then(|value| plist_value_first_string(value, &["CFBundleExecutable"]));
    let bundle_short_name = info
        .as_ref()
        .and_then(|value| plist_value_first_string(value, &["CFBundleName"]));
    let mut candidates = vec![bundle_name, fallback];
    candidates.extend(bundle_short_name);
    candidates.extend(executable);
    candidates.extend(
        identifier
            .as_deref()
            .map(super::identifier_aliases)
            .unwrap_or_default(),
    );
    let aliases = super::build_aliases(&name, localized_name.as_deref(), candidates);

    let icon_path = icon_source_with_name(path, icon_name.as_deref())
        .map(|source| source.to_string_lossy().to_string());

    AppMetadata {
        name,
        localized_name,
        icon_path,
        aliases,
    }
}

/// Localization directories read for the Latin name, in order. `Base.lproj`
/// last: it is the development language, which is English for most bundles but
/// not for all of them, so a real `en.lproj` is the better answer when present.
const ENGLISH_LOCALES: [&str; 4] = ["en.lproj", "en_US.lproj", "English.lproj", "Base.lproj"];
const CHINESE_LOCALES: [&str; 4] = [
    "zh-Hans.lproj",
    "zh_CN.lproj",
    "zh-Hant.lproj",
    "zh_TW.lproj",
];

/// Split a bundle's names into the one to show and the one to show under it.
///
/// The launcher titles a result with `localized_name` and subtitles it with
/// `name`, so the pair has to come out as (Latin, what the user sees on their
/// own desktop) whichever way round the bundle stores them:
///
/// * `Safari` with a `zh-Hans` name is the ordinary case, and the localization
///   is the title.
/// * 企业微信 ships no Chinese localization *because Chinese is what its
///   `Info.plist` says*, and its `en.lproj` carries `WeCom`. The bundle's own
///   name is the title there, and the English one the subtitle — the reverse of
///   where each was read from.
/// * An application with one name and nothing else keeps the row to itself.
///
/// A localization that turns out to be the Latin name again counts for nothing:
/// WeCom ships a `zh-Hant` file whose display name is `WeCom`, and taking it at
/// face value would leave 企业微信 titled in English on a Chinese desktop.
///
/// The two can also arrive the wrong way round — UU远程 keeps its Chinese name
/// in `Info.plist` and an English one under a Chinese locale — so which is
/// which is decided by the script each is written in, not by where it was read.
fn resolve_names(
    bundle_name: String,
    english: Option<String>,
    chinese: Option<String>,
) -> (String, Option<String>) {
    let name = english.unwrap_or_else(|| bundle_name.clone());
    let display = chinese
        .filter(|localized| *localized != name)
        .unwrap_or(bundle_name);
    let (name, display) = if has_cjk(&name) && !has_cjk(&display) {
        (display, name)
    } else {
        (name, display)
    };
    let localized_name = (display != name).then_some(display);
    (name, localized_name)
}

/// Whether a name is written in Han characters, kana or Hangul — the scripts
/// the launcher titles a row with, as against the Latin spelling it puts
/// underneath.
fn has_cjk(value: &str) -> bool {
    value.chars().any(|c| {
        matches!(c,
            '\u{2e80}'..='\u{9fff}'   // radicals, kana, CJK unified ideographs
            | '\u{ac00}'..='\u{d7af}' // Hangul syllables
            | '\u{f900}'..='\u{faff}' // compatibility ideographs
        )
    })
}

/// The bundle's display name in the first of `locales` that provides one.
///
/// Only the listed directories are read. An earlier version fell back to
/// whichever `.lproj` happened to come first on disk, which handed the launcher
/// a German or Japanese name for an application localized into neither of the
/// languages the UI speaks.
fn localized_app_name(path: &Path, locales: &[&str]) -> Option<String> {
    let resource_dir = path.join("Contents").join("Resources");

    locales.iter().find_map(|locale| {
        localized_name_from_strings(&resource_dir.join(locale).join("InfoPlist.strings"))
    })
}

fn localized_name_from_strings(path: &Path) -> Option<String> {
    plist_first_string(path, &["CFBundleDisplayName", "CFBundleName"])
        .or_else(|| localized_name_from_strings_text(path))
}

fn localized_name_from_strings_text(path: &Path) -> Option<String> {
    let bytes = fs::read(path).ok()?;
    let content = if bytes.starts_with(&[0xff, 0xfe]) {
        let units = bytes[2..]
            .chunks_exact(2)
            .map(|chunk| u16::from_le_bytes([chunk[0], chunk[1]]))
            .collect::<Vec<_>>();
        String::from_utf16(&units).ok()?
    } else if bytes.starts_with(&[0xfe, 0xff]) {
        let units = bytes[2..]
            .chunks_exact(2)
            .map(|chunk| u16::from_be_bytes([chunk[0], chunk[1]]))
            .collect::<Vec<_>>();
        String::from_utf16(&units).ok()?
    } else {
        String::from_utf8_lossy(&bytes).to_string()
    };

    plist_strings_value(&content, "CFBundleDisplayName")
        .or_else(|| plist_strings_value(&content, "CFBundleName"))
}

fn plist_first_string(path: &Path, keys: &[&str]) -> Option<String> {
    let value = plist::Value::from_file(path).ok()?;
    plist_value_first_string(&value, keys)
}

fn plist_value_first_string(value: &plist::Value, keys: &[&str]) -> Option<String> {
    let dictionary = value.as_dictionary()?;
    keys.iter().find_map(|key| {
        dictionary
            .get(key)
            .and_then(plist::Value::as_string)
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_string)
    })
}

/// The value of `key` in an old-style `.strings` file.
///
/// Both spellings of a key are read. Xcode writes `"CFBundleName" = "…";`, but
/// the format allows the quotes to be left off, and hand-written files use that
/// form — 企业微信 spells its English name `CFBundleDisplayName = "WeCom";`, so
/// a parser that insists on the quoted key concludes the bundle has no Latin
/// name at all and leaves the launcher with nothing but the Chinese one.
///
/// Each candidate occurrence has to be a whole key immediately followed by its
/// assignment, which is what keeps the key's own name inside a comment (every
/// one of these files opens with one) from being read as an entry.
fn plist_strings_value(content: &str, key: &str) -> Option<String> {
    let content = strip_strings_comments(content);
    let mut cursor = 0;

    while let Some(offset) = content[cursor..].find(key) {
        let start = cursor + offset;
        let end = start + key.len();
        cursor = end;

        let statement_start = content[..start].rfind(';').map_or(0, |index| index + 1);
        let prefix = content[statement_start..start].trim();
        if !prefix.is_empty() && prefix != "\"" {
            continue;
        }
        let preceding = content[..start].chars().next_back();
        if preceding.is_some_and(|c| c.is_alphanumeric() || c == '_') {
            continue;
        }

        let mut rest = content[end..].trim_start();
        if prefix == "\"" {
            rest = rest.strip_prefix('"')?.trim_start();
        }
        let Some(rest) = rest.strip_prefix('=') else {
            continue;
        };
        let Some(rest) = rest.trim_start().strip_prefix('"') else {
            continue;
        };
        let Some(value) = quoted_strings_value(rest) else {
            continue;
        };
        let value = value.trim();
        if !value.is_empty() {
            return Some(decode_strings_escapes(value));
        }
    }

    None
}

fn strip_strings_comments(content: &str) -> String {
    let mut output = String::with_capacity(content.len());
    let mut chars = content.chars().peekable();
    let mut quoted = false;
    let mut escaped = false;
    while let Some(character) = chars.next() {
        if quoted {
            output.push(character);
            if escaped {
                escaped = false;
            } else if character == '\\' {
                escaped = true;
            } else if character == '"' {
                quoted = false;
            }
            continue;
        }
        if character == '"' {
            quoted = true;
            output.push(character);
        } else if character == '/' && chars.peek() == Some(&'/') {
            chars.next();
            for next in chars.by_ref() {
                if next == '\n' {
                    output.push('\n');
                    break;
                }
            }
        } else if character == '/' && chars.peek() == Some(&'*') {
            chars.next();
            let mut previous = '\0';
            for next in chars.by_ref() {
                if previous == '*' && next == '/' {
                    break;
                }
                previous = next;
            }
            output.push(' ');
        } else {
            output.push(character);
        }
    }
    output
}

fn quoted_strings_value(content: &str) -> Option<String> {
    let mut output = String::new();
    let mut escaped = false;
    for character in content.chars() {
        if character == '"' && !escaped {
            return Some(output);
        }
        output.push(character);
        if escaped {
            escaped = false;
        } else if character == '\\' {
            escaped = true;
        }
    }
    None
}

fn decode_strings_escapes(value: &str) -> String {
    let mut output = String::new();
    let chars = value.chars().collect::<Vec<_>>();
    let mut index = 0;
    while index < chars.len() {
        let ch = chars[index];
        index += 1;
        if ch != '\\' {
            output.push(ch);
            continue;
        }

        let escaped = chars.get(index).copied();
        index += usize::from(escaped.is_some());
        match escaped {
            Some('"') => output.push('"'),
            Some('\\') => output.push('\\'),
            Some('n') => output.push('\n'),
            Some('r') => output.push('\r'),
            Some('t') => output.push('\t'),
            Some('U') | Some('u') => {
                let Some(code) = decode_hex_quad(&chars, index) else {
                    continue;
                };
                index += 4;
                let code = if (0xd800..=0xdbff).contains(&code)
                    && chars.get(index) == Some(&'\\')
                    && matches!(chars.get(index + 1), Some('u' | 'U'))
                {
                    if let Some(low) = decode_hex_quad(&chars, index + 2)
                        .filter(|low| (0xdc00..=0xdfff).contains(low))
                    {
                        index += 6;
                        0x1_0000 + ((code - 0xd800) << 10) + (low - 0xdc00)
                    } else {
                        code
                    }
                } else {
                    code
                };
                if let Some(decoded) = char::from_u32(code) {
                    output.push(decoded);
                }
            }
            Some(other) => output.push(other),
            None => break,
        }
    }

    output
}

fn decode_hex_quad(chars: &[char], start: usize) -> Option<u32> {
    let value = chars.get(start..start + 4)?.iter().collect::<String>();
    u32::from_str_radix(&value, 16).ok()
}

pub fn icon_path(app: &AppHandle, path: &Path, source_hint: Option<&str>) -> Option<String> {
    let target = icon_cache_path(app, path, "png")?;
    let source = source_hint
        .map(PathBuf::from)
        .filter(|source| source.is_file())
        .or_else(|| icon_source(path))?;
    if cached_icon_is_fresh(&target, &source) {
        return Some(target.to_string_lossy().to_string());
    }
    let status = Command::new("/usr/bin/sips")
        .args(["-s", "format", "png"])
        .arg(&source)
        .args(["--out"])
        .arg(&target)
        .status()
        .ok()?;

    if status.success() && target.exists() {
        mark_icon_cached(&target, &source);
        Some(target.to_string_lossy().to_string())
    } else {
        None
    }
}

fn icon_source(path: &Path) -> Option<PathBuf> {
    let info_path = path.join("Contents").join("Info.plist");
    let icon_name = plist_first_string(&info_path, &["CFBundleIconFile"]);
    icon_source_with_name(path, icon_name.as_deref())
}

fn icon_source_with_name(path: &Path, icon_name: Option<&str>) -> Option<PathBuf> {
    let resource_dir = path.join("Contents").join("Resources");
    if let Some(icon_name) = icon_name {
        let icon_file = if icon_name.ends_with(".icns") {
            icon_name.to_string()
        } else {
            format!("{icon_name}.icns")
        };
        let icon_path = resource_dir.join(icon_file);
        if icon_path.exists() {
            return Some(icon_path);
        }
    }

    fs::read_dir(resource_dir)
        .ok()?
        .flatten()
        .map(|entry| entry.path())
        .find(|path| {
            path.extension()
                .and_then(|ext| ext.to_str())
                .is_some_and(|ext| ext.eq_ignore_ascii_case("icns"))
        })
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The spelling Xcode generates.
    #[test]
    fn reads_a_quoted_key() {
        assert_eq!(
            plist_strings_value(
                "\"CFBundleDisplayName\" = \"WeCom\";",
                "CFBundleDisplayName"
            ),
            Some("WeCom".to_string()),
        );
    }

    /// The spelling a person writes, which the format allows and which several
    /// widely installed bundles use for their English name.
    #[test]
    fn reads_an_unquoted_key() {
        assert_eq!(
            plist_strings_value("CFBundleDisplayName = \"WeCom\";\n", "CFBundleDisplayName"),
            Some("WeCom".to_string()),
        );
    }

    /// Every one of these files opens with a comment naming the file, and some
    /// of them name the keys in it. A mention is not an entry.
    #[test]
    fn ignores_the_key_inside_a_comment() {
        let content = "/* CFBundleDisplayName = \"Wrong\"; */\nCFBundleDisplayName = \"WeCom\";";
        assert_eq!(
            plist_strings_value(content, "CFBundleDisplayName"),
            Some("WeCom".to_string()),
        );
    }

    /// A key that only appears as the tail of a longer one is not that key.
    #[test]
    fn does_not_match_a_longer_key() {
        assert_eq!(
            plist_strings_value("NSCFBundleName = \"Wrong\";", "CFBundleName"),
            None,
        );
    }

    /// `.strings` files escape non-ASCII characters, so the escapes have to be
    /// decoded before the name is shown or searched.
    #[test]
    fn decodes_escapes_in_the_value() {
        assert_eq!(
            plist_strings_value(r#"CFBundleName = "Fran\U00e7ais";"#, "CFBundleName"),
            Some("Français".to_string()),
        );
    }

    #[test]
    fn keeps_escaped_quotes_and_utf16_surrogate_pairs() {
        assert_eq!(
            plist_strings_value(
                r#"CFBundleName = "Say \"Hi\" \uD83D\uDE80";"#,
                "CFBundleName"
            ),
            Some("Say \"Hi\" 🚀".to_string()),
        );
    }

    #[test]
    fn dotted_directories_are_not_automatically_bundles() {
        assert!(!path_is_bundle(Path::new("Company.Tools")));
        assert!(path_is_bundle(Path::new("Widget.framework")));
    }

    /// The ordinary case: an English bundle with a Chinese localization.
    #[test]
    fn titles_a_localized_bundle_with_its_localization() {
        assert_eq!(
            resolve_names(
                "Safari".into(),
                Some("Safari".into()),
                Some("Safari浏览器".into())
            ),
            ("Safari".to_string(), Some("Safari浏览器".to_string())),
        );
    }

    /// A bundle published for a Chinese audience puts the Chinese name in its
    /// `Info.plist` and the English one in `en.lproj` — the other way round, and
    /// the launcher has to read it as such rather than titling the row `WeCom`.
    #[test]
    fn titles_a_chinese_bundle_with_its_own_name() {
        assert_eq!(
            resolve_names("企业微信".into(), Some("WeCom".into()), None),
            ("WeCom".to_string(), Some("企业微信".to_string())),
        );
    }

    /// The same bundle in the shape it actually ships: a `zh-Hant` file that
    /// repeats the English name. It localizes nothing, so the bundle's own name
    /// is still the one to show.
    #[test]
    fn ignores_a_localization_that_repeats_the_latin_name() {
        assert_eq!(
            resolve_names(
                "企业微信".into(),
                Some("WeCom".into()),
                Some("WeCom".into())
            ),
            ("WeCom".to_string(), Some("企业微信".to_string())),
        );
    }

    /// The pair can arrive inverted — a Chinese `Info.plist` name with an
    /// English one filed under a Chinese locale. The row is still titled in
    /// Chinese, because that is the name the user knows the application by.
    #[test]
    fn puts_the_chinese_name_on_top_whichever_side_it_came_from() {
        assert_eq!(
            resolve_names("UU远程".into(), None, Some("UURemote".into())),
            ("UURemote".to_string(), Some("UU远程".to_string())),
        );
    }

    /// One name and nothing else: there is no second line to show.
    #[test]
    fn leaves_a_single_name_alone() {
        assert_eq!(
            resolve_names("小米互联服务".into(), None, None),
            ("小米互联服务".to_string(), None),
        );
        assert_eq!(
            resolve_names("Code".into(), Some("Code".into()), None),
            ("Code".to_string(), None),
        );
    }
}
