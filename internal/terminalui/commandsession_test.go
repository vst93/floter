package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
)

// captureTerminal is a NewTerminal that keeps the options each session was
// built with, so a test can drive the plugin's own callbacks.
func captureTerminal(t *testing.T) (func(terminal.Options) (*terminal.Terminal, error), *[]terminal.Options) {
	t.Helper()
	options := []terminal.Options{}
	return func(opts terminal.Options) (*terminal.Terminal, error) {
		options = append(options, opts)
		return &terminal.Terminal{}, nil
	}, &options
}

// R62's rule, the reference's own predicate: only a command-started session on
// the terminal page takes the page with it.
func TestShouldExitWithCommandSession(t *testing.T) {
	cases := []struct {
		name           string
		commandStarted bool
		onScreen       bool
		want           bool
	}{
		{"a command on the page", true, true, true},
		{"a command with the page gone", true, false, false},
		{"a bare shell on the page", false, true, false},
		{"a bare shell with the page gone", false, false, false},
	}
	for _, tc := range cases {
		if got := ShouldExitWithCommandSession(tc.commandStarted, tc.onScreen); got != tc.want {
			t.Errorf("%s: = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The exit callback carries the session it belongs to, not whichever one is on
// screen: the surface holds several and a background program can end while the
// user looks at another.
func TestEverySessionNamesItsOwnExit(t *testing.T) {
	newTerminal, options := captureTerminal(t)
	var exits []*Session
	a := New(newStore(), Actions{Exit: func(session *Session, code int) {
		session.ExitCode = code
		exits = append(exits, session)
	}}, newTerminal)

	a.NewSession()
	first := a.Current()
	a.NewSession()
	second := a.Current()

	(*options)[0].OnExit(1)
	(*options)[1].OnExit(3)

	if len(exits) != 2 || exits[0] != first || exits[1] != second {
		t.Fatalf("the exits were attributed to %v, want [%p %p]", exits, first, second)
	}
	if first.ExitCode != 1 || second.ExitCode != 3 {
		t.Errorf("codes = %d, %d, want 1, 3", first.ExitCode, second.ExitCode)
	}
	if a.IndexOf(first) != 0 || a.IndexOf(second) != 1 {
		t.Errorf("indexes = %d, %d, want 0, 1", a.IndexOf(first), a.IndexOf(second))
	}
	if a.IndexOf(&Session{}) != -1 {
		t.Error("a session that is not in the list has an index")
	}
}

// A structured command is command-started; a shell line typed into the user's
// own shell is not, unless the launcher itself asked for it to be carried.
func TestOnlyACommandSessionCarriesThePage(t *testing.T) {
	newTerminal, _ := captureTerminal(t)
	a := New(newStore(), Actions{}, newTerminal)

	if err := a.RunCommand([]string{"echo", "hi"}, "current", nil); err != nil {
		t.Fatal(err)
	}
	if !a.Current().CommandStarted {
		t.Fatal("a structured command is not command-started")
	}

	// The install hand-off: the user's shell hosts the line and outlives it.
	a.RunShellWithCommand("echo install")
	if a.Current().CommandStarted {
		t.Error("an install session was marked command-started")
	}

	// The launcher's own shell-syntax line: the session exists to carry it.
	a.RunCommandLine("ls -la | grep go")
	if !a.Current().CommandStarted {
		t.Error("the launcher's shell line is not command-started")
	}
}

// The rule in full, as the shell applies it: a command session on the page is
// dropped (without a snapshot) and reports that the caller should leave; a bare
// shell, a command session the user has moved away from, and a page that is not
// the terminal all keep their session.
func TestCommandSessionEnded(t *testing.T) {
	newTerminal, _ := captureTerminal(t)
	a := New(newStore(), Actions{}, newTerminal)
	if err := a.RunCommand([]string{"echo", "hi"}, "current", nil); err != nil {
		t.Fatal(err)
	}

	// The terminal is not the surface on screen: the frame is held instead.
	if a.CommandSessionEnded(0, false) {
		t.Error("a command session ended while the terminal was not on screen")
	}
	if len(a.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(a.Sessions))
	}

	// A session the user has moved away from does not take the page either.
	a.NewSession()
	if a.CommandSessionEnded(0, true) {
		t.Error("a background command session took the page")
	}
	if len(a.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(a.Sessions))
	}

	// A bare shell on the page holds it.
	a.Active = 1
	if a.CommandSessionEnded(1, true) {
		t.Error("a bare shell took the page")
	}
	if len(a.Sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(a.Sessions))
	}

	// The command session, on screen, goes — and takes its frame with it. Its
	// terminal is put aside first: a terminal with a real program can take the
	// close and a test's cannot (the same accommodation the sessions tests
	// make).
	a.Active = 0
	a.Sessions[0].Term = nil
	if !a.CommandSessionEnded(0, true) {
		t.Fatal("the command session did not take the page")
	}
	if len(a.Sessions) != 1 || a.Active != 0 {
		t.Errorf("sessions = %d, active = %d, want 1, 0", len(a.Sessions), a.Active)
	}
	if a.Sessions[0].CommandStarted {
		t.Error("the bare shell behind it was dropped too")
	}
}
