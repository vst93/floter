//go:build windows

package spawn

import (
	"os/exec"
	"syscall"
)

// detach gives the child no console of its own to inherit, so a program
// started from the app does not flash a console window or hold the app's.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS
	}
}
