//go:build !darwin && !linux

package termsession

import "io"

// PTY is what this platform does not have: a program in a pseudo-terminal.
//
// Windows has ConPTY, which is a separate slice of this arc (S1b): it needs
// its own handle plumbing, and a stubbed shell that only *looks* like a
// session would be the exact "pretend the platform is there" this project
// forbids. Until it lands, Start reports ErrUnsupported and nothing else
// happens — the app keeps the in-process sessions it has today.
type PTY struct{}

// startPTY reports ErrUnsupported: no pseudo-terminal on this platform.
func startPTY(Options, int, int) (*PTY, error) { return nil, ErrUnsupported }

func (p *PTY) Read([]byte) (int, error)    { return 0, io.EOF }
func (p *PTY) Write(b []byte) (int, error) { return 0, ErrUnsupported }
func (p *PTY) Resize(int, int) error       { return ErrUnsupported }
func (p *PTY) ResizePixels(int, int, int, int) error {
	return ErrUnsupported
}
func (p *PTY) Exited() <-chan struct{} { c := make(chan struct{}); close(c); return c }
func (p *PTY) ExitCode() int           { return -1 }
func (p *PTY) Pid() int                { return 0 }
func (p *PTY) Kill() error             { return ErrUnsupported }
func (p *PTY) Close() error            { return nil }
