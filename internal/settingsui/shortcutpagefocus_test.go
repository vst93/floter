package settingsui

import (
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestBuiltinRefusalKeepsFullPageRecorderFocus(t *testing.T) {
	checkBuiltinRefusalFocus(t, false)
}

func TestSummonRefusalKeepsFullPageRecorderFocus(t *testing.T) {
	checkBuiltinRefusalFocus(t, true)
}

func checkBuiltinRefusalFocus(t *testing.T, summon bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := New(newStore(t), Actions{SetShortcut: func(string, string) {}, ShortcutRefused: func(string) bool { return true }})
			tt := shortcutRecoveryFrame(a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsReset)
			tt.Key(0, ui.KeyTab)
			wantID, name := "", "summon-refused"
			if !summon {
				tt.Key(0, ui.KeyTab)
				wantID, name = settings.ShortcutNewCommand, "record-refused"
			}
			tt.Key(0, ui.KeyEnter)
			if !a.recording || a.ShortcutID != wantID {
				t.Fatal("wrong recorder armed")
			}
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			shortcutRecoveryDump(t, tt, name, dark)
			if !tt.Focused(a.copy().ShortcutRecord) {
				t.Error("full-page refusal lost original recorder focus")
			}
			tt.Key(0, ui.KeyEnter)
			if !a.recording || a.ShortcutID != wantID {
				t.Error("Enter did not retry original action")
			}
		})
	}
}
