//! Local application discovery.
//!
//! The Tauri commands, the in-memory cache and the icon cache live here;
//! everything that depends on *how* a platform stores its applications
//! (`.app` bundles, `.desktop` entries, Start Menu shortcuts) is delegated to
//! the `platform` module picked at compile time.

use serde::{Deserialize, Serialize};
use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
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

    pub fn source_signature(_roots: &[PathBuf]) -> u64 {
        0
    }

    pub fn signature_check_interval() -> std::time::Duration {
        std::time::Duration::from_secs(60)
    }

    pub fn max_cache_age() -> Option<std::time::Duration> {
        None
    }

    pub fn icon_path(_app: &AppHandle, _path: &Path, _source_hint: Option<&str>) -> Option<String> {
        None
    }

    pub fn open(_path: &Path) -> Result<(), String> {
        Err("Launching applications is not supported on this platform".to_string())
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalApplication {
    pub name: String,
    pub localized_name: Option<String>,
    pub path: String,
    /// Platform-specific icon source discovered during the application scan:
    /// an `.icns` path on macOS, an `Icon=` value on Linux, or a shortcut/
    /// executable target on Windows. The frontend never loads this directly;
    /// [`application_icon`] turns it into an asset-cache path.
    pub icon_path: Option<String>,
    /// One-line description, when the platform ships one (Linux `.desktop`
    /// entries carry `Comment=`); used as the launcher subtitle.
    pub comment: Option<String>,
    /// Search key for a Latin keyboard: every letter and digit of the name with
    /// the separators removed, and each CJK character replaced by the first
    /// letter of its pinyin. Filled in by [`list_applications`], not by the
    /// platform scanners — see [`compute_initials`].
    pub initials: String,
    /// The other names the platform knows the application by: its bundle
    /// identifier and executable on macOS, the `Exec`/`Keywords`/`StartupWMClass`
    /// of a `.desktop` entry on Linux, the shortcut's target program on Windows.
    ///
    /// None of these are ever shown. They exist because an application's visible
    /// name is frequently the only one it has — "企业微信" ships no Latin name at
    /// all — while the thing behind it is still called `WXWork`, and a query has
    /// to be able to reach it from a keyboard that cannot type the name.
    ///
    /// `default` so that a cache written before this field existed still loads;
    /// the entries in it simply have no aliases until the next scan.
    #[serde(default)]
    pub aliases: Vec<String>,
}

#[derive(Default)]
pub struct ApplicationState {
    cache: Mutex<ApplicationCache>,
    /// Serializes cold/forced scans across all command entry points.
    ///
    /// The launcher normally coalesces its own requests, but catalog searches
    /// can ask for applications independently while the initial scan is still
    /// running. Without a backend guard those calls each walk every application
    /// directory and whichever finishes last wins the cache snapshot.
    scan_lock: tokio::sync::Mutex<()>,
}

impl ApplicationState {
    pub fn new() -> Self {
        Self {
            cache: Mutex::new(ApplicationCache::default()),
            scan_lock: tokio::sync::Mutex::new(()),
        }
    }
}

#[derive(Default)]
struct ApplicationCache {
    signature: u64,
    apps: Vec<LocalApplication>,
    scanned_at: u64,
    persistent_loaded: bool,
    last_signature_check: Option<Instant>,
}

#[derive(Serialize, Deserialize)]
struct PersistentApplicationCache {
    signature: u64,
    scanned_at: u64,
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
/// Calls are cheap during the platform-specific cooldown. Once it expires, the
/// relevant application entries are walked on a blocking thread and compared
/// with the cached signature. Windows also expires the cache on a timer as a
/// fallback for registry value edits that do not update the parent key.
#[tauri::command]
pub async fn check_applications(
    app: AppHandle,
    state: State<'_, ApplicationState>,
) -> Result<ApplicationsStatus, String> {
    let now = Instant::now();
    {
        let mut cache = state.cache.lock().map_err(|e| e.to_string())?;
        load_persistent_cache(&app, &mut cache);
        if cache.apps.is_empty() {
            return Ok(ApplicationsStatus {
                up_to_date: false,
                count: 0,
            });
        }
        if cache_is_expired(cache.scanned_at, platform::max_cache_age()) {
            return Ok(ApplicationsStatus {
                up_to_date: false,
                count: cache.apps.len(),
            });
        }
        if cache.last_signature_check.is_some_and(|checked| {
            now.duration_since(checked) < platform::signature_check_interval()
        }) {
            return Ok(ApplicationsStatus {
                up_to_date: true,
                count: cache.apps.len(),
            });
        }
        // Mark before starting the walk so concurrent summons coalesce.
        cache.last_signature_check = Some(now);
    }

    let roots = platform::roots();
    let signature =
        tauri::async_runtime::spawn_blocking(move || platform::source_signature(&roots))
            .await
            .map_err(|error| error.to_string())?;
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
    app: AppHandle,
    state: State<'_, ApplicationState>,
    force_refresh: Option<bool>,
) -> Result<Vec<LocalApplication>, String> {
    // Keep this guard for the whole read/scan/fill sequence. A second caller
    // arriving during a cold scan will re-check the now-populated cache after
    // waiting and return it without doing another filesystem walk.
    let _scan_guard = state.scan_lock.lock().await;
    let force_refresh = force_refresh.unwrap_or(false);
    {
        let mut cache = state.cache.lock().map_err(|e| e.to_string())?;
        load_persistent_cache(&app, &mut cache);
    }

    // Read before the scan rather than after it: this is the state of the
    // sources the scan below is about to observe. A later reading could claim
    // that an application installed mid-scan was already included.
    let roots = platform::roots();
    let signature_roots = roots.clone();
    let signature =
        tauri::async_runtime::spawn_blocking(move || platform::source_signature(&signature_roots))
            .await
            .map_err(|error| error.to_string())?;

    {
        let mut cache = state.cache.lock().map_err(|e| e.to_string())?;
        let current = !cache.apps.is_empty()
            && cache.signature == signature
            && !cache_is_expired(cache.scanned_at, platform::max_cache_age());
        cache.last_signature_check = Some(Instant::now());
        if !force_refresh && current {
            return Ok(cache.apps.clone());
        }
    }

    let apps = tauri::async_runtime::spawn_blocking(move || {
        let mut apps = platform::scan(&roots);
        for app in &mut apps {
            app.initials = compute_initials(&app.name, &app.localized_name);
        }
        apps.sort_by_cached_key(|application| application.name.to_lowercase());
        apps
    })
    .await
    .map_err(|e| e.to_string())?;

    let scanned_at = unix_now();
    let snapshot = PersistentApplicationCache {
        signature,
        scanned_at,
        apps: apps.clone(),
    };
    {
        let mut cache = state.cache.lock().map_err(|e| e.to_string())?;
        cache.signature = signature;
        cache.scanned_at = scanned_at;
        cache.last_signature_check = Some(Instant::now());
        cache.apps = apps.clone();
    }
    save_persistent_cache(&app, &snapshot);
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
            initials.extend(
                pinyin
                    .first_letter()
                    .chars()
                    .map(|c| c.to_ascii_lowercase()),
            );
        }
        // Anything else — whitespace, punctuation, a CJK character with no
        // pinyin — is a separator and contributes nothing.
    }
    initials
}

