package termsession

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"
)

// P13-a: the session host's pins.
//
// These run a real program in a real pseudo-terminal on Linux and macOS, and
// they are the whole point of this slice: a session whose program lives
// outside the client. On Windows there is no pseudo-terminal yet (S1b), so the
// tests that need one skip **by platform** — the host reports ErrUnsupported
// there and nothing else is claimed (see TestTheHostRefusesToPretend).

// TestTheRingKeepsTheNewestBytesWithinItsBound pins the ring's one rule: it is
// bounded, and what it drops is the **oldest** bytes. A ring that kept the
// oldest would hand an attaching client the beginning of a session and not its
// end, which is the one thing a scrollback must not do.
func TestTheRingKeepsTheNewestBytesWithinItsBound(t *testing.T) {
	r := NewRing(16)
	r.Write([]byte("abcdefgh"))
	r.Write([]byte("ijklmnop"))
	if got := string(r.Bytes()); got != "abcdefghijklmnop" {
		t.Fatalf("ring = %q, want the whole run", got)
	}
	r.Write([]byte("qrstuvwx"))
	if got := string(r.Bytes()); got != "ijklmnopqrstuvwx" {
		t.Errorf("ring = %q, want the newest 16 bytes", got)
	}
	if r.Len() != 16 {
		t.Errorf("len = %d, want 16", r.Len())
	}
	// A single write larger than the ring keeps its tail, and no stale head
	// stays in the buffer.
	r.Write([]byte(strings.Repeat("0123456789", 4)))
	if got := string(r.Bytes()); got != "4567890123456789" {
		t.Errorf("ring after an oversized write = %q, want its last 16 bytes", got)
	}
	// A ring of nothing keeps nothing, and says so instead of allocating.
	empty := NewRing(-1)
	empty.Write([]byte("ignored"))
	if empty.Bytes() != nil {
		t.Errorf("ring with no limit holds %q", empty.Bytes())
	}
}

// TestTheFrameRoundTrips pins the wire shape: a tag, a four-byte length, the
// payload. Every frame the protocol has must survive a write and a read, and a
// header asking for more than the bound must be refused rather than allocated.
func TestTheFrameRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	tags := []byte{tagHeader, tagScreen, tagOutput, tagExit, tagLag}
	for _, tag := range tags {
		if err := writeFrame(&buf, tag, []byte("payload")); err != nil {
			t.Fatalf("%c: %v", tag, err)
		}
	}
	var scratch []byte
	for _, tag := range tags {
		gotTag, payload, err := readFrame(&buf, scratch)
		if err != nil {
			t.Fatalf("%c: %v", tag, err)
		}
		if gotTag != tag || string(payload) != "payload" {
			t.Errorf("read %c %q, want %c payload", gotTag, payload, tag)
		}
		scratch = payload
	}
	if err := writeFrame(&buf, tagOutput, make([]byte, frameSize+1)); err == nil {
		t.Error("a frame over the bound was written")
	}
	// A header whose length is over the bound is refused by the reader too:
	// the number is not a request for that many bytes.
	bad := []byte{tagOutput, 0xFF, 0xFF, 0xFF, 0xFF}
	if _, _, err := readFrame(bytes.NewReader(bad), nil); err == nil {
		t.Error("a header over the bound was accepted")
	}
}

// TestTheRecordRoundTrips pins the header payload: a session's name, state,
// exit code, size and seq go over the wire and come back the same.
func TestTheRecordRoundTrips(t *testing.T) {
	want := Info{ID: "zsh-9", Name: "zsh", State: StateExited, ExitCode: 3, Seq: 7, Cols: 120, Rows: 40, Pid: 9, Dir: "/tmp"}
	got, err := decodeInfo(encodeInfo(want))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("record = %+v, want %+v", got, want)
	}
	if _, err := decodeInfo([]byte("{")); err == nil {
		t.Error("a malformed record was accepted")
	}
}

