package shell

import (
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/settings"
)

// spawnTestApp is a shell whose terminal is a stub that counts sessions: the
// plugin's Terminal is built without a program, so a session is created and
// never runs anything (commandsession_test.go does the same).
func spawnTestApp(t *testing.T, built *int, clock *time.Time) *App {
	t.Helper()
	a := New(Options{
		Store: settings.NewStore(settings.Default()),
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			*built++
			return &terminal.Terminal{}, nil
		},
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
	})
	a.now = func() time.Time { return *clock }
	return a
}

// Every route that starts a session in the panel is single-flight: a held key
// repeats at a rate nothing on screen can follow, and each repeat used to fork
// another shell — the report 「这边的某些快捷键会导致反复弹出终端，从而导致卡死」.
// The gate is one gate (allowSpawn, launch.go) and every route asks it, so a
// route that forgets is the regression this test catches.
func TestEveryInPanelTerminalRouteIsSingleFlight(t *testing.T) {
	entry := extensions.CommandEntry{Program: "echo", Args: []string{"hi"}}
	routes := []struct {
		name string
		run  func(a *App)
	}{
		{"RunInTerminal", func(a *App) { a.Launcher.Actions.RunInTerminal([]string{"echo", "hi"}) }},
		{"RunInTerminalWithEnv", func(a *App) {
			a.Launcher.Actions.RunInTerminalWithEnv([]string{"echo", "hi"}, []string{"A=1"})
		}},
		{"RunInShell", func(a *App) { a.Launcher.Actions.RunInShell("echo hi") }},
		{"OpenInstallSession", func(a *App) { a.Launcher.Actions.OpenInstallSession("brew install x") }},
		{"OpenInTerminal", func(a *App) { a.Launcher.Actions.OpenInTerminal(t.TempDir()) }},
		{"RunCommand", func(a *App) { a.runCommand(entry, nil) }},
		{"OpenConfiguration", func(a *App) { a.openConfiguration("tool", []string{"echo", "cfg"}) }},
		{"NewSession", func(a *App) { a.Settings.Actions.NewSession() }},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			built := 0
			clock := time.Unix(4000, 0)
			a := spawnTestApp(t, &built, &clock)

			route.run(a)
			if built != 1 {
				t.Fatalf("the first press built %d sessions, want 1", built)
			}
			if a.Surf != SurfaceTerminal {
				t.Fatalf("the first press left the surface at %v", a.Surf)
			}

			// The same key, repeated: the session the first press started is
			// the one to show, not a second shell beside it.
			clock = clock.Add(20 * time.Millisecond)
			route.run(a)
			if built != 1 {
				t.Errorf("a repeat inside the gap built %d sessions, want 1", built)
			}
			if a.Surf != SurfaceTerminal {
				t.Errorf("a repeat left the surface at %v, want the terminal", a.Surf)
			}

			// A real second press after the gap is a request.
			clock = clock.Add(spawnGap + time.Millisecond)
			route.run(a)
			if built != 2 {
				t.Errorf("a press after the gap built %d sessions in total, want 2", built)
			}
		})
	}
}

// A custom shortcut bound to a command line is a process too: a held key must
// not run it once per repeat.
func TestAHeldCustomShortcutRunsItsCommandOncePerPress(t *testing.T) {
	ran := 0
	clock := time.Unix(5000, 0)
	a := New(Options{
		Store: settings.NewStore(settings.Default()),
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			return &terminal.Terminal{}, nil
		},
		RunSilentCommand: func(string) error { ran++; return nil },
		OpenAtLogin:      func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
	})
	a.now = func() time.Time { return clock }

	a.runCustomShortcut("echo hi")
	for range 5 {
		clock = clock.Add(20 * time.Millisecond)
		a.runCustomShortcut("echo hi")
	}
	if ran != 1 {
		t.Fatalf("the held shortcut ran its command %d times, want 1", ran)
	}
	clock = clock.Add(spawnGap)
	a.runCustomShortcut("echo hi")
	if ran != 2 {
		t.Errorf("a press after the gap ran %d commands in total, want 2", ran)
	}
}
