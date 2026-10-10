package terminalui

import (
	"os"
	"strings"
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

// TestRealSnapshotRoundTrip proves the snapshot really carries a session's
// screen: one terminal prints something, its snapshot is fed into a second
// terminal, and the second shows the same text.
//
// Opt-in for the same reason as the test above.
func TestRealSnapshotRoundTrip(t *testing.T) {
	if os.Getenv("FLOTER_TERMINAL_TEST") == "" {
		t.Skip("set FLOTER_TERMINAL_TEST=1 to load libghostty-vt")
	}
	source, err := terminal.New(terminal.Options{
		Command: []string{"/bin/sh", "-c", "echo snapshot-content; sleep 2"},
	})
	if err != nil {
		t.Fatalf("the source session did not start: %v", err)
	}
	defer source.Close()
	waitForText(t, source, "snapshot-content")

	snapshot := source.Snapshot()
	if len(snapshot) == 0 {
		t.Fatal("the snapshot is empty")
	}

	// A quiet session of the same size, so the snapshot is all it shows.
	target, err := terminal.New(terminal.Options{Command: []string{"/bin/sh", "-c", "sleep 2"}})
	if err != nil {
		t.Fatalf("the target session did not start: %v", err)
	}
	defer target.Close()
	cols, rows := source.Size()
	target.Resize(cols, rows)
	target.Feed(snapshot)
	waitForText(t, target, "snapshot-content")
}

// waitForText waits until the terminal's text holds want.
func waitForText(t *testing.T, term *terminal.Terminal, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(term.Text(), want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the terminal never showed %q: %q", want, term.Text())
}

// TestRealShellWithCommand proves the install-session hand-off: a bare session
// (the user's own shell) is opened and the command typed into it, so the
// session stays interactive after the command finishes.
//
// Opt-in for the same reason as the session test above.
func TestRealShellWithCommand(t *testing.T) {
	if os.Getenv("FLOTER_TERMINAL_TEST") == "" {
		t.Skip("set FLOTER_TERMINAL_TEST=1 to load libghostty-vt")
	}
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{Title: func(string) {}, Exit: func(int) {}}, terminal.New)
	a.RunShellWithCommand("echo install-session-marker")
	if a.Err != nil {
		t.Fatalf("the session did not start: %v", a.Err)
	}
	defer a.Close()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(a.Term.Text(), "install-session-marker") {
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(a.Term.Text(), "install-session-marker") {
		t.Fatalf("the command never landed: %q", a.Term.Text())
	}
	// The session stays interactive: the shell's prompt comes back after the
	// command, rather than the session exiting when it finished.
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(a.Term.Text(), "exited") {
		t.Errorf("the session exited after the command: %q", a.Term.Text())
	}
}
