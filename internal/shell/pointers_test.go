package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/settingsui"
)

// The haze veil paints over the face it composes the glass with, and a
// plain box over the rows would eat their clicks: the veil is a color,
// never a control, so it lets the pointer through. This is the one bug a
// row could hide behind — the row showed (Find reads labels) but no press
// ever reached it.
func TestHazeVeilLetsThePointerThrough(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()
	for _, label := range []string{"Open settings", "Open terminal"} {
		if err := tt.Click(label); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		tt.Frame()
		switch label {
		case "Open settings":
			if a.Surf != SurfaceSettings {
				t.Errorf("%s left surf=%v", label, a.Surf)
			}
		case "Open terminal":
			if a.Surf != SurfaceTerminal {
				t.Errorf("%s left surf=%v", label, a.Surf)
			}
		}
		// Back to the launcher for the next row: the terminal surface
		// holds the window's height, so the launcher's rows sit where the
		// launcher's own band puts them again.
		a.Open(SurfaceLauncher)
		tt.SetSize(720, 420)
		tt.Frame()
	}
	// Quit is the last row: a fresh app whose quit is a recorded call, so
	// the assert outlives the window.
	quits := 0
	a2 := New(Options{Store: settings.NewStore(settings.Default()), Quit: func() { quits++ }})
	tt2 := ui.NewTester(func(c *ui.Context) { a2.View(c) }, 720, 420)
	tt2.Frame()
	if err := tt2.Click("Quit floter"); err != nil {
		t.Fatalf("quit: %v", err)
	}
	tt2.Frame()
	if quits != 1 {
		t.Errorf("Quit ran %d times, want 1", quits)
	}
}

// The settings surface carries the same card, so its rows answer too: the
// sidebar switches the page through the veil.
func TestSettingsRowsAnswerThroughTheVeil(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default())})
	a.Open(SurfaceSettings)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 560)
	tt.Frame()
	if err := tt.Click("Integrations"); err != nil {
		t.Fatalf("sidebar: %v", err)
	}
	tt.Frame()
	if a.Settings.Page != settingsui.PageIntegrations {
		t.Errorf("page = %v, want integrations", a.Settings.Page)
	}
}
