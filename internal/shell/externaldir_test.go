package shell

import (
	"testing"
	"time"

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

// The hand-off's process probe is not the drawing thread's work: `spawn.
// TerminalAt` walks candidate emulators and watches each for an early death
// (`spawn.earlyExitWindow`) under a `spawn.probeBudget` of 400ms, so on a
// machine whose first candidate is broken the press blocked the frame that
// drew it — and a key the desktop repeats queued one probe per repeat behind
// that frame (the 「反复弹出终端，从而导致卡死」 report). The probe therefore
// runs off the press, and this pins it the only way it can be pinned without a
// stopwatch: `openTerminalWindow` returns while the probe has not run yet.
func TestTheTerminalProbeLeavesTheDrawingThread(t *testing.T) {
	release := make(chan struct{})
	ran := make(chan struct{})
	original := terminalAt
	terminalAt = func(string) error {
		<-release // hold the probe until the test has seen the press return
		close(ran)
		return nil
	}
	t.Cleanup(func() { terminalAt = original })

	a := launchTestApp(t, &launchCalls{})
	a.openExternalTerminal = nil
	a.openTerminalWindow()
	// The press has returned and the probe has still not finished: it is off
	// the frame.
	select {
	case <-ran:
		t.Fatal("the probe ran on the drawing thread")
	default:
	}
	close(release)
	select {
	case <-ran:
	case <-time.After(3 * time.Second):
		t.Fatal("the probe never ran")
	}
}
