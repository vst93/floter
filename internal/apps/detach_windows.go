//go:build windows

package apps

import (
	"os/exec"
	"syscall"
)

// detachExec puts a launched application in its own process group, detached
// from the console (internal/spawn's own detach, restated).
func detachExec(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS
	}
}
