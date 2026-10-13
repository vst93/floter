package shell

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/settings"
)

// TestACommandSessionTakesTheTerminalPageWithIt is R62 end to end: the
// launcher runs a structured command, the program ends, and the terminal page
// leaves for the launcher with the session dropped — while the same exit for a
// bare shell keeps the resident page the old build held.
func TestACommandSessionTakesTheTerminalPageWithIt(t *testing.T) {
	options := []terminal.Options{}
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store: store,
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(opts terminal.Options) (*terminal.Terminal, error) {
			options = append(options, opts)
			return &terminal.Terminal{}, nil
		},
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
	})

	// A structured command from the launcher's action bar: the session runs
	// the program itself, so it is command-started.
	a.Launcher.Actions.RunInTerminal([]string{"echo", "hi"})
	if a.Surf != SurfaceTerminal || len(options) != 1 {
		t.Fatalf("the command did not reach the terminal: surface=%v built=%d", a.Surf, len(options))
	}
	if !a.Terminal.Current().CommandStarted {
		t.Fatal("a structured command is not command-started")
	}

	// The program ends, with the terminal on screen: the page leaves. The
	// session's terminal is put aside first — a terminal with a real program
	// can take the close and a test's cannot.
	a.Terminal.Sessions[0].Term = nil
	options[0].OnExit(0)
	if a.Surf != SurfaceLauncher {
		t.Errorf("the page stayed after a command session ended: %v", a.Surf)
	}
	if len(a.Terminal.Sessions) != 0 {
		t.Errorf("the finished command session was not dropped: %d", len(a.Terminal.Sessions))
	}

	// A bare shell on the same surface keeps its page: the user's own shell
	// ending is the user closing their terminal.
	a.Open(SurfaceTerminal)
	if len(a.Terminal.Sessions) != 1 || len(options) != 2 {
		t.Fatalf("the bare session did not start: sessions=%d built=%d", len(a.Terminal.Sessions), len(options))
	}
	options[1].OnExit(7)
	if a.Surf != SurfaceTerminal {
		t.Errorf("a bare shell took the page: %v", a.Surf)
	}
	current := a.Terminal.Current()
	if current == nil || !current.Resident || current.ExitCode != 7 {
		t.Errorf("the bare session was not held: %+v", current)
	}
}

// The launcher's shell-syntax line is the other command-started hand-off: the
// session exists to carry a pipe or a redirect, so its end takes the page. The
// install rows' own hand-off (openInstallSession) keeps the user's shell.
func TestAShellLineSessionLeavesWithTheLauncher(t *testing.T) {
	options := []terminal.Options{}
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store: store,
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(opts terminal.Options) (*terminal.Terminal, error) {
			options = append(options, opts)
			return &terminal.Terminal{}, nil
		},
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
	})

	a.Launcher.Actions.RunInShell("ls -la | grep go")
	if a.Surf != SurfaceTerminal || len(options) != 1 {
		t.Fatalf("the line did not reach the terminal: surface=%v built=%d", a.Surf, len(options))
	}
	if !a.Terminal.Current().CommandStarted {
		t.Fatal("the launcher's shell line is not command-started")
	}
	a.Terminal.Sessions[0].Term = nil
	options[0].OnExit(0)
	if a.Surf != SurfaceLauncher || len(a.Terminal.Sessions) != 0 {
		t.Errorf("surface=%v sessions=%d, want launcher with none", a.Surf, len(a.Terminal.Sessions))
	}

	// The install row's hand-off is bare: its shell stays.
	a.openInstallSession("npm install -g @egoist/tools")
	if len(options) != 2 || a.Terminal.Current().CommandStarted {
		t.Fatalf("the install session is not a bare shell: built=%d", len(options))
	}
	options[1].OnExit(0)
	if a.Surf != SurfaceTerminal || len(a.Terminal.Sessions) != 1 {
		t.Errorf("the install session left: surface=%v sessions=%d", a.Surf, len(a.Terminal.Sessions))
	}
}
