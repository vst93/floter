// The app's own home for the framework's per-window state.
//
// mygo remembers a window's geometry under a `StateKey` (the detached output
// windows and the pinned text window; see shell/detach.go and shell/pinned.go).
// By default it keeps that in `window-state.json` beside the settings file.
// `mygo.App.SetWindowStateStore` (v0.4.6) lets the app choose the store
// instead, so floter keeps it where the rest of its state lives: the settings
// file, under a carried-through key, written by the same atomic writer and
// covered by the same backup fallback as every other setting.
//
// The framework loads the states once, when the first window with a StateKey
// is created, and saves them when such a window closes and when the app quits,
// on the main thread. A state the app has never written is seeded from the
// `window-state.json` the migration (windowstate.go) produced, so a build that
// upgrades in place still opens a detached window where the user left it.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// windowStatesKey is the carried-through key the states live under in the
// settings file. It is not a key this package owns, so Encode writes it back
// exactly as it was set, alongside the settings the UI edits.
const windowStatesKey = "window_states"

// WindowStore keeps the framework's window states in the app's settings store.
// It implements mygo.WindowStateStore; the framework calls it on the main
// thread, so it returns quickly.
type WindowStore struct {
	store *Store
}

// NewWindowStore builds the window-state store over an open settings store.
func NewWindowStore(store *Store) *WindowStore { return &WindowStore{store: store} }

// LoadWindowStates returns the saved states: the ones in the settings file, or
// the `window-state.json` the migration left, the first time. Without either,
// the framework gives every window its options.
func (w *WindowStore) LoadWindowStates() (map[string]mygo.WindowState, error) {
	if raw, ok := w.store.Snapshot().Extra()[windowStatesKey]; ok {
		if states, err := decodeWindowStates(raw); err == nil {
			return states, nil
		}
	}
	return w.loadLegacy()
}

// SaveWindowStates writes the states into the settings file through the
// store, so the change is atomic and preserves every key the store carries.
func (w *WindowStore) SaveWindowStates(states map[string]mygo.WindowState) error {
	return w.store.Update(func(s *Settings) {
		if len(states) == 0 {
			s.SetExtra(windowStatesKey, nil)
			return
		}
		s.SetExtra(windowStatesKey, states)
	})
}

// loadLegacy reads the migration's own window-state.json, one time, so a build
// that upgrades in place keeps the geometry it already had. It is the same file
// the default store would have read, next to the settings file.
func (w *WindowStore) loadLegacy() (map[string]mygo.WindowState, error) {
	path := w.store.Path()
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(path), windowStateFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var states map[string]mygo.WindowState
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, err // a damaged file is ignored, as the framework ignores its own
	}
	return states, nil
}

// decodeWindowStates turns a value read back from the settings file into the
// framework's shape. The file round-trips through JSON, so the map comes back
// as generic values with float64 numbers; re-encoding reads it as the typed
// map without a second decoder.
func decodeWindowStates(raw any) (map[string]mygo.WindowState, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var states map[string]mygo.WindowState
	if err := json.Unmarshal(data, &states); err != nil {
		return nil, err
	}
	return states, nil
}
