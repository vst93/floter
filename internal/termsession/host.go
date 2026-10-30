package termsession

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Options describe a session to host.
type Options struct {
	// Path is the program, Args its arguments with Args[0] what the program
	// sees as its name ("-zsh" starts a login shell).
	Path string
	Args []string
	// Dir is the working directory, Env the environment ("KEY=value").
	Dir string
	Env []string
	// Name is what a list shows; empty means the program's own name.
	Name string
	// Cols and Rows are the size the program starts at. Zero means 80×24,
	// and the first client that attaches with a size resizes it.
	Cols, Rows int
	// Ring is how many bytes of output to keep for an attach. Zero means
	// DefaultRing, negative means keep nothing above the screen.
	Ring int
	// Linger is how long the host waits for a client before it ends the
	// program. Zero means DefaultLinger, negative means wait for ever.
	Linger time.Duration
	// Socket is where the host listens. Empty means a name of its own under
	// the user's cache directory.
	Socket string
}

// Ring is the bounded scrollback a session keeps: the newest bytes, the
// oldest dropped. It is what an attach is handed before live output, and the
// reason a host never has to grow with a program's output.
type Ring struct {
	mu    sync.Mutex
	buf   []byte
	limit int
	// head is where the next byte goes, n is how many of buf hold bytes.
	head, n int
}

// NewRing makes a ring of limit bytes; limit < 0 keeps nothing.
func NewRing(limit int) *Ring {
	if limit < 0 {
		limit = 0
	}
	return &Ring{buf: make([]byte, limit), limit: limit}
}

// Write adds p, dropping the oldest bytes to stay within the limit.
func (r *Ring) Write(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.limit == 0 {
		return
	}
	if len(p) > r.limit {
		// Everything older is dropped, and so is the head of p.
		p = p[len(p)-r.limit:]
		r.head, r.n = 0, 0
	}
	for len(p) > 0 {
		room := r.limit - r.head
		if room > len(p) {
			room = len(p)
		}
		copy(r.buf[r.head:], p[:room])
		r.head = (r.head + room) % r.limit
		r.n += room
		p = p[room:]
	}
	if r.n > r.limit {
		r.n = r.limit
	}
}

// Bytes returns what the ring holds, oldest first: the program's output since
// the oldest byte still kept.
func (r *Ring) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.n == 0 {
		return nil
	}
	out := make([]byte, r.n)
	start := (r.head - r.n + r.limit*2) % r.limit
	if start+len(out) <= r.limit {
		copy(out, r.buf[start:start+len(out)])
		return out
	}
	copy(out, r.buf[start:])
	copy(out[len(r.buf)-start:], r.buf[:r.head])
	return out
}

// Len is how many bytes the ring holds.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.n
}

// Session is a hosted session: the program, its output, and who is attached.
type Session struct {
	mu      sync.Mutex
	info    Info
	ring    *Ring
	pty     *PTY
	clients map[*conn]struct{}
	// closed marks a session the host has stopped serving: a client that
	// attaches then is told the program is gone.
	closed bool
}

// conn is one attached client: its socket and the queue the program's output
// waits in. A client that falls behind (more than clientQueue frames) is told
// to resync instead of being given an unbounded backlog.
type conn struct {
	c      net.Conn
	send   chan frame
	done   chan struct{}
	once   sync.Once
	mu     sync.Mutex
	lagged bool
}

type frame struct {
	tag     byte
	payload []byte
}

func (c *conn) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.c.Close()
	})
}

// Info returns the session's record.
func (s *Session) Info() Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.info
}

// Seq returns the record's sequence number, which rises on every change.
func (s *Session) Seq() uint64 { return s.Info().Seq }

// bump raises the sequence number; s.mu is held.
func (s *Session) bump() { s.info.Seq++ }

