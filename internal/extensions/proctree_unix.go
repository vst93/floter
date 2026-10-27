//go:build !windows

package extensions

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup puts an extension process in a process group of its
// own, so that killing the group reaches a script's grandchildren. This is the
// reference's `command.process_group(0)`
// (`main:src-tauri/src/extensions/process_cleanup.rs:16-22`).
//
// A probe asks for a session instead: `setsid` also starts a new process group
// (with the child's pid as its pgid), and it additionally takes away the
// controlling terminal, which is what a probe needs. The two bits are mutually
// exclusive on purpose — `setpgid(0,0)` makes the child a process group leader
// and `setsid` then fails with EPERM.
func configureProcessGroup(cmd *exec.Cmd, session bool) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	if session {
		cmd.SysProcAttr.Setsid = true
		return
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup kills the whole process group of an extension process, then
// the direct child as a fallback (a restricted sandbox can refuse the group
// kill, and the child still has to die). SIGKILL is deliberate: the reference
// notes that a provider may trap or ignore softer signals.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// `configureProcessGroup` made the child's pid its pgid (a session leader's
	// pid is its pgid as well), so the negative pid names the group.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}
