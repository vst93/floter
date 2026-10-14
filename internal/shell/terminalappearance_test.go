package shell

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/settings"
)

// newAppearanceApp builds a shell whose terminal needs no native library: the
// session can never start, but every appearance refresh is counted, so the
// re-entry rule is observable without a window or a PTY.
func newAppearanceApp(t *testing.T) (*App, *settings.Store, *int) {
	t.Helper()
	store := settings.NewStore(settings.Default())
	refreshes := 0
	a := New(Options{
		Store: store,
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			return nil, errors.New("no library in tests")
		},
		RefreshTerminal: func() { refreshes++ },
	})
	return a, store, &refreshes
}

// A session that outlived a visit to the settings surface takes the appearance
// the user changed while it was away. The reference's terminal view re-ran its
// font and theme effects on every mount; this build's session is kept alive,
// so the equivalent rule is the refresh on the way back in.
func TestReopeningTheTerminalAppliesAppearanceChangedElsewhere(t *testing.T) {
	a, store, refreshes := newAppearanceApp(t)

	// The user is on the settings page and moves the terminal's own font size.
	a.Surf = SurfaceSettings
	if err := store.Update(func(s *settings.Settings) { s.FontSize = settings.MinFontSize + 3 }); err != nil {
		t.Fatal(err)
	}
	if *refreshes != 0 {
		t.Fatalf("a settings-page change refreshed the off-screen terminal %d times", *refreshes)
	}

	// Coming back applies it to the session that stayed alive.
	a.Open(SurfaceTerminal)
	if *refreshes != 1 {
		t.Fatalf("entering the terminal refreshed %d times, want 1", *refreshes)
	}
}

// On the terminal surface itself the running session keeps up with a change in
// the same gesture, as it always did: the drawer writes the store and the next
// change applies to the session on screen.
func TestAnAppearanceChangeOnTheTerminalAppliesAtOnce(t *testing.T) {
	a, store, refreshes := newAppearanceApp(t)
	a.Surf = SurfaceTerminal

	if err := store.Update(func(s *settings.Settings) { s.TerminalTheme = "ink" }); err != nil {
		t.Fatal(err)
	}
	if *refreshes != 1 {
		t.Fatalf("a change on the terminal surface refreshed %d times, want 1", *refreshes)
	}
}
