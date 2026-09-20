//! The one `PATH` the host resolves toolchains through.
//!
//! A macOS app started from Finder/Dock is handed the bare launchd baseline
//! (`/usr/bin:/bin:/usr/sbin:/sbin`). Everything the user installed with
//! Homebrew (`/opt/homebrew/bin`), rustup (`~/.cargo/bin`) or a per-user
//! installer (`~/.local/bin`) lives outside it. The terminal daemon already
//! worked around this for *shells* (`terminal::broker`), but the extension
//! runtime resolved its interpreters straight from `std::env::var_os("PATH")`
//! — so a script integration could pass the runtime check and then fail to
//! spawn, or refuse to be saved at all, depending on how Floter was launched.
//!
//! This module is the single answer. Both the availability check
//! (`find_script_interpreter`) and the execution path (`find_system_executable`
//! plus the environment a run hands its child) go through it, so "the check
//! found it" and "the run found it" cannot diverge — the same rule the
//! toolchain table already enforces for candidate names.
//!
//! Entries are only ever **appended** to whatever the process inherited:
//! existing entries keep their position (and therefore their priority), so a
//! directory the user actually has on `PATH` can never be shadowed by a
//! fallback.

use std::ffi::{OsStr, OsString};
use std::path::PathBuf;
#[cfg(target_os = "macos")]
use std::sync::OnceLock;
#[cfg(target_os = "macos")]
use std::time::Duration;

/// Conventional macOS tool directories a Finder/Dock launch never inherits:
/// the standard Homebrew prefixes for Apple Silicon and Intel, plus the
/// `path_helper` command baseline.
#[cfg(target_os = "macos")]
pub(crate) const MACOS_BASELINE_PATH_DIRS: &[&str] = &[
    "/usr/local/bin",
    "/usr/local/sbin",
    "/opt/homebrew/bin",
    "/opt/homebrew/sbin",
];

/// How long the one-time login-shell probe may take. A shell that blocks on
/// an interactive prompt (a password, a `read`) must not stall the app, so the
/// probe is killed and the static baseline is used alone.
#[cfg(target_os = "macos")]
const LOGIN_SHELL_PATH_TIMEOUT: Duration = Duration::from_secs(5);

/// The sentinel the login-shell probe prints before `$PATH`, so a shell whose
/// rc files write a banner to stdout cannot corrupt the parse: everything
/// before the last sentinel is discarded.
#[cfg(target_os = "macos")]
const LOGIN_SHELL_PATH_SENTINEL: &str = "__floter_path__";

/// The `PATH` a login shell would have, captured once per process.
///
/// The static baseline above only covers the *conventional* install prefixes.
/// A version manager (nvm, pyenv, asdf, rustup's shell hooks) puts its
/// toolchain somewhere else entirely — `~/.nvm/versions/node/v24/bin` — and
/// only the user's own shell rc knows where. `path_helper` runs inside the
/// login shell, which is exactly the chain a Finder launch never entered, so
/// asking a login shell is the only way to see what the user's terminal sees.
///
/// Runs `-ilc` (interactive **and** login) because a version manager usually
/// initializes in the interactive rc while `path_helper` is a login one; both
/// halves are needed for the same answer the user's terminal gives.
///
/// Best-effort by contract: any failure, timeout or unparseable output yields
/// an empty list, and the static baseline still applies. Memoized because this
/// spawns a process and the answer cannot change within one app session.
#[cfg(target_os = "macos")]
fn login_shell_path() -> &'static [String] {
    static CACHED: OnceLock<Vec<String>> = OnceLock::new();
    CACHED.get_or_init(|| {
        let shell = std::env::var("SHELL")
            .ok()
            .filter(|value| !value.trim().is_empty())
            .unwrap_or_else(|| "/bin/zsh".to_string());
        let mut command = std::process::Command::new(shell);
        command
            .args([
                "-ilc",
                &format!("printf '%s%s' '{LOGIN_SHELL_PATH_SENTINEL}' \"$PATH\""),
            ])
            .stdin(std::process::Stdio::null())
            .stdout(std::process::Stdio::piped())
            .stderr(std::process::Stdio::null());
        // `output()` has no timeout, so the wait is bounded by a watchdog
        // thread that kills the child; a hung rc file would otherwise pin the
        // app's first toolchain lookup forever.
        let mut child = match command.spawn() {
            Ok(child) => child,
            Err(_) => return Vec::new(),
        };
        let pid = child.id();
        let finished = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
        let watch = finished.clone();
        std::thread::spawn(move || {
            std::thread::sleep(LOGIN_SHELL_PATH_TIMEOUT);
            // The child was already reaped (and its pid may have been reused),
            // so the kill is gated on the flag rather than sent blind.
            if !watch.load(std::sync::atomic::Ordering::SeqCst) {
                // SAFETY: `kill` on a pid this process spawned and has not yet
                // reaped. A process that exited in between makes it a harmless
                // ESRCH.
                unsafe {
                    libc::kill(pid as libc::pid_t, libc::SIGKILL);
                }
            }
        });
        let output = child.wait_with_output();
        finished.store(true, std::sync::atomic::Ordering::SeqCst);
        let Ok(output) = output else {
            return Vec::new();
        };
        if !output.status.success() {
            return Vec::new();
        }
        let stdout = String::from_utf8_lossy(&output.stdout).into_owned();
        let Some((_, path)) = stdout.rsplit_once(LOGIN_SHELL_PATH_SENTINEL) else {
            return Vec::new();
        };
        path.split(':')
            .filter(|entry| !entry.is_empty())
            .map(str::to_string)
            .collect()
    })
}

