package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

func newStore(t *testing.T) *settings.Store {
	t.Helper()
	return settings.NewStore(settings.Default())
}

func render(t *testing.T, a *App, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
	}, w, h)
	tt.Frame()
	return tt
}

func TestSettingsShowsThePagesAndTheGeneralControls(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 580)

	for _, want := range []string{
		"Settings", "General", "Sessions", "Shortcuts", "Integrations", "About",
		"Appearance", "Language", "Liquid glass effect", "App transparency",
		"Terminal transparency", "Interface size",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

func TestSettingsControlsWriteThroughTheStore(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 580)

	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().Theme; got != "dark" {
		t.Errorf("theme = %q, want dark", got)
	}

	if err := tt.Click("Liquid Max"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().GlassStep; got != "liquid" {
		t.Errorf("glass step = %q, want liquid", got)
	}

	if err := tt.Click("Large"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().UIScale; got != "large" {
		t.Errorf("ui scale = %q, want large", got)
	}

	// The language switch is last: it relabels every control.
	if err := tt.Click("中文"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := store.Snapshot().Language; got != "zh" {
		t.Errorf("language = %q, want zh", got)
	}
	if !tt.HasText("设置") {
		t.Errorf("the panel did not switch language: %q", tt.Texts())
	}
}

func TestSettingsOpacityShowsAndFollowsTheStore(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 580)

	if !tt.HasText("47%") || !tt.HasText("46%") {
		t.Errorf("opacity values missing: %q", tt.Texts())
	}
	if err := store.Update(func(s *settings.Settings) { s.MainOpacity = 80 }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("80%") {
		t.Errorf("the control did not follow the store: %q", tt.Texts())
	}
}

func TestSetWritesThroughAndClamps(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	a.set(func(s *settings.Settings) { s.MainOpacity = 250 })
	if got := store.Snapshot().MainOpacity; got != settings.MaxWindowOpacity {
		t.Errorf("opacity = %d, want the %d ceiling", got, settings.MaxWindowOpacity)
	}
	a.set(func(s *settings.Settings) { s.MainOpacity = 1 })
	if got := store.Snapshot().MainOpacity; got != settings.MinWindowOpacity {
		t.Errorf("opacity = %d, want the %d floor", got, settings.MinWindowOpacity)
	}
	for _, tc := range []struct {
		in   float64
		want uint8
	}{{47.4, 47}, {47.6, 48}, {0, settings.MinWindowOpacity}, {250, settings.MaxWindowOpacity}} {
		if got := clampPercent(tc.in); got != tc.want {
			t.Errorf("clampPercent(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSettingsSidebarRoutesAndEscapeCloses(t *testing.T) {
	closed := 0
	a := New(newStore(t), Actions{Close: func() { closed++ }})
	tt := render(t, a, 720, 580)

	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Page != PageSessions {
		t.Fatalf("page = %d, want sessions", a.Page)
	}
	if !tt.HasText("This page arrives in a later round.") {
		t.Errorf("the placeholder did not show: %q", tt.Texts())
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if closed != 1 || a.Page != PageSessions {
		t.Errorf("Escape closed %d times, page %d", closed, a.Page)
	}
}

func TestTerminalAppearanceControls(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	tt := render(t, a, 720, 620)

	for _, want := range []string{
		"Terminal appearance", "Font size", "Font family", "Cursor shape",
		"Blinking cursor", "Line height", "Padding", "Terminal palette",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// The terminal group sits below the fold: scroll the body to reach it.
	tt.Scroll(400, 300, 0, 900)
	tt.Frame()

	if err := tt.Click("Block"); err != nil {
		t.Fatalf("cursor shape: %v", err)
	}
	tt.Frame()
	if got := store.Snapshot().CursorShape; got != "block" {
		t.Errorf("cursor shape = %q, want block", got)
	}

	if err := tt.Click("Blinking cursor"); err != nil {
		t.Fatalf("blink: %v", err)
	}
	tt.Frame()
	if store.Snapshot().CursorBlink {
		t.Error("the blink switch did not turn off")
	}

	if err := tt.Click("Relaxed"); err != nil {
		t.Fatalf("padding: %v", err)
	}
	tt.Frame()
	if got := store.Snapshot().TerminalPadding; got != "relaxed" {
		t.Errorf("padding = %q, want relaxed", got)
	}

	if err := tt.Click("Terminal palette"); err != nil {
		t.Fatalf("palette trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Forest"); err != nil {
		t.Fatalf("palette option: %v", err)
	}
	tt.Frame()
	if got := store.Snapshot().TerminalTheme; got != "forest" {
		t.Errorf("palette = %q, want forest", got)
	}
}
