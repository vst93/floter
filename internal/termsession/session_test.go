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

// TestAFullQueueDropsTheFrameInsteadOfGrowingTheBacklog pins the second bound a
// client is held to: the queue of frames. Credit bounds what a client is given
// in bytes; this bounds how many frames may be waiting whatever their size, so a
// program writing tiny frames cannot make the host grow a queue per client.
//
// P13-b 重定基线：旧行为是「队列满 = 立刻一条 `L`」，现在是「队列满 = 丢掉这一帧」——
// 丢掉的字节本来就在环形缓冲里，而重绘的时机归 clientBacklog 管
// （TestAStalledClientIsRedrawnOncePerBacklog）。旧名字叫
// TestALaggingClientResyncsOnTheScreen，它守的其实是「队列有界」，不是「重绘」。
func TestAFullQueueDropsTheFrameInsteadOfGrowingTheBacklog(t *testing.T) {
	s := &Session{clients: map[*conn]struct{}{}, info: Info{State: StateRunning}}
	c := &conn{send: make(chan frame, 2), done: make(chan struct{}), credit: clientWindow}
	s.clients[c] = struct{}{}
	s.broadcast(tagOutput, []byte("a"))
	s.broadcast(tagOutput, []byte("b"))
	s.broadcast(tagOutput, []byte("c"))
	// The queue holds the two frames it had room for, in order, and the third
	// was dropped rather than queued behind them.
	if len(c.send) != 2 {
		t.Fatalf("the queue holds %d frames, want the 2 it has room for", len(c.send))
	}
	if f := <-c.send; string(f.payload) != "a" {
		t.Errorf("first frame = %q, want the first broadcast", f.payload)
	}
	if f := <-c.send; string(f.payload) != "b" {
		t.Errorf("second frame = %q, want the second broadcast", f.payload)
	}
	// And no notice yet: one frame is pacing, not "behind".
	if len(c.send) != 0 {
		t.Errorf("the host pushed %d frames to a client that only lost one frame", len(c.send))
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
