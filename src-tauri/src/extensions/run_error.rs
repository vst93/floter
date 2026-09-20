//! Keyed, readable failures for a manual run.
//!
//! Before this module a failed run reached the user as whatever the OS said —
//! `Cannot spawn command: No such file or directory (os error 2)` — or, on the
//! terminal route, as the launcher's generic "Could not start this command".
//! Neither names *what* was not found or *where* the host looked, which is the
//! only information that lets a user fix it.
//!
//! Every message here is a **stable key plus a JSON payload**, the same shape
//! `run_param_required:<id>` already used: the frontend owns the words (and
//! both languages), the backend owns the facts. A payload is JSON rather than
//! a delimiter-joined string because the payloads are filesystem paths, and a
//! path may legally contain any delimiter we might pick.
//!
//! Nothing here swallows a failure. Each variant exists because there is a
//! distinct thing the user can do about it.

use serde::Serialize;
use std::path::{Path, PathBuf};

/// The script file the manifest points at is gone.
pub const RUN_SCRIPT_MISSING: &str = "run_script_missing";
/// The program the plan would spawn does not exist.
pub const RUN_PROGRAM_MISSING: &str = "run_program_missing";
/// The program exists but is not an executable file (a directory, or a file
/// without the execute bit).
pub const RUN_PROGRAM_NOT_EXECUTABLE: &str = "run_program_not_executable";
/// The interpreter for a script language is not on the host's search path.
pub const RUN_INTERPRETER_MISSING: &str = "run_interpreter_missing";
/// The OS refused the spawn (permissions, a resource limit, a bad image).
pub const RUN_SPAWN_FAILED: &str = "run_spawn_failed";
/// The run exceeded its budget and was killed.
pub const RUN_TIMEOUT: &str = "run_timeout";

#[derive(Serialize)]
struct InterpreterMissing<'a> {
    language: &'a str,
    /// The candidate binary names, in the order they were looked for.
    names: Vec<String>,
    /// The directories that were searched, in order. This is the half the old
    /// message never carried: "not found" without "where" is not actionable.
    searched: Vec<String>,
}

#[derive(Serialize)]
struct PathPayload<'a> {
    path: &'a str,
}

#[derive(Serialize)]
struct SpawnFailed<'a> {
    path: &'a str,
    /// The OS's own text. Kept because for a failure that is not one of the
    /// named cases this is the only detail there is.
    detail: &'a str,
}

#[derive(Serialize)]
struct TimeoutPayload {
    seconds: u64,
}

/// Serialize a keyed payload. A `serde_json` failure cannot happen for these
/// shapes (plain strings and integers), so the fallback keeps the key with an
/// empty object rather than losing the failure entirely.
fn keyed<T: Serialize>(key: &str, payload: &T) -> String {
    let body = serde_json::to_string(payload).unwrap_or_else(|_| "{}".to_string());
    format!("{key}:{body}")
}

pub fn script_missing(path: &Path) -> String {
    keyed(
        RUN_SCRIPT_MISSING,
        &PathPayload {
            path: &path.to_string_lossy(),
        },
    )
}

pub fn program_missing(path: &Path) -> String {
    keyed(
        RUN_PROGRAM_MISSING,
        &PathPayload {
            path: &path.to_string_lossy(),
        },
    )
}

pub fn program_not_executable(path: &Path) -> String {
    keyed(
        RUN_PROGRAM_NOT_EXECUTABLE,
        &PathPayload {
            path: &path.to_string_lossy(),
        },
    )
}

pub fn interpreter_missing(language: &str, names: &[String], searched: &[PathBuf]) -> String {
    keyed(
        RUN_INTERPRETER_MISSING,
        &InterpreterMissing {
            language,
            names: names.to_vec(),
            searched: searched
                .iter()
                .map(|directory| directory.to_string_lossy().into_owned())
                .collect(),
        },
    )
}

/// Map the OS's spawn failure to a keyed message.
///
/// `NotFound` and `PermissionDenied` are named because each has a distinct
/// remedy (install it / fix the mode); everything else keeps the raw text so
/// nothing is hidden behind a generic sentence.
pub fn spawn_failed(program: &Path, error: &std::io::Error) -> String {
    let path = program.to_string_lossy();
    match error.kind() {
        std::io::ErrorKind::NotFound => program_missing(program),
        std::io::ErrorKind::PermissionDenied => {
            keyed(RUN_PROGRAM_NOT_EXECUTABLE, &PathPayload { path: &path })
        }
        _ => keyed(
            RUN_SPAWN_FAILED,
            &SpawnFailed {
                path: &path,
                detail: &error.to_string(),
            },
        ),
    }
}

