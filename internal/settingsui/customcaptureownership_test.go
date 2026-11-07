package settingsui

import (
	"testing"

	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func TestCustomSuccessfulNeighborCaptureKeepsSavedRefusal(t *testing.T) {
	checkCaptureRefusal(t, false)
}
func TestCustomSuccessfulSavedCaptureKeepsDraftRefusal(t *testing.T) { checkCaptureRefusal(t, true) }

func checkCaptureRefusal(t *testing.T, draft bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			name, row, key := "capture-saved-refusal", 1, "Ctrl+Shift+B"
			if draft {
				name, row, key = "capture-draft-refusal", -1, ""
				ownershipClick(t, tt, a.copy().ShortcutsAdd)
			} else {
				ownershipClick(t, tt, shortcuts.Display(key))
			}
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyA)
			tt.Frame()
			if !tt.HasText(a.copy().ShortcutsConflict) {
				t.Fatal("no initial conflict")
			}
			ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+C"))
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyR)
			tt.Frame()
			tt.Frame()
			refusalDump(t, tt, name, dark)
			if a.customWarning(a.copy(), key, row) != a.copy().ShortcutsConflict {
				t.Error("successful neighbor capture erased unrelated refusal")
			}
			if len(*entries) != 3 || (*entries)[2].Key != "Ctrl+Shift+R" || (*entries)[2].Action != "plugin:clipboard" {
				t.Fatalf("wrong captured binding: %v", *entries)
			}
		})
	}
}
