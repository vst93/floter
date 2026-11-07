package settingsui

import (
	"testing"

	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func TestCustomRecorderRefreshFollowsItsBinding(t *testing.T)     { checkRecorderRefresh(t, false) }
func TestCustomRecorderRefreshRetiresMissingBinding(t *testing.T) { checkRecorderRefresh(t, true) }

func checkRecorderRefresh(t *testing.T, removed bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			// Tab to the second recorder using the actual control, not state injection.
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Key(ui.Shift, ui.KeyTab)
			tt.Key(0, ui.KeyEnter)
			if !a.customRecording || a.customRecordRow != 1 {
				t.Fatalf("second recorder not armed: %v/%d", a.customRecording, a.customRecordRow)
			}
			first, target, last := (*entries)[0], (*entries)[1], (*entries)[2]
			name := "recorder-reorder"
			*entries = []settings.CustomShortcut{target, first, last}
			if removed {
				name = "recorder-removed"
				*entries = []settings.CustomShortcut{first, last}
			}
			tt.Frame()
			feedbackDump(t, tt, name, dark)
			if removed {
				if a.customRecording || tt.HasText(a.copy().ShortcutRecording) {
					t.Error("missing binding left a recorder on its neighbor")
				}
			} else if !a.customRecording || a.customRecordRow != 0 {
				t.Errorf("reordered recorder targets row %d, want 0", a.customRecordRow)
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyR)
			want, _ := shortcuts.FromKey(ui.Cmd|ui.Shift, ui.KeyR)
			if removed {
				if (*entries)[0] != first || (*entries)[1] != last || a.customKey != "" {
					t.Errorf("removed recorder changed another binding: %+v draft=%q", *entries, a.customKey)
				}
			} else if (*entries)[0].Key != want || (*entries)[0].Action != target.Action || (*entries)[1] != first || (*entries)[2] != last {
				t.Errorf("recorded key reached wrong binding: %+v", *entries)
			}
		})
	}
}
