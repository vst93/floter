package extensions

import "os/exec"

// Windows has no process groups to configure: the reference's `kill_group` is
// an empty function there too (`main:src-tauri/src/extensions/process_cleanup.rs`,
// `#[cfg(not(unix))]`). `exec.CommandContext` already kills the direct child
// when the context is done, and that is the whole contract on this platform —
// including for a probe, which has no controlling terminal to step away from.
func configureProcessGroup(cmd *exec.Cmd, session bool) {}

// killProcessGroup kills the direct child, the only process this platform
// tracks.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