/// Reduce the raw strings a platform scanner collected to the alias set stored
/// on a [`LocalApplication`].
///
/// Dropped: anything that repeats a name the user can already see, anything
/// already in the list, and anything with no ASCII letter or digit in it —
/// being reachable from a Latin keyboard is the whole purpose of the field, so
/// a second Chinese spelling adds nothing the name and its pinyin initials do
/// not already cover.
pub(super) fn build_aliases<I, S>(
    name: &str,
    localized_name: Option<&str>,
    candidates: I,
) -> Vec<String>
where
    I: IntoIterator<Item = S>,
    S: AsRef<str>,
{
    let mut aliases: Vec<String> = Vec::new();

    for candidate in candidates {
        let candidate = candidate.as_ref().trim();
        if candidate.is_empty() || !candidate.chars().any(|c| c.is_ascii_alphanumeric()) {
            continue;
        }
        if candidate.eq_ignore_ascii_case(name)
            || localized_name.is_some_and(|localized| candidate.eq_ignore_ascii_case(localized))
            || aliases
                .iter()
                .any(|existing| existing.eq_ignore_ascii_case(candidate))
        {
            continue;
        }
        aliases.push(candidate.to_string());
    }

    aliases
}

/// The searchable halves of a reverse-DNS identifier.
///
/// `com.tencent.WeWorkMac` is worth knowing as `WeWorkMac` and as `tencent`,
/// and never as `com`: a leading `com`/`org`/`io` is shared by half the
/// machine, so indexing it would put every installed application behind the
/// letter `c`.
#[cfg(any(target_os = "macos", target_os = "linux"))]
pub(super) fn identifier_aliases(identifier: &str) -> Vec<String> {
    let segments: Vec<&str> = identifier
        .split('.')
        .map(str::trim)
        .filter(|segment| !segment.is_empty())
        .collect();

    match segments.as_slice() {
        [] => Vec::new(),
        // The vendor is only dropped when there is a domain suffix in front of
        // it to identify it by; `firefox` and `mozilla.firefox` keep everything.
        [.., vendor, application] if segments.len() > 2 => {
            vec![application.to_string(), vendor.to_string()]
        }
        [.., application] => vec![application.to_string()],
    }
}

