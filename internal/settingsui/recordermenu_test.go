package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func recorderDetailDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JC_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jc/" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSavedRecorderMenuSelection(t *testing.T) { checkRecorderMenu(t, false, false) }
func TestSavedRecorderMenuEscape(t *testing.T)    { checkRecorderMenu(t, false, true) }

func checkRecorderMenu(t *testing.T, draft, cancel bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = []settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "say original"}}
			if draft {
				*entries = nil
			}
			writes := 0
			a.Actions.SetCustomShortcuts = func(list []settings.CustomShortcut) []CustomShortcutRejection { writes++; *entries = list; return nil }
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			name := "saved-menu"
			if draft {
				name = "draft-menu"
				ownershipClick(t, tt, copy.ShortcutsAdd)
			} else {
				ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+B"))
			}
			ownershipClick(t, tt, copy.ShortcutsAction)
			if !tt.HasText(copy.ShortcutsActions[1].Label) {
				t.Fatal("menu did not open")
			}
			if cancel {
				name += "-escape"
				tt.Key(0, ui.KeyEscape)
			} else {
				name += "-select"
				if draft {
					tt.Key(0, ui.KeyEnd)
				} else {
					tt.Key(0, ui.KeyHome)
					tt.Key(0, ui.KeyDown)
				}
				tt.Key(0, ui.KeyEnter)
			}
			tt.Frame()
			tt.Frame()
			recorderDetailDump(t, tt, name, dark)
			if tt.HasText(copy.ShortcutsActions[2].Label) {
				t.Error("recorder left action menu open after keyboard exit")
			}
			if !a.customRecording || !tt.Focused(copy.ShortcutRecording) {
				t.Error("menu exit canceled or lost the original recorder")
			}
			if !cancel {
				if draft {
					if a.customAction != commandAction || !tt.HasText(copy.ShortcutsCommand) {
						t.Error("draft menu could not select command mode")
					}
				} else if (*entries)[0].Action != copy.ShortcutsActions[1].ID || writes != 1 {
					t.Error("saved menu did not commit keyboard-selected action")
				}
			} else if writes != 0 {
				t.Error("menu Escape wrote a binding")
			}
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			tt.Frame()
			if draft {
				if a.customKey != "Ctrl+Shift+Z" || len(*entries) != 0 {
					t.Error("menu return did not record into the same draft")
				}
				if !cancel && a.customAction == commandAction {
					ownershipClick(t, tt, copy.ShortcutsCommand)
					tt.Type("say draft")
				}
				ownershipClick(t, tt, copy.ShortcutsBind)
			}
			if len(*entries) != 1 || (*entries)[0].Key != "Ctrl+Shift+Z" {
				t.Error("menu return cannot finish the original binding")
			}
		})
	}
}
