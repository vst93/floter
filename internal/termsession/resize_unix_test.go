//go:build darwin || linux

package termsession

import (
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// P13-b: the size a client sends reaches the program's terminal — including the
// size of a cell in pixels, which is what a program reads out of TIOCGWINSZ to
// answer a size query in pixels, and what pointer reports in pixels (mode 1016)
// are measured in. Before this slice `ResizePixels` was `Resize` with extra
// arguments, and the terminal's xpixel/ypixel were written as zero, so a
// program asking for pixels got zeroes.
//
// The measurement is the program's own: the host's master side, read with
// TIOCGWINSZ exactly as the program reads its slave side.
func TestTheResizeCarriesTheCellSize(t *testing.T) {
	h, cl := startTestHost(t, Options{Path: "/bin/sh", Linger: -1})
	if err := cl.ResizePixels(100, 30, 9, 18); err != nil {
		t.Fatal(err)
	}
	want := winSize{row: 30, col: 100, xpixel: 900, ypixel: 540}
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, err := readWinSize(h.pty.master)
		if err != nil {
			t.Fatal(err)
		}
		if got == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the terminal's size = %+v, want %+v", got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// The record keeps the size in cells; pixels are the terminal's, not the
	// session's to publish.
	if info := h.Session().Info(); info.Cols != 100 || info.Rows != 30 {
		t.Errorf("record after the resize = %dx%d, want 100x30", info.Cols, info.Rows)
	}
	// A client that only knows its cells leaves the pixels alone: the four-byte
	// form is still read, so `Resize` does not turn its pixels into zeroes by
	// accident (it writes them as zero on purpose — unknown, not measured).
	if err := cl.Resize(90, 25); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		got, err := readWinSize(h.pty.master)
		if err != nil {
			t.Fatal(err)
		}
		if got.row == 25 && got.col == 90 {
			if got.xpixel != 0 || got.ypixel != 0 {
				t.Errorf("after a cells-only resize the pixels = %dx%d, want 0 (unknown)", got.xpixel, got.ypixel)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the terminal's size = %+v, want 90x25", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// winSize is the terminal's size as TIOCGWINSZ reports it: two counts of cells
// and two counts of pixels.
type winSize struct {
	row, col, xpixel, ypixel uint16
}

// readWinSize reads a terminal's size, which is how a program learns it.
func readWinSize(f interface {
	SyscallConn() (syscall.RawConn, error)
}) (winSize, error) {
	var ws winSize
	c, err := f.SyscallConn()
	if err != nil {
		return ws, err
	}
	var errno syscall.Errno
	if err := c.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	}); err != nil {
		return ws, err
	}
	if errno != 0 {
		return ws, errno
	}
	return ws, nil
}
