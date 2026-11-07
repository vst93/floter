package settingsui

import (
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestBuiltinRefusalKeepsFullPageRecorderFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := New(newStore(t), Actions{SetShortcut: func(string, string) {}, ShortcutRefused: func(string) bool { return true }})
			tt := shortcutRecoveryFrame(a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsReset)
			tt.Key(0, ui.KeyTab)
			tt.Key(0, ui.KeyTab)
			tt.Key(0, ui.KeyEnter)
			if !a.recording || a.ShortcutID != settings.ShortcutNewCommand {
				t.Fatal("wrong recorder armed")
			}
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			shortcutRecoveryDump(t, tt, "record-refused", dark)
			if !tt.Focused(a.copy().ShortcutRecord) {
				t.Error("full-page refusal lost original recorder focus")
			}
			tt.Key(0, ui.KeyEnter)
			if !a.recording || a.ShortcutID != settings.ShortcutNewCommand {
				t.Error("Enter did not retry original action")
			}
		})
	}
}
