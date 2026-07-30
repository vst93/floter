//! System power actions.
//!
//! There is no portable API for restarting or shutting a machine down, so each
//! platform gets the command line its own desktop blesses. The tables below are
//! ordered by preference; see [`spawn_first`] for what a second entry buys.

use std::process::Command;

/// Restart or shut down the machine.
///
/// The command is spawned and deliberately not waited on: whatever carries the
/// action out tears down this process' own session on the way, so a `wait()`
/// here would block until the desktop dies and then have nothing to report. A
/// successful spawn is therefore all this returns — a refusal further down the
/// line (polkit declining, or the user cancelling macOS' confirmation dialog)
/// leaves the machine running and is invisible from here.
#[tauri::command]
pub fn system_power(action: String) -> Result<(), String> {
    match action.as_str() {
        "restart" => spawn_first(RESTART),
        "shutdown" => spawn_first(SHUTDOWN),
        other => Err(format!("Unknown system action: {other}")),
    }
}

/// macOS: the Apple event is what GUI applications are expected to use. It goes
/// through loginwindow, which asks the user to confirm exactly as the Apple menu
/// does; the `shutdown` binary would need root.
#[cfg(target_os = "macos")]
const RESTART: &[&[&str]] = &[&[
    "osascript",
    "-e",
    "tell application \"System Events\" to restart",
]];
#[cfg(target_os = "macos")]
const SHUTDOWN: &[&[&str]] = &[&[
    "osascript",
    "-e",
    "tell application \"System Events\" to shut down",
]];

/// Linux: `systemctl` is accepted from an unprivileged seat session through
/// polkit on every systemd distribution. The SysV binaries follow it for the
/// rare init that is not systemd.
#[cfg(target_os = "linux")]
const RESTART: &[&[&str]] = &[&["systemctl", "reboot"], &["reboot"]];
#[cfg(target_os = "linux")]
const SHUTDOWN: &[&[&str]] = &[&["systemctl", "poweroff"], &["poweroff"]];

/// Windows: `shutdown.exe` needs no elevation to end the calling user's own
/// session. `/t 0` skips the default grace period.
#[cfg(target_os = "windows")]
const RESTART: &[&[&str]] = &[&["shutdown", "/r", "/t", "0"]];
#[cfg(target_os = "windows")]
const SHUTDOWN: &[&[&str]] = &[&["shutdown", "/s", "/t", "0"]];

/// Anything else (BSDs, ...) still builds; the action simply reports that the
/// platform has no supported way to do this.
#[cfg(not(any(target_os = "macos", target_os = "linux", target_os = "windows")))]
const RESTART: &[&[&str]] = &[];
#[cfg(not(any(target_os = "macos", target_os = "linux", target_os = "windows")))]
const SHUTDOWN: &[&[&str]] = &[];

/// Spawn the first candidate that starts, keeping the last failure to report.
///
/// Only a missing binary is ever fallen through on, because that is the only
/// failure `spawn` can see — an init that exists but refuses the request has
/// already been handed the job by then (see [`system_power`]).
fn spawn_first(candidates: &[&[&str]]) -> Result<(), String> {
    let mut failure = "Power actions are not supported on this platform".to_string();
    for candidate in candidates {
        let Some((program, args)) = candidate.split_first() else {
            continue;
        };
        match Command::new(program).args(args).spawn() {
            Ok(_) => return Ok(()),
            Err(error) => failure = format!("{program}: {error}"),
        }
    }
    Err(failure)
}
