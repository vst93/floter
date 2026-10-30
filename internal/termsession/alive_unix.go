//go:build darwin || linux

package termsession

import (
	"errors"
	"os"
	"syscall"
)

// Alive reports whether pid names a live process — the question a list asks
// about a session's host.
//
// Signal 0 is the probe: it is checked against the process without sending
// anything. EPERM counts as alive: a process this user may not signal is still
// a process, and the alternative is calling someone else's session lost.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	// ErrProcessDone is a process that has ended and been waited for; EPERM
	// is one this user does not own. Everything else (ESRCH) is gone.
	return errors.Is(err, syscall.EPERM)
}
