//go:build windows

package spawn

import (
	"os/exec"
	"syscall"
)

// The two flags `syscall` does not name, spelled out: `DETACHED_PROCESS` and
// `CREATE_NO_WINDOW`.
const (
	detachedProcess = 0x00000008
	createNoWindow  = 0x08000000
)

// detach gives the child no console of its own to inherit, so a program
// started from the app does not flash a console window or hold the app's.
//
// The flag set is the reference's own (`main:src-tauri/src/process_launch.rs:
// 74-86`): `DETACHED_PROCESS` removes the console, `CREATE_NEW_PROCESS_GROUP`
// keeps the child out of the app's Ctrl-C group, and `CREATE_NO_WINDOW` is the
// flag the terminal daemon's own spawns used to set. Windows documents
// `CREATE_NO_WINDOW` as ignored when `DETACHED_PROCESS` is also given, so this
// is the reference's flag set byte for byte and not a behaviour change.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP |
			detachedProcess |
			createNoWindow,
	}
}
