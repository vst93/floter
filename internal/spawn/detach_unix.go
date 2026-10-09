//go:build !windows

package spawn

import (
	"os/exec"
	"syscall"
)

// detach puts the child in its own session, so it does not share the app's
// terminal or process group and survives the app quitting.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
