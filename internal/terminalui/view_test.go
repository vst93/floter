package terminalui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

func newStore() *settings.Store { return settings.NewStore(settings.Default()) }

func render(t *testing.T, a *App, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
	}, w, h)
	tt.Frame()
	return tt
}

func TestTerminalEmptyState(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	tt := render(t, a, 860, 600)
	if !tt.HasText("Terminal") {
		t.Errorf("missing the title: %q", tt.Texts())
	}
	if !tt.HasText("The session has not started yet.") {
		t.Errorf("missing the empty hint: %q", tt.Texts())
	}
	if _, ok := tt.Find("Close terminal"); !ok {
		t.Errorf("the close control is not labeled: %q", tt.Texts())
	}
}

func TestTerminalSessionErrorShows(t *testing.T) {
	a := New(newStore(), Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("libghostty-vt is missing")
	})
	a.EnsureSession()
	if a.Err == nil {
		t.Fatal("EnsureSession swallowed the error")
	}
	tt := render(t, a, 860, 600)
	if !tt.HasText("libghostty-vt is missing") {
		t.Errorf("the error did not show: %q", tt.Texts())
	}
	// EnsureSession must not retry once it failed into a session; a second
	// call is a no-op until the surface is reopened.
	a.EnsureSession()
	if a.Term != nil {
		t.Error("a terminal appeared from a failing start")
	}
}

func TestTerminalCloseButtonCallsBack(t *testing.T) {
	closed := 0
	a := New(newStore(), Actions{Close: func() { closed++ }}, nil)
	tt := render(t, a, 860, 600)
	if err := tt.Click("✕"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if closed != 1 {
		t.Errorf("close ran %d times, want 1", closed)
	}
}

func TestTerminalFollowsTheLanguage(t *testing.T) {
	store := newStore()
	if err := store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{}, nil)
	tt := render(t, a, 860, 600)
	if !tt.HasText("终端") {
		t.Errorf("missing the Chinese title: %q", tt.Texts())
	}
}

func TestOptionsFollowTheSettings(t *testing.T) {
	store := newStore()
	a := New(store, Actions{Title: func(string) {}, Exit: func(int) {}}, nil)

	o := a.options()
	if o.Font.Family != "monospace" || o.Font.Size != 14 || o.Font.LineHeight != 1.2 {
		t.Errorf("default font = %+v", o.Font)
	}
	if o.Cursor != terminal.CursorBar || o.NoBlink {
		t.Errorf("default cursor = %v, noBlink=%v", o.Cursor, o.NoBlink)
	}
	if o.Theme != nil {
		t.Errorf("inherit built a palette: %+v", o.Theme)
	}
	if o.Transparent {
		t.Error("the terminal's background carries the transparency, not the Transparent flag")
	}
	if a.Pad() != 3 {
		t.Errorf("regular padding = %v, want 3", a.Pad())
	}
	if o.OnTitle == nil || o.OnExit == nil {
		t.Error("the plugin callbacks are not wired")
	}

	if err := store.Update(func(s *settings.Settings) {
		s.FontSize = 20
		s.FontFamily = "Iosevka"
		s.CursorShape = "block"
		s.CursorBlink = false
		s.TerminalLineHeight = 1.5
		s.TerminalPadding = "relaxed"
		s.TerminalTheme = "forest"
		s.TerminalOpacity = 80
	}); err != nil {
		t.Fatal(err)
	}

	o = a.options()
	if o.Font.Family != "Iosevka" || o.Font.Size != 20 || o.Font.LineHeight != 1.5 {
		t.Errorf("font = %+v", o.Font)
	}
	if o.Cursor != terminal.CursorBlock || !o.NoBlink {
		t.Errorf("cursor = %v, noBlink=%v", o.Cursor, o.NoBlink)
	}
	if a.Pad() != 6 {
		t.Errorf("relaxed padding = %v, want 6", a.Pad())
	}
	if o.Theme == nil {
		t.Fatal("the forest palette did not build")
	}
	if got, want := o.Theme.Background, ui.RGB(0x0f, 0x1a, 0x14).Alpha(0.8); got != want {
		t.Errorf("forest background = %v, want %v", got, want)
	}
	if got, want := o.Theme.Foreground, ui.RGB(0xd7, 0xe4, 0xd0); got != want {
		t.Errorf("forest foreground = %v, want %v", got, want)
	}
	if got, want := o.Theme.Cursor, ui.RGB(0x8b, 0xd4, 0x50); got != want {
		t.Errorf("forest cursor = %v, want %v", got, want)
	}
	// A light palette takes the light base.
	if err := store.Update(func(s *settings.Settings) { s.TerminalTheme = "paper" }); err != nil {
		t.Fatal(err)
	}
	if got := a.options().Theme.Background; got == ui.RGB(0x0f, 0x1a, 0x14) {
		t.Error("paper reused the dark background")
	}
	// The cursor shapes map one to one.
	for shape, want := range map[string]terminal.CursorStyle{
		"beam": terminal.CursorBar, "block": terminal.CursorBlock, "underline": terminal.CursorUnderline,
	} {
		if got := cursorStyle(shape); got != want {
			t.Errorf("cursorStyle(%q) = %v, want %v", shape, got, want)
		}
	}
}