#[tauri::command]
pub fn application_icon(
    app: AppHandle,
    state: State<'_, ApplicationState>,
    path: String,
) -> Result<Option<String>, String> {
    let source_hint = {
        let mut cache = state.cache.lock().map_err(|error| error.to_string())?;
        load_persistent_cache(&app, &mut cache);
        cache
            .apps
            .iter()
            .find(|application| application.path == path)
            .and_then(|application| application.icon_path.clone())
    };
    Ok(platform::icon_path(
        &app,
        Path::new(&path),
        source_hint.as_deref(),
    ))
}

#[tauri::command]
pub fn open_application(path: String) -> Result<(), String> {
    let path = PathBuf::from(path);
    if !path.exists() {
        return Err("Application not found".to_string());
    }
    platform::open(&path)
}

const APPLICATION_CACHE_FILE: &str = "applications-v3.json";
/// Caches from earlier schemas. They are readable — every field added since has
/// a `default` — but the entries in them were scanned without knowing about the
/// fields, so they are dropped rather than migrated, and the first summon after
/// an upgrade rescans. Removed once the new file has been written.
const LEGACY_APPLICATION_CACHE_FILES: [&str; 2] = ["applications-v1.json", "applications-v2.json"];

fn application_cache_path(app: &AppHandle) -> Option<PathBuf> {
    Some(
        app.path()
            .app_cache_dir()
            .ok()?
            .join(APPLICATION_CACHE_FILE),
    )
}

fn load_persistent_cache(app: &AppHandle, cache: &mut ApplicationCache) {
    if cache.persistent_loaded {
        return;
    }
    cache.persistent_loaded = true;

    let Some(path) = application_cache_path(app) else {
        return;
    };
    let Ok(bytes) = fs::read(path) else {
        return;
    };
    let Ok(snapshot) = serde_json::from_slice::<PersistentApplicationCache>(&bytes) else {
        return;
    };
    cache.signature = snapshot.signature;
    cache.scanned_at = snapshot.scanned_at;
    cache.apps = snapshot.apps;
}

fn save_persistent_cache(app: &AppHandle, snapshot: &PersistentApplicationCache) {
    let Some(path) = application_cache_path(app) else {
        return;
    };
    let Some(parent) = path.parent() else {
        return;
    };
    let Ok(bytes) = serde_json::to_vec(snapshot) else {
        return;
    };
    let saved = fs::create_dir_all(parent).is_ok()
        && tempfile::NamedTempFile::new_in(parent)
            .and_then(|mut temporary| {
                temporary.write_all(&bytes)?;
                temporary.flush()?;
                temporary
                    .persist(&path)
                    .map(|_| ())
                    .map_err(|error| error.error)
            })
            .is_ok();
    if saved {
        for legacy in LEGACY_APPLICATION_CACHE_FILES {
            let _ = fs::remove_file(parent.join(legacy));
        }
    }
}

fn cache_is_expired(scanned_at: u64, max_age: Option<Duration>) -> bool {
    let Some(max_age) = max_age else {
        return false;
    };
    scanned_at == 0 || unix_now().saturating_sub(scanned_at) >= max_age.as_secs()
}

