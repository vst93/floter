package settings

import (
	"reflect"
	"sync"
)

// Store owns the app's settings for the life of a run: it loads once, hands
// out normalized snapshots, and writes a change back to the same file it read,
// preserving every key this package does not own.
//
// It is safe from any goroutine. Views read Snapshot; the settings surface
// calls Update from the UI thread.
type Store struct {
	mu        sync.Mutex
	path      string
	current   Settings
	confirmed Settings
	listeners []func(Settings)
	lastErr   error
	// version advances every time the in-memory value changes, so a reader
	// that memoizes a snapshot (the launcher's frame does) can tell a stale
	// copy from the current one without comparing the whole value.
	version uint64
	// snapshots counts the copies `Snapshot` has taken. A copy is a deep copy
	// of the whole value (`clone`), so a frame that takes one per row is a
	// measurable cost; this counter is what the pins read to prove a surface
	// takes one per frame instead (P11-co). It is one increment under the lock
	// the read already takes.
	snapshots uint64
	// writes counts the file writes this store has attempted: one for every
	// `save` it hands the file path to, whichever of Update/Commit/Save made
	// it. `Preview` does not count — it never writes. It is what a pin reads
	// to prove a gesture wrote the file once rather than once per frame
	// (P11-cp); the counter is bumped under the lock the write already holds.
	writes uint64

	// save is the write path, a field so tests can observe or refuse one
	// without a real file. Production always uses Save.
	save func(string, Settings) error
}

// NewStore builds a store over an in-memory value, without a file: tests and
// callers that only read use it.
func NewStore(s Settings) *Store {
	normalized := s.normalized()
	return &Store{current: normalized, confirmed: clone(normalized), save: Save}
}

// OpenStore loads the settings file at path into a store. A missing file
// yields the defaults with a nil error, exactly as Load does; a file that
// cannot be read or parsed yields the defaults *and* the error, with the
// store still usable so the app keeps working. The shell logs the error.
func OpenStore(path string) (*Store, error) {
	s, err := Load(path)
	st := &Store{path: path, current: s.normalized(), lastErr: err, save: Save}
	st.confirmed = clone(st.current)
	return st, err
}

// Path is the file the store writes to, empty for an in-memory store.
func (st *Store) Path() string { return st.path }

// Version is a counter that advances with every accepted change to the
// in-memory settings. It is the cheap key a reader memoizes a snapshot by:
// same version, same value.
func (st *Store) Version() uint64 {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.version
}

// Snapshot returns a copy of the current settings, normalized. The copy is
// safe to keep: mutating it does not touch the store.
func (st *Store) Snapshot() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.snapshots++
	return clone(st.current)
}

// Snapshots counts the copies this store has handed out, so a test can see
// how many a frame took: a surface that memoizes the value takes one per
// change, however many rows read it (P11-co).
func (st *Store) Snapshots() uint64 {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.snapshots
}

// Writes counts the file writes this store has attempted, by every path that
// writes (Update, Commit, Save). A preview never writes, so a gesture that
// previews per frame and commits once reports one write (P11-cp).
func (st *Store) Writes() uint64 {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.writes
}

// writeLocked is the store's one write: it counts the attempt and hands the
// value to the save function. Every caller holds the lock and has already
// checked that path is set. Counting here rather than at the call sites keeps
// the number honest when a test replaces `save` to refuse a write.
func (st *Store) writeLocked(s Settings) error {
	st.writes++
	return st.save(st.path, s)
}

// Update mutates the settings and writes them back. The mutation runs on a
// copy; if it changed nothing the file is left alone and no listener runs.
// The in-memory value only advances when the write succeeds: a refused write
// is rolled back to the snapshot the store had before the mutation (the old
// build's `rollbackRejectedSettings`), so the next frame cannot read a value
// the file never accepted. The error is remembered (LastError) and returned.
func (st *Store) Update(mutate func(*Settings)) error {
	st.mu.Lock()
	previous := clone(st.current)
	next := clone(previous)
	mutate(&next)
	next = next.normalized()
	if equal(next, st.current) {
		st.mu.Unlock()
		return nil
	}
	if st.path != "" {
		if err := st.writeLocked(next); err != nil {
			st.lastErr = err
			rollback := clone(st.confirmed)
			changed := !equal(previous, rollback)
			st.current = rollback
			st.version++
			listeners := append([]func(Settings){}, st.listeners...)
			rolled := clone(rollback)
			st.mu.Unlock()
			if changed {
				for _, fn := range listeners {
					if fn != nil {
						fn(clone(rolled))
					}
				}
			}
			return err
		}
	}
	st.current = next
	st.version++
	st.confirmed = clone(next)
	st.lastErr = nil
	listeners := append([]func(Settings){}, st.listeners...)
	changed := clone(next)
	st.mu.Unlock()

	for _, fn := range listeners {
		if fn != nil {
			fn(clone(changed))
		}
	}
	return nil
}

