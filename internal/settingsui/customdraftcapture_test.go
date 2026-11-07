package settingsui

import (
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestCustomDraftCaptureKeepsSavedRefusal(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+B"))
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyA)
			tt.Frame()
			ownershipClick(t, tt, a.copy().ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			tt.Frame()
			refusalDump(t, tt, "draft-capture-saved-refusal", dark)
			if !tt.HasText(a.copy().ShortcutsConflict) || a.customWarning(a.copy(), "Ctrl+Shift+B", 1) != a.copy().ShortcutsConflict {
				t.Error("successful draft capture erased saved refusal")
			}
			if a.customKey != "Ctrl+Shift+Z" || len(*entries) != 3 {
				t.Fatal("draft capture prematurely committed")
			}
		})
	}
}

func TestCustomDraftCancelRetiresRefusal(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = nil
			accept := false
			a.Actions.SetCustomShortcuts = func(list []settings.CustomShortcut) []CustomShortcutRejection {
				if !accept {
					return []CustomShortcutRejection{{Key: "Ctrl+Shift+Z", Reason: CustomShortcutReasonOccupied}}
				}
				*entries = list
				return nil
			}
			tt := customStateFrame(t, a, dark)
			ownershipClick(t, tt, a.copy().ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			ownershipClick(t, tt, a.copy().ShortcutsBind)
			if !tt.HasText(a.copy().ShortcutsOccupied) {
				t.Fatal("no initial system refusal")
			}
			ownershipClick(t, tt, a.copy().ShortcutsRemove)
			accept = true
			ownershipClick(t, tt, a.copy().ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			refusalDump(t, tt, "draft-cancel-refusal", dark)
			if tt.HasText(a.copy().ShortcutsOccupied) {
				t.Error("new draft inherited canceled draft refusal")
			}
			if len(*entries) != 0 {
				t.Fatal("unsubmitted replacement was stored")
			}
			ownershipClick(t, tt, a.copy().ShortcutsBind)
			if len(*entries) != 1 || (*entries)[0].Key != "Ctrl+Shift+Z" {
				t.Fatal("fresh draft cannot bind")
			}
		})
	}
}
