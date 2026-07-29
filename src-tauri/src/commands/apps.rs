use serde::Serialize;
use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};
use tauri::{AppHandle, Manager, State};

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalApplication {
    pub name: String,
    pub localized_name: Option<String>,
    pub path: String,
    pub icon_path: Option<String>,
}

#[derive(Default)]
pub struct ApplicationState {
    cache: Mutex<ApplicationCache>,
}

impl ApplicationState {
    pub fn new() -> Self {
        Self::default()
    }
}

#[derive(Default)]
struct ApplicationCache {
    signature: u64,
    apps: Vec<LocalApplication>,
}

#[tauri::command]
pub fn list_applications(
    state: State<'_, ApplicationState>,
    force_refresh: Option<bool>,
) -> Result<Vec<LocalApplication>, String> {
    let roots = application_roots();
    let signature = roots_signature(&roots);
    let force_refresh = force_refresh.unwrap_or(false);

    let mut cache = state.cache.lock().map_err(|e| e.to_string())?;
    if !force_refresh && !cache.apps.is_empty() && cache.signature == signature {
        return Ok(cache.apps.clone());
    }

    let apps = scan_applications(&roots);
    cache.signature = signature;
    cache.apps = apps.clone();
    Ok(apps)
}

#[tauri::command]
pub fn application_icon(app: AppHandle, path: String) -> Result<Option<String>, String> {
    Ok(app_icon_path(&app, Path::new(&path)))
}

#[tauri::command]
pub fn open_application(path: String) -> Result<(), String> {
    if !Path::new(&path).exists() {
        return Err("Application not found".to_string());
    }

    #[cfg(target_os = "macos")]
    {
        Command::new("open")
            .arg(path)
            .spawn()
            .map_err(|e| e.to_string())?;
        return Ok(());
    }

    #[cfg(not(target_os = "macos"))]
    {
        Command::new(path).spawn().map_err(|e| e.to_string())?;
        Ok(())
    }
}

fn scan_applications(roots: &[PathBuf]) -> Vec<LocalApplication> {
    let mut apps = Vec::new();
    let mut seen = HashSet::new();

    for dir in roots {
        collect_apps(dir, 0, &mut seen, &mut apps);
    }

    apps.sort_by(|a, b| a.name.to_lowercase().cmp(&b.name.to_lowercase()));
    apps
}

fn application_roots() -> Vec<PathBuf> {
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

fn roots_signature(roots: &[PathBuf]) -> u64 {
    roots
        .iter()
        .map(|root| dir_signature(root))
        .fold(0_u64, |signature, value| signature.wrapping_mul(31).wrapping_add(value))
}

fn dir_signature(dir: &Path) -> u64 {
    let metadata = match fs::metadata(dir) {
        Ok(metadata) => metadata,
        Err(_) => return 0,
    };

    let modified = metadata
        .modified()
        .ok()
        .and_then(system_time_secs)
        .unwrap_or(0);

    let entry_count = fs::read_dir(dir)
        .map(|entries| entries.flatten().count() as u64)
        .unwrap_or(0);

    modified ^ entry_count.rotate_left(17)
}

fn system_time_secs(time: SystemTime) -> Option<u64> {
    time.duration_since(UNIX_EPOCH).ok().map(|duration| duration.as_secs())
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
    let preferred = ["zh-Hans.lproj", "zh_CN.lproj", "zh-Hant.lproj", "zh_TW.lproj", "Base.lproj"];

    for locale in preferred {
        let value = localized_name_from_strings(&resource_dir.join(locale).join("InfoPlist.strings"));
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

fn app_icon_path(app: &AppHandle, path: &Path) -> Option<String> {
    let source = app_icon_source(path)?;
    let cache_dir = app.path().app_cache_dir().ok()?.join("app-icons");
    fs::create_dir_all(&cache_dir).ok()?;
    let filename = sanitize_filename(&path.to_string_lossy());
    let target = cache_dir.join(format!("{filename}.png"));

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

fn app_icon_source(path: &Path) -> Option<PathBuf> {
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

fn sanitize_filename(value: &str) -> String {
    value
        .chars()
        .map(|c| if c.is_ascii_alphanumeric() { c } else { '_' })
        .collect()
}
