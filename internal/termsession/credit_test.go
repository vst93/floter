package termsession

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"
)

// P13-b: the protocol's flow control, by bytes.
//
// The old bound was a frame count: a client was given clientQueue frames and
// then told to resync. That is a bound on frames, not on bytes, and a frame may
// be a megabyte — so the host's "bounded" backlog was larger than the ring it
// keeps for the whole session. The reference daemon bounds it by credit
// instead: a client is handed a window, says how much of it it consumed, and a
// client that stops reading costs the host the window and nothing more.
//
// These tests are the two halves of that: the host spends the window and
// resyncs when it is gone, and the client hands it back as its terminal draws.

// quietConn is a client connection for the session's own tests: a queue of its
// own and a socket that is never written to.
func quietConn(t *testing.T, queue int) *conn {
	t.Helper()
	client, host := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = host.Close()
	})
	c := newConn(host)
	c.send = make(chan frame, queue)
	return c
}

// TestTheHostSendsOnlyWhatTheClientTook pins the byte bound: a client is given
// one window of output, and the frame after it is the resync notice — never the
// overflow. "Behind" is therefore something the host knows about the client's
// terminal, not something it guesses from its own socket.
func TestTheHostSendsOnlyWhatTheClientTook(t *testing.T) {
	s := &Session{clients: map[*conn]struct{}{}, info: Info{State: StateRunning}}
	c := quietConn(t, 256)
	s.clients[c] = struct{}{}

	const chunk = 64 << 10
	payload := bytes.Repeat([]byte("x"), chunk)
	// A client that reads and never hands the window back: the frames leave
	// the queue (so the frame count is not the bound that fires), and what
	// the host is still willing to write is the credit alone.
	var sent, frames int
	var notice bool
	for i := 0; i < 64 && !notice; i++ {
		s.broadcast(tagOutput, payload)
		for len(c.send) > 0 {
			f := <-c.send
			if f.tag == tagLag {
				notice = true
				continue
			}
			if notice {
				t.Fatalf("frame %c after the resync notice", f.tag)
			}
			if f.tag != tagOutput {
				t.Fatalf("frame %c, want output", f.tag)
			}
			frames++
			sent += len(f.payload)
		}
	}
	if !notice {
		t.Fatalf("the host sent %d frames (%d bytes) and never resynced the client", frames, sent)
	}
	if sent > clientWindow {
		t.Errorf("the host sent %d bytes of output to a client that took none, want at most the %d-byte window", sent, clientWindow)
	}
	if sent != clientWindow {
		t.Errorf("the host sent %d bytes, want the whole window (%d) and no more", sent, clientWindow)
	}
	// A client that was warned takes nothing until it is handed the screen,
	// whatever the next frames are: those bytes are in the ring the screen is
	// taken from, so queueing them as well would draw them twice.
	s.broadcast(tagOutput, []byte("after"))
	if len(c.send) != 0 {
		t.Errorf("the warned client's queue holds %d frames, want none", len(c.send))
	}
}

