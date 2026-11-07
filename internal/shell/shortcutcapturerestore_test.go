package shell

import (
	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestShortcutLifecycleCapturedSummonStaysRegistered(t *testing.T) {
	checkLifecycleCapture(t, false)
}
func TestShortcutLifecycleCapturedCustomStaysRegistered(t *testing.T) { checkLifecycleCapture(t, true) }

func checkLifecycleCapture(t *testing.T, custom bool) {
	h := newShortcutLifecycle(t, "Ctrl+Alt+J")
	tt := h.frame(false)
	old, next := "Ctrl+Alt+J", "Ctrl+Shift+Z"
	if custom {
		old = "Ctrl+Shift+B"
	}
	lifecycleRecord(t, h, tt, old)
	// The first acquisition succeeds. Releasing it creates a chance for a
	// competing app to acquire it before a redundant second registration.
	h.limit[next] = 1
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
	tt.Frame()
	tt.Frame()
	if h.claims[next] != 1 || h.live[next] == nil {
		t.Errorf("accepted chord was claimed again or lost on resume: claims=%d live=%v", h.claims[next], h.live[next] != nil)
	}
	if h.live[old] != nil {
		t.Error("old binding still active")
	}
	if custom {
		entries := settings.CustomShortcutsOf(h.app.Store.Snapshot())
		if len(entries) != 1 || entries[0].Key != next || entries[0].Action != "say original" {
			t.Fatalf("stored custom action changed: %v", entries)
		}
	} else if SummonShortcut(h.app.Store.Snapshot()) != next {
		t.Error("captured summon key not saved")
	}
}

func TestShortcutLifecycleResetKeepsRecorderSuspended(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "summon", true: "custom"}[custom], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			tt := h.frame(false)
			key := "Ctrl+Alt+J"
			if custom {
				key = "Ctrl+Shift+B"
			}
			lifecycleRecord(t, h, tt, key)
			if err := tt.Click(i18n.For("en").Settings.ShortcutsReset); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !h.app.shortcutsSuspended || !tt.HasText(i18n.For("en").Settings.ShortcutRecording) {
				t.Fatal("reset changed recording keyboard ownership")
			}
			if len(h.live) != 0 {
				t.Errorf("reset activated global shortcuts under active recorder: %v", keysOf(h.live))
			}
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			tt.Frame()
			if h.live[settings.DefaultSummonShortcut] == nil || h.live["Ctrl+Shift+B"] == nil || h.live["Ctrl+Alt+J"] != nil {
				t.Errorf("cancel after reset restored wrong keys: %v", keysOf(h.live))
			}
		})
	}
}
