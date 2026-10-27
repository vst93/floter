package extensions

import (
	"context"
	"os/exec"
)

// One process, one rule.
//
// Every extension process this package starts — a background run the user
// clicked, a provider's describe/complete operation, a lifecycle probe, the
// connect-time `--help` derivation, a script toolchain's `--version`, the login
// shell's PATH, the host OS version — goes through the two constructors below.
// The reference has the same shape
// (`main:src-tauri/src/extensions/process_cleanup.rs`): one `configure_command`
// that puts the child in its own process group, one `ChildCleanup` that kills
// the *group* rather than the direct child, and one `finish_after_wait` that
// sweeps the group again after a normal exit.
//
// The reason is scripts: an extension command is usually a shell script, and a
// direct-child kill leaves the script's grandchildren running. Six spawn sites
// with six different answers is how that happens, so there is exactly one
// answer here.
//
// Windows has no equivalent: the reference's `kill_group` is an empty function
// there (`#[cfg(not(unix))]`), and this package matches it — a direct child is
// all a Windows extension process gets.

// newExtensionCommand is the one place an extension process is created: a
// context-bounded command in its own process group, whose cancellation kills
// the group.
func newExtensionCommand(ctx context.Context, program string, args ...string) *exec.Cmd {
	return newCommand(ctx, false, program, args...)
}

// newProbeCommand is newExtensionCommand plus the probe isolation: a probe must
// not reach the terminal floter was started from, so on Unix it gets a session
// of its own instead of only a process group (a new session is a new process
// group too, which is why it replaces the group bit rather than adding to it:
// `setpgid(0,0)` would make the child a group leader, and a group leader cannot
// `setsid`). On Windows both constructors are the same command — there is no
// controlling terminal or job control to step away from.
func newProbeCommand(ctx context.Context, program string, args ...string) *exec.Cmd {
	return newCommand(ctx, true, program, args...)
}

// newCommand is the single implementation behind the two constructors.
func newCommand(ctx context.Context, session bool, program string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, program, args...)
	configureProcessGroup(cmd, session)
	cmd.Cancel = func() error {
		// Best effort, and nil either way: a process that already exited is an
		// expected race, and `Wait` should report the command's own outcome
		// rather than a kill error.
		killProcessGroup(cmd)
		return nil
	}
	return cmd
}

// runExtensionCommand waits for one extension process and then sweeps its
// process group: a script that exited while leaving a grandchild behind must
// not leak it into the next run. The reference does the same in
// `finish_after_wait`, and for the same reason.
//
// The sweep happens after `Run` returns, so a grandchild that holds the run's
// pipes is swept when the wait's `WaitDelay` closes them rather than a moment
// earlier; it is still swept, which is the promise.
func runExtensionCommand(cmd *exec.Cmd) error {
	err := cmd.Run()
	killProcessGroup(cmd)
	return err
}
