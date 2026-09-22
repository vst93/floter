//! Chromium-family browser discovery.
//!
//! The launcher has to answer one question without the user configuring
//! anything: *which browsers are installed on this machine, and which profiles
//! do they have?* Chromium's on-disk layout answers it — every Chromium-family
//! browser keeps a `User Data` (Windows) / base directory that holds one folder
//! per profile (`Default`, `Profile 1`, …), each with its own `Bookmarks`,
//! `History` and `Preferences`.
//!
//! Discovery is a directory listing, never a scan: only the base directory and
//! the `Local State` file are touched, and the result is cached until either
//! one's mtime changes. Opening the launcher therefore costs nothing after the
//! first call, which is what keeps this plugin from slowing the launcher down.

use serde::Serialize;
use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};
use std::path::{Path, PathBuf};
use std::sync::Mutex;

/// One browser/profile pair as the frontend sees it.
///
/// `profile_key` is the handle every other command takes: `"<browser_id>/<profile_dir_name>"`.
/// It is opaque to the frontend — it only ever passes back what discovery gave
/// it — so the internal path layout never leaks into the UI.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct BrowserProfileInfo {
    pub browser_id: String,
    pub browser_name: String,
    pub profile_key: String,
    pub profile_dir_name: String,
    /// Display name from `Local State` (`profile.info_cache`), falling back to
    /// the directory name when the browser never wrote one.
    pub profile_name: String,
    pub base_dir: String,
    pub has_bookmarks: bool,
    pub has_history: bool,
}

/// A discovered Chromium-family base directory.
#[derive(Debug, Clone)]
pub struct BrowserCandidate {
    pub browser_id: &'static str,
    pub browser_name: &'static str,
    pub base_dir: PathBuf,
}

/// The `(browser_id, browser_name, base_dir)` table for the host platform.
///
/// `home` and `local_appdata` are passed in rather than read here so tests can
/// point them at a fixture tree. Each platform's table is its own `#[cfg]`
/// branch; the macOS one is factored into [`macos_base_dirs`] so its *content*
/// can be asserted from any host — a data bug (a wrong directory) cannot be
/// caught by a branch the test host compiles out.
#[cfg(target_os = "macos")]
pub fn platform_base_dirs(
    home: &Path,
    _local_appdata: Option<&Path>,
) -> Vec<(&'static str, &'static str, PathBuf)> {
    macos_base_dirs(home)
}

/// The macOS table.
///
/// Compiled for macOS *and* for the test build (`cfg(test)`), so the table can
/// be asserted from any host: a macOS-only `#[cfg]` would make its content
/// untestable on the CI/dev machine that runs the suite, which is how the
/// `Microsoft/Edge` two-level path below shipped wrong in the first place.
///
/// Path layout differs per browser and the differences are load-bearing:
///
/// * Chrome nests under a vendor folder (`Google/Chrome`).
/// * Edge does **not**: its data directory is a single `Microsoft Edge` folder
///   directly under `Application Support` — the space is part of the name.
///   The old `Microsoft/Edge` guess listed nothing, so discovery returned no
///   Edge profiles and the launcher reported "no supported browser".
/// * Brave nests under `BraveSoftware/Brave-Browser`; Chromium is a single
///   top-level folder.
/// * Every Edge channel has its own single top-level folder, suffix and all
///   (`Microsoft Edge Beta`, `Microsoft Edge Dev`, `Microsoft Edge Canary`).
#[cfg(any(target_os = "macos", test))]
fn macos_base_dirs(home: &Path) -> Vec<(&'static str, &'static str, PathBuf)> {
    let support = home.join("Library").join("Application Support");
    vec![
        (
            "chrome",
            "Google Chrome",
            support.join("Google").join("Chrome"),
        ),
        ("edge", "Microsoft Edge", support.join("Microsoft Edge")),
        (
            "edge-beta",
            "Microsoft Edge Beta",
            support.join("Microsoft Edge Beta"),
        ),
        (
            "edge-dev",
            "Microsoft Edge Dev",
            support.join("Microsoft Edge Dev"),
        ),
        (
            "edge-canary",
            "Microsoft Edge Canary",
            support.join("Microsoft Edge Canary"),
        ),
        (
            "brave",
            "Brave",
            support.join("BraveSoftware").join("Brave-Browser"),
        ),
        ("chromium", "Chromium", support.join("Chromium")),
    ]
}

