package shell

import (
	"errors"
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

type shortcutLifecycle struct {
	app     *App
	live    map[string]func()
	blocked map[string]bool
	claims  map[string]int
	limit   map[string]int
}

func newShortcutLifecycle(t *testing.T, summon string) *shortcutLifecycle {
	t.Helper()
	s := settings.Default()
	s.Language = "en"
	s.SetExtra("hotkey", summon)
	s.SetShortcut(settings.ShortcutToggleWindow, summon)
	s.SetCustomShortcuts([]settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "say original"}})
	h := &shortcutLifecycle{live: map[string]func(){}, blocked: map[string]bool{}, claims: map[string]int{}, limit: map[string]int{}}
	h.app = New(Options{Store: settings.NewStore(s), Paths: extensions.FromRoot(t.TempDir()),
		RegisterShortcut: func(key string, fn func()) error {
			h.claims[key]++
			if h.blocked[key] || h.live[key] != nil || (h.limit[key] > 0 && h.claims[key] > h.limit[key]) {
				return errors.New("occupied")
			}
			h.live[key] = fn
			return nil
		},
		UnregisterShortcut: func(key string) { delete(h.live, key) },
		RunSilentCommand:   func(string) error { return nil },
	})
	h.app.summonKey = ""
	h.app.ApplySummonShortcut()
	h.app.ApplyCustomShortcuts()
	h.app.Settings.Page = settingsui.PageShortcuts
	return h
}

func (h *shortcutLifecycle) frame(dark bool) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		surface := launcher.Resolve(h.app.Store.Snapshot(), c.Theme().Dark, c.Preferences())
		c.SetTheme(surface.Tokens.Theme)
		h.app.Settings.View(c)
	}, 960, 1100)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt
}

func lifecycleDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	tag := os.Getenv("FLOTER_P13IW_DUMP")
	if tag == "" {
		return
	}
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	f, err := os.Create("/tmp/floter-P13-iw-" + name + "-" + appearance + "-" + tag + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func lifecycleRecord(t *testing.T, h *shortcutLifecycle, tt *ui.Tester, key string) {
	t.Helper()
	if err := tt.Click(shortcuts.Display(key)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !h.app.shortcutsSuspended {
		t.Fatal("real recorder did not suspend global keys")
	}
}

func TestShortcutLifecycleResumeSummonFeedback(t *testing.T) { checkLifecycleResume(t, false) }
func TestShortcutLifecycleResumeCustomFeedback(t *testing.T) { checkLifecycleResume(t, true) }

func checkLifecycleResume(t *testing.T, custom bool) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			tt := h.frame(dark)
			key, name := "Ctrl+Alt+J", "resume-summon"
			copy := i18n.For("en").Settings
			warning := copy.ShortcutRejected
			if custom {
				key, name, warning = "Ctrl+Shift+B", "resume-custom", copy.ShortcutsOccupied
			}
			lifecycleRecord(t, h, tt, key)
			h.blocked[key] = true
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			tt.Frame()
			lifecycleDump(t, tt, name, dark)
			if h.live[key] != nil || h.app.shortcutsSuspended {
				t.Fatal("resume did not reach actual OS refusal")
			}
			if !tt.HasText(warning) {
				t.Error("resume refusal has no visible row feedback")
			}
			// A second real recording/cancel is a fresh registration attempt.
			if !tt.Focused(copy.ShortcutRecord) {
				t.Error("resume feedback lost recorder keyboard focus")
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if !h.app.shortcutsSuspended {
				t.Fatal("Enter cannot retry original recorder after resume feedback")
			}
			h.blocked[key] = false
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			tt.Frame()
			lifecycleDump(t, tt, name+"-recovered", dark)
			if h.live[key] == nil || tt.HasText(warning) {
				t.Error("successful resume did not recover key and retire availability warning")
			}
			if h.app.Store.Writes() != 0 {
				t.Error("cancel/recovery wrote settings")
			}
		})
	}
}
