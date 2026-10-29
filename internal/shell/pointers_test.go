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
	// Each row gets a fresh app: the first arms a power confirmation, which
	// would otherwise be the state the second row reads. P11-bk: the built-in
	// rows answer a typed word now (the empty page is the recents' block), so
	// each case types the word its row answers to. The six rows are still the
	// reference's (no settings, no quit — both are tray and app menu rows), and
	// the veil is still exercised on a power row and on the terminal.
	for _, tc := range []struct{ query, label string }{
		{"restart", "Restart the computer"},
		{"terminal", "Terminal"},
	} {
		a := New(Options{Store: settings.NewStore(settings.Default())})
		a.Launcher.SetQuery(tc.query)
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
		tt.Frame()
		if err := tt.Click(tc.label); err != nil {
			t.Fatalf("%s: %v", tc.label, err)
		}
		tt.Frame()
		switch tc.label {
		case "Restart the computer":
			if a.Launcher.PowerPending() == "" {
				t.Errorf("%s did not arm the confirmation", tc.label)
			}
		case "Terminal":
			if a.Surf != SurfaceTerminal {
				t.Errorf("%s left surf=%v", tc.label, a.Surf)
			}
		}
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