// Preview applies a mutation to the in-memory settings and notifies listeners,
// without writing the file. It is the sliders' half of the reference's 180ms
// debounce: the value is live in the frame at once, and Commit writes it once
// after the gesture stops. A preview is not durable until Commit succeeds.
func (st *Store) Preview(mutate func(*Settings)) {
	st.mu.Lock()
	next := clone(st.current)
	mutate(&next)
	next = next.normalized()
	if equal(next, st.current) {
		st.mu.Unlock()
		return
	}
	st.current = next
	st.version++
	listeners := append([]func(Settings){}, st.listeners...)
	changed := clone(next)
	st.mu.Unlock()

	for _, fn := range listeners {
		if fn != nil {
			fn(clone(changed))
		}
	}
}

// Commit writes the previewed in-memory settings back to disk. A refused write
// rolls the visible value back to the last confirmed snapshot and notifies
// listeners, so the frame cannot keep showing a value the file rejected. A
// commit with nothing to write is a no-op.
func (st *Store) Commit() error {
	st.mu.Lock()
	if equal(st.current, st.confirmed) {
		st.mu.Unlock()
		return nil
	}
	if st.path == "" {
		st.confirmed = clone(st.current)
		st.lastErr = nil
		st.mu.Unlock()
		return nil
	}
	if err := st.writeLocked(st.current); err != nil {
		st.lastErr = err
		rollback := clone(st.confirmed)
		changed := !equal(st.current, rollback)
		st.current = rollback
		st.version++
		listeners := append([]func(Settings){}, st.listeners...)
		rolled := clone(rollback)
		st.mu.Unlock()
		if changed {
			for _, fn := range listeners {
				if fn != nil {
					fn(clone(rolled))
				}
			}
		}
		return err
	}
	st.confirmed = clone(st.current)
	st.lastErr = nil
	st.mu.Unlock()
	return nil
}

// Save writes the current settings to disk, for a forced flush. It is a
// no-op for an in-memory store.
func (st *Store) Save() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.path == "" {
		return nil
	}
	err := st.writeLocked(st.current)
	st.lastErr = err
	if err == nil {
		st.confirmed = clone(st.current)
	}
	return err
}

// LastError is the last read or write error, or nil.
func (st *Store) LastError() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.lastErr
}

// OnChange registers a listener for committed changes, on the goroutine that
// made them. The returned function removes it.
func (st *Store) OnChange(fn func(Settings)) func() {
	st.mu.Lock()
	st.listeners = append(st.listeners, fn)
	index := len(st.listeners) - 1
	st.mu.Unlock()
	return func() {
		st.mu.Lock()
		defer st.mu.Unlock()
		if index < len(st.listeners) {
			st.listeners[index] = nil
		}
	}
}

// clone copies a Settings value, including its carried-through keys.
func clone(s Settings) Settings {
	out := s
	if s.extra != nil {
		out.extra = make(map[string]any, len(s.extra))
		for key, value := range s.extra {
			out.extra[key] = value
		}
	}
	return out
}

// equal reports whether two settings would encode to the same file. The
// carried-through keys compare deeply, since a value may be any JSON shape.
func equal(a, b Settings) bool {
	if a.Theme != b.Theme || a.GlassStep != b.GlassStep || a.Language != b.Language ||
		a.MainOpacity != b.MainOpacity || a.TerminalOpacity != b.TerminalOpacity ||
		a.UIScale != b.UIScale ||
		a.HideOnBlur != b.HideOnBlur || a.SurfaceResidencySeconds != b.SurfaceResidencySeconds ||
		a.LaunchAtStartup != b.LaunchAtStartup ||
		a.FontSize != b.FontSize || a.FontFamily != b.FontFamily ||
		a.CursorShape != b.CursorShape || a.CursorBlink != b.CursorBlink ||
		a.TerminalLineHeight != b.TerminalLineHeight || a.TerminalPadding != b.TerminalPadding ||
		a.TerminalTheme != b.TerminalTheme || a.TerminalScrollbar != b.TerminalScrollbar ||
		a.TerminalWheelLines != b.TerminalWheelLines || a.TerminalBold != b.TerminalBold ||
		a.TerminalSelectCopy != b.TerminalSelectCopy || a.TerminalPasteSafe != b.TerminalPasteSafe ||
		a.TerminalWidth != b.TerminalWidth || a.TerminalHeight != b.TerminalHeight {
		return false
	}
	return reflect.DeepEqual(a.extra, b.extra)
}
