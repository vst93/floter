//go:build !windows

package apps

import (
	"os/exec"
	"syscall"
)

// detachExec puts a launched application in its own session: on Linux, a
// `.desktop`'s Exec ran in the launcher's process group and session, where a
// signal meant for either reached both - the launcher died when the
// application's session was torn down, and the application died with the
// launcher. internal/spawn's own detach, restated so the two cannot drift.
func detachExec(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
