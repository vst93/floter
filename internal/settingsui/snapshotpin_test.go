package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P11-co · **一帧只拷一份设置。**
//
// `Store.Snapshot` is a deep copy, and this surface read it from every card,
// every row and every word it drew: instrumented (a counter in `Snapshot`) a
// General frame took **15** copies and an Integrations frame — twenty cards of
// five commands — took **53**. `App.settings` answers one per frame from a
// `settings.Memo`, so the copy count must stay at one however many rows the
// page has, and the page must still draw the value the store holds *now*.
func TestASettingsFrameCopiesTheSettingsOnce(t *testing.T) {
	integrations := func() []Integration {
		out := make([]Integration, 0, 20)
		for i := 0; i < 20; i++ {
			commands := make([]Command, 0, 5)
			for j := 0; j < 5; j++ {
				commands = append(commands, Command{ID: "c", Name: "Command", Enabled: true, Available: true})
			}
			out = append(out, Integration{
				ID: "ext", Name: "Integration", Enabled: true, Running: true,
				Permissions: []string{"filesystem-read", "environment"},
				Enforced:    map[string]bool{"environment": true},
				Commands:    commands,
			})
		}
		return out
	}

	for _, arm := range []struct {
		name   string
		page   int
		setup  func(*App)
		before uint64
	}{
		{name: "the General page", page: PageGeneral},
		{name: "the Integrations page", page: PageIntegrations, setup: func(a *App) { a.Integrations = integrations }},
	} {
		store := newStore(t)
		a := New(store, Actions{
			SetIntegrationEnabled: func(string, bool, func(error)) {},
			SetCommandEnabled:     func(string, string, bool) {},
		})
		if arm.setup != nil {
			arm.setup(a)
		}
		a.Page = arm.page
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
		tt.Frame()

		// A change is a new version, so the next frame takes exactly one copy
		// — not one per row, and not none (a frame must not draw a stale page).
		if err := store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
			t.Fatalf("%s: the update failed: %v", arm.name, err)
		}
		before := store.Snapshots()
		tt.Frame()
		if got := store.Snapshots() - before; got != 1 {
			t.Errorf("%s: one frame took %d settings snapshots, want 1", arm.name, got)
		}

		// A steady frame takes none at all: the value did not move.
		before = store.Snapshots()
		tt.Frame()
		if got := store.Snapshots() - before; got != 0 {
			t.Errorf("%s: a steady frame took %d settings snapshots, want 0", arm.name, got)
		}
	}
}

// The cache must never be what a page shows instead of the value the store
// holds: a frame after a change draws the changed words.
func TestASettingsFrameDrawsTheChangedValue(t *testing.T) {
	store := newStore(t)
	a := New(store, Actions{})
	a.Page = PageGeneral
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	tt.Frame()

	if err := store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatalf("the update failed: %v", err)
	}
	tt.Frame()
	if got := a.settings().Language; got != "zh" {
		t.Fatalf("the frame served language %q, want zh", got)
	}
}
