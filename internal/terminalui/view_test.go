package terminalui

import (
	"errors"
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