// TestALaggingClientResyncsOnTheScreen pins the flow control's floor: a client
// whose queue is full is told to resync (a `L` frame) instead of being given an
// unbounded backlog, and it is told **once** — a client that never reads must
// not make the host push a `L` per frame for ever.
func TestALaggingClientResyncsOnTheScreen(t *testing.T) {
	s := &Session{clients: map[*conn]struct{}{}, info: Info{State: StateRunning}}
	c := &conn{send: make(chan frame, 2), done: make(chan struct{})}
	s.clients[c] = struct{}{}
	// Fill the client's queue, then broadcast past it.
	s.broadcast(tagOutput, []byte("a"))
	s.broadcast(tagOutput, []byte("b"))
	s.broadcast(tagOutput, []byte("c"))
	select {
	case f := <-c.send:
		if f.tag != tagLag {
			t.Fatalf("the lagging client got %c, want the resync notice %c", f.tag, tagLag)
		}
	default:
		t.Fatal("the lagging client got no resync notice")
	}
	// The queue now holds nothing but the notice: everything it had not read
	// is gone, which is what "never an unbounded backlog" means.
	if len(c.send) != 0 {
		t.Errorf("the lagging client's queue still holds %d frames", len(c.send))
	}
	// A further broadcast does not push a second notice.
	s.broadcast(tagOutput, []byte("d"))
	if len(c.send) != 1 {
		t.Errorf("the host pushed %d frames to a client it already warned", len(c.send))
	}
}

// TestAClientTakesTheScreenAfterALag pins the client's half of the resync: a
// `L` drops what the client had not drawn yet, and the screen that follows
// replaces it whole — the chunks are collected until the empty frame ends
// them, so the terminal is never handed half a screen as if it were output.
func TestAClientTakesTheScreenAfterALag(t *testing.T) {
	client, host := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = host.Close() }()
	cl := &Client{c: client, info: Info{State: StateRunning}}

	// The stale byte is written and read first, so the pin is about what
	// happens *after* the notice, not about a race with the read that was
	// already in flight.
	go func() { _ = writeFrame(host, tagOutput, []byte("stale")) }()
	if got := string(readFor(cl, 200*time.Millisecond)); got != "stale" {
		t.Fatalf("the first output = %q", got)
	}

	go func() {
		_ = writeFrame(host, tagLag, nil)
		_ = writeFrame(host, tagHeader, encodeInfo(Info{State: StateRunning, Seq: 9}))
		_ = writeFrame(host, tagScreen, []byte("SCREEN-A"))
		_ = writeFrame(host, tagScreen, []byte("SCREEN-B"))
		_ = writeFrame(host, tagScreen, nil)
		_ = writeFrame(host, tagOutput, []byte("live"))
	}()

	if got, want := string(readFor(cl, 2*time.Second)), "SCREEN-ASCREEN-Blive"; got != want {
		t.Errorf("after a lag the client drew %q, want %q", got, want)
	}
	if info := cl.Info(); info.Seq != 9 {
		t.Errorf("record after a lag = %+v, want the one in the resync's header", info)
	}
}

// TestAClientDetachLeavesTheProgram is the client's contract in one line: Close
// sends `D`, so the host keeps the program. The host's own pin
// (TestTheDetachKeepsTheProgram) proves the other half.
func TestAClientDetachLeavesTheProgram(t *testing.T) {
	client, host := net.Pipe()
	defer func() { _ = host.Close() }()
	cl := &Client{c: client}
	// net.Pipe is synchronous: the write waits for the read, so the read is
	// what runs here.
	got := make(chan byte, 1)
	go func() {
		tag, _, err := readFrame(host, nil)
		if err != nil {
			got <- 0
			return
		}
		got <- tag
	}()
	if err := cl.Close(); err != nil {
		t.Fatal(err)
	}
	if tag := <-got; tag != tagDetach {
		t.Errorf("Close sent %q, want the detach %q", tag, tagDetach)
	}
	// Close twice is Close once: a second detach would be a frame about a
	// connection that is already gone.
	if err := cl.Close(); err != nil {
		t.Errorf("the second Close = %v", err)
	}
}
