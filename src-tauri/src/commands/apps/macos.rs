//! macOS application discovery: `.app` bundles read through `Info.plist`.

use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::Duration;
use tauri::AppHandle;

use super::{icon_cache_path, paths_signature, LocalApplication};

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
                let (name, localized_name, icon_path) = app_metadata(&path);
                apps.push(LocalApplication {
                    name,
                    localized_name,
                    path: canonical,
                    icon_path,
                    comment: None,
                    // Filled in by `list_applications` once the scan is done, so
                    // the pinyin lookup lives in one place rather than in every
                    // scanner.
                    initials: String::new(),
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
    path.extension().and_then(|ext| ext.to_str()).is_some()
}

fn app_metadata(path: &Path) -> (String, Option<String>, Option<String>) {
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
    let localized_name = localized_app_name(path).filter(|name| name != &bundle_name);
    let icon_path = icon_source_with_name(path, icon_name.as_deref())
        .map(|source| source.to_string_lossy().to_string());

    (bundle_name, localized_name, icon_path)
}

fn localized_app_name(path: &Path) -> Option<String> {
    let resource_dir = path.join("Contents").join("Resources");
    let preferred = [
        "zh-Hans.lproj",
        "zh_CN.lproj",
        "zh-Hant.lproj",
        "zh_TW.lproj",
        "Base.lproj",
    ];

    for locale in preferred {
        let value =
            localized_name_from_strings(&resource_dir.join(locale).join("InfoPlist.strings"));
        if value.is_some() {
            return value;
        }
    }

    let entries = fs::read_dir(resource_dir).ok()?;
    for entry in entries.flatten() {
        let locale_dir = entry.path();
        if locale_dir.extension().and_then(|ext| ext.to_str()) != Some("lproj") {
            continue;
        }
        let value = localized_name_from_strings(&locale_dir.join("InfoPlist.strings"));
        if value.is_some() {
            return value;
        }
    }

    None
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

fn plist_strings_value(content: &str, key: &str) -> Option<String> {
    let key_marker = format!("\"{key}\"");
    let after_key = content.split_once(&key_marker)?.1;
    let after_equals = after_key.split_once('=')?.1;
    let after_quote = after_equals.split_once('"')?.1;
    let value = after_quote.split_once('"')?.0.trim();
    if value.is_empty() {
        None
    } else {
        Some(decode_strings_escapes(value))
    }
}

fn decode_strings_escapes(value: &str) -> String {
    let mut output = String::new();
    let mut chars = value.chars().peekable();

    while let Some(ch) = chars.next() {
        if ch != '\\' {
            output.push(ch);
            continue;
        }

        match chars.next() {
            Some('"') => output.push('"'),
            Some('\\') => output.push('\\'),
            Some('n') => output.push('\n'),
            Some('r') => output.push('\r'),
            Some('t') => output.push('\t'),
            Some('U') | Some('u') => {
                let hex = chars.by_ref().take(4).collect::<String>();
                if let Ok(code) = u32::from_str_radix(&hex, 16) {
                    if let Some(decoded) = char::from_u32(code) {
                        output.push(decoded);
                    }
                }
            }
            Some(other) => output.push(other),
            None => break,
        }
    }

    output
}

pub fn icon_path(
    app: &AppHandle,
    path: &Path,
    source_hint: Option<&str>,
) -> Option<String> {
    let target = icon_cache_path(app, path, "png")?;
    if target.exists() {
        return Some(target.to_string_lossy().to_string());
    }

    let source = source_hint
        .map(PathBuf::from)
        .filter(|source| source.is_file())
        .or_else(|| icon_source(path))?;
    let status = Command::new("/usr/bin/sips")
        .args(["-s", "format", "png"])
        .arg(&source)
        .args(["--out"])
        .arg(&target)
        .status()
        .ok()?;

    if status.success() && target.exists() {
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
