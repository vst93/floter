//go:build darwin || linux

package termsession

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// P13-a: the host's pins, on the platforms that can host a session at all.
//
// These run a real program in a real pseudo-terminal, and they are the whole
// point of this slice: a session whose program lives outside the client. The
// Windows arm has no pseudo-terminal yet (S1b), so it is not claimed: the host
// reports ErrUnsupported there, and `TestTheHostRefusesToPretend` pins exactly
// that.

// TestTheDetachKeepsTheProgram pins the reason this slice exists: a client that
// closes its connection leaves the program running, and the next attach finds
// it — screen and all — still alive.
func TestTheDetachKeepsTheProgram(t *testing.T) {
	h, first := startTestHost(t, Options{Path: "/bin/sh", Linger: -1})
	writeAndWait(t, first, "echo STILL-HERE\r", "STILL-HERE")
	pid := h.Session().Info().Pid
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	// The program outlives the client: the process is there and the record
	// still says running.
	if err := processAlive(pid); err != nil {
		t.Fatalf("the program ended with the client: %v", err)
	}
	if info := h.Session().Info(); info.State != StateRunning {
		t.Errorf("record after a detach = %+v, want running", info)
	}
	second, err := Dial(h.Socket())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()
	// The screen the session kept is the first thing the second client is
	// handed, before anything the program writes afterwards.
	if got := readSome(t, second, 1); !strings.Contains(got, "STILL-HERE") {
		t.Errorf("the second attach's screen = %q, want what the program printed before", got)
	}
	if _, err := second.Write([]byte("echo AGAIN\r")); err != nil {
		t.Fatal(err)
	}
	if got := readUntil(t, second, "AGAIN", 5*time.Second); !strings.Contains(got, "AGAIN") {
		t.Errorf("the second attach's output = %q, want the program still answering", got)
	}
}

