//! File drops → launcher rows: turning the OS drag-drop payload into the
//! normalized file descriptions the launcher renders.
//!
//! The webview's `tauri://drag-drop` event hands over whatever the platform
//! reported: absolute paths on every desktop we ship, but the event is a
//! transport, not a contract. This module is the contract. It
//!
//!   * resolves `~` against the home directory,
//!   * resolves a relative path against the working directory the drop was
//!     resolved in (never against the process's own cwd guess),
//!   * drops entries that do not exist,
//!
//! and returns the four fields a result row actually needs.
//!
//! Deliberately a *description* and nothing more: no action is taken, no
//! process is spawned, no clipboard is written. The launcher's rule is that a
//! result is offered, never run — the three actions a dropped file carries are
//! executed only when the user presses Enter on one (see
//! `src/launcher/file-drops.ts`).

use serde::Serialize;
use std::path::{Path, PathBuf};

/// One dropped path, normalized and ready to render.
#[derive(Clone, Debug, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DroppedFile {
    /// Absolute, `~`-expanded path. The single value every action is built on.
    pub path: String,
    /// The file name as the row's title.
    pub name: String,
    /// The containing directory, shown as the row's small text. For a path
    /// whose parent cannot be read (a root, say) this is the path itself.
    pub directory: String,
    /// A folder is a file too: it opens in the file manager, `cd`s into
    /// itself, and copies its own path. Only the row's icon differs.
    pub is_directory: bool,
}

/// Expand `~`, make the path absolute, and keep only paths that exist.
///
/// `home` and `cwd` are parameters rather than process state so the rules can
/// be exercised without touching the machine the test runs on. Order is
/// preserved: the drop order is the order the user selected the files in, and
/// the launcher shows them in it.
pub fn normalize_dropped_paths(paths: &[String], home: &Path, cwd: &Path) -> Vec<DroppedFile> {
    paths
        .iter()
        .filter_map(|path| normalize_dropped_path(path, home, cwd))
        .collect()
}

/// One path through the rules in [`normalize_dropped_paths`].
fn normalize_dropped_path(path: &str, home: &Path, cwd: &Path) -> Option<DroppedFile> {
    let trimmed = path.trim();
    if trimmed.is_empty() {
        return None;
    }
    let resolved = resolve(path, home, cwd);
    // `symlink_metadata` rather than `metadata`: a dangling symlink is still a
    // thing the user dropped and can still be opened, `cd`-ed into and copied
    // — refusing it because its target is gone would drop the row silently.
    if std::fs::symlink_metadata(&resolved).is_err() {
        return None;
    }
    let name = resolved
        .file_name()
        .map(|name| name.to_string_lossy().into_owned())
        // A path with no final component (`/`) is its own name; the row needs
        // something to show.
        .filter(|name| !name.is_empty())
        .unwrap_or_else(|| resolved.to_string_lossy().into_owned());
    let directory = resolved
        .parent()
        .filter(|parent| !parent.as_os_str().is_empty())
        .map(|parent| parent.to_string_lossy().into_owned())
        .unwrap_or_else(|| resolved.to_string_lossy().into_owned());
    Some(DroppedFile {
        path: resolved.to_string_lossy().into_owned(),
        name,
        directory,
        is_directory: resolved.is_dir(),
    })
}

/// `~`, `~/…`, a relative path, or an absolute one — resolved to an absolute
/// path without touching the filesystem.
///
/// The same `~` rule `open_path` uses (see `commands::actions`): a leading `~`
/// that is the whole path or is followed by a separator is the home directory;
/// `~alice` is a directory literally called that, because another user's home
/// is not something this process can portably resolve.
fn resolve(path: &str, home: &Path, cwd: &Path) -> PathBuf {
    let candidate = if let Some(rest) = path.strip_prefix('~') {
        if rest.is_empty() || rest.starts_with(std::path::is_separator) {
            let rest = rest.trim_start_matches(std::path::is_separator);
            if rest.is_empty() {
                home.to_path_buf()
            } else {
                home.join(rest)
            }
        } else {
            home.join(path)
        }
    } else {
        PathBuf::from(path)
    };
    if candidate.is_absolute() {
        candidate
    } else {
        cwd.join(candidate)
    }
}

/// The window label the drop listener is allowed to act on.
pub const MAIN_WINDOW_LABEL: &str = "main";

