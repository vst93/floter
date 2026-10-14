package terminalui

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
)

// countingTerminal is a NewTerminal that hands out sessions the test can tell
// apart: each one is a distinct terminal, and the test can close them.
func countingTerminal(t *testing.T) (func(terminal.Options) (*terminal.Terminal, error), *int) {
	t.Helper()
	made := 0
	return func(terminal.Options) (*terminal.Terminal, error) {
		made++
		return &terminal.Terminal{}, nil
	}, &made
}

// The surface holds the sessions it started: a new one is shown, and the one
// that was on screen keeps running behind it — the old build's own sessions,
// where "new session" never ended the old one.
func TestSessionsAccumulate(t *testing.T) {
	newTerminal, made := countingTerminal(t)
	a := New(newStore(), Actions{}, newTerminal)

	a.EnsureSession()
	if a.Current() == nil || *made != 1 {
		t.Fatalf("the first session did not start: %d", *made)
	}
	first := a.Current()

	a.NewSession()
	if *made != 2 {
		t.Fatalf("a second session did not start: %d", *made)
	}
	if a.Current() == first {
		t.Fatal("the new session is not the one on screen")
	}
	if len(a.Sessions) != 2 || a.Active != 1 {
		t.Fatalf("sessions = %d, active = %d", len(a.Sessions), a.Active)
	}
	// The first one is still there, and still running.
	if a.Sessions[0] != first || a.Exited(0) {
		t.Error("the session behind the new one was lost")
	}
	// Naming them is what the sessions list shows.
	a.SetTitle("vim")
	if a.LabelOf(1) != "vim" {
		t.Errorf("label = %q", a.LabelOf(1))
	}
	if a.LabelOf(0) == "vim" || a.LabelOf(0) == "" {
		t.Errorf("the unnamed session is labelled %q", a.LabelOf(0))
	}
}

// Showing a session is switching to it; the one on screen becomes a session
// behind it rather than closing.
func TestSelectSessionSwitches(t *testing.T) {
	newTerminal, _ := countingTerminal(t)
	a := New(newStore(), Actions{}, newTerminal)
	a.EnsureSession()
	a.NewSession()
	second := a.Current()

	a.SelectSession(0)
	if a.Current() == second || a.Active != 0 {
		t.Fatalf("selecting did not switch: active = %d", a.Active)
	}
	// An index that is not a session changes nothing.
	a.SelectSession(9)
	if a.Active != 0 {
		t.Errorf("an out-of-range index moved the choice to %d", a.Active)
	}
}

// Closing a session forgets it and shows its neighbour; closing the last one
// leaves the surface empty rather than showing a closed screen.
//
// The sessions here are put together without a program: closing one ends its
// terminal, and a terminal the plugin never started has none to end. The state
// the surface keeps is what this test is about.
func TestCloseSessionPicksTheNextOne(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	a.Sessions = []*Session{{}, {}, {}}
	a.Active = 2
	if len(a.Sessions) != 3 {
		t.Fatalf("sessions = %d", len(a.Sessions))
	}
	// Closing the one on screen (the last) shows the one before it.
	a.Close()
	if len(a.Sessions) != 2 || a.Active != 1 {
		t.Fatalf("after closing the last: %d sessions, active %d", len(a.Sessions), a.Active)
	}
	// Closing a session *below* the active one keeps the active one on screen.
	a.SelectSession(1)
	a.CloseSession(0)
	if a.Active != 0 {
		t.Errorf("the active index followed the wrong session: %d", a.Active)
	}
	// The last close leaves nothing to show.
	a.Close()
	if len(a.Sessions) != 0 || a.Active != -1 {
		t.Fatalf("sessions = %d, active = %d", len(a.Sessions), a.Active)
	}
	// A new session after that starts cleanly, through the surface's own
	// constructor.
	newTerminal, made := countingTerminal(t)
	a.NewTerminal = newTerminal
	a.NewSession()
	if a.Current() == nil || len(a.Sessions) != 1 || *made != 1 {
		t.Errorf("a session did not start after the last one closed")
	}
}

// Running a command or opening a shell in a directory starts a session of its
// own: the panel keeps what was on screen, as the old build's broker did.
func TestRunningACommandOpensASession(t *testing.T) {
	newTerminal, made := countingTerminal(t)
	a := New(newStore(), Actions{}, newTerminal)
	a.EnsureSession()
	first := a.Current()

	if err := a.RunCommand([]string{"/bin/echo", "hi"}, "current", nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	if *made != 2 || len(a.Sessions) != 2 {
		t.Fatalf("sessions = %d, started = %d", len(a.Sessions), *made)
	}
	if a.Current() == first {
		t.Error("the command did not become the session on screen")
	}
	a.RunShellIn(t.TempDir())
	if *made != 3 || len(a.Sessions) != 3 {
		t.Errorf("a shell in a directory did not start a session: %d", *made)
	}
	// A command that cannot start is an error and starts nothing.
	bad := New(newStore(), Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no terminal")
	})
	if err := bad.RunCommand([]string{"/bin/echo"}, "", nil); err == nil {
		t.Error("a failed command reported success")
	}
	if len(bad.Sessions) != 0 {
		t.Errorf("a failed command left %d sessions", len(bad.Sessions))
	}
}

