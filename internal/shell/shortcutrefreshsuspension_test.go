package shell

import (
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestShortcutLifecycleSummonRefreshStaysSuspended(t *testing.T) { checkLifecycleRefresh(t, false) }
func TestShortcutLifecycleCustomRefreshStaysSuspended(t *testing.T) { checkLifecycleRefresh(t, true) }

func checkLifecycleRefresh(t *testing.T, custom bool) {
	h := newShortcutLifecycle(t, "Ctrl+Alt+J")
	tt := h.frame(false)
	lifecycleRecord(t, h, tt, "Ctrl+Alt+J")
	if err := h.app.Store.Update(func(s *settings.Settings) {
		if custom {
			s.SetCustomShortcuts([]settings.CustomShortcut{{Key: "Ctrl+Shift+Z", Action: "say refreshed"}})
		} else {
			s.SetExtra("hotkey", "Ctrl+Shift+Z")
			s.SetShortcut(settings.ShortcutToggleWindow, "Ctrl+Shift+Z")
		}
	}); err != nil {
		t.Fatal(err)
	}
	if custom {
		h.app.ApplyCustomShortcuts()
	} else {
		h.app.ApplySummonShortcut()
	}
	tt.Frame()
	if len(h.live) != 0 {
		t.Errorf("settings refresh re-enabled global keys while recording: %v", keysOf(h.live))
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if h.live["Ctrl+Shift+Z"] == nil || h.app.shortcutsSuspended {
		t.Error("cancel did not restore latest configured key")
	}
}