/// Normalize the paths the OS reported for a drop that landed on `window_label`.
///
/// The label is checked here as well as in the frontend listener: terminal and
/// settings are modes of the same window in this build, but the moment a second
/// window exists the guard has to be in the layer that owns the payload, not
/// only in the one that subscribes to the event.
#[tauri::command]
pub fn resolve_dropped_files(
    paths: Vec<String>,
    window_label: String,
) -> Result<Vec<DroppedFile>, String> {
    if window_label != MAIN_WINDOW_LABEL {
        return Err(format!(
            "Refusing a file drop outside the main window: {window_label}"
        ));
    }
    let home = dirs::home_dir().ok_or("Cannot find home directory")?;
    // A relative drop path is relative to the process's working directory,
    // which for a bundled app is wherever the launcher started it. Falling back
    // to home keeps the result absolute even then.
    let cwd = std::env::current_dir().unwrap_or_else(|_| home.clone());
    Ok(normalize_dropped_paths(&paths, &home, &cwd))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;

    fn home() -> PathBuf {
        PathBuf::from("/users/example")
    }

    #[test]
    fn expands_tilde_and_resolves_relative_paths() {
        let home = home();
        let cwd = Path::new("/work");
        assert_eq!(resolve("~", &home, cwd), home);
        assert_eq!(resolve("~/Downloads", &home, cwd), home.join("Downloads"));
        assert_eq!(resolve("~/a/b.txt", &home, cwd), home.join("a/b.txt"));
        assert_eq!(resolve("notes.txt", &home, cwd), cwd.join("notes.txt"));
        assert_eq!(resolve("./notes.txt", &home, cwd), cwd.join("./notes.txt"));
        assert_eq!(resolve("/tmp/file", &home, cwd), PathBuf::from("/tmp/file"));
        // `~alice` is a directory of that name, not another user's home.
        assert_eq!(resolve("~alice", &home, cwd), home.join("~alice"));
    }

    #[test]
    fn rejects_paths_that_do_not_exist() {
        let temp = tempfile::tempdir().unwrap();
        let home = temp.path();
        let cwd = temp.path();
        let existing = temp.path().join("kept.txt");
        fs::write(&existing, b"x").unwrap();

        let dropped = normalize_dropped_paths(
            &[
                existing.to_string_lossy().into_owned(),
                temp.path()
                    .join("missing.txt")
                    .to_string_lossy()
                    .into_owned(),
                String::new(),
                "   ".to_string(),
            ],
            home,
            cwd,
        );
        assert_eq!(
            dropped.len(),
            1,
            "only the existing path survives: {dropped:?}"
        );
        assert_eq!(dropped[0].name, "kept.txt");
        assert_eq!(dropped[0].directory, temp.path().to_string_lossy());
        assert!(!dropped[0].is_directory);
    }

    #[test]
    fn a_relative_drop_resolves_against_the_working_directory() {
        let temp = tempfile::tempdir().unwrap();
        fs::write(temp.path().join("relative.txt"), b"x").unwrap();
        let dropped = normalize_dropped_paths(
            &["relative.txt".to_string()],
            Path::new("/elsewhere"),
            temp.path(),
        );
        assert_eq!(dropped.len(), 1);
        assert_eq!(
            dropped[0].path,
            temp.path().join("relative.txt").to_string_lossy()
        );
        assert_eq!(dropped[0].directory, temp.path().to_string_lossy());
    }

    #[test]
    fn a_dropped_folder_is_described_as_a_file() {
        let temp = tempfile::tempdir().unwrap();
        let folder = temp.path().join("Project");
        fs::create_dir(&folder).unwrap();
        let dropped = normalize_dropped_paths(
            &[folder.to_string_lossy().into_owned()],
            temp.path(),
            temp.path(),
        );
        assert_eq!(dropped.len(), 1);
        assert_eq!(dropped[0].name, "Project");
        assert!(dropped[0].is_directory);
    }

    #[cfg(unix)]
    #[test]
    fn a_dangling_symlink_is_still_a_drop() {
        let temp = tempfile::tempdir().unwrap();
        let link = temp.path().join("dangling");
        std::os::unix::fs::symlink(temp.path().join("gone"), &link).unwrap();

        let dropped = normalize_dropped_paths(
            &[link.to_string_lossy().into_owned()],
            temp.path(),
            temp.path(),
        );
        assert_eq!(
            dropped.len(),
            1,
            "a dangling symlink can still be opened or copied"
        );
        assert_eq!(dropped[0].name, "dangling");
    }

    #[test]
    fn drop_order_is_preserved() {
        let temp = tempfile::tempdir().unwrap();
        for name in ["b.txt", "a.txt", "c.txt"] {
            fs::write(temp.path().join(name), b"x").unwrap();
        }
        let paths: Vec<String> = ["b.txt", "a.txt", "c.txt"]
            .iter()
            .map(|name| temp.path().join(name).to_string_lossy().into_owned())
            .collect();
        let dropped = normalize_dropped_paths(&paths, temp.path(), temp.path());
        assert_eq!(
            dropped
                .iter()
                .map(|file| file.name.as_str())
                .collect::<Vec<_>>(),
            ["b.txt", "a.txt", "c.txt"],
        );
    }

    #[test]
    fn only_the_main_window_may_resolve_a_drop() {
        // A terminal/settings window label must not be able to describe a drop:
        // the guard lives on the payload, not only in the listener.
        let result = resolve_dropped_files(vec!["/tmp".to_string()], "terminal".to_string());
        assert!(result.is_err(), "a non-main label must be refused");
        let error = result.unwrap_err();
        assert!(
            error.contains("terminal"),
            "the refusal names the window: {error}"
        );
    }
}
