//! Handing a URL or a filesystem path to the desktop that owns it.
//!
//! Both actions are a single spawn of the platform's own opener rather than
//! anything from a crate: "open this the way a double-click would" is exactly
//! what these binaries are for, every desktop ships one, and the answer to which
//! application should handle a link belongs to the user's settings rather than
//! to this process.
//!
//! Like [`crate::commands::system::system_power`], the child is spawned and never
//! waited on. `open` and `xdg-open` usually hand the request to a running
//! desktop and exit immediately, but when the handler has to be *started* they
//! stay alive for as long as that application runs — waiting would block the
//! webview for the rest of the session. A successful spawn is therefore all
//! either command can report; a handler that then refuses the file is invisible
//! from here.

use std::process::Command;

/// The binary that opens a URL or a path the way the desktop's own file manager
/// would.
///
/// `explorer` covers both on Windows because it calls `ShellExecute` on its
/// argument. It is used in preference to `cmd /c start`, which would re-parse
/// the string as a shell command line — `&` is a command separator to `cmd.exe`
/// even inside quotes, and a URL query string is full of them.
#[cfg(target_os = "macos")]
const OPENER: Option<&str> = Some("open");
#[cfg(target_os = "linux")]
const OPENER: Option<&str> = Some("xdg-open");
#[cfg(target_os = "windows")]
const OPENER: Option<&str> = Some("explorer");
/// Anything else (BSDs, ...) still builds; the action reports that the platform
/// has no supported way to do this.
#[cfg(not(any(target_os = "macos", target_os = "linux", target_os = "windows")))]
const OPENER: Option<&str> = None;

/// Open a `http`, `https` or `ftp` URL in the default browser.
///
/// The scheme is checked here and not only where the action bar offers it,
/// because a command is reachable from anything running in the webview. Without
/// the check this would pass `file://` — or, on macOS, any scheme an installed
/// application has registered — to the system opener, which is a far wider door
/// than "follow a link" is meant to be. The action bar only ever offers these
/// three schemes, so nothing a user can actually ask for is refused.
#[tauri::command]
pub fn open_url(url: String) -> Result<(), String> {
    if !is_web_url(&url) {
        return Err(format!("Refusing to open a non-web URL: {url}"));
    }
    spawn_opener(&url)
}

/// Whether the query carries one of the three schemes a link may be followed
/// through.
///
/// Kept apart from [`open_url`] so the decision can be exercised on its own: the
/// command spawns a browser the moment it says yes, which is the one thing a
/// test must not reach.
fn is_web_url(url: &str) -> bool {
    ["http://", "https://", "ftp://"].iter().any(|scheme| {
        // `get` rather than a slice: a URL can be cut short mid-character by the
        // scheme's length, and indexing off a char boundary would panic.
        url.get(..scheme.len())
            .is_some_and(|head| head.eq_ignore_ascii_case(scheme))
    })
}

/// Open a filesystem path in the file manager, or in whatever application owns
/// the file type.
///
/// The path is required to exist, so that a typo answers the query instead of
/// silently opening nothing: the frontend reads the error as "leave the launcher
/// up so it can be corrected".
#[tauri::command]
pub fn open_path(path: String) -> Result<(), String> {
    let home = dirs::home_dir().ok_or("Cannot find home directory")?;
    let resolved = resolve_path(&path, &home);
    if !resolved.exists() {
        return Err(format!("Path does not exist: {}", resolved.display()));
    }
    spawn_opener(&resolved)
}

/// `~` and `~/…` resolved against the home directory.
///
/// The shell would have expanded this before a command ever ran, and the action
/// bar hands the query over as typed — so a path action has to do it itself.
/// Only a leading `~` that is the whole path or is followed by a separator: to a
/// shell `~alice` is another user's home, which nothing portable can resolve,
/// and a directory really called `~alice` is the likelier thing a launcher is
/// being asked about.
fn resolve_path(path: &str, home: &std::path::Path) -> std::path::PathBuf {
    let Some(rest) = path.strip_prefix('~') else {
        let path = std::path::Path::new(path);
        return if path.is_absolute() {
            path.to_path_buf()
        } else {
            home.join(path)
        };
    };
    if !rest.is_empty() && !rest.starts_with(std::path::is_separator) {
        return home.join(path);
    }
    // The separators have to come off before joining: joining a path that starts
    // with one discards everything to its left and yields the filesystem root.
    // Once they are gone, `~` and `~/` are both simply the home directory.
    let rest = rest.trim_start_matches(std::path::is_separator);
    if rest.is_empty() {
        return home.to_path_buf();
    }
    home.join(rest)
}

fn spawn_opener(target: impl AsRef<std::ffi::OsStr>) -> Result<(), String> {
    let Some(opener) = OPENER else {
        return Err("Opening links is not supported on this platform".to_string());
    };
    Command::new(opener)
        .arg(target)
        .spawn()
        .map(|_| ())
        .map_err(|error| format!("{opener}: {error}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;

    #[test]
    fn resolves_explicit_relative_paths_from_home() {
        let home = Path::new("/users/example");
        assert_eq!(resolve_path("./Documents", home), home.join("./Documents"));
        assert_eq!(resolve_path("../Shared", home), home.join("../Shared"));
        assert_eq!(resolve_path("~/Downloads", home), home.join("Downloads"));
        assert_eq!(resolve_path("/tmp/file", home), Path::new("/tmp/file"));
    }
}
