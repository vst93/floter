package shell

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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
	clock := time.Unix(1700000000, 0)
	a.now = func() time.Time { return clock }
	opened := []string{}
	a.openURLCommand = func(browserID, url string) error { opened = append(opened, "url:"+url); return nil }

	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A web URL goes to the browser, whatever its scheme's spelling.
	for _, link := range []string{"https://example.com/a", "HTTPS://example.com/b"} {
		if !a.openTerminalLink(link) {
			t.Errorf("%q did not open", link)
		}
	}
	if len(opened) != 2 || opened[0] != "url:https://example.com/a" || opened[1] != "url:HTTPS://example.com/b" {
		t.Errorf("opened = %v", opened)
	}

	// A non-web scheme is refused by the generic opener. The terminal callback
	// still claims the click so the framework does not fall back to its own
	// unrestricted opener.
	if !a.openTerminalLink("mailto:someone@example.com") {
		t.Errorf("a refused non-web link was not claimed")
	}
	if len(opened) != 2 {
		t.Errorf("a non-web link reached the browser: %v", opened)
	}

	// A path goes to the file manager.
	if !a.openTerminalLink(file) {
		t.Errorf("a path did not open")
	}
	if len(calls.opened) != 1 || calls.opened[0] != file {
		t.Errorf("opened paths = %v", calls.opened)
	}
	// A repeat of the same click inside the spawn gap is the key repeating,
	// not a second request: one file, one window (see allowActivate).
	if !a.openTerminalLink(file) {
		t.Errorf("a repeated click was not claimed")
	}
	if len(calls.opened) != 1 {
		t.Errorf("a repeat opened the path twice: %v", calls.opened)
	}
	// A compiler's `:line:col` is stripped when the whole string is not a file.
	clock = clock.Add(time.Second)
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

// The path action resolves what the user typed the way the old `open_path`
// did — `~` and a relative path against the home directory — and refuses a path
// that is not there: a typo answers the query instead of silently opening
// nothing.
func TestOpenPathResolvesAndRequiresExistence(t *testing.T) {
	calls := &launchCalls{}
	a := launchTestApp(t, calls)
	home := t.TempDir()
	a.homeDir = home
	clock := time.Unix(1700000000, 0)
	a.now = func() time.Time { return clock }
	file := filepath.Join(home, "note.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	// `~` and a bare relative path both land in the home directory. They are
	// two presses a second apart: inside the spawn gap the second one is a
	// repeat of the first (see allowActivate).
	a.openPath("~/note.txt")
	clock = clock.Add(time.Second)
	a.openPath("note.txt")
	if want := []string{file, file}; !reflect.DeepEqual(calls.opened, want) {
		t.Fatalf("opened = %v, want %v", calls.opened, want)
	}
	// The same path twice within the gap is one open.
	a.openPath("note.txt")
	if len(calls.opened) != 2 {
		t.Fatalf("a repeat opened the path again: %v", calls.opened)
	}

	// A path that is not there never reaches the opener, and says so in the
	// docked row.
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 700)
	tt.Frame()
	before := len(calls.opened)
	a.openPath("missing.txt")
	if len(calls.opened) != before {
		t.Errorf("a missing path reached the opener: %v", calls.opened)
	}
	tt.Frame()
	if want := i18n.For("en").Launcher.OpenPathFailed; !tt.HasText(want) {
		t.Errorf("the refusal %q was not drawn: %q", want, tt.Texts())
	}
}

