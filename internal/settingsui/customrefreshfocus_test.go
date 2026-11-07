package settingsui

import (
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestCustomActionRefreshReturnsToPicker(t *testing.T) { checkCustomRefreshFocus(t, false) }
func TestCustomMissingRowReturnsToAdd(t *testing.T)       { checkCustomRefreshFocus(t, true) }

func checkCustomRefreshFocus(t *testing.T, removed bool) {
	for _, dark := range []bool{false, true} {
		origins := []string{"command", "elsewhere"}
		if removed {
			origins = append(origins, "recording", "picker", "remove")
		}
		for _, origin := range origins {
			t.Run(appearanceName(dark)+"/"+origin, func(t *testing.T) {
				a, entries := customStateApp(t)
				*entries = (*entries)[1:2]
				writes := 0
				a.Actions.SetCustomShortcuts = func(list []settings.CustomShortcut) []CustomShortcutRejection { writes++; *entries = list; return nil }
				tt := customStateFrame(t, a, dark)
				copy := a.copy()
				optionExitFocus(t, tt, copy.ShortcutsCommand)
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("say unfinished")
				switch origin {
				case "elsewhere":
					optionExitFocus(t, tt, copy.ShortcutsAdd)
				case "picker":
					optionExitFocus(t, tt, copy.ShortcutsAction)
				case "remove":
					optionExitFocus(t, tt, copy.ShortcutsRemove)
				case "recording":
					for i := 0; i < 3; i++ {
						tt.Key(ui.Shift, ui.KeyTab)
					}
					tt.Key(0, ui.KeyEnter)
					if !a.customRecording {
						t.Fatal("recorder not armed")
					}
				}
				name, want := "refresh-action", copy.ShortcutsAction
				if removed {
					*entries = nil
					name, want = "refresh-removed", copy.ShortcutsAdd
				} else {
					(*entries)[0].Action = "plugin:clipboard"
				}
				if origin == "elsewhere" {
					want = copy.ShortcutsAdd
				}
				tt.Frame()
				tt.Frame()
				if origin == "command" {
					recordingReturnDump(t, tt, name, dark)
				}
				if !tt.Focused(want) {
					t.Errorf("%s from %s lost its visible outlet %q", name, origin, want)
				}
				if writes != 0 {
					t.Error("refresh wrote a binding")
				}
				tt.Key(0, ui.KeyEnter)
				if want == copy.ShortcutsAdd {
					if !a.customDraftOpen || !a.customRecording {
						t.Error("refreshed outlet cannot add")
					}
				} else if !tt.HasText(copy.ShortcutsActions[1].Label) {
					t.Error("refreshed picker cannot open")
				}
			})
		}
	}
}