// broadcast sends a frame to every attached client; s.mu is held.
func (s *Session) broadcast(tag byte, payload []byte) {
	for c := range s.clients {
		select {
		case c.send <- frame{tag, payload}:
		default:
			// The client is behind: drop what it has not read and make it
			// take the screen again. Never an unbounded backlog — a program
			// writing a gigabyte while nobody reads is exactly the case a
			// server has to survive.
			c.mu.Lock()
			if !c.lagged {
				c.lagged = true
				drain(c.send)
				select {
				case c.send <- frame{tagLag, nil}:
				default:
				}
			}
			c.mu.Unlock()
		}
	}
}

// drain empties a client's queue without blocking.
func drain(ch chan frame) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// screen is the frame an attach starts with: the ring's bytes, chunked.
func (s *Session) screen() [][]byte {
	data := s.ring.Bytes()
	if len(data) == 0 {
		return nil
	}
	var out [][]byte
	for len(data) > 0 {
		n := len(data)
		if n > frameSize {
			n = frameSize
		}
		out = append(out, append([]byte(nil), data[:n]...))
		data = data[n:]
	}
	return out
}

// Host is a session host: it listens on a socket and serves the session.
type Host struct {
	opts    Options
	pty     *PTY
	ring    *Ring
	session *Session
	ln      net.Listener
	// listeners tracks the per-client goroutines so Stop can wait for them.
	wg sync.WaitGroup
	// attach and detach count clients, for the linger timer.
	clients int
	mu      sync.Mutex
	exited  chan struct{}
	once    sync.Once
}

// Run hosts a session until its program ends (and no client is left) or the
// host is stopped. It is the body of the host process: the program it starts
// is the user's, and it outlives every client that attaches.
func Run(o Options) error {
	h, err := Start(o)
	if err != nil {
		return err
	}
	defer h.Stop()
	return h.Serve()
}

// Start builds a host: socket first, so a caller learns the name before the
// program runs, then the program itself.
func Start(o Options) (*Host, error) {
	if strings.TrimSpace(o.Path) == "" {
		return nil, errors.New("termsession: no program to host")
	}
	if o.Cols <= 0 {
		o.Cols = 80
	}
	if o.Rows <= 0 {
		o.Rows = 24
	}
	limit := o.Ring
	if limit == 0 {
		limit = DefaultRing
	}
	if o.Linger == 0 {
		o.Linger = DefaultLinger
	}
	if o.Socket == "" {
		o.Socket = DefaultSocket()
	}
	if err := os.MkdirAll(filepath.Dir(o.Socket), 0o700); err != nil {
		return nil, fmt.Errorf("termsession: %w", err)
	}
	// A socket left by a host that died is not a host: a stale one refuses
	// the next host's bind, so it goes. The mode keeps the session to the
	// user who started it — a terminal is everything that user can do.
	_ = os.Remove(o.Socket)
	ln, err := net.Listen("unix", o.Socket)
	if err != nil {
		return nil, fmt.Errorf("termsession: listening: %w", err)
	}
	if err := os.Chmod(o.Socket, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("termsession: %w", err)
	}
	pty, err := startPTY(o, o.Cols, o.Rows)
	if err != nil {
		_ = ln.Close()
		_ = os.Remove(o.Socket)
		return nil, err
	}
	name := o.Name
	if name == "" {
		name = filepath.Base(o.Path)
	}
	h := &Host{
		opts: o, pty: pty, ring: NewRing(limit), ln: ln,
		exited: make(chan struct{}),
	}
	h.session = &Session{
		ring:    h.ring,
		pty:     pty,
		clients: map[*conn]struct{}{},
		info: Info{
			ID:    fmt.Sprintf("%s-%d", name, pty.Pid()),
			Name:  name,
			State: StateRunning,
			Seq:   1,
			Cols:  o.Cols,
			Rows:  o.Rows,
			Pid:   pty.Pid(),
			Dir:   o.Dir,
		},
	}
	go h.read()
	go h.watch()
	return h, nil
}

// Socket returns where the host listens.
func (h *Host) Socket() string { return h.opts.Socket }

// Session returns the hosted session.
func (h *Host) Session() *Session { return h.session }

