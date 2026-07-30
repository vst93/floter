//! Local application discovery.
//!
//! The Tauri commands, the in-memory cache and the icon cache live here;
//! everything that depends on *how* a platform stores its applications
//! (`.app` bundles, `.desktop` entries, Start Menu shortcuts) is delegated to
//! the `platform` module picked at compile time.

use serde::Serialize;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};
use tauri::{AppHandle, Manager, State};

#[cfg(target_os = "macos")]
#[path = "macos.rs"]
mod platform;

#[cfg(target_os = "linux")]
#[path = "linux.rs"]
mod platform;

#[cfg(target_os = "windows")]
#[path = "windows.rs"]
mod platform;

/// Anything else (BSDs, ...) still builds: the launcher simply lists no apps
/// and falls back to running the query as a shell command.
#[cfg(not(any(target_os = "macos", target_os = "linux", target_os = "windows")))]
mod platform {
    use super::LocalApplication;
    use std::path::{Path, PathBuf};
    use tauri::AppHandle;

    pub fn roots() -> Vec<PathBuf> {
        Vec::new()
    }

    pub fn scan(_roots: &[PathBuf]) -> Vec<LocalApplication> {
        Vec::new()
    }

    pub fn icon_path(_app: &AppHandle, _path: &Path) -> Option<String> {
        None
    }

