// Package termsession keeps a terminal session alive outside the app: a
// host process owns the pseudo-terminal and its program, and the app — or any
// other client — attaches to it over a socket, shows what the host sends, and
// types into it.
//
// The shape is the one a terminal server has, and the one the terminal
// plugin's own documentation names: `Terminal.Snapshot()`'s reader is "a
// terminal attaching again to a session a server keeps" — a terminal with
// **no program**. Here that is literally true: the client's terminal is
// built on `Options.Conn` (`mygo/plugins/terminal`), so it lays out the host's
// bytes and never starts a program of its own. Nothing a dead program left
// behind can reach a live one, which is exactly what the old in-process
// replay could not promise (see terminalui's note on `Snapshot()`).
//
// The session is also what a program's own state is kept in: the host holds
// the last screen and a bounded scrollback, so an attach gets the screen
// first (`S`) and only the output written after it (`O`), never a full
// re-export of the program's terminal modes. That is the difference between
// "the client draws the host's screen" and "the client's terminal is fed a
// program's escape sequences".
package termsession

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// State is what became of a session's program.
type State string

const (
	// StateStarting is a session whose program has not started yet.
	StateStarting State = "starting"
	// StateRunning is a program that is alive.
	StateRunning State = "running"
	// StateExited is a program that ended on its own.
	StateExited State = "exited"
	// StateKilled is a program the user ended (a close, a kill).
	StateKilled State = "killed"
	// StateLost is a program the host no longer has: it crashed, or the
	// host did. A client attaching to a lost session gets a new shell, and
	// never the old command re-run (see host.go).
	StateLost State = "lost"
)

// Info is a session's record: what a client shows, and what a list would.
//
// Seq is monotonic and rises on every change of the record, so a client that
// has fallen behind can tell which snapshot it holds and whether it is older
// than the one in hand — the heartbeat the protocol has instead of polling.
type Info struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	State    State  `json:"state"`
	ExitCode int    `json:"exitCode,omitempty"`
	Seq      uint64 `json:"seq"`
	Cols     int    `json:"cols"`
	Rows     int    `json:"rows"`
	Pid      int    `json:"pid,omitempty"`
	Dir      string `json:"dir,omitempty"`
}

// ErrUnsupported is returned on a platform with no pseudo-terminal of its
// own (Windows: ConPTY is a separate slice, and it must not be faked).
var ErrUnsupported = errors.New("termsession: no pseudo-terminal on this platform")

// frame tags. A host sends H (the record, once per attach), S (the screen at
// the size the host's terminal is, once per attach and after a lag), O
// (output written after that screen), E (the program ended, with its record),
// and L (you fell behind: a screen follows).
//
// A client sends I (input, typed bytes), R (the size of the client's screen),
// D (detach: keep the program), and K (kill the program).
const (
	tagHeader = 'H'
	tagScreen = 'S'
	tagOutput = 'O'
	tagExit   = 'E'
	tagLag    = 'L'
	tagInput  = 'I'
	tagResize = 'R'
	tagDetach = 'D'
	tagKill   = 'K'
	// frameSize bounds a frame: the screen and the output are chunks, and
	// a chunk larger than this is split. It keeps a malformed or hostile
	// header from asking for an unbounded allocation.
	frameSize = 1 << 20
)

// writeFrame writes one frame: the tag, the payload's length, the payload.
// A payload longer than frameSize is the caller's bug (the host splits).
func writeFrame(w interface{ Write([]byte) (int, error) }, tag byte, payload []byte) error {
	if len(payload) > frameSize {
		return fmt.Errorf("termsession: frame of %d bytes", len(payload))
	}
	head := make([]byte, 0, 5)
	head = append(head, tag, 0, 0, 0, 0)
	putUint32(head[1:], uint32(len(payload)))
	if _, err := w.Write(head); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

// putUint32 writes v big-endian into b, which must hold four bytes.
func putUint32(b []byte, v uint32) {
	b[0], b[1], b[2], b[3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v)
}

// readFrame reads one frame's tag and payload. A payload larger than
// frameSize, or a header that does not end, is an error rather than an
// allocation.
func readFrame(r interface{ Read([]byte) (int, error) }, buf []byte) (byte, []byte, error) {
	head := make([]byte, 5)
	if _, err := readFull(r, head); err != nil {
		return 0, nil, err
	}
	n := int(head[1])<<24 | int(head[2])<<16 | int(head[3])<<8 | int(head[4])
	if n < 0 || n > frameSize {
		return 0, nil, fmt.Errorf("termsession: frame of %d bytes", n)
	}
	if n > cap(buf) {
		buf = make([]byte, n)
	}
	buf = buf[:n]
	if n > 0 {
		if _, err := readFull(r, buf); err != nil {
			return 0, nil, err
		}
	}
	return head[0], buf, nil
}

// readFull is io.ReadFull without importing io for one call.
func readFull(r interface{ Read([]byte) (int, error) }, b []byte) (int, error) {
	n := 0
	for n < len(b) {
		m, err := r.Read(b[n:])
		n += m
		if err != nil {
			return n, err
		}
		if m == 0 {
			return n, errNoProgress
		}
	}
	return n, nil
}

var errNoProgress = errors.New("termsession: no progress reading a frame")

// encodeInfo is a record as a header payload.
func encodeInfo(info Info) []byte {
	data, err := json.Marshal(info)
	if err != nil {
		return nil
	}
	return data
}

// decodeInfo reads a record from a header payload.
func decodeInfo(payload []byte) (Info, error) {
	var info Info
	err := json.Unmarshal(payload, &info)
	return info, err
}

// defaults for a host.
const (
	// DefaultRing is how much output a session keeps for an attach: the
	// newest bytes, the oldest dropped. 4 MiB is the reference daemon's
	// limit for a client's backlog, and the same order as the plugin's own
	// 10 MB scrollback, which the screen export carries separately.
	DefaultRing = 4 << 20
	// DefaultLinger is how long a host with no client waits for one before
	// it ends its program: long enough to restart the app, short enough
	// that a forgotten session does not hold a shell for ever.
	DefaultLinger = 30 * time.Second
	// clientQueue is how many frames a client holds before it is behind;
	// the reference's 4 MiB backlog is this in bytes, and this many frames
	// is the same order for a terminal that writes a frame per read.
	clientQueue = 256
)
