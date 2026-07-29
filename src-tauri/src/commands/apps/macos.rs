//! macOS application discovery: `.app` bundles read through `Info.plist`.

use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use tauri::AppHandle;

use super::{icon_cache_path, LocalApplication};

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
                let (name, localized_name) = app_names(&path);
                apps.push(LocalApplication {
                    name,
                    localized_name,
                    path: canonical,
                    icon_path: None,
                    comment: None,
                });
            }
            continue;
        }

        if depth < 2 && path.is_dir() && !path_is_bundle(&path) {
            collect_apps(&path, depth + 1, seen, apps);
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

fn app_names(path: &Path) -> (String, Option<String>) {
    let fallback = path
        .file_stem()
        .and_then(|name| name.to_str())
        .unwrap_or("Application")
        .to_string();

    let info_path = path.join("Contents").join("Info.plist");
    let bundle_name = plist_key(&info_path, "CFBundleDisplayName")
        .or_else(|| plist_key(&info_path, "CFBundleName"))
        .unwrap_or_else(|| fallback.clone());
    let localized_name = localized_app_name(path).filter(|name| name != &bundle_name);

    (bundle_name, localized_name)
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
        let path = entry.path().join("InfoPlist.strings");
        let value = localized_name_from_strings(&path);
        if value.is_some() {
            return value;
        }
    }

    None
}

fn localized_name_from_strings(path: &Path) -> Option<String> {
    plist_key(path, "CFBundleDisplayName")
        .or_else(|| plist_key(path, "CFBundleName"))
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

fn plist_key(path: &Path, key: &str) -> Option<String> {
    let output = Command::new("/usr/bin/plutil")
        .args(["-extract", key, "raw"])
        .arg(path)
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    let value = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if value.is_empty() || value == "(null)" {
        None
    } else {
        Some(value)
    }
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

pub fn icon_path(app: &AppHandle, path: &Path) -> Option<String> {
    let source = icon_source(path)?;
    let target = icon_cache_path(app, path, "png")?;

    if target.exists() {
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
        Some(target.to_string_lossy().to_string())
    } else {
        None
    }
}

fn icon_source(path: &Path) -> Option<PathBuf> {
    let resource_dir = path.join("Contents").join("Resources");
    let info_path = path.join("Contents").join("Info.plist");

    if let Some(icon_name) = plist_key(&info_path, "CFBundleIconFile") {
        let icon_file = if icon_name.ends_with(".icns") {
            icon_name
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
