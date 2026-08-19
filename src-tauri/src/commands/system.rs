//! System power actions.
//!
//! There is no portable API for restarting or shutting a machine down, so each
//! platform gets the command line its own desktop blesses. The tables below are
//! ordered by preference; see [`run_first`] for what a second entry buys.

use std::io::ErrorKind;
use std::time::Duration;
use tokio::process::Command;

const POWER_ACTION_TIMEOUT: Duration = Duration::from_secs(30);

/// Restart or shut down the machine.
///
/// Power utilities return after the request is accepted; they do not stay alive
/// until the machine comes back. Waiting for that result is important because a
/// successful spawn can still end in a polkit, loginwindow, or privilege
/// refusal. The timeout keeps a broken utility from leaving the launcher hidden
/// indefinitely.
#[tauri::command]
pub async fn system_power(action: String) -> Result<(), String> {
    match action.as_str() {
        "restart" => run_first(RESTART, POWER_ACTION_TIMEOUT).await,
        "shutdown" => run_first(SHUTDOWN, POWER_ACTION_TIMEOUT).await,
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

/// Run the first available candidate and require it to accept the request.
///
/// A missing program falls through so Linux can support both systemd and SysV
/// systems. Once a program starts, its refusal is authoritative: trying a more
/// privileged fallback after that would undermine the desktop's decision.
async fn run_first(candidates: &[&[&str]], timeout: Duration) -> Result<(), String> {
    let mut missing = "Power actions are not supported on this platform".to_string();
    for candidate in candidates {
        let Some((program, args)) = candidate.split_first() else {
            continue;
        };
        let mut command = Command::new(program);
        command.args(args).kill_on_drop(true);
        let output = match tokio::time::timeout(timeout, command.output()).await {
            Ok(Ok(output)) => output,
            Ok(Err(error)) if error.kind() == ErrorKind::NotFound => {
                missing = format!("{program}: {error}");
                continue;
            }
            Ok(Err(error)) => return Err(format!("{program}: {error}")),
            Err(_) => {
                return Err(format!(
                    "{program}: timed out while requesting power action"
                ))
            }
        };
        if output.status.success() {
            return Ok(());
        }
        let detail = String::from_utf8_lossy(&output.stderr).trim().to_string();
        let status = output
            .status
            .code()
            .map(|code| format!("exit code {code}"))
            .unwrap_or_else(|| "terminated by signal".to_string());
        return Err(if detail.is_empty() {
            format!("{program}: {status}")
        } else {
            format!("{program}: {status}: {detail}")
        });
    }
    Err(missing)
}

#[cfg(all(test, unix))]
mod tests {
    use super::*;

    #[tokio::test]
    async fn missing_preferred_utility_falls_back() {
        let candidates: &[&[&str]] = &[
            &["floter-command-that-does-not-exist"],
            &["sh", "-c", "exit 0"],
        ];

        assert!(run_first(candidates, Duration::from_secs(1)).await.is_ok());
    }

    #[tokio::test]
    async fn refusal_does_not_try_a_more_privileged_fallback() {
        let candidates: &[&[&str]] = &[
            &["sh", "-c", "printf 'permission denied' >&2; exit 7"],
            &["sh", "-c", "exit 0"],
        ];

        let error = run_first(candidates, Duration::from_secs(1))
            .await
            .unwrap_err();
        assert!(error.contains("exit code 7"));
        assert!(error.contains("permission denied"));
    }

    #[tokio::test]
    async fn hung_utility_times_out() {
        let candidates: &[&[&str]] = &[&["sh", "-c", "sleep 2"]];

        let error = run_first(candidates, Duration::from_millis(20))
            .await
            .unwrap_err();
        assert!(error.contains("timed out"));
    }

    #[tokio::test]
    async fn unknown_actions_are_rejected() {
        let error = system_power("hibernate".to_string()).await.unwrap_err();
        assert_eq!(error, "Unknown system action: hibernate");
    }
}