// TestTheHostReportsTheProgramsExit pins the end of a session: the record's
// state leaves running, the exit code is the program's, and the sequence
// number advanced — the heartbeat a list reads instead of polling.
func TestTheHostReportsTheProgramsExit(t *testing.T) {
	h, cl := startTestHost(t, Options{Path: "/bin/sh", Linger: -1})
	before := h.Session().Info().Seq
	if _, err := cl.Write([]byte("exit 3\r")); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info := h.Session().Info()
		if info.State != StateRunning {
			if info.State != StateExited || info.ExitCode != 3 {
				t.Fatalf("record after the program ended = %+v, want exited 3", info)
			}
			if info.Seq <= before {
				t.Errorf("seq = %d, want it advanced past %d", info.Seq, before)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the program never ended")
}

// TestTheAttachScreenComesBeforeLiveOutput pins the order an attach is served
// in: the record, then the screen the session holds, then what the program
// writes after it. A client given output first would draw a screen it is older
// than, and would then have to be given the screen again.
func TestTheAttachScreenComesBeforeLiveOutput(t *testing.T) {
	h, first := startTestHost(t, Options{Path: "/bin/sh", Linger: -1})
	writeAndWait(t, first, "echo SCREEN-MARK\r", "SCREEN-MARK")

	second, err := Dial(h.Socket())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()
	if info := second.Info(); info.State != StateRunning || info.Cols != 80 || info.Rows != 24 {
		t.Errorf("record on attach = %+v, want a running 80x24 session", info)
	}
	if got := readSome(t, second, 1); !strings.Contains(got, "SCREEN-MARK") {
		t.Fatalf("the attach's screen = %q, want what the program printed", got)
	}
	if _, err := second.Write([]byte("echo AFTER-MARK\r")); err != nil {
		t.Fatal(err)
	}
	if got := readUntil(t, second, "AFTER-MARK", 5*time.Second); !strings.Contains(got, "AFTER-MARK") {
		t.Errorf("output after the screen = %q, want the line typed after it", got)
	}
}

// TestTheAttachFrameOrder pins the attach's frame sequence at the wire: the
// record, the screen (zero or more chunks), the **empty screen frame** that
// ends it, and only then live output. That order is what lets a client tell
// "the screen" from "what came after it" without guessing; a host that wrote
// the terminator first would hand out an empty screen and leave the real one
// to arrive as live output.
func TestTheAttachFrameOrder(t *testing.T) {
	h, first := startTestHost(t, Options{Path: "/bin/sh", Linger: -1})
	writeAndWait(t, first, "echo ORDER-MARK\r", "ORDER-MARK")

	nc, err := net.Dial("unix", h.Socket())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Close() }()
	var tags []byte
	var screen []byte
	seenTerminator := false
	for i := 0; i < 64 && !seenTerminator; i++ {
		tag, payload, err := readFrame(nc, nil)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		tags = append(tags, tag)
		switch tag {
		case tagHeader:
			if i != 0 {
				t.Fatalf("frame %d is the record; it must come first", i)
			}
		case tagScreen:
			if len(payload) == 0 {
				seenTerminator = true
			} else {
				screen = append(screen, payload...)
			}
		case tagOutput:
			t.Fatalf("output at frame %d, before the screen ended", i)
		}
	}
	if !seenTerminator {
		t.Fatalf("no empty screen frame in %q", tags)
	}
	if !strings.Contains(string(screen), "ORDER-MARK") {
		t.Errorf("the screen = %q, want what the program printed before the attach", screen)
	}
}

// TestTheKillEndsTheProgram pins the one way a client ends a session: the
// record says killed, not exited, and the program is gone.
func TestTheKillEndsTheProgram(t *testing.T) {
	h, cl := startTestHost(t, Options{Path: "/bin/sh", Linger: -1})
	pid := h.Session().Info().Pid
	if err := cl.Kill(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info := h.Session().Info()
		if info.State == StateKilled {
			if err := processAlive(pid); err != nil {
				return
			}
			t.Fatalf("the record says killed but process %d is alive", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the record = %+v, want killed", h.Session().Info())
}

// startTestHost starts a host for a shell and attaches one client to it.
func startTestHost(t *testing.T, o Options) (*Host, *Client) {
	t.Helper()
	if o.Socket == "" {
		o.Socket = filepath.Join(t.TempDir(), "s.sock")
	}
	h, err := Start(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Stop)
	go func() { _ = h.Serve() }()
	cl, err := Dial(h.Socket())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return h, cl
}

// writeAndWait types a line and waits for the mark it should print.
func writeAndWait(t *testing.T, cl *Client, line, mark string) {
	t.Helper()
	if _, err := cl.Write([]byte(line)); err != nil {
		t.Fatal(err)
	}
	if got := readUntil(t, cl, mark, 5*time.Second); !strings.Contains(got, mark) {
		t.Fatalf("read %q, want %q", got, mark)
	}
}

// readUntil reads until needle shows or the window passes. The read is
// bounded by a socket deadline: a test that waits for a byte a program never
// writes must fail, not hang.
func readUntil(t *testing.T, cl *Client, needle string, window time.Duration) string {
	t.Helper()
	var out strings.Builder
	deadline := time.Now().Add(window)
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		if err := cl.c.SetReadDeadline(deadline); err != nil {
			t.Fatal(err)
		}
		n, err := cl.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			if strings.Contains(out.String(), needle) {
				return out.String()
			}
		}
		if err != nil {
			break
		}
	}
	_ = cl.c.SetReadDeadline(time.Time{})
	return out.String()
}

// readSome reads for at most a second, so a test can say "the screen, and not
// what comes after it".
func readSome(t *testing.T, cl *Client, seconds int) string {
	t.Helper()
	var out strings.Builder
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		if err := cl.c.SetReadDeadline(deadline); err != nil {
			t.Fatal(err)
		}
		n, err := cl.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	_ = cl.c.SetReadDeadline(time.Time{})
	return out.String()
}

// processAlive reports whether pid names a live process.
func processAlive(pid int) error {
	if pid <= 0 {
		return os.ErrProcessDone
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Signal(syscall.Signal(0))
}
