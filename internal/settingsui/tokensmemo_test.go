package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// A settings frame asks for the card language from every row it draws —
// `a.tokens(c)` has 97 call sites on this surface, all of them while drawing,
// and each ask used to build a whole `ui.Theme` and palette plus a full
// settings snapshot (`internal/theme`'s profile put the build at 29% of
// `BenchmarkIntegrationsFrame`). One frame must resolve one palette, however
// many rows it draws.
func TestOneSettingsFrameResolvesOneCardLanguage(t *testing.T) {
	a := New(settings.NewStore(settings.Default()), Actions{})
	ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580).Frame()
	if got := a.tokensMemo.Builds(); got != 1 {
		t.Errorf("one General frame built %d palettes, want 1", got)
	}

	// The integrations page is the one with the most rows: a card per
	// integration and a row per command.
	a.Integrations = func() []Integration {
		out := make([]Integration, 0, 10)
		for i := 0; i < 10; i++ {
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
	a.Page = PageIntegrations
	ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580).Frame()
	if got := a.tokensMemo.Builds(); got != 1 {
		t.Errorf("with the integrations page drawn the memo built %d palettes over two frames, want 1", got)
	}
}

// The memo is keyed by the store's version and its identity, so it must never
// mask a change: a slider's preview (a settings update) reaches the next frame,
// and a store swapped under a built App (both start at version zero) is not
// served the other one's palette.
func TestTheCardLanguageFollowsTheSettingsAndTheStore(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{})
	var got theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		got = a.tokens(c)
		a.View(c)
	}, 720, 580)
	tt.Frame()
	first := got

	if err := store.Update(func(s *settings.Settings) { s.UIScale = "large" }); err != nil {
		t.Fatalf("Update: %v", err)
	}
	tt.Frame()
	if got.Scale == first.Scale {
		t.Errorf("the interface step did not reach the card language: both scale %.2f (a stale memo)", got.Scale)
	}
	if n := a.tokensMemo.Builds(); n != 2 {
		t.Fatalf("the update left the memo at %d builds, want 2", n)
	}

	// A different store, still at version zero, with the default step.
	a.Store = settings.NewStore(settings.Default())
	tt.Frame()
	if got.Scale != first.Scale {
		t.Errorf("after the store was swapped the card language still says scale %.2f, want the new store's %.2f",
			got.Scale, first.Scale)
	}
	if n := a.tokensMemo.Builds(); n != 1 {
		t.Errorf("the swap left the memo at %d builds, want 1 (a fresh memo for the new store)", n)
	}
}