// read moves the program's output into the ring and to the clients.
func (h *Host) read() {
	buf := make([]byte, 32<<10)
	for {
		n, err := h.pty.Read(buf)
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			h.ring.Write(data)
			h.session.mu.Lock()
			for len(data) > 0 {
				chunk := len(data)
				if chunk > frameSize {
					chunk = frameSize
				}
				h.session.broadcast(tagOutput, data[:chunk])
				data = data[chunk:]
			}
			h.session.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

// watch waits for the program to end and tells everyone: the record carries
// why (exited, or killed when a client asked), and the frame is the last one.
func (h *Host) watch() {
	<-h.pty.Exited()
	h.session.mu.Lock()
	if h.session.closed {
		h.session.mu.Unlock()
		return
	}
	h.session.bump()
	if h.session.info.State == StateKilled {
		h.session.info.State = StateKilled
	} else {
		h.session.info.State = StateExited
	}
	h.session.info.ExitCode = h.pty.ExitCode()
	h.session.broadcast(tagExit, encodeInfo(h.session.info))
	h.session.mu.Unlock()
	h.once.Do(func() { close(h.exited) })
}

// serve accepts clients until the program ends and the last client detaches,
// or the linger window passes with none attached.
func (h *Host) Serve() error {
	defer func() { _ = h.ln.Close() }()
	go func() {
		for {
			c, err := h.ln.Accept()
			if err != nil {
				return
			}
			h.wg.Add(1)
			go func() {
				defer h.wg.Done()
				h.attach(c)
			}()
		}
	}()
	// The program's lifetime is the session's: when it ends, the host waits
	// for whoever is still attached to read the last frame, and then goes.
	idle := time.NewTimer(h.linger())
	defer idle.Stop()
	for {
		select {
		case <-h.exited:
			return h.waitForClients()
		case <-idle.C:
			if h.clientCount() > 0 {
				idle.Reset(h.linger())
				continue
			}
			// Nothing is attached and nothing will be: a program left alone
			// behind a dead app is a process nobody can reach, so it ends
			// with the host. A session meant to outlive the app passes a
			// negative Linger and never takes this branch.
			if h.opts.Linger < 0 {
				idle.Reset(h.linger())
				continue
			}
			_ = h.pty.Kill()
			<-h.exited
			return h.waitForClients()
		}
	}
}

// waitForClients waits for the last attached client to go, at most one linger
// window: a client that vanished without closing (a killed app) must not hold
// the socket open for ever.
func (h *Host) waitForClients() error {
	deadline := time.Now().Add(h.linger())
	for time.Now().Before(deadline) {
		if h.clientCount() == 0 {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

// clientCount is how many clients are attached.
func (h *Host) clientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.clients
}

// linger is how long the host waits with no client: a negative Linger means
// it waits for ever, which a caller asks for with a large window.
func (h *Host) linger() time.Duration {
	if h.opts.Linger < 0 {
		return 24 * time.Hour
	}
	return h.opts.Linger
}

// attach serves one client: the record, the screen, then live output, and its
// input back into the program.
func (h *Host) attach(nc net.Conn) {
	c := &conn{c: nc, send: make(chan frame, clientQueue), done: make(chan struct{})}
	h.mu.Lock()
	h.clients++
	h.mu.Unlock()
	defer func() {
		c.close()
		h.mu.Lock()
		h.clients--
		h.mu.Unlock()
	}()

	// The client joins the broadcast before the screen is taken, so that no
	// output written between the two is lost: everything after the screen is
	// in its queue, and everything before it is in the ring the screen comes
	// from.
	h.session.mu.Lock()
	if h.session.closed {
		h.session.mu.Unlock()
		return
	}
	h.session.clients[c] = struct{}{}
	info := h.session.info
	screen := h.session.screen()
	h.session.mu.Unlock()
	defer func() {
		h.session.mu.Lock()
		delete(h.session.clients, c)
		h.session.mu.Unlock()
	}()

	go h.writeLoop(c, info, screen)
	h.readLoop(c)
}

// writeLoop is the client's outgoing half.
func (h *Host) writeLoop(c *conn, info Info, screen [][]byte) {
	write := func(tag byte, payload []byte) error {
		return writeFrame(c.c, tag, payload)
	}
	if err := write(tagHeader, encodeInfo(info)); err != nil {
		c.close()
		return
	}
	for _, chunk := range screen {
		if err := write(tagScreen, chunk); err != nil {
			c.close()
			return
		}
	}
	// An empty screen frame ends it: a client knows the screen is whole and
	// that everything after it is live output, without having to wait for
	// the program to say something.
	if err := write(tagScreen, nil); err != nil {
		c.close()
		return
	}
	for {
		select {
		case f := <-c.send:
			if f.tag == tagLag {
				// Behind: take the screen again, then go on with what is
				// written after it. The ring is the screen, so a client that
				// fell behind is a client that re-attached.
				h.session.mu.Lock()
				info := h.session.info
				chunks := h.session.screen()
				h.session.mu.Unlock()
				c.mu.Lock()
				c.lagged = false
				c.mu.Unlock()
				if err := write(tagHeader, encodeInfo(info)); err != nil {
					c.close()
					return
				}
				for _, chunk := range chunks {
					if err := write(tagScreen, chunk); err != nil {
						c.close()
						return
					}
				}
				if err := write(tagScreen, nil); err != nil {
					c.close()
					return
				}
				continue
			}
			if err := write(f.tag, f.payload); err != nil {
				c.close()
				return
			}
		case <-c.done:
			return
		}
	}
}

// readLoop is the client's incoming half: input, a resize, a detach, a kill.
func (h *Host) readLoop(c *conn) {
	var buf []byte
	for {
		tag, payload, err := readFrame(c.c, buf)
		if err != nil {
			return
		}
		buf = payload
		switch tag {
		case tagInput:
			_, _ = h.pty.Write(payload)
		case tagResize:
			cols, rows := decodeSize(payload)
			if cols <= 0 || rows <= 0 {
				continue
			}
			_ = h.pty.Resize(cols, rows)
			h.session.mu.Lock()
			h.session.bump()
			h.session.info.Cols, h.session.info.Rows = cols, rows
			h.session.mu.Unlock()
		case tagDetach:
			return
		case tagKill:
			h.session.mu.Lock()
			h.session.info.State = StateKilled
			h.session.mu.Unlock()
			_ = h.pty.Kill()
			return
		}
	}
}

// decodeSize reads a resize payload: two big-endian uint16s.
func decodeSize(payload []byte) (int, int) {
	if len(payload) < 4 {
		return 0, 0
	}
	return int(payload[0])<<8 | int(payload[1]), int(payload[2])<<8 | int(payload[3])
}

// encodeSize writes a resize payload.
func encodeSize(cols, rows int) []byte {
	if cols > 0xFFFF {
		cols = 0xFFFF
	}
	if rows > 0xFFFF {
		rows = 0xFFFF
	}
	return []byte{byte(cols >> 8), byte(cols), byte(rows >> 8), byte(rows)}
}

// stop ends the host: the socket goes, and the program with it unless a
// client already detached from a running one.
func (h *Host) Stop() {
	h.session.mu.Lock()
	h.session.closed = true
	for c := range h.session.clients {
		c.close()
	}
	h.session.mu.Unlock()
	_ = h.ln.Close()
	_ = h.pty.Close()
	_ = os.Remove(h.opts.Socket)
	h.wg.Wait()
}

// DefaultSocket is where a session of this user lives: under the user's
// cache directory, named by the program, so two shells are two sockets.
func DefaultSocket() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "floter", "terminal", fmt.Sprintf("session-%d.sock", os.Getpid()))
}

// Client is an attached session: what the host sends shows, what is typed
// goes to the program. It is `terminal.Options.Conn`: a terminal built on it
// draws the host's screen and starts no program of its own.
type Client struct {
	c    net.Conn
	info Info
	// mu guards pending, the bytes read from the host and not yet handed to
	// the terminal.
	mu      sync.Mutex
	pending []byte
	// closed is set by Close: the host keeps the program, the client leaves.
	closed bool
}

// Dial attaches to a host at path. It reads the record and the screen before
// it returns, so the terminal built on it shows the session from its first
// frame rather than an empty one.
//
// The attach's frames are the record, the screen, and then live output — and
// the screen is what ends at the **empty screen frame** the host writes after
// it. That terminator is what makes "the screen, and not what came after it"
// a fact a client can check instead of a race: without it, a client would have
// to stop reading at some point and call the rest the screen.
func Dial(path string) (*Client, error) {
	nc, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("termsession: attaching: %w", err)
	}
	cl := &Client{c: nc}
	info, screen, err := readAttach(nc)
	if err != nil {
		_ = nc.Close()
		return nil, fmt.Errorf("termsession: attaching: %w", err)
	}
	cl.info = info
	cl.pending = screen
	return cl, nil
}

// readAttach reads an attach's opening frames: the record, the screen, and
// then the live output that arrived while the screen was being sent.
//
// The screen ends at the host's **empty screen frame**; that is what makes
// "the screen, and not what came after it" something the client checks rather
// than guesses. The live output is kept in order and handed out after the
// screen, so a client never draws output it is older than.
func readAttach(r interface{ Read([]byte) (int, error) }) (Info, []byte, error) {
	var info Info
	var screen []byte
	var live [][]byte
	for {
		tag, payload, err := readFrame(r, nil)
		if err != nil {
			return info, nil, err
		}
		switch tag {
		case tagHeader, tagExit:
			if got, err := decodeInfo(payload); err == nil {
				info = got
			}
		case tagScreen:
			if len(payload) == 0 {
				for _, chunk := range live {
					screen = append(screen, chunk...)
				}
				return info, screen, nil
			}
			screen = append(screen, payload...)
		case tagOutput:
			live = append(live, append([]byte(nil), payload...))
		}
	}
}

// Info is the session's record as of the last frame the client read.
func (c *Client) Info() Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.info
}

