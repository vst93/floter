package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func runExitDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IM_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-im-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkRunChoiceRefresh(t *testing.T, removed bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamSelect, Default: "one", Options: []string{"one", "two", "three"}})
			tt := runRecoveryFrame(t, a, in, dark)
			optionExitFocus(t, tt, "one")
			tt.Key(0, ui.KeyRight)
			if !tt.Focused("two") || a.runValues[in.ID]["target"] != "two" {
				t.Fatal("choice did not start at two")
			}
			name := "choice-reorder"
			in.Params[0].Options = []string{"two", "three", "one"}
			if removed {
				name = "choice-removed"
				in.Params[0].Options = []string{"one", "three"}
			}
			tt.Frame()
			tt.Frame()
			runExitDump(t, tt, name, dark)
			if removed {
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if a.runValues[in.ID]["target"] != "two" {
					t.Error("withdrawn choice activated its replacement")
				}
				tt.Key(0, ui.KeyTab)
				tt.Frame()
				if !tt.Focused("one") {
					t.Error("withdrawn choice lost group Tab origin")
				}
			} else if !tt.Focused("two") {
				t.Error("reordered choice focus moved to another value")
			}
			tt.Key(0, ui.KeyRight)
			tt.Frame()
			if a.runValues[in.ID]["target"] != "three" || !tt.Focused("three") {
				t.Errorf("choice keyboard recovery failed: %v", a.runValues[in.ID])
			}
		})
	}
}

func TestRunExitChoiceReordered(t *testing.T) { checkRunChoiceRefresh(t, false) }
func TestRunExitChoiceWithdrawn(t *testing.T) { checkRunChoiceRefresh(t, true) }
