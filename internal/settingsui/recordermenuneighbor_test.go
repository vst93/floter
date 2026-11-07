package settingsui

import (
	"testing"

	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func TestUpperActionMenuKeepsKeyboardAboveLowerRecorder(t *testing.T) {
	for _, draft := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			name := "upper-menu-saved-recorder"
			if draft {
				name = "upper-menu-draft-recorder"
			}
			t.Run(name+"/"+appearanceName(dark), func(t *testing.T) {
				a, entries := customStateApp(t)
				*entries = []settings.CustomShortcut{{Key: "Ctrl+Shift+A", Action: "plugin:clipboard"}, {Key: "Ctrl+Shift+B", Action: "say original"}}
				tt := customStateFrame(t, a, dark)
				if draft {
					ownershipClick(t, tt, a.copy().ShortcutsAdd)
				} else {
					ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+B"))
				}
				ownershipClick(t, tt, a.copy().ShortcutsActions[0].Label)
				tt.Key(0, ui.KeyHome)
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				tt.Frame()
				recorderDetailDump(t, tt, name, dark)
				if (*entries)[0].Action != a.copy().ShortcutsActions[1].ID || tt.HasText(a.copy().ShortcutsActions[2].Label) {
					t.Error("lower recorder stole upper menu keyboard selection")
				}
				if !a.customRecording {
					t.Error("upper menu canceled lower recording")
				}
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
				tt.Frame()
				tt.Frame()
				if draft {
					if a.customKey != "Ctrl+Shift+Z" || len(*entries) != 2 {
						t.Error("upper menu changed lower draft owner")
					}
				} else if (*entries)[1].Key != "Ctrl+Shift+Z" || (*entries)[1].Action != "say original" {
					t.Error("upper menu changed lower binding owner")
				}
			})
		}
	}
}
