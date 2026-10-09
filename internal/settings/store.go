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
	listeners []func(Settings)
	lastErr   error
}

// NewStore builds a store over an in-memory value, without a file: tests and
// callers that only read use it.
func NewStore(s Settings) *Store {
	return &Store{current: s.normalized()}
}

// OpenStore loads the settings file at path into a store. A missing file
// yields the defaults with a nil error, exactly as Load does; a file that
// cannot be read or parsed yields the defaults *and* the error, with the
// store still usable so the app keeps working. The shell logs the error.
func OpenStore(path string) (*Store, error) {
	s, err := Load(path)
	st := &Store{path: path, current: s.normalized(), lastErr: err}
	return st, err
}

// Path is the file the store writes to, empty for an in-memory store.
func (st *Store) Path() string { return st.path }

// Snapshot returns a copy of the current settings, normalized. The copy is
// safe to keep: mutating it does not touch the store.
func (st *Store) Snapshot() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return clone(st.current)
}

// Update mutates the settings and writes them back. The mutation runs on a
// copy; if it changed nothing the file is left alone and no listener runs.
// An error from the write is remembered (LastError) and returned, with the
// in-memory value kept so the user's change still shows.
func (st *Store) Update(mutate func(*Settings)) error {
	st.mu.Lock()
	next := clone(st.current)
	mutate(&next)
	next = next.normalized()
	if equal(next, st.current) {
		st.mu.Unlock()
		return nil
	}
	st.current = next
	var err error
	if st.path != "" {
		err = Save(st.path, next)
	}
	st.lastErr = err
	listeners := append([]func(Settings){}, st.listeners...)
	changed := clone(next)
	st.mu.Unlock()

	for _, fn := range listeners {
		if fn != nil {
			fn(clone(changed))
		}
	}
	return err
}

// Save writes the current settings to disk, for a forced flush. It is a
// no-op for an in-memory store.
func (st *Store) Save() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.path == "" {
		return nil
	}
	err := Save(st.path, st.current)
	st.lastErr = err
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
