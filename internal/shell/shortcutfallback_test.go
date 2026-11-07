package shell

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func fallbackDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JC_DUMP"); tag != "" {
		appearance := map[bool]string{false: "light", true: "dark"}[dark]
		f, err := os.Create("/tmp/floter-P13-jc/" + name + "-" + appearance + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestShortcutFallbackRefreshFeedback(t *testing.T)   { checkShortcutFallback(t, false) }
func TestShortcutFallbackRecordingPersists(t *testing.T) { checkShortcutFallback(t, true) }

func newFallbackLifecycle(t *testing.T) *shortcutLifecycle {
	t.Helper()
	s := settings.Default()
	s.Language = "en"
	s.SetExtra("hotkey", "Ctrl+Alt+J")
	s.SetShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+J")
	s.SetCustomShortcuts([]settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "say original"}})
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(path, s); err != nil {
		t.Fatal(err)
	}
	store, err := settings.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	h := &shortcutLifecycle{live: map[string]func(){}, blocked: map[string]bool{}, claims: map[string]int{}}
	h.app = New(Options{Store: store, Paths: extensions.FromRoot(t.TempDir()),
		RegisterShortcut: func(key string, fn func()) error {
			h.claims[key]++
			if h.blocked[key] || h.live[key] != nil {
				return errors.New("occupied")
			}
			h.live[key] = fn
			return nil
		},
		UnregisterShortcut: func(key string) { delete(h.live, key) }, RunSilentCommand: func(string) error { return nil },
	})
	h.app.summonKey = ""
	h.app.ApplySummonShortcut()
	h.app.ApplyCustomShortcuts()
	h.app.Settings.Page = settingsui.PageShortcuts
	return h
}

func checkShortcutFallback(t *testing.T, record bool) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newFallbackLifecycle(t)
			tt := h.frame(dark)
			h.blocked["Ctrl+Alt+K"] = true
			if err := h.app.Store.Update(func(s *settings.Settings) {
				s.SetExtra("hotkey", "Ctrl+Alt+K")
				s.SetShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+K")
			}); err != nil {
				t.Fatal(err)
			}
			h.app.ApplySummonShortcut()
			tt.Frame()
			tt.Frame()
			copy := i18n.For("en").Settings
			if !record {
				fallbackDump(t, tt, "fallback-refused", dark)
				if !tt.HasText(copy.ShortcutRejected) {
					t.Error("refused settings refresh is invisible")
				}
				if h.live["Ctrl+Alt+J"] == nil || h.live["Ctrl+Alt+K"] != nil {
					t.Fatal("refusal lost working fallback")
				}
				claims := h.claims["Ctrl+Alt+J"]
				if err := h.app.Store.Update(func(s *settings.Settings) {
					s.SetExtra("hotkey", "Ctrl+Alt+J")
					s.SetShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+J")
				}); err != nil {
					t.Fatal(err)
				}
				h.app.ApplySummonShortcut()
				tt.Frame()
				tt.Frame()
				fallbackDump(t, tt, "fallback-reverted", dark)
				if tt.HasText(copy.ShortcutRejected) || h.app.ShortcutErr != nil || h.app.shortcutWasRefused(settings.ShortcutToggleWindow) {
					t.Error("working settings refresh retained old refusal")
				}
				if h.claims["Ctrl+Alt+J"] != claims || h.live["Ctrl+Alt+J"] == nil {
					t.Error("reverted settings reclaimed an already-held key")
				}
				return
			}
			writes := h.app.Store.Writes()
			lifecycleRecord(t, h, tt, "Ctrl+Alt+J")
			tt.Key(ui.Ctrl|ui.Alt, ui.KeyJ)
			tt.Frame()
			tt.Frame()
			fallbackDump(t, tt, "fallback-recorded", dark)
			if SummonShortcut(h.app.Store.Snapshot()) != "Ctrl+Alt+J" || settings.Shortcut(h.app.Store.Snapshot(), settings.ShortcutToggleWindow) != "Ctrl+Alt+J" || h.app.Store.Writes() != writes+1 {
				t.Errorf("recording fallback did not save the working key: summon=%q map=%q writes=%d want=%d", SummonShortcut(h.app.Store.Snapshot()), settings.Shortcut(h.app.Store.Snapshot(), settings.ShortcutToggleWindow), h.app.Store.Writes(), writes+1)
			}
			if h.live["Ctrl+Alt+J"] == nil || h.live["Ctrl+Alt+K"] != nil || tt.HasText(copy.ShortcutRejected) {
				t.Error("recorded fallback is not available without stale feedback")
			}
			if !tt.Focused(copy.ShortcutRecord) {
				t.Error("fallback acceptance lost keyboard retry")
			}
			reopened, err := settings.OpenStore(h.app.Store.Path())
			if err != nil || SummonShortcut(reopened.Snapshot()) != "Ctrl+Alt+J" {
				t.Fatal("accepted fallback was not persisted on disk")
			}
			h.app.Open(SurfaceLauncher)
			h.app.Open(SurfaceSettings)
			tt.Frame()
			tt.Frame()
			if h.app.Settings.Shortcut != "Ctrl+Alt+J" {
				t.Error("reopen did not retain accepted fallback")
			}
		})
	}
}
