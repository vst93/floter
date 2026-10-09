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
	a := New(newStore(), Actions{}, func() (*terminal.Terminal, error) {
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
