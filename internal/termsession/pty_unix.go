//go:build darwin || linux

package termsession

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// PTY is a program running in a pseudo-terminal: Read returns its output,
// Write types into it, Resize tells it its window changed.
//
// It is written here rather than taken from a dependency because the whole
// point of this slice is that the app stays pure Go on every platform it
// builds for: `/dev/ptmx` and a handful of ioctls on macOS and Linux, and
// nothing at all on the platforms that have no such device.
type PTY struct {
	master *os.File
	cmd    *exec.Cmd
	code   int
	err    error
	done   chan struct{}
}

// startPTY starts program in a pseudo-terminal of cols×rows.
func startPTY(o Options, cols, rows int) (*PTY, error) {
	master, slaveName, err := openMaster()
	if err != nil {
		return nil, err
	}
	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		return nil, fmt.Errorf("termsession: opening %s: %w", slaveName, err)
	}
	// The child gets its own copy; this one only existed to hand it over.
	defer func() { _ = slave.Close() }()
	if err := setWinSize(master, cols, rows); err != nil {
		_ = master.Close()
		return nil, err
	}
	path, args := o.Path, o.Args
	if len(args) == 0 {
		args = []string{path}
	}
	cmd := &exec.Cmd{Path: path, Args: args, Dir: o.Dir, Env: o.Env,
		Stdin: slave, Stdout: slave, Stderr: slave}
	// A session of its own whose controlling terminal is the slave, so that
	// the terminal's signals reach the program's process group — the same
	// arrangement a terminal emulator gives the shell it starts.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		_ = master.Close()
		return nil, err
	}
	p := &PTY{master: master, cmd: cmd, done: make(chan struct{})}
	go func() {
		p.code, p.err = p.wait()
		close(p.done)
	}()
	return p, nil
}

// Read reads the program's output; io.EOF once nothing has the terminal open
// any more.
func (p *PTY) Read(b []byte) (int, error) {
	n, err := p.master.Read(b)
	if err != nil && n == 0 {
		// Linux reports EIO when the last process holding the terminal
		// closes it; that is the end of the output, not a fault.
		if errors.Is(err, syscall.EIO) || errors.Is(err, os.ErrClosed) {
			err = io.EOF
		}
	}
	return n, err
}

// Write types b into the terminal.
func (p *PTY) Write(b []byte) (int, error) { return p.master.Write(b) }

// Resize sets the terminal's size, which sends the program SIGWINCH.
func (p *PTY) Resize(cols, rows int) error { return setWinSize(p.master, cols, rows) }

// Exited is closed once the program ended; ExitCode is meaningful then.
func (p *PTY) Exited() <-chan struct{} { return p.done }

// ExitCode is what the program returned: 128 plus the signal's number for
// one a signal killed.
func (p *PTY) ExitCode() int { return p.code }

// wait waits for the program and reads its status.
func (p *PTY) wait() (int, error) {
	err := p.cmd.Wait()
	if e, ok := errors.AsType[*exec.ExitError](err); ok {
		if ws, ok := e.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()), nil
		}
		return e.ExitCode(), nil
	}
	if err != nil {
		return -1, err
	}
	return 0, nil
}

// Pid is the program's process id.
func (p *PTY) Pid() int {
	if p.cmd == nil || p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// Kill ends the program and its process group.
func (p *PTY) Kill() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL); err == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

// Close closes the terminal, which hangs up whatever has it open. It does not
// wait for them: blocking here would hold the host's frame loop.
func (p *PTY) Close() error {
	if p.master == nil {
		return nil
	}
	return p.master.Close()
}

// setWinSize sets rows/cols on the terminal, which is how a program learns
// its window changed.
func setWinSize(f *os.File, cols, rows int) error {
	ws := struct{ row, col, xpixel, ypixel uint16 }{clampCell(rows), clampCell(cols), 0, 0}
	return ioctl(f, syscall.TIOCSWINSZ, unsafe.Pointer(&ws))
}

// clampCell fits a count into the 16 bits a window size holds.
func clampCell(v int) uint16 {
	if v < 0 {
		return 0
	}
	if v > 0xFFFF {
		return 0xFFFF
	}
	return uint16(v)
}

// ioctl calls ioctl on f with a pointer, converted in the system call so that
// what it points at cannot move meanwhile.
func ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error {
	c, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var errno syscall.Errno
	if err := c.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	}); err != nil {
		return err
	}
	if errno != 0 {
		return errno
	}
	return nil
}
