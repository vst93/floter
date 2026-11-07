package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func refusalDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IZ_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-iz/" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCustomAddKeepsSavedRefusal(t *testing.T)              { checkRefusalOwner(t, "add") }
func TestCustomNeighborRecorderKeepsSavedRefusal(t *testing.T) { checkRefusalOwner(t, "record") }

func checkRefusalOwner(t *testing.T, scenario string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+B"))
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyA)
			tt.Frame()
			if !tt.HasText(a.copy().ShortcutsConflict) {
				t.Fatal("no refusal from duplicate capture")
			}
			if scenario == "add" {
				ownershipClick(t, tt, a.copy().ShortcutsAdd)
			} else {
				ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+C"))
			}
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			refusalDump(t, tt, "refusal-"+scenario, dark)
			if !tt.HasText(a.copy().ShortcutsConflict) || a.customWarning(a.copy(), "Ctrl+Shift+B", 1) != a.copy().ShortcutsConflict {
				t.Error("unrelated recorder erased saved refusal")
			}
			if len(*entries) != 3 || (*entries)[1].Key != "Ctrl+Shift+B" {
				t.Fatal("cancel changed stored bindings")
			}
		})
	}
}
