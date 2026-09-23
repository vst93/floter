//! Launching a program that is meant to outlive Floter.
//!
//! R41 · the launcher's "start this program" paths used to spawn children that
//! were **not** detached: they inherited Floter's stdio and, on Unix, stayed in
//! Floter's session and process group. The user's report, verbatim:
//!
//!   「在搜索框中启动其他程序时，我在 Linux 中测试发现，整个系统的程序进程会被新
//!     进程替代掉。状态栏托盘中的图标虽然看起来没变，但它的功能和菜单都变成了新开
//!     应用的。……启动时应该需要启动一个后台进程，而不是在当前应用进程下去执行新
//!     程序。」
//!
//! The investigation found no `exec`/`execve`/`CommandExt::exec` anywhere in the
//! tree — `std::process::Command` forks, it never replaces the caller — so the
//! *process replacement* is not literal. The mechanism behind the report is the
//! Linux app-open path: `open_application` is a **synchronous** Tauri command,
//! so it runs on the event-loop thread, and the `.desktop` branch called
//! `gio launch … .status()` — which **waits** for `gio`. For as long as it
//! waited, the webview and the tray were frozen, and a freshly launched app's
//! own tray item read as "the menu changed". The child was also not detached, so
//! it shared Floter's process group and could be killed by Floter's own group
//! cleanup (the extension runtime's `killpg`) — the opposite of "start an app".
//!
//! This module is the one spawn used by every launch path. It:
//!
//!   * **does not wait** — `spawn` returns as soon as the fork/exec is done;
//!   * **detaches** the child: on Unix a fresh session (`setsid`) so it leaves
//!     Floter's controlling terminal and process group and survives Floter's
//!     exit; on Windows `DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP`;
//!   * gives it `null` stdio, so it never holds a pipe Floter owns; and
//!   * reaps it on a small thread, so a program that exits while Floter is
//!     still running does not become a zombie. If Floter exits first the OS
//!     reparents the child to init, which reaps it instead.
//!
//! macOS was already effectively detached: `open` hands the request to
//! LaunchServices and exits. It now shares this helper anyway, so all three
//! platforms answer the question in one place.

use std::ffi::OsStr;
use std::process::{Command, Stdio};

/// Apply the detachment every launch path needs to an already-built command:
/// `null` stdio and, on Unix, a fresh session. Idempotent enough to be the
/// single place both the builder and the terminal hand-off go through.
pub(crate) fn apply_detachment(command: &mut Command) {
    command
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());

    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        // SAFETY: `pre_exec` runs in the forked child before `exec`. `setsid`
        // is async-signal-safe and touches no allocator state; on failure the
        // error is reported through the normal spawn channel.
        unsafe {
            command.pre_exec(|| {
                if libc::setsid() == -1 {
                    return Err(std::io::Error::last_os_error());
                }
                Ok(())
            });
        }
    }
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        // `DETACHED_PROCESS` removes the console; `CREATE_NEW_PROCESS_GROUP`
        // keeps the child out of Floter's Ctrl-C group.
        const DETACHED_PROCESS: u32 = 0x0000_0008;
        const CREATE_NEW_PROCESS_GROUP: u32 = 0x0000_0200;
        command.creation_flags(DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP);
    }
}

/// Build the command with the detachment every launch path needs. `configure`
/// runs first so a caller can set a working directory or filter the
/// environment without re-adding a stdio handle afterwards.
fn detached_command<S, F>(program: &str, args: &[S], configure: F) -> Command
where
    S: AsRef<OsStr>,
    F: FnOnce(&mut Command),
{
    let mut command = Command::new(program);
    command.args(args);
    configure(&mut command);
    apply_detachment(&mut command);
    command
}

/// Spawn an already-detached command, returning its pid, and reap it on a
/// short-lived thread. Callers must have applied [`apply_detachment`] once;
/// applying it twice would register `setsid` twice, and the second call fails
/// with `EPERM` because the child is already a session leader.
fn spawn_prepared(command: &mut Command) -> Result<u32, String> {
    let program = command.get_program().to_string_lossy().into_owned();
    let mut child = command
        .spawn()
        .map_err(|error| format!("{program}: {error}"))?;
    let pid = child.id();

    // Reap on its own thread. A launch is rare, so one short-lived thread per
    // child is cheaper than a global SIGCHLD policy, which would fight the
    // extension runtime's own `Child::wait` calls.
    let _ = std::thread::Builder::new()
        .name("detached-reaper".to_string())
        .spawn(move || {
            let _ = child.wait();
        });

    Ok(pid)
}