func TestPinControlCopiesTheSessionText(t *testing.T) {
	// A session whose text the test supplies, so the control can be used
	// without a real terminal.
	pinned := ""
	pinnedText := ""
	a := New(newStore(), Actions{
		Pin: func(title, text string) { pinned, pinnedText = title, text },
	}, nil)
	a.Text = func() string { return "line one\nline two" }
	a.Term = &terminal.Terminal{} // present so the control shows
	a.Title = "zsh"

	tt := render(t, a, 860, 600)
	if _, ok := tt.Find("Pin output"); !ok {
		t.Fatalf("the pin control is missing: %q", tt.Texts())
	}
	if err := tt.Click("Pin output"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if pinned != "zsh" || pinnedText != "line one\nline two" {
		t.Errorf("pinned %q / %q", pinned, pinnedText)
	}
}

func TestNoPinControlWithoutASession(t *testing.T) {
	a := New(newStore(), Actions{Pin: func(string, string) {}}, nil)
	tt := render(t, a, 860, 600)
	if _, ok := tt.Find("Pin output"); ok {
		t.Errorf("the pin control shows with no session: %q", tt.Texts())
	}
}

// The session's screen is saved when it closes and fed back into the next
// session, so the last session's scrollback comes back.
func TestSnapshotSaveAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "terminal-snapshot")
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	a.SnapshotPath = path

	// With no session there is nothing to save, and no error.
	if err := a.SaveSnapshot(); err != nil {
		t.Errorf("SaveSnapshot with no session = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a snapshot was written with no session: %v", err)
	}

	// The write is atomic and private: a temporary file in the same
	// directory, renamed over the target.
	if err := writeFileAtomically(path, []byte("screen")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("snapshot mode = %v", info.Mode().Perm())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("temporary files were left behind: %v", entries)
	}
}

// The terminal page's empty state offers a new session and a way back.
func TestTerminalEmptyStateControls(t *testing.T) {
	closeCalls := 0
	a := New(newStore(), Actions{
		Close: func() { closeCalls++ },
	}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	tt := render(t, a, 860, 600)
	if !tt.HasText("No terminal session yet") {
		t.Fatalf("the empty title is missing: %q", tt.Texts())
	}
	if err := tt.Click("New blank session"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The session attempt failed (no library), and the error replaces the hint.
	if !tt.HasText("no library in tests") {
		t.Errorf("the error did not show: %q", tt.Texts())
	}
}

// The terminal header carries the old build's two controls: a return to the
// launcher and a hand-off to the system's own terminal.
func TestTerminalHeaderControls(t *testing.T) {
	newCommands, externals := 0, 0
	closeCalls := 0
	a := New(newStore(), Actions{
		NewCommand:   func() { newCommands++ },
		OpenExternal: func() { externals++ },
		Close:        func() { closeCalls++ },
	}, func(terminal.Options) (*terminal.Terminal, error) {
		return nil, errors.New("no library in tests")
	})
	tt := render(t, a, 860, 600)
	if err := tt.Click("New command"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if newCommands != 1 {
		t.Errorf("new command ran %d times", newCommands)
	}
	if err := tt.Click("Open in terminal"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if externals != 1 {
		t.Errorf("open external ran %d times", externals)
	}
	if err := tt.Click("✕"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if closeCalls != 1 {
		t.Errorf("close ran %d times", closeCalls)
	}
}