#[cfg(target_os = "windows")]
pub fn platform_base_dirs(
    _home: &Path,
    local_appdata: Option<&Path>,
) -> Vec<(&'static str, &'static str, PathBuf)> {
    let Some(local) = local_appdata else {
        return Vec::new();
    };
    vec![
        (
            "chrome",
            "Google Chrome",
            local.join("Google").join("Chrome").join("User Data"),
        ),
        (
            "edge",
            "Microsoft Edge",
            local.join("Microsoft").join("Edge").join("User Data"),
        ),
        (
            "brave",
            "Brave",
            local
                .join("BraveSoftware")
                .join("Brave-Browser")
                .join("User Data"),
        ),
        (
            "chromium",
            "Chromium",
            local.join("Chromium").join("User Data"),
        ),
    ]
}

#[cfg(target_os = "linux")]
pub fn platform_base_dirs(
    home: &Path,
    _local_appdata: Option<&Path>,
) -> Vec<(&'static str, &'static str, PathBuf)> {
    let config = home.join(".config");
    vec![
        ("chrome", "Google Chrome", config.join("google-chrome")),
        ("edge", "Microsoft Edge", config.join("microsoft-edge")),
        (
            "brave",
            "Brave",
            config.join("BraveSoftware").join("Brave-Browser"),
        ),
        ("chromium", "Chromium", config.join("chromium")),
    ]
}

#[cfg(not(any(target_os = "macos", target_os = "windows", target_os = "linux")))]
pub fn platform_base_dirs(
    _home: &Path,
    _local_appdata: Option<&Path>,
) -> Vec<(&'static str, &'static str, PathBuf)> {
    Vec::new()
}

/// The host's candidates, with `HOME`/`LOCALAPPDATA` resolved.
pub fn candidate_browsers() -> Vec<BrowserCandidate> {
    let home = dirs::home_dir().unwrap_or_default();
    let local = std::env::var_os("LOCALAPPDATA").map(PathBuf::from);
    platform_base_dirs(&home, local.as_deref())
        .into_iter()
        .map(|(browser_id, browser_name, base_dir)| BrowserCandidate {
            browser_id,
            browser_name,
            base_dir,
        })
        .collect()
}

/// Whether a directory inside a base dir looks like a Chromium profile.
///
/// A profile always has at least one of `Preferences`, `Bookmarks` or
/// `History`; requiring one of them keeps stray folders (`Crashpad`, `Shader
/// Cache`, …) out of the list. `Default` is accepted by name as well, because a
/// profile that has never been opened has none of the three files yet.
pub fn is_profile_dir(path: &Path) -> bool {
    let Some(name) = path.file_name().and_then(|name| name.to_str()) else {
        return false;
    };
    if name == "Default" || name == "Guest Profile" || name.starts_with("Profile ") {
        return true;
    }
    ["Preferences", "Bookmarks", "History"]
        .iter()
        .any(|file| path.join(file).is_file())
}

/// The profile's display name from `Local State`'s `profile.info_cache`.
///
/// `Local State` is a plain JSON file the browser keeps up to date; reading it
/// is the only way to learn the user's chosen profile label. A missing or
/// unreadable file is not an error — the caller falls back to the directory
/// name.
pub fn local_state_profile_name(base_dir: &Path, profile_dir_name: &str) -> Option<String> {
    let bytes = std::fs::read(base_dir.join("Local State")).ok()?;
    let value: serde_json::Value = serde_json::from_slice(&bytes).ok()?;
    value
        .get("profile")?
        .get("info_cache")?
        .get(profile_dir_name)?
        .get("name")?
        .as_str()
        .map(str::to_string)
        .filter(|name| !name.trim().is_empty())
}