pub fn timed_out(timeout: std::time::Duration) -> String {
    keyed(
        RUN_TIMEOUT,
        &TimeoutPayload {
            seconds: timeout.as_secs(),
        },
    )
}

/// The key of a keyed message, or `None` when it is a plain string. The
/// frontend uses the same split; keeping it here too lets the backend's own
/// tests assert the key without re-parsing JSON by hand.
pub fn message_key(message: &str) -> Option<&str> {
    let (key, rest) = message.split_once(':')?;
    if !key.starts_with("run_") || !rest.starts_with('{') {
        return None;
    }
    Some(key)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn payload(message: &str) -> serde_json::Value {
        let (_, body) = message.split_once(':').expect("keyed message");
        serde_json::from_str(body).expect("payload is JSON")
    }

    #[test]
    fn interpreter_missing_names_the_binary_and_the_directories_searched() {
        // The whole point of the mapping table: "not found" is useless without
        // "what" and "where". Mutation: drop `searched` and this fails.
        let message = interpreter_missing(
            "js",
            &["node".to_string()],
            &[
                PathBuf::from("/usr/bin"),
                PathBuf::from("/opt/homebrew/bin"),
            ],
        );
        assert_eq!(message_key(&message), Some(RUN_INTERPRETER_MISSING));
        let payload = payload(&message);
        assert_eq!(payload["language"], "js");
        assert_eq!(payload["names"][0], "node");
        assert_eq!(payload["searched"][0], "/usr/bin");
        assert_eq!(payload["searched"][1], "/opt/homebrew/bin");
    }

    #[test]
    fn a_missing_program_and_a_missing_script_are_different_keys() {
        assert_eq!(
            message_key(&program_missing(Path::new("/nope/tool"))),
            Some(RUN_PROGRAM_MISSING)
        );
        assert_eq!(
            message_key(&script_missing(Path::new("/nope/provider.sh"))),
            Some(RUN_SCRIPT_MISSING)
        );
        // …and each carries the path it is about.
        assert_eq!(
            payload(&script_missing(Path::new("/nope/provider.sh")))["path"],
            "/nope/provider.sh"
        );
    }

    #[test]
    fn an_enoent_spawn_maps_to_the_missing_program_key() {
        // The spec's table test: input ENOENT, output the keyed message.
        let error = std::io::Error::from(std::io::ErrorKind::NotFound);
        let message = spawn_failed(Path::new("/nope/tool"), &error);
        assert_eq!(message_key(&message), Some(RUN_PROGRAM_MISSING));
        assert_eq!(payload(&message)["path"], "/nope/tool");
    }

    #[test]
    fn a_permission_denied_spawn_maps_to_the_not_executable_key() {
        let error = std::io::Error::from(std::io::ErrorKind::PermissionDenied);
        let message = spawn_failed(Path::new("/nope/tool"), &error);
        assert_eq!(message_key(&message), Some(RUN_PROGRAM_NOT_EXECUTABLE));
    }

    #[test]
    fn any_other_spawn_failure_keeps_the_raw_detail() {
        // Nothing is hidden behind a generic sentence: an unnamed failure still
        // carries the OS's own words.
        let error = std::io::Error::other("Exec format error");
        let message = spawn_failed(Path::new("/nope/tool"), &error);
        assert_eq!(message_key(&message), Some(RUN_SPAWN_FAILED));
        let payload = payload(&message);
        assert_eq!(payload["path"], "/nope/tool");
        assert!(
            payload["detail"]
                .as_str()
                .unwrap()
                .contains("Exec format error"),
            "{message}"
        );
    }

    #[test]
    fn a_timeout_carries_its_budget() {
        let message = timed_out(std::time::Duration::from_secs(300));
        assert_eq!(message_key(&message), Some(RUN_TIMEOUT));
        assert_eq!(payload(&message)["seconds"], 300);
    }

    #[test]
    fn a_plain_string_is_not_mistaken_for_a_keyed_message() {
        // The frontend keeps its ordinary toast for these, so the split has to
        // be conservative.
        assert_eq!(message_key("Script toolchain is not available"), None);
        assert_eq!(message_key("run_param_required:note"), None);
        assert_eq!(message_key("run_already_in_flight:local.a"), None);
    }
}
