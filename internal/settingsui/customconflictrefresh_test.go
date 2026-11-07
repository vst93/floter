package settingsui

import (
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestCustomConflictRefreshFollowsItsBinding(t *testing.T)     { checkConflictRefresh(t, false) }
func TestCustomConflictRefreshRetiresMissingBinding(t *testing.T) { checkConflictRefresh(t, true) }

func checkConflictRefresh(t *testing.T, removed bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			for i := 0; i < 3; i++ {
				tt.Key(ui.Shift, ui.KeyTab)
			}
			tt.Key(0, ui.KeyEnter)
			if !a.customRecording || a.customRecordRow != 1 {
				t.Fatal("second recorder not armed")
			}
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyA)
			if a.customWarning(a.copy(), (*entries)[1].Key, 1) != a.copy().ShortcutsConflict {
				t.Fatal("duplicate gesture produced no warning")
			}
			first, target, last := (*entries)[0], (*entries)[1], (*entries)[2]
			*entries = []settings.CustomShortcut{target, first, last}
			name := "conflict-reorder"
			if removed {
				name = "conflict-removed"
				*entries = []settings.CustomShortcut{first, last}
			}
			tt.Frame()
			feedbackDump(t, tt, name, dark)
			if removed {
				if a.customDuplicate != "" || tt.HasText(a.copy().ShortcutsConflict) {
					t.Error("missing binding left a conflict on its neighbor")
				}
			} else {
				if a.customWarning(a.copy(), target.Key, 0) != a.copy().ShortcutsConflict || a.customWarning(a.copy(), first.Key, 1) != "" {
					t.Errorf("conflict followed row index instead of binding: row=%d", a.customDuplicateRow)
				}
			}
			if (*entries)[0].Key == "Ctrl+Shift+R" {
				t.Fatal("refresh wrote a shortcut")
			}
		})
	}
}