/// Per-user tool directories, relative to `$HOME`. rustup installs `rustc`
/// into `~/.cargo/bin` and the XDG convention puts user binaries in
/// `~/.local/bin`; neither is on a Finder launch's `PATH` either.
#[cfg(unix)]
const HOME_BASELINE_PATH_DIRS: &[&str] = &[".cargo/bin", ".local/bin"];

/// The absolute fallback directories, in append order. Kept as its own
/// function (rather than a constant) because the `$HOME`-relative entries have
/// to be expanded, and a test must be able to observe the expansion.
pub(crate) fn baseline_path_dirs() -> Vec<PathBuf> {
    #[allow(unused_mut)]
    let mut dirs: Vec<PathBuf> = Vec::new();
    #[cfg(target_os = "macos")]
    {
        dirs.extend(MACOS_BASELINE_PATH_DIRS.iter().map(PathBuf::from));
        // The user's own shell wins over the static guesses, so its entries
        // come first; `merge_path_entries` still appends behind whatever the
        // process already inherited.
        dirs.splice(0..0, login_shell_path().iter().map(PathBuf::from));
    }
    #[cfg(unix)]
    if let Some(home) = dirs::home_dir() {
        dirs.extend(HOME_BASELINE_PATH_DIRS.iter().map(|name| home.join(name)));
    }
    dirs
}

/// Append `entries` to `current`, dropping empty values and duplicates.
///
/// Existing entries keep their position (and therefore their priority); the new
/// entries are appended after them. The result is joined with the POSIX `:`
/// separator, which is what a macOS `PATH` is — deliberately not
/// `std::env::join_paths`, whose rejection of an embedded `:` is irrelevant
/// here. Pure over its inputs so a test never has to mutate the process
/// environment.
pub(crate) fn merge_path_entries(current: Option<&OsStr>, entries: &[PathBuf]) -> OsString {
    let mut merged: Vec<String> = Vec::new();
    if let Some(current) = current {
        for entry in current.to_string_lossy().split(':') {
            if !entry.is_empty() && !merged.iter().any(|existing| existing == entry) {
                merged.push(entry.to_string());
            }
        }
    }
    for entry in entries {
        let entry = entry.to_string_lossy();
        if !entry.is_empty() && !merged.iter().any(|existing| *existing == *entry) {
            merged.push(entry.into_owned());
        }
    }
    OsString::from(merged.join(":"))
}

/// The `PATH` the host resolves toolchains through: the process's own `PATH`
/// with the conventional tool directories appended.
pub(crate) fn search_path() -> OsString {
    merge_path_entries(std::env::var_os("PATH").as_deref(), &baseline_path_dirs())
}

/// [`search_path`] split into directories, for a `PATH` scan. A process with no
/// `PATH` at all still gets the baseline rather than an empty search.
pub(crate) fn search_directories() -> Vec<PathBuf> {
    std::env::split_paths(&search_path()).collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn merge_appends_baseline_without_shadowing() {
        let merged = merge_path_entries(
            Some(OsStr::new("/usr/bin:/bin")),
            &[PathBuf::from("/opt/homebrew/bin")],
        );
        assert_eq!(merged, "/usr/bin:/bin:/opt/homebrew/bin");
    }

    #[test]
    fn merge_drops_duplicates_and_empty_entries() {
        // A Finder launch's baseline already has /usr/bin, and a corrupted
        // `PATH` can carry an empty leading entry (`:foo`). Neither may be
        // duplicated or preserved as a bare separator.
        let merged = merge_path_entries(
            Some(OsStr::new(":/usr/bin:/usr/bin")),
            &[
                PathBuf::from("/usr/bin"),
                PathBuf::from("/opt/homebrew/bin"),
                PathBuf::from(""),
            ],
        );
        assert_eq!(merged, "/usr/bin:/opt/homebrew/bin");
    }

    #[test]
    fn merge_survives_a_missing_path() {
        let merged = merge_path_entries(None, &[PathBuf::from("/opt/homebrew/bin")]);
        assert_eq!(merged, "/opt/homebrew/bin");
        assert_eq!(
            merge_path_entries(Some(OsStr::new("/bin")), &[]),
            OsString::from("/bin")
        );
    }

    #[cfg(unix)]
    #[test]
    fn the_search_path_is_a_superset_of_the_process_path() {
        // The whole point of the module: whatever the process had stays, and
        // the conventional tool dirs are reachable on top of it.
        let process = std::env::var_os("PATH").unwrap_or_default();
        let search = search_path();
        let directories = std::env::split_paths(&search).collect::<Vec<_>>();
        for entry in std::env::split_paths(&process) {
            assert!(
                directories.contains(&entry),
                "{} was dropped from the search path",
                entry.display()
            );
        }
        // Deduplication may legitimately *shrink* the list (a `PATH` with a
        // repeated entry), so the guarantee is containment, not length.
        let unique_process =
            std::env::split_paths(&process).collect::<std::collections::BTreeSet<_>>();
        assert!(directories.len() >= unique_process.len());
    }
}
