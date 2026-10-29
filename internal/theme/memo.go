package theme

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// A Memo answers one frame's theme tokens.
//
// `ForPreferences` is not cheap: it normalizes the settings value, resolves
// the appearance, builds a whole `ui.Theme` and lays the palette over it
// (`paletteFor`, and `contrast` on a desktop that asked for more contrast).
// Surfaces draw with tokens from many places — the settings surface asks at 97
// call sites, all of them while drawing — so resolving them per call was most
// of a heavy frame's CPU: the profile of `BenchmarkIntegrationsFrame` put the
// work at 29% of the frame before this memo existed (P11-bs).
//
// The key is exact: the store's version (settings.Store.Version: "same version,
// same value"), the appearance the frame resolved, and the desktop's
// preferences. Nothing else reaches `ForPreferences`, so a hit is the value the
// miss would have built, and a change — a slider's preview included — bumps the
// version and the next read builds again. The settings value itself is loaded
// only on a miss, so a frame that resolves one palette also copies the settings
// once rather than once per row.
//
// A Memo belongs to one reader (an App field): it is not safe for concurrent
// use, and it keeps the last value it built.
type Memo struct {
	key   memoKey
	tok   Tokens
	valid bool
	built uint64
}

// memoKey is what the tokens depend on.
type memoKey struct {
	version uint64
	dark    bool
	prefs   ui.Preferences
}

// Tokens returns the tokens of this frame: those of the last build when
// nothing the look depends on changed, and `ForPreferences(load(), dark,
// prefs)` otherwise. load is called only when it builds.
func (m *Memo) Tokens(version uint64, dark bool, prefs ui.Preferences, load func() settings.Settings) Tokens {
	key := memoKey{version: version, dark: dark, prefs: prefs}
	if m.valid && m.key == key {
		return m.tok
	}
	m.tok = ForPreferences(load(), dark, prefs)
	m.key, m.valid = key, true
	m.built++
	return m.tok
}

// Builds counts the palettes the memo built, so a test can see that a frame
// builds one however many rows ask for it.
func (m *Memo) Builds() uint64 { return m.built }

// Reset forgets what the memo built. A reader resets it when the store behind
// the version changed identity, as two stores both start at version zero.
func (m *Memo) Reset() { *m = Memo{} }