// Read hands the terminal the screen first and then the program's output,
// which is what `Options.Conn`'s reader is for.
//
// The host writes a screen after a lag too, and that screen replaces nothing
// here: the client has drawn every byte it was handed (pending is only filled
// by a frame it just read, and the loop hands it out before reading again), so
// a resync's screen is simply the next thing the terminal draws — which is the
// whole point of the notice: what the host could not send was never drawn.
func (c *Client) Read(p []byte) (int, error) {
	for {
		c.mu.Lock()
		if len(c.pending) > 0 {
			n := copy(p, c.pending)
			c.pending = c.pending[n:]
			c.mu.Unlock()
			return n, nil
		}
		c.mu.Unlock()

		tag, payload, err := readFrame(c.c, nil)
		if err != nil {
			// A read error is the caller's to see, not a state: the socket
			// may have a deadline on it (a test's, or a client that reads in
			// windows), and a deadline is not the end of the session.
			return 0, err
		}
		c.mu.Lock()
		switch tag {
		case tagHeader, tagExit:
			if info, err := decodeInfo(payload); err == nil {
				c.info = info
			}
		case tagScreen:
			// The chunks of a screen arrive in order, and the empty frame
			// that ends them is a frame with nothing to draw.
			c.pending = append(c.pending, payload...)
		case tagOutput:
			c.pending = append(c.pending, payload...)
		}
		c.mu.Unlock()
	}
}

// Write types into the program.
func (c *Client) Write(p []byte) (int, error) {
	if err := writeFrame(c.c, tagInput, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Resize tells the program the client's screen changed, which is what a
// `terminal.Options.Conn` with Resize is asked to do.
func (c *Client) Resize(cols, rows int) error {
	return writeFrame(c.c, tagResize, encodeSize(cols, rows))
}

// ResizePixels is Resize with the cells' size, which this slice does not
// carry: the host's program reads its size in cells.
func (c *Client) ResizePixels(cols, rows, cellWidth, cellHeight int) error {
	return c.Resize(cols, rows)
}

// Close detaches: the host keeps the program for the next attach.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	_ = writeFrame(c.c, tagDetach, nil)
	return c.c.Close()
}

// Kill ends the program and the session with it.
func (c *Client) Kill() error {
	return writeFrame(c.c, tagKill, nil)
}
