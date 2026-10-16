//go:build !windows

package extensions

import (
	"os/exec"
	"syscall"
)

// isolateProbe keeps a tool's terminal setup and job-control signals away
// from the host. Closing stdin alone still leaves /dev/tty accessible when
// floter was started from a terminal.
func isolateProbe(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
