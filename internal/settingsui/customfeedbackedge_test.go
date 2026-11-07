package settingsui

import (
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestCustomUnknownActionDoesNotPretendToBeCommand(t *testing.T) {
	for _, action := range []string{"plugin:removed-tool", "action:removed-action"} {
		for _, dark := range []bool{false, true} {
			t.Run(action+"/"+appearanceName(dark), func(t *testing.T) {
				a, entries := customStateApp(t)
				*entries = []settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: action}}
				tt := customStateFrame(t, a, dark)
				ownershipDump(t, tt, "unknown-"+action[:6], dark)
				if !tt.HasText(action) {
					t.Errorf("unknown action %q disguised as command mode", action)
				}
				ownershipChoose(t, tt, a.copy().ShortcutsAction, true)
				if !a.customDraftOpen || len(*entries) != 0 {
					t.Error("unknown action cannot switch to real command mode")
				}
			})
		}
	}
}

func TestCustomDraftBindKeepsSavedConflict(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			ownershipClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+B"))
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyA)
			if !tt.HasText(copy.ShortcutsConflict) {
				t.Fatal("no conflict")
			}
			ownershipClick(t, tt, copy.ShortcutsBind)
			ownershipDump(t, tt, "bind-neighbor-conflict", dark)
			if len(*entries) != 4 {
				t.Fatal("draft not bound")
			}
			if !tt.HasText(copy.ShortcutsConflict) {
				t.Error("binding unrelated draft erased saved refusal")
			}
		})
	}
}