// The resolver is the old `resolve_path` exactly: `~` is home, `~alice` is a
// directory literally named that, and a relative path is joined to home.
func TestResolvePathMatchesTheOldResolver(t *testing.T) {
	home := t.TempDir()
	abs := filepath.Join(t.TempDir(), "x")
	cases := []struct{ in, want string }{
		{"~", home},
		{"~/", home},
		{"~//notes", filepath.Join(home, "notes")},
		{"~/notes/a.md", filepath.Join(home, "notes", "a.md")},
		{"~alice", filepath.Join(home, "~alice")},
		{"notes/a.md", filepath.Join(home, "notes", "a.md")},
		{abs, abs},
	}
	for _, tc := range cases {
		if got := resolvePath(tc.in, home); got != tc.want {
			t.Errorf("resolvePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A terminal link printed with `~` resolves against home, the same resolution
// the path action uses.
func TestTerminalLinkResolvesTilde(t *testing.T) {
	calls := &launchCalls{}
	a := launchTestApp(t, calls)
	home := t.TempDir()
	a.homeDir = home
	file := filepath.Join(home, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !a.openTerminalLink("~/main.go:12") {
		t.Fatal("a `~` link was not claimed")
	}
	if len(calls.opened) != 1 || calls.opened[0] != file {
		t.Errorf("opened = %v, want [%s]", calls.opened, file)
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

// A shortcut the desktop repeats while its key is held must start one
// terminal, not one per repeat: the hand-off is a process (an emulator window,
// or a shell in the panel), and a held key outruns anything the user can see.
// The report 「这边的某些快捷键会导致反复弹出终端，从而导致卡死」 is that
// storm; the second press inside the gap is a repeat.
func TestTerminalHandOffIgnoresRepeats(t *testing.T) {
	calls := &launchCalls{}
	a := launchTestApp(t, calls)
	clock := time.Unix(1000, 0)
	a.now = func() time.Time { return clock }

	a.openTerminalWindow()
	a.openTerminalWindow()
	a.openTerminalWindow()
	if calls.external != 1 {
		t.Fatalf("three presses started %d terminals, want 1", calls.external)
	}
	clock = clock.Add(spawnGap + time.Millisecond)
	a.openTerminalWindow()
	if calls.external != 2 {
		t.Errorf("a press after the gap started %d terminals in total, want 2", calls.external)
	}
}

// Every route to the user's own emulator goes through the same gate: the
// launcher's row, the terminal page's hint, and a custom shortcut's action.
// Before this, only the launcher's own callback was gated, so the other two
// started a window per key repeat — the same storm by another door.
func TestEveryExternalTerminalRouteIsSingleFlight(t *testing.T) {
	calls := &launchCalls{}
	a := launchTestApp(t, calls)
	clock := time.Unix(2000, 0)
	a.now = func() time.Time { return clock }

	a.runAppAction(settings.CustomActionOpenExternalTerminal)
	a.runAppAction(settings.CustomActionOpenExternalTerminal)
	if calls.external != 1 {
		t.Fatalf("the custom action started %d terminals, want 1", calls.external)
	}

	clock = clock.Add(spawnGap + time.Millisecond)
	a.spawnExternalTerminal(func(error) {})
	if calls.external != 2 {
		t.Errorf("a hand-off after the gap started %d terminals in total, want 2", calls.external)
	}
	// And the terminal page's hint shares the same clock: an immediate repeat
	// is dropped rather than opening a third window.
	a.spawnExternalTerminal(func(error) {})
	if calls.external != 2 {
		t.Errorf("a repeat right after the gap started %d terminals, want 2", calls.external)
	}
}

// A held summon key strobes the panel: the desktop repeats it, so every repeat
// asked for a reveal or a dismissal, and the window flips as fast as the keys
// arrive. One toggle per press is the intent.
func TestSummonToggleIgnoresKeyRepeats(t *testing.T) {
	a := launchTestApp(t, &launchCalls{})
	clock := time.Unix(3000, 0)
	a.now = func() time.Time { return clock }

	if !a.allowToggle() {
		t.Fatal("the first press was dropped")
	}
	for i := range 5 {
		if a.allowToggle() {
			t.Fatalf("repeat %d was treated as a press", i+1)
		}
		clock = clock.Add(20 * time.Millisecond)
	}
	clock = clock.Add(toggleGap)
	if !a.allowToggle() {
		t.Error("a press after the gap was dropped")
	}
}

// A row activation is a process on the desktop, and Enter repeats while it is
// held: one open per press. The rule is keyed by the row's own target, so a
// repeat of the same row is dropped while stepping to another row and pressing
// Enter still opens the second thing — the report 「反复弹出终端，从而导致卡死」
// is the same storm one row over.
func TestARowActivationIsOneProcessPerPress(t *testing.T) {
	clock := time.Unix(1700000000, 0)
	a := launchTestApp(t, &launchCalls{})
	a.now = func() time.Time { return clock }

	if !a.allowActivate("path\x00/a/one.txt") {
		t.Fatal("the first open of a row was refused")
	}
	if a.allowActivate("path\x00/a/one.txt") {
		t.Error("a repeat of the same row inside the gap was let through")
	}
	if !a.allowActivate("path\x00/a/two.txt") {
		t.Error("a different row inside the gap was refused")
	}
	clock = clock.Add(spawnGap)
	if !a.allowActivate("path\x00/a/one.txt") {
		t.Error("the same row after the gap was refused")
	}
	// A target-less activation is the handler's own decision, not a repeat.
	if !a.allowActivate("") || !a.allowActivate("") {
		t.Error("a target-less activation was gated")
	}
	// The table does not grow without bound over a long session.
	for i := 0; i < 200; i++ {
		clock = clock.Add(spawnGap)
		a.allowActivate("url\x00https://example.com/" + string(rune('a'+i%26)) + string(rune('a'+i/26)))
	}
	if len(a.spawnAt) > 65 {
		t.Errorf("the spawn table grew to %d entries", len(a.spawnAt))
	}
}