// A session whose program has ended is held on screen, and the note is that
// session's own: another session's exit does not mark it.
func TestResidentIsPerSession(t *testing.T) {
	newTerminal, made := countingTerminal(t)
	a := New(newStore(), Actions{}, newTerminal)
	a.EnsureSession()
	a.NewSession()

	// The shell reports the exit of the session on screen.
	a.Resident(2)
	if a.Current() == nil || !a.Current().Resident || a.Current().ExitCode != 2 {
		t.Fatalf("the resident state is not the current session's: %+v", a.Current())
	}
	if a.Sessions[0].Resident {
		t.Error("the exit marked the session behind it")
	}
	if !a.Exited(1) || a.Exited(0) {
		t.Errorf("exited states: %v, %v", a.Exited(1), a.Exited(0))
	}

	// Entering the surface again drops the held session and shows the one
	// behind it, rather than starting a third: the sessions the user has are
	// the ones worth showing. The held session's own terminal is closed, which
	// a terminal with a program can take and a test's cannot — so the held one
	// is put together here, and only the *other* keeps its terminal.
	held := a.Sessions[1]
	held.Term = nil
	a.EnsureSession()
	if len(a.Sessions) != 1 || *made != 2 {
		t.Fatalf("sessions = %d, started = %d", len(a.Sessions), *made)
	}
	if a.Current() != a.Sessions[0] || a.Current().Resident {
		t.Errorf("the session behind the held one is not on screen: %+v", a.Current())
	}
	if a.Sessions[0].Term == nil {
		t.Error("the other session was closed with it")
	}

	// A held session with nothing behind it is replaced by a fresh one.
	only := New(newStore(), Actions{}, newTerminal)
	only.EnsureSession()
	only.Sessions[0].Resident = true
	only.Sessions[0].Term = nil
	only.EnsureSession()
	if len(only.Sessions) != 1 || only.Current().Resident {
		t.Errorf("the held session was not replaced: %d, %+v", len(only.Sessions), only.Current())
	}
}

// Every session gets the appearance, not just the one on screen.
func TestRefreshAppliesToEverySession(t *testing.T) {
	newTerminal, _ := countingTerminal(t)
	a := New(newStore(), Actions{}, newTerminal)
	a.EnsureSession()
	a.NewSession()
	a.Refresh()
	for i, session := range a.Sessions {
		if session.Term == nil {
			t.Errorf("session %d has no terminal", i)
		}
	}
	// A surface with no sessions refreshes nothing, and does not panic.
	empty := New(newStore(), Actions{}, newTerminal)
	empty.Refresh()
	if len(empty.Sessions) != 0 {
		t.Errorf("refresh started %d sessions", len(empty.Sessions))
	}
}

// Closing everything is what quitting does: a program left running behind a
// closed panel would be one nobody can reach.
func TestCloseAll(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	a.Sessions = []*Session{{}, {}, {}}
	a.Active = 1
	a.CloseAll()
	if len(a.Sessions) != 0 || a.Active != -1 {
		t.Errorf("sessions = %d, active = %d", len(a.Sessions), a.Active)
	}
	_ = time.Now()
}

// The empty state's two lines are its own keys: Enter starts a session and the
// user's new-command binding returns to the launcher, and each line prints the
// key that works in a badge beside its label — the old build's
// `.terminal-empty__line`, whose second keycap was
// `formatShortcut(shortcuts.new_command)`, never a literal Escape. Escape is
// the shell's on this surface (the old build's `DISMISS_TABLE.terminal.escape`
// is null), so the empty page leaves it alone.
func TestEmptyStateLines(t *testing.T) {
	newCommands, closeCalls := 0, 0
	store := newStore()
	if err := store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutNewCommand, "Alt+B")
	}); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{
		NewCommand: func() { newCommands++ },
		Close:      func() { closeCalls++ },
	}, nil)
	tt := render(t, a, 860, 600)
	tt.Frame()
	copy := i18n.For("en").Terminal
	wantBadge := shortcuts.Display("Alt+B")
	for _, want := range []string{"\u21a9", wantBadge, copy.EmptyNew, copy.EmptyBack} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	if tt.HasText("esc") {
		t.Errorf("the back keycap still reads Escape: %q", tt.Texts())
	}
	// Escape is the shell's on this surface, not the empty page's: it runs
	// nothing here.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if newCommands != 0 || closeCalls != 0 {
		t.Errorf("Escape ran new-command %d / close %d times, want none", newCommands, closeCalls)
	}
	// Clicking a line runs it too.
	if err := tt.Click(copy.EmptyBack); err != nil {
		t.Fatalf("the back line: %v", err)
	}
	tt.Frame()
	if newCommands != 1 {
		t.Errorf("the back line ran new-command %d times, want 1", newCommands)
	}
}
