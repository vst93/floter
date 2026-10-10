package shell

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/drops"
	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
)

// launchApp is a shell whose hand-offs to the system are recorded instead of
// run: opening a path, the terminal in a directory, the user's own terminal,
// and a power action.
type launchCalls struct {
	opened     []string
	terminals  []string
	external   int
	installs   []string
	powerCalls []string
}

func launchTestApp(t *testing.T, calls *launchCalls) *App {
	t.Helper()
	return New(Options{
		Store: settings.NewStore(settings.Default()),
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			return nil, errors.New("no library in tests")
		},
		OpenPath:         func(path string) error { calls.opened = append(calls.opened, path); return nil },
		RunPowerCommand:  func(action string) error { calls.powerCalls = append(calls.powerCalls, action); return nil },
		RunSilentCommand: func(string) error { return nil },
		OpenExternalTerminal: func() error {
			calls.external++
			return nil
		},
	})
}

// A dropped file's rows hand their path to the system: "open" opens it, and
// the directory row opens the terminal surface in the file's directory. Both
// were launcher actions the shell never answered.
func TestDroppedFileRowsReachTheSystem(t *testing.T) {
	calls := &launchCalls{}
	a := launchTestApp(t, calls)
	dir := t.TempDir()
	path := dir + "/note.txt"
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.Launcher.SetDropped([]drops.File{{Path: path, Name: "note.txt", Directory: dir}})
	a.Launcher.EnterFiles()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 700)
	tt.Frame()

	copy := i18n.For("en").Launcher
	if err := tt.Click(copy.FileOpen + "  note.txt"); err != nil {
		t.Fatalf("open: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if len(calls.opened) != 1 || calls.opened[0] != path {
		t.Errorf("opened = %v, want %q", calls.opened, path)
	}
	// The directory row opens the terminal surface in the file's own
	// directory: a file's "cd" stands next to it.
	a.Launcher.EnterFiles()
	tt.Frame()
	if err := tt.Click(copy.FileCd + "  note.txt"); err != nil {
		t.Fatalf("cd: %v (texts %v)", err, tt.Texts())
	}
	tt.Frame()
	if a.Surf != SurfaceTerminal {
		t.Errorf("the cd row left the surface at %v", a.Surf)
	}
}

// The external-terminal shortcut opens the user's own terminal emulator: the
// action the shell never answered, so the shortcut did nothing.
func TestExternalTerminalShortcutOpensATerminal(t *testing.T) {
	calls := &launchCalls{}
	a := launchTestApp(t, calls)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()
	// The launcher's own action is what the shortcut fires, and the shell
	// answers it now.
	a.Launcher.Actions.OpenTerminalWindow()
	if calls.external != 1 {
		t.Errorf("the terminal was opened %d times", calls.external)
	}
	_ = launcher.PowerRestart
}

// Every action the launcher declares is answered by the shell. A callback the
// shell never sets is a feature that silently does nothing — five were, before
// this test: the power rows, opening a path, the terminal in a directory, the
// user's own terminal, and the install rows.
func TestEveryLauncherActionIsWired(t *testing.T) {
	a := launchTestApp(t, &launchCalls{})
	actions := reflect.ValueOf(a.Launcher.Actions)
	kind := actions.Type()
	for i := 0; i < kind.NumField(); i++ {
		field := kind.Field(i)
		if field.Type.Kind() != reflect.Func {
			continue
		}
		if actions.Field(i).IsNil() {
			t.Errorf("the shell never answered launcher.Actions.%s", field.Name)
		}
	}
}