// TestTheClientGivesBackWhatItTook pins the client's half: the window is spent
// against what the terminal **drew**, so the credit goes back on Read, and it
// goes back in batches — one frame per creditFlush bytes, not one per read.
func TestTheClientGivesBackWhatItTook(t *testing.T) {
	client, host := net.Pipe()
	defer func() { _ = host.Close() }()
	cl := &Client{c: client}
	cl.pending = bytes.Repeat([]byte("y"), 2*creditFlush)
	defer func() { _ = client.Close() }()

	// The host's side reads one frame: the credit for the bytes Read handed
	// out. It is read before the bytes are consumed, so net.Pipe's synchronous
	// write has a reader waiting for it.
	got := make(chan int, 1)
	fail := make(chan error, 1)
	go func() {
		_ = host.SetReadDeadline(time.Now().Add(2 * time.Second))
		tag, payload, err := readFrame(host, nil)
		if err != nil {
			fail <- err
			return
		}
		if tag != tagCredit {
			fail <- errNotCredit(tag)
			return
		}
		got <- decodeCredit(payload)
	}()

	buf := make([]byte, creditFlush)
	n, err := cl.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != creditFlush {
		t.Fatalf("Read = %d bytes, want %d", n, creditFlush)
	}
	select {
	case err := <-fail:
		t.Fatal(err)
	case grant := <-got:
		if grant != creditFlush {
			t.Errorf("credit = %d bytes, want the %d the terminal took", grant, creditFlush)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the client never gave its window back")
	}
}

// errNotCredit is a frame that was not the credit the pin expects.
func errNotCredit(tag byte) error {
	return fmt.Errorf("termsession: got frame %q, want credit", string(tag))
}

// TestAStalledClientIsRedrawnOncePerBacklog pins the second level: spent credit
// is pacing, not "behind". A client that is drawing a screen is out of credit
// for the frames that arrive while it draws, and answering each of those with a
// screen would send a megabyte per frame — the amplification P13-b measured on
// a real pty (18.8 MB delivered for a 1.9 MB output). The host therefore drops
// output until clientBacklog has accumulated, and redraws once.
func TestAStalledClientIsRedrawnOncePerBacklog(t *testing.T) {
	s := &Session{clients: map[*conn]struct{}{}, info: Info{State: StateRunning}}
	c := quietConn(t, 4096)
	s.clients[c] = struct{}{}
	// Spend the window, then keep writing: the drops below clientBacklog are
	// pacing and must not produce a notice.
	payload := bytes.Repeat([]byte("z"), 64<<10)
	for len(c.send) < 4 {
		s.broadcast(tagOutput, payload)
	}
	for len(c.send) > 0 {
		if f := <-c.send; f.tag == tagLag {
			t.Fatal("the client was told to resync while it was merely pacing")
		}
	}
	// One mebibyte of drops and it is behind for real: exactly one notice,
	// once, not one per frame.
	dropped := 0
	for dropped < clientBacklog {
		s.broadcast(tagOutput, payload)
		dropped += len(payload)
	}
	if got := len(c.send); got != 1 {
		t.Fatalf("the stalled client's queue holds %d frames, want just the resync notice", got)
	}
	if f := <-c.send; f.tag != tagLag {
		t.Errorf("the stalled client got %c, want the resync notice %c", f.tag, tagLag)
	}
	for i := 0; i < 4; i++ {
		s.broadcast(tagOutput, payload)
	}
	if got := len(c.send); got != 0 {
		t.Errorf("the host pushed %d frames to a client it already warned", got)
	}
}

// TestAClientCannotWidenItsOwnWindow pins the ceiling on a hand-back: a client
// that reports more than it was given cannot make the host hold more than one
// window for it. Without the clamp, a client (or a broken one) could ask for a
// gigabyte and the host would queue it.
func TestAClientCannotWidenItsOwnWindow(t *testing.T) {
	c := quietConn(t, 4)
	c.grant(clientWindow * 8)
	if got := c.credit; got != clientWindow {
		t.Errorf("credit after an oversized hand-back = %d, want the window %d", got, clientWindow)
	}
	c.grant(-1)
	if got := c.credit; got != clientWindow {
		t.Errorf("credit after a negative hand-back = %d, want it unchanged", got)
	}
}

// TestTheClientWritesOneFrameAtATime pins the write lock. A frame is a header
// and a payload in two writes, and the terminal really does write from two
// goroutines at once: mygo asks Conn for a resize from the goroutine its
// tellConn starts, while the user's typing goes out on another. Without one
// lock around the pair, two frames interleave and the host reads a header that
// describes the wrong payload.
//
// The socket here yields after every write, because a real one can: a write to
// a socket with a full buffer waits, and any other goroutine may write in the
// meantime. A pin that relied on catching a lucky preemption between the two
// writes would be a flake that passes on a quiet machine and fails on a busy
// one; this one fails whenever the lock is missing.
func TestTheClientWritesOneFrameAtATime(t *testing.T) {
	client, host := net.Pipe()
	defer func() { _ = host.Close() }()
	cl := &Client{c: yieldingConn{client}}
	defer func() { _ = client.Close() }()

	const writers, each = 8, 24
	type read struct {
		tags map[byte]int
		err  error
	}
	done := make(chan read, 1)
	go func() {
		tags := map[byte]int{}
		var err error
		for i := 0; i < writers*each; i++ {
			tag, payload, rerr := readFrame(host, nil)
			if rerr != nil {
				err = rerr
				break
			}
			switch tag {
			case tagInput:
				if string(payload) != "typed" {
					err = errBentPayload(len(payload))
				}
			case tagResize:
				if len(payload) != 8 && len(payload) != 4 {
					err = errBentPayload(len(payload))
				}
			default:
				err = errNotCredit(tag)
			}
			if err != nil {
				break
			}
			tags[tag]++
		}
		done <- read{tags, err}
		// Keep the pipe drained, so a read that failed halfway cannot leave a
		// writer blocked for ever: this pin must fail, not hang.
		_, _ = io.Copy(io.Discard, host)
	}()

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < each; j++ {
				if i%2 == 0 {
					_, _ = cl.Write([]byte("typed"))
					continue
				}
				_ = cl.ResizePixels(80+i, 24, 9, 18)
			}
		}(i)
	}
	wg.Wait()

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("the host could not read the frames the client wrote: %v", got.err)
		}
		if got.tags[tagInput] == 0 || got.tags[tagResize] == 0 {
			t.Errorf("tags = %v, want both input and resize frames", got.tags)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the host never finished reading the client's frames")
	}
}

// errBentPayload is a frame whose payload does not match its tag.
func errBentPayload(n int) error {
	return fmt.Errorf("termsession: frame payload of %d bytes belongs to another tag", n)
}

// yieldingConn is a socket that lets the scheduler run after every write, which
// is what a real socket does when its buffer is full.
type yieldingConn struct{ net.Conn }

func (c yieldingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	runtime.Gosched()
	return n, err
}