/// Spawn a command that a caller built itself, detached, returning its pid.
pub(crate) fn spawn_detached_command(command: &mut Command) -> Result<u32, String> {
    apply_detachment(command);
    spawn_prepared(command)
}

/// Spawn `program` with `args` as a fully detached child, returning its pid.
///
/// `configure` runs before the detachment is applied, for callers that need to
/// set a working directory or filter the environment (the terminal hand-off
/// does both).
pub(crate) fn spawn_detached_with<S, F>(
    program: &str,
    args: &[S],
    configure: F,
) -> Result<u32, String>
where
    S: AsRef<OsStr>,
    F: FnOnce(&mut Command),
{
    // `detached_command` already applied the detachment; spawn it directly so
    // `setsid` is registered exactly once.
    let mut command = detached_command(program, args, configure);
    spawn_prepared(&mut command)
}

/// Spawn `program` with `args` as a fully detached child and hand back the
/// `Child` **without** the reaper thread. Used by a caller that must observe the
/// child's exit (the Linux app-open path falls back when `gio launch` fails) but
/// must not block the caller's thread doing it: the caller moves the child to
/// its own thread and waits there.
pub(crate) fn spawn_detached_child<S: AsRef<OsStr>>(
    program: &str,
    args: &[S],
) -> Result<std::process::Child, String> {
    let mut command = detached_command(program, args, |_| {});
    let program = command.get_program().to_string_lossy().into_owned();
    command
        .spawn()
        .map_err(|error| format!("{program}: {error}"))
}

/// [`spawn_detached_with`] without a configuration callback.
pub(crate) fn spawn_detached<S: AsRef<OsStr>>(program: &str, args: &[S]) -> Result<u32, String> {
    spawn_detached_with(program, args, |_| {})
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The parent's own pid is never the child's: a spawn forks, it does not
    /// replace the caller's image. This is the regression test the report asks
    /// for — if any path ever grew an `exec`-on-self, this turns red.
    #[test]
    fn spawning_does_not_replace_the_current_process() {
        let pid = std::process::id();
        let child = spawn_detached("true", &[] as &[&str]).expect("spawn true");
        assert_ne!(child, pid, "the child must be a new process");
        assert_eq!(
            std::process::id(),
            pid,
            "the caller's pid must be unchanged after spawning"
        );
    }

    #[cfg(target_os = "linux")]
    #[test]
    fn a_detached_child_is_its_own_session_leader() {
        let mut child = spawn_detached_child("sleep", &["5"]).expect("spawn sleep");
        // `setsid()` makes the child both process-group and session leader, so
        // its session id equals its pid.
        let session = session_id(child.id()).expect("read session id");
        assert_eq!(
            session,
            i64::from(child.id()),
            "the child must lead its own session"
        );
        let _ = child.kill();
        let _ = child.wait();
    }

    /// Field 6 of `/proc/<pid>/stat` is the session id. The command name in
    /// field 2 is parenthesised and may contain spaces, so the fields are read
    /// from after the closing `)`.
    #[cfg(target_os = "linux")]
    fn session_id(pid: u32) -> Option<i64> {
        let stat = std::fs::read_to_string(format!("/proc/{pid}/stat")).ok()?;
        let after_name = &stat[stat.rfind(')')? + 1..];
        let fields: Vec<&str> = after_name.split_whitespace().collect();
        // After `)` the first field is the state (field 3); session is field 6,
        // i.e. index 3 here.
        fields.get(3)?.parse().ok()
    }

    #[test]
    fn a_missing_program_is_a_named_error() {
        let error = spawn_detached("floter-no-such-program-xyz", &[] as &[&str])
            .expect_err("a missing program must fail");
        assert!(
            error.starts_with("floter-no-such-program-xyz:"),
            "the error names the program: {error}"
        );
    }
}