fn unix_now() -> u64 {
    system_time_secs(SystemTime::now()).unwrap_or(0)
}

fn system_time_secs(time: SystemTime) -> Option<u64> {
    time.duration_since(UNIX_EPOCH)
        .ok()
        .map(|duration| duration.as_secs())
}

/// Deterministic signature for platform-selected application sources.
///
/// Platform scanners supply only paths that can change the launcher list (for
/// example `.desktop` files, not every icon below `/usr/share`). Sorting makes
/// the result stable even though `read_dir` order is unspecified.
pub(super) fn paths_signature(mut paths: Vec<PathBuf>) -> u64 {
    const FNV_OFFSET: u64 = 0xcbf2_9ce4_8422_2325;
    const FNV_PRIME: u64 = 0x0000_0100_0000_01b3;

    paths.sort_unstable();
    paths.dedup();
    let mut hash = FNV_OFFSET;
    for path in paths {
        for byte in path.to_string_lossy().as_bytes() {
            hash ^= u64::from(*byte);
            hash = hash.wrapping_mul(FNV_PRIME);
        }
        match fs::metadata(&path) {
            Ok(metadata) => {
                let modified = metadata
                    .modified()
                    .ok()
                    .and_then(|time| time.duration_since(UNIX_EPOCH).ok())
                    .map(|duration| duration.as_nanos())
                    .unwrap_or(0);
                for byte in metadata
                    .len()
                    .to_le_bytes()
                    .into_iter()
                    .chain(modified.to_le_bytes())
                {
                    hash ^= u64::from(byte);
                    hash = hash.wrapping_mul(FNV_PRIME);
                }
                hash ^= u64::from(metadata.is_dir());
                hash = hash.wrapping_mul(FNV_PRIME);
            }
            Err(_) => {
                hash ^= 0xff;
                hash = hash.wrapping_mul(FNV_PRIME);
            }
        }
    }
    hash
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
    let hash = stable_hash(key.to_string_lossy().as_bytes());
    Some(dir.join(format!("{hash:016x}.{extension}")))
}

#[allow(dead_code)]
fn cached_icon_is_fresh(target: &Path, source: &Path) -> bool {
    let Some(source_signature) = icon_source_signature(source) else {
        return false;
    };
    target.is_file()
        && fs::read_to_string(icon_signature_path(target))
            .ok()
            .as_deref()
            == Some(source_signature.as_str())
}

#[allow(dead_code)]
fn mark_icon_cached(target: &Path, source: &Path) {
    if let Some(signature) = icon_source_signature(source) {
        let _ = fs::write(icon_signature_path(target), signature);
    }
}

fn icon_signature_path(target: &Path) -> PathBuf {
    let mut path = target.as_os_str().to_os_string();
    path.push(".source");
    PathBuf::from(path)
}

fn icon_source_signature(source: &Path) -> Option<String> {
    let metadata = fs::metadata(source).ok()?;
    let modified = metadata
        .modified()
        .ok()?
        .duration_since(UNIX_EPOCH)
        .ok()?
        .as_nanos();
    Some(format!("{}:{modified}", metadata.len()))
}

