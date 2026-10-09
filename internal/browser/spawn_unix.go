//go:build !windows

package browser

import (
	"os/exec"
	"syscall"
)

// detach puts the child in its own session, so the browser does not share the
// app's terminal or process group and survives the app quitting.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