    pub fn open(_path: &Path) -> Result<(), String> {
        Err("Launching applications is not supported on this platform".to_string())
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalApplication {
    pub name: String,
    pub localized_name: Option<String>,
    pub path: String,
    pub icon_path: Option<String>,
    /// One-line description, when the platform ships one (Linux `.desktop`
    /// entries carry `Comment=`); used as the launcher subtitle.
    pub comment: Option<String>,
    /// Search key for a Latin keyboard: every letter and digit of the name with
    /// the separators removed, and each CJK character replaced by the first
    /// letter of its pinyin. Filled in by [`list_applications`], not by the
    /// platform scanners — see [`compute_initials`].
    pub initials: String,
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

/// Whether the cached application list still matches what is on disk.
#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ApplicationsStatus {
    pub up_to_date: bool,
    pub count: usize,
}

/// Ask whether a rescan would find anything new, without performing one.
///
/// This is what the frontend calls on every summon. It only stats the
/// application directories — no directory is walked and no `.desktop` file or
/// `Info.plist` is parsed — so it stays well under a millisecond and can run on
/// a path where the panel is already being shown. An empty cache counts as out
/// of date, so the very first call after startup still asks for a scan.
#[tauri::command]
pub fn check_applications(state: State<'_, ApplicationState>) -> Result<ApplicationsStatus, String> {
    let roots = platform::roots();
    let signature = roots_signature(&roots);
    let cache = state.cache.lock().map_err(|e| e.to_string())?;
    Ok(ApplicationsStatus {
        up_to_date: !cache.apps.is_empty() && cache.signature == signature,
        count: cache.apps.len(),
    })
}

/// The application list, from the cache when it is still current.
///
/// `async` for one reason: a cold scan walks every application directory and
/// reads a metadata file per entry, which is far too much to do on the thread
/// that also drives the window. Tauri polls async commands on its own runtime,
/// and the walk itself is handed to [`tauri::async_runtime::spawn_blocking`] so
/// it never occupies an async worker either.
///
/// The lock is taken twice — once to test the cache, once to fill it — rather
/// than being held across the scan: a guard living across an `.await` would make
/// this future `!Send`, and it would also block [`check_applications`] for the
/// whole duration of a scan.
#[tauri::command]
pub async fn list_applications(
    state: State<'_, ApplicationState>,
    force_refresh: Option<bool>,
) -> Result<Vec<LocalApplication>, String> {
    let force_refresh = force_refresh.unwrap_or(false);

    // Read before the scan rather than after it: this is the state of the
    // directories the scan below is about to observe. Storing a *later* reading
    // would let an application installed while the scan was running pass for one
    // the scan already saw, and it would then stay missing until the next
    // unrelated change to the same directory.
    let signature = roots_signature(&platform::roots());

    {
        let cache = state.cache.lock().map_err(|e| e.to_string())?;
        if !force_refresh && !cache.apps.is_empty() && cache.signature == signature {
            return Ok(cache.apps.clone());
        }
    }

    let apps = tauri::async_runtime::spawn_blocking(|| {
        let roots = platform::roots();
        let mut apps = platform::scan(&roots);
        for app in &mut apps {
            app.initials = compute_initials(&app.name, &app.localized_name);
        }
        apps.sort_by(|a, b| a.name.to_lowercase().cmp(&b.name.to_lowercase()));
        apps
    })
    .await
    .map_err(|e| e.to_string())?;

    let mut cache = state.cache.lock().map_err(|e| e.to_string())?;
    cache.signature = signature;
    cache.apps = apps.clone();
    Ok(apps)
}

/// The launcher's Latin search key for an application name.
///
/// Two things a plain fuzzy match over the display name cannot do:
///
/// * A Chinese name has no letters at all, so on a Latin keyboard it can only be
///   reached through pinyin. Only the *first letter* of each character is kept —
///   "网易云音乐" becomes `wyyyy`, which is how it is typed into every other
///   launcher, and the full spelling would drown the initials in a subsequence
///   match.
/// * Separators are dropped rather than preserved, so a query typed as one word
///   still matches across them: `visualstudio` reaches "Visual Studio Code".
///
/// The original and the localized name are folded into one key. They are
/// searched together because either can be the one the user thinks in, and a
/// single key means one score instead of two.
///
/// Computed once per scan rather than per keystroke — the frontend receives the
/// finished string, and the pinyin table lookup never runs inside a search.
fn compute_initials(name: &str, localized_name: &Option<String>) -> String {
    use pinyin::ToPinyin;

    let combined = match localized_name {
        Some(localized) if !localized.is_empty() => format!("{name} {localized}"),
        _ => name.to_string(),
    };

    let mut initials = String::with_capacity(combined.len());
    for c in combined.chars() {
        if c.is_ascii_alphanumeric() {
            initials.push(c.to_ascii_lowercase());
        } else if let Some(pinyin) = c.to_pinyin() {
            // `first_letter` is ASCII for every entry in the table, so the
            // lowercase mapping below is a byte operation.
            initials.extend(pinyin.first_letter().chars().map(|c| c.to_ascii_lowercase()));
        }
        // Anything else — whitespace, punctuation, a CJK character with no
        // pinyin — is a separator and contributes nothing.
    }
    initials
}

#[tauri::command]
pub fn application_icon(app: AppHandle, path: String) -> Result<Option<String>, String> {
    Ok(platform::icon_path(&app, Path::new(&path)))
}

#[tauri::command]
pub fn open_application(path: String) -> Result<(), String> {
    let path = PathBuf::from(path);
    if !path.exists() {
        return Err("Application not found".to_string());
    }
    platform::open(&path)
}

fn roots_signature(roots: &[PathBuf]) -> u64 {
    roots
        .iter()
        .map(|root| dir_signature(root))
        .fold(0_u64, |signature, value| {
            signature.wrapping_mul(31).wrapping_add(value)
        })
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
    time.duration_since(UNIX_EPOCH)
        .ok()
        .map(|duration| duration.as_secs())
}

/// Path of the cached icon for `key`, created lazily under the app cache dir.
///
/// Icons are copied/converted into this directory because the asset protocol is
/// scoped to `$APPCACHE/app-icons/**`; the extension is preserved so the
/// webview receives the right mime type (a `.svg` served as `.png` will not
/// render).
#[allow(dead_code)]
fn icon_cache_path(app: &AppHandle, key: &Path, extension: &str) -> Option<PathBuf> {
    let dir = app.path().app_cache_dir().ok()?.join("app-icons");
    fs::create_dir_all(&dir).ok()?;
    let name = sanitize_filename(&key.to_string_lossy());
    Some(dir.join(format!("{name}.{extension}")))
}

#[allow(dead_code)]
fn sanitize_filename(value: &str) -> String {
    value
        .chars()
        .map(|c| if c.is_ascii_alphanumeric() { c } else { '_' })
        .collect()
}
