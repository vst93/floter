package shell

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/terminalui"
)

// The hand-off asks for the session's own directory and never for a directory it
// made up: `terminalui` reports one only once the shell has said where it is
// (`OSC 7`), and a panel with no session at all is a login shell, not a panic.
func TestTheTerminalHandOffAsksForTheSessionsDirectory(t *testing.T) {
	var got []string
	original := terminalAt
	terminalAt = func(dir string) error {
		got = append(got, dir)
		return nil
	}
	t.Cleanup(func() { terminalAt = original })

	bare := &App{}
	if err := bare.OpenExternalTerminal(); err != nil {
		t.Fatalf("OpenExternalTerminal with no terminal: %v", err)
	}
	// A session whose program has not reported a directory yet (a zero terminal
	// reports "" — see terminal.Terminal.Dir), which is the login-shell case the
	// port always had.
	silent := &App{Terminal: &terminalui.App{
		Sessions: []*terminalui.Session{{Term: &terminal.Terminal{}}},
		Active:   0,
	}}
	if err := silent.OpenExternalTerminal(); err != nil {
		t.Fatalf("OpenExternalTerminal with a silent session: %v", err)
	}
	want := []string{"", ""}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("the hand-off opened at %q, want %q (the session's own, empty while unknown)", got, want)
	}
	// The seam the tests stand in for is the one the app uses: a hand-off that
	// was told a directory would pass it on rather than dropping it.
	if got := silent.externalTerminalDir(); got != "" {
		t.Errorf("a session with no report is %q, want the empty login-shell directory", got)
	}
}
