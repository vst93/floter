package shell

import (
	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestShortcutLifecycleInitialSummonFailureVisible(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			h.app.unregisterShortcut(h.app.summonKey)
			h.app.summonKey = ""
			h.blocked["Ctrl+Alt+J"] = true
			h.app.ApplySummonShortcut()
			tt := h.frame(dark)
			lifecycleDump(t, tt, "startup-summon", dark)
			if !tt.HasText(i18n.For("en").Settings.ShortcutRejected) {
				t.Error("initial unavailable summon key looks usable")
			}
		})
	}
}

func TestShortcutLifecycleDefaultResetRetriesUnavailableKey(t *testing.T) {
	h := newShortcutLifecycle(t, settings.DefaultSummonShortcut)
	tt := h.frame(false)
	lifecycleRecord(t, h, tt, settings.DefaultSummonShortcut)
	h.blocked[settings.DefaultSummonShortcut] = true
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if h.live[settings.DefaultSummonShortcut] != nil {
		t.Fatal("expected OS refusal")
	}
	h.blocked[settings.DefaultSummonShortcut] = false
	if err := tt.Click(i18n.For("en").Settings.ShortcutsReset); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
	if h.live[settings.DefaultSummonShortcut] == nil {
		t.Error("reset of unavailable default key did not retry registration")
	}
}

func TestShortcutLifecycleInitialFailureRecoversOriginalChord(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "summon", true: "custom"}[custom], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			key, mods, pressed := "Ctrl+Alt+J", ui.Ctrl|ui.Alt, ui.KeyJ
			warning := i18n.For("en").Settings.ShortcutRejected
			if custom {
				key, mods, pressed = "Ctrl+Shift+B", ui.Ctrl|ui.Shift, ui.KeyB
				warning = i18n.For("en").Settings.ShortcutsOccupied
			}
			h.blocked[key] = true
			if custom {
				h.app.ApplyCustomShortcuts()
			} else {
				h.app.unregisterShortcut(key)
				h.app.summonKey = ""
				h.app.ApplySummonShortcut()
			}
			tt := h.frame(false)
			if !tt.HasText(warning) {
				t.Fatal("initial unavailable key is not visible")
			}
			lifecycleRecord(t, h, tt, key)
			h.blocked[key] = false
			tt.Key(mods, pressed)
			tt.Frame()
			tt.Frame()
			if h.live[key] == nil || tt.HasText(warning) {
				t.Fatal("re-recording original unavailable key did not recover its availability")
			}
			if !custom && (h.app.ShortcutErr != nil || h.app.shortcutWasRefused(settings.ShortcutToggleWindow)) {
				t.Error("successful original-key recovery retained host refusal")
			}
		})
	}
}