/// Rank a profile directory name: `Default` first, then `Profile N` in numeric
/// order, then everything else alphabetically.
fn profile_rank(name: &str) -> (u8, u32, String) {
    if name == "Default" {
        return (0, 0, String::new());
    }
    if let Some(rest) = name.strip_prefix("Profile ") {
        if let Ok(number) = rest.trim().parse::<u32>() {
            return (1, number, String::new());
        }
    }
    (2, 0, name.to_string())
}

/// Enumerate the profiles of one base directory.
pub fn profiles_in_base_dir(
    browser_id: &str,
    browser_name: &str,
    base_dir: &Path,
) -> Vec<BrowserProfileInfo> {
    let Ok(entries) = std::fs::read_dir(base_dir) else {
        return Vec::new();
    };
    let mut names: Vec<String> = entries
        .flatten()
        .filter(|entry| entry.file_type().map(|kind| kind.is_dir()).unwrap_or(false))
        .filter_map(|entry| entry.file_name().to_str().map(str::to_string))
        .filter(|name| is_profile_dir(&base_dir.join(name)))
        .collect();
    names.sort_by_key(|name| profile_rank(name));

    names
        .into_iter()
        .map(|profile_dir_name| {
            let profile_path = base_dir.join(&profile_dir_name);
            let has_bookmarks = profile_path.join("Bookmarks").is_file();
            let has_history = profile_path.join("History").is_file();
            let profile_name = local_state_profile_name(base_dir, &profile_dir_name)
                .unwrap_or_else(|| profile_dir_name.clone());
            BrowserProfileInfo {
                browser_id: browser_id.to_string(),
                browser_name: browser_name.to_string(),
                profile_key: format!("{browser_id}/{profile_dir_name}"),
                profile_dir_name,
                profile_name,
                base_dir: base_dir.to_string_lossy().into_owned(),
                has_bookmarks,
                has_history,
            }
        })
        .collect()
}

/// Every profile of every candidate browser, plus the optional custom base dir.
///
/// `custom_base_dir` is the user's escape hatch for a browser this table does
/// not know (or a non-standard install location). It is discovered exactly like
/// a built-in one, under the synthetic `custom` browser id.
pub fn discover_profiles_with(custom_base_dir: Option<&Path>) -> Vec<BrowserProfileInfo> {
    let mut profiles = Vec::new();
    for candidate in candidate_browsers() {
        profiles.extend(profiles_in_base_dir(
            candidate.browser_id,
            candidate.browser_name,
            &candidate.base_dir,
        ));
    }
    if let Some(custom) = custom_base_dir {
        profiles.extend(custom_profiles(custom));
    }
    profiles
}

/// Discover the user's custom base dir.
///
/// The path may be either the browser's base directory (holding `Default`,
/// `Profile 1`, …) or one profile inside it — a user who copied the path out
/// of the browser's *About* page gets the second shape. Trying the base-dir
/// reading first handles both without a mode switch.
fn custom_profiles(custom: &Path) -> Vec<BrowserProfileInfo> {
    const ID: &str = "custom";
    const NAME: &str = "Custom browser";

    let as_base = profiles_in_base_dir(ID, NAME, custom);
    if !as_base.is_empty() {
        return as_base;
    }
    let Some(parent) = custom.parent() else {
        return Vec::new();
    };
    let Some(dir_name) = custom.file_name().and_then(|name| name.to_str()) else {
        return Vec::new();
    };
    profiles_in_base_dir(ID, NAME, parent)
        .into_iter()
        .filter(|profile| profile.profile_dir_name == dir_name)
        .collect()
}