fn stable_hash(bytes: &[u8]) -> u64 {
    const FNV_OFFSET: u64 = 0xcbf2_9ce4_8422_2325;
    const FNV_PRIME: u64 = 0x0000_0100_0000_01b3;
    bytes.iter().fold(FNV_OFFSET, |hash, byte| {
        (hash ^ u64::from(*byte)).wrapping_mul(FNV_PRIME)
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The Latin part of a name is kept in full rather than reduced to its
    /// initials, and the separators come out — a query typed as one word has to
    /// reach across them.
    #[test]
    fn keeps_latin_letters_and_drops_separators() {
        assert_eq!(
            compute_initials("Visual Studio Code", &None),
            "visualstudiocode"
        );
    }

    /// A Chinese name carries no letters, so each character contributes the
    /// first letter of its pinyin — the form it is typed as on a Latin keyboard.
    #[test]
    fn reduces_chinese_characters_to_pinyin_initials() {
        assert_eq!(compute_initials("网易云音乐", &None), "wyyyl");
    }

    /// Both halves of a mixed name land in the same key, in the order typed.
    #[test]
    fn folds_a_mixed_name_into_one_key() {
        assert_eq!(compute_initials("VSCode 编辑器", &None), "vscodebjq");
    }

    /// The localized name is appended to the same key instead of being scored
    /// separately: either name can be the one the user thinks in.
    #[test]
    fn appends_the_localized_name() {
        assert_eq!(
            compute_initials("Code", &Some("代码".to_string())),
            "codedm"
        );
    }

    /// An empty localized name is not a name. It must not contribute the
    /// separator that the `format!` above would otherwise put between the two.
    #[test]
    fn ignores_an_empty_localized_name() {
        assert_eq!(compute_initials("Code", &Some(String::new())), "code");
        assert_eq!(compute_initials("Code", &None), "code");
    }

    /// The point of an alias: an application whose only visible name is Chinese
    /// is still reachable by whatever the platform calls the thing behind it.
    #[test]
    fn keeps_the_latin_names_behind_a_chinese_one() {
        assert_eq!(
            build_aliases("企业微信", None, ["WXWork", "WeWorkMac", "tencent"]),
            ["WXWork", "WeWorkMac", "tencent"],
        );
    }

    /// A name the user can already see scores on its own; repeating it as an
    /// alias would only score the same match again, under a lower ceiling.
    #[test]
    fn drops_aliases_that_repeat_a_visible_name() {
        assert_eq!(
            build_aliases("Code", Some("代码"), ["code", "代码", "vscode"]),
            ["vscode"],
        );
    }

    /// Aliases exist to be typed on a Latin keyboard. One with nothing typable
    /// in it is covered by the name's own pinyin initials instead.
    #[test]
    fn drops_aliases_with_nothing_latin_in_them() {
        assert_eq!(
            build_aliases("Player", None, ["播放器", "  ", "mpv"]),
            ["mpv"],
        );
    }

    /// Duplicates within the list itself go too, whatever their case.
    #[test]
    fn keeps_one_spelling_of_each_alias() {
        assert_eq!(
            build_aliases("Firefox", None, ["mozilla", "Mozilla"]),
            ["mozilla"]
        );
    }

    /// A reverse-DNS identifier is searched by the application and its vendor.
    /// The domain suffix in front of them is not a name — it is shared by half
    /// the machine, and indexing it would put every application behind `c`.
    #[cfg(any(target_os = "macos", target_os = "linux"))]
    #[test]
    fn splits_an_identifier_into_application_and_vendor() {
        assert_eq!(
            identifier_aliases("com.tencent.WeWorkMac"),
            ["WeWorkMac", "tencent"],
        );
        assert_eq!(
            identifier_aliases("org.mozilla.firefox"),
            ["firefox", "mozilla"]
        );
    }

    /// Nothing is dropped from an identifier that has no suffix to drop.
    #[cfg(any(target_os = "macos", target_os = "linux"))]
    #[test]
    fn keeps_every_segment_of_a_short_identifier() {
        assert_eq!(identifier_aliases("google-chrome"), ["google-chrome"]);
        assert_eq!(identifier_aliases("mozilla.firefox"), ["firefox"]);
        assert!(identifier_aliases("").is_empty());
    }

    #[test]
    fn path_signature_is_stable_and_tracks_path_changes() {
        let dir = std::env::temp_dir().join(format!(
            "floter-app-signature-{}-{}",
            std::process::id(),
            unix_now()
        ));
        fs::create_dir_all(&dir).unwrap();
        let first_path = dir.join("first.desktop");
        fs::write(&first_path, b"application").unwrap();

        let expected = paths_signature(vec![dir.clone(), first_path.clone()]);
        assert_eq!(
            expected,
            paths_signature(vec![first_path.clone(), dir.clone(), first_path.clone()])
        );

        let second_path = dir.join("second.desktop");
        fs::rename(&first_path, &second_path).unwrap();
        assert_ne!(expected, paths_signature(vec![dir.clone(), second_path]));
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn cache_expiry_respects_the_platform_max_age() {
        let now = unix_now();
        assert!(!cache_is_expired(0, None));
        assert!(cache_is_expired(0, Some(Duration::from_secs(60))));
        assert!(!cache_is_expired(now, Some(Duration::from_secs(60))));
        assert!(cache_is_expired(
            now.saturating_sub(61),
            Some(Duration::from_secs(60))
        ));
    }
}
