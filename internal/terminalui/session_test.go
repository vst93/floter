package terminalui

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/settings"
)

// TestRealSessionStartsWithTheSettings runs the plugin for real: it loads
// libghostty-vt (downloading it into the user's cache on the first run) and
// starts a shell with the options this package builds.
//
// It is opt-in because it needs the network on the first run and a real
// pseudo-terminal: FLOTER_TERMINAL_TEST=1 go test ./internal/terminalui
func TestRealSessionStartsWithTheSettings(t *testing.T) {
	if os.Getenv("FLOTER_TERMINAL_TEST") == "" {
		t.Skip("set FLOTER_TERMINAL_TEST=1 to load libghostty-vt")
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) {
		s.FontSize = 16
		s.TerminalTheme = "amber"
		s.TerminalPadding = "relaxed"
	}); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{
		Title: func(string) {},
		Exit:  func(int) {},
	}, terminal.New)

	a.EnsureSession()
	if a.Err != nil {
		t.Fatalf("the session did not start: %v", a.Err)
	}
	defer a.Close()
	if a.Term == nil {
		t.Fatal("no session")
	}

	// The shell prints its prompt shortly after starting; wait for it, so
	// the test proves a program really runs, not just that New returned.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && a.Term.Text() == "" {
		time.Sleep(50 * time.Millisecond)
	}
	if a.Term.Text() == "" {
		t.Fatal("the session produced no output")
	}

	// The appearance can be applied to a running session.
	a.Refresh()
	if err := store.Update(func(s *settings.Settings) { s.FontSize = 18; s.CursorShape = "block" }); err != nil {
		t.Fatal(err)
	}
	a.Refresh()
}