/// A cheap fingerprint of everything discovery depends on: each base
/// directory's mtime plus its `Local State` mtime. Adding a profile changes the
/// base directory's mtime; renaming one changes `Local State`'s. Anything else
/// inside a profile is irrelevant here (the per-query readers open those
/// files directly).
fn discovery_signature(custom_base_dir: Option<&Path>) -> u64 {
    let mut hasher = DefaultHasher::new();
    let mut dirs: Vec<PathBuf> = candidate_browsers()
        .into_iter()
        .map(|candidate| candidate.base_dir)
        .collect();
    if let Some(custom) = custom_base_dir {
        dirs.push(custom.to_path_buf());
    }
    for dir in dirs {
        dir.hash(&mut hasher);
        if let Ok(metadata) = std::fs::metadata(&dir) {
            if let Ok(modified) = metadata.modified() {
                if let Ok(duration) = modified.duration_since(std::time::UNIX_EPOCH) {
                    duration.as_nanos().hash(&mut hasher);
                }
            }
        }
        if let Ok(metadata) = std::fs::metadata(dir.join("Local State")) {
            if let Ok(modified) = metadata.modified() {
                if let Ok(duration) = modified.duration_since(std::time::UNIX_EPOCH) {
                    duration.as_nanos().hash(&mut hasher);
                }
            }
        }
    }
    hasher.finish()
}

static DISCOVERY_CACHE: Mutex<Option<(u64, Vec<BrowserProfileInfo>)>> = Mutex::new(None);

/// Drop the discovery cache so the next read rescans from scratch.
///
/// The cache is signature-keyed and the signature includes the custom base dir,
/// so a *change* to that path is normally picked up on its own. This exists for
/// the case the signature cannot see: a path that did not exist when it was
/// cached, or a directory whose contents changed without its own mtime moving.
/// Saving the plugin's settings is the one moment the user is explicitly asking
/// for a rescan, so it is the one moment worth paying for one.
pub fn clear_discovery_cache() {
    if let Ok(mut cache) = DISCOVERY_CACHE.lock() {
        *cache = None;
    }
}

/// Cached [`discover_profiles_with`]: the directory listing is only redone when
/// the signature changes. The cache is process-wide and short-lived — a browser
/// installed while Floter runs is picked up on the next call without a restart.
pub fn discover_profiles_cached(custom_base_dir: Option<&Path>) -> Vec<BrowserProfileInfo> {
    let signature = discovery_signature(custom_base_dir);
    if let Ok(cache) = DISCOVERY_CACHE.lock() {
        if let Some((cached_signature, profiles)) = cache.as_ref() {
            if *cached_signature == signature {
                return profiles.clone();
            }
        }
    }
    let profiles = discover_profiles_with(custom_base_dir);
    if let Ok(mut cache) = DISCOVERY_CACHE.lock() {
        *cache = Some((signature, profiles.clone()));
    }
    profiles
}

