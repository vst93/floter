package shell

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// A Command+click in the terminal opens what it points at: a URL in the user's
// browser, an absolute path in the file manager. A path a compiler printed with
// a `:line:col` suffix is opened without it, and a relative path is left alone
// rather than guessed against the app's own directory.
func TestTerminalLinkOpening(t *testing.T) {
	calls := &launchCalls{}
	a := launchTestApp(t, calls)
	opened := []string{}
	a.openURLCommand = func(browserID, url string) { opened = append(opened, "url:"+url) }

	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A URL goes to the browser, whatever its scheme's spelling.
	for _, link := range []string{"https://example.com/a", "mailto:someone@example.com"} {
		if !a.openTerminalLink(link) {
			t.Errorf("%q did not open", link)
		}
	}
	if len(opened) != 2 || !strings.HasPrefix(opened[0], "url:https://") {
		t.Errorf("opened = %v", opened)
	}

	// A path goes to the file manager.
	if !a.openTerminalLink(file) {
		t.Errorf("a path did not open")
	}
	if len(calls.opened) != 1 || calls.opened[0] != file {
		t.Errorf("opened paths = %v", calls.opened)
	}
	// A compiler's `:line:col` is stripped when the whole string is not a file.
	if !a.openTerminalLink(file + ":12:3") {
		t.Errorf("a path with a line suffix did not open")
	}
	if len(calls.opened) != 2 || calls.opened[1] != file {
		t.Errorf("opened paths = %v", calls.opened)
	}
	// A relative path is not opened: the session's directory is not ours.
	if a.openTerminalLink("src/main.go:12") {
		t.Error("a relative path was opened")
	}
	if a.openTerminalLink("") {
		t.Error("an empty link was opened")
	}
}

// The suffix stripper only takes digits, and at most a line and a column.
func TestStripLineSuffix(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"/a/b.go:12", "/a/b.go", true},
		{"/a/b.go:12:3", "/a/b.go", true},
		{"/a/b.go", "", false},
		{"/a/b.go:", "", false},
		{"/a/b.go:x", "", false},
		{"/a/12/b.go", "", false},
		{"C:/a/b.go:12", "C:/a/b.go", true},
	}
	for _, tc := range cases {
		got, ok := stripLineSuffix(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("stripLineSuffix(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

// A program's own notification request goes through the app's channel, with the
// program's words; an empty request says nothing.
func TestProgramNotification(t *testing.T) {
	notes := [][2]string{}
	a := New(Options{
		Store:  settings.NewStore(settings.Default()),
		Notify: func(title, body string) { notes = append(notes, [2]string{title, body}) },
	})
	a.notifyFromProgram("Build", "done in 3s")
	if len(notes) != 1 || notes[0][0] != "Build" || notes[0][1] != "done in 3s" {
		t.Errorf("notes = %v", notes)
	}
	// A body with no title is named by the app, not left blank.
	a.notifyFromProgram("", "something happened")
	if len(notes) != 2 || notes[1][0] == "" {
		t.Errorf("notes = %v", notes)
	}
	// Nothing asked, nothing said.
	a.notifyFromProgram("", "   ")
	if len(notes) != 2 {
		t.Errorf("an empty request said something: %v", notes)
	}
}

// A query with shell syntax is handed to the user's own shell, not to argv:
// the shell answers `Actions.RunInShell` by opening a session and typing the
// line, so the surface is where the user sees it (the injected constructor
// fails, so the surface shows the error instead of a session — the point is
// the hand-off, exactly as the argv route's test above).
func TestAShellSyntaxLineOpensTheTerminalSurface(t *testing.T) {
	calls := &launchCalls{}
	a := launchTestApp(t, calls)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()

	a.Launcher.Actions.RunInShell("ls -la | grep go")
	if a.Surf != SurfaceTerminal {
		t.Errorf("surface = %v, want terminal", a.Surf)
	}
}