/// Split a `profile_key` back into its `(browser_id, profile_dir_name)` parts.
///
/// A profile directory name can never contain `/` on any platform we ship, so
/// the first separator is unambiguous. Keys without a separator are rejected
/// rather than guessed at.
pub fn parse_profile_key(profile_key: &str) -> Option<(String, String)> {
    let (browser_id, profile_dir_name) = profile_key.split_once('/')?;
    if browser_id.is_empty() || profile_dir_name.is_empty() {
        return None;
    }
    Some((browser_id.to_string(), profile_dir_name.to_string()))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn write(path: &Path, contents: &str) {
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent).unwrap();
        }
        std::fs::write(path, contents).unwrap();
    }

    /// Build a fixture base directory with two profiles and a `Local State`.
    fn fixture_base_dir(root: &Path) -> PathBuf {
        let base = root.join("Chrome");
        write(&base.join("Default").join("Preferences"), "{}");
        write(&base.join("Default").join("Bookmarks"), "{}");
        write(&base.join("Default").join("History"), "");
        write(&base.join("Profile 1").join("Preferences"), "{}");
        // A non-profile folder must never be listed.
        write(&base.join("Crashpad").join("settings.dat"), "");
        write(
            &base.join("Local State"),
            r#"{"profile":{"info_cache":{"Default":{"name":"Personal"},"Profile 1":{"name":"Work"}}}}"#,
        );
        base
    }

    #[test]
    fn profiles_are_enumerated_with_default_first_and_names_from_local_state() {
        let temp = tempfile::tempdir().unwrap();
        let base = fixture_base_dir(temp.path());
        let profiles = profiles_in_base_dir("chrome", "Google Chrome", &base);

        let dir_names: Vec<&str> = profiles
            .iter()
            .map(|profile| profile.profile_dir_name.as_str())
            .collect();
        assert_eq!(dir_names, vec!["Default", "Profile 1"]);

        assert_eq!(profiles[0].profile_name, "Personal");
        assert_eq!(profiles[1].profile_name, "Work");
        assert_eq!(profiles[0].profile_key, "chrome/Default");
        assert!(profiles[0].has_bookmarks);
        assert!(profiles[0].has_history);
        assert!(!profiles[1].has_bookmarks);
        assert!(!profiles[1].has_history);
    }

    #[test]
    fn profile_name_falls_back_to_the_directory_name_without_local_state() {
        let temp = tempfile::tempdir().unwrap();
        let base = temp.path().join("Edge");
        write(&base.join("Profile 2").join("Preferences"), "{}");
        let profiles = profiles_in_base_dir("edge", "Microsoft Edge", &base);
        assert_eq!(profiles.len(), 1);
        assert_eq!(profiles[0].profile_name, "Profile 2");
    }

    #[test]
    fn a_missing_base_directory_yields_nothing() {
        let temp = tempfile::tempdir().unwrap();
        let profiles = profiles_in_base_dir("brave", "Brave", &temp.path().join("nope"));
        assert!(profiles.is_empty());
    }

    #[test]
    fn profile_rank_orders_default_then_numbered_then_others() {
        let mut names = vec!["Profile 10", "Default", "Alpha", "Profile 2"];
        names.sort_by_key(|name| profile_rank(name));
        assert_eq!(names, vec!["Default", "Profile 2", "Profile 10", "Alpha"]);
    }

    #[test]
    fn profile_key_round_trips() {
        assert_eq!(
            parse_profile_key("chrome/Profile 1"),
            Some(("chrome".to_string(), "Profile 1".to_string()))
        );
        assert_eq!(parse_profile_key("chrome"), None);
        assert_eq!(parse_profile_key("/Default"), None);
        assert_eq!(parse_profile_key("chrome/"), None);
    }

    #[test]
    fn custom_base_dir_is_discovered_under_the_custom_id() {
        let temp = tempfile::tempdir().unwrap();
        let base = temp.path().join("Weird Browser");
        write(&base.join("Default").join("Preferences"), "{}");
        let profiles = discover_profiles_with(Some(&base));
        let custom: Vec<_> = profiles
            .iter()
            .filter(|profile| profile.browser_id == "custom")
            .collect();
        assert_eq!(custom.len(), 1);
        assert_eq!(custom[0].profile_key, "custom/Default");
    }

    #[test]
    fn custom_dir_pointing_at_a_single_profile_is_recognized() {
        let temp = tempfile::tempdir().unwrap();
        let base = temp.path().join("Weird Browser");
        let profile = base.join("Profile 7");
        write(&profile.join("Preferences"), "{}");
        let profiles = discover_profiles_with(Some(&profile));
        let custom: Vec<_> = profiles
            .iter()
            .filter(|profile| profile.browser_id == "custom")
            .collect();
        assert_eq!(custom.len(), 1);
        assert_eq!(custom[0].profile_dir_name, "Profile 7");
    }

    #[test]
    fn the_signature_changes_when_a_profile_is_added() {
        let temp = tempfile::tempdir().unwrap();
        let base = fixture_base_dir(temp.path());
        let before = discovery_signature(Some(&base));
        // Same tree, same signature: the cache is not invalidated by a no-op.
        assert_eq!(before, discovery_signature(Some(&base)));
        std::thread::sleep(std::time::Duration::from_millis(10));
        write(&base.join("Profile 3").join("Preferences"), "{}");
        assert_ne!(before, discovery_signature(Some(&base)));
    }

    /// The host platform's table points at the paths the real browsers use.
    /// Only the compiled-for platform's branch runs; the other two are compiled
    /// out, which is why this is one test rather than three.
    #[test]
    fn the_platform_table_names_the_shipped_browsers() {
        let home = Path::new("/home/example");
        let local = Path::new("/users/example/AppData/Local");
        let dirs = platform_base_dirs(home, Some(local));
        let ids: Vec<&str> = dirs.iter().map(|(id, _, _)| *id).collect();
        assert!(ids.contains(&"chrome"));
        assert!(ids.contains(&"edge"));
        assert!(ids.contains(&"brave"));

        #[cfg(target_os = "macos")]
        {
            assert_eq!(
                dirs[0].2,
                Path::new("/home/example/Library/Application Support/Google/Chrome")
            );
        }
        #[cfg(target_os = "windows")]
        {
            assert_eq!(
                dirs[0].2,
                Path::new("/users/example/AppData/Local/Google/Chrome/User Data")
            );
        }
        #[cfg(target_os = "linux")]
        {
            assert_eq!(dirs[0].2, Path::new("/home/example/.config/google-chrome"));
        }
    }

    #[cfg(target_os = "windows")]
    #[test]
    fn windows_without_local_appdata_lists_nothing() {
        assert!(platform_base_dirs(Path::new("/home/example"), None).is_empty());
    }

    /// R26-C · the macOS table's *content*, asserted from any host.
    ///
    /// `macos_base_dirs` is compiled under `cfg(any(target_os = "macos",
    /// test))`, so a Linux CI run exercises the same literal table a macOS
    /// build ships — the regression this guards (`Microsoft/Edge` instead of
    /// the real single `Microsoft Edge` directory) is a data bug, and the data
    /// has to be reachable by the test.
    #[test]
    fn the_macos_table_points_at_the_real_application_support_directories() {
        let home = Path::new("/Users/example");
        let support = home.join("Library").join("Application Support");
        let dirs = macos_base_dirs(home);
        let path_of = |id: &str| {
            dirs.iter()
                .find(|(browser_id, _, _)| *browser_id == id)
                .map(|(_, _, path)| path.clone())
                .unwrap_or_else(|| panic!("the macOS table must know {id}"))
        };

        assert_eq!(path_of("chrome"), support.join("Google").join("Chrome"));
        assert_eq!(
            path_of("brave"),
            support.join("BraveSoftware").join("Brave-Browser")
        );
        assert_eq!(path_of("chromium"), support.join("Chromium"));

        // The bug: Edge's macOS data directory is one top-level folder whose
        // name contains a space, NOT a `Microsoft` vendor folder holding an
        // `Edge` child. The real path is asserted exactly.
        assert_eq!(path_of("edge"), support.join("Microsoft Edge"));
        assert_eq!(path_of("edge-beta"), support.join("Microsoft Edge Beta"));
        assert_eq!(path_of("edge-dev"), support.join("Microsoft Edge Dev"));
        assert_eq!(
            path_of("edge-canary"),
            support.join("Microsoft Edge Canary")
        );

        // No macOS path may split the vendor into two components: a
        // `Microsoft/Edge` join is the exact shape that read_dir'd empty.
        for (browser_id, _, path) in &dirs {
            let text = path.to_string_lossy();
            assert!(
                !text.contains("Microsoft/Edge"),
                "{browser_id} must not use a Microsoft/Edge two-level directory: {text}"
            );
        }
        // Every entry is unique and rooted under Application Support.
        let mut ids: Vec<&str> = dirs.iter().map(|(id, _, _)| *id).collect();
        let total = ids.len();
        ids.sort_unstable();
        ids.dedup();
        assert_eq!(ids.len(), total, "browser ids must be unique");
        assert!(dirs.iter().all(|(_, _, path)| path.starts_with(&support)));
    }
}
