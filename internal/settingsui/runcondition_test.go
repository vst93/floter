package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func runConditionDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IE_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ie-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkRunCondition(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "old"}, Param{ID: "note", Label: "Note", Kind: extensions.ParamText, Default: "note"})
			calls := 0
			a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure {
				calls++
				return &extensions.ParamFailure{Key: extensions.RunErrorParamInvalid, Param: "target"}
			}
			tt := runRecoveryFrame(t, a, in, dark)
			copy := i18n.For("en").Settings
			if err := tt.Click(copy.RunLabel); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			message := copy.RunParamInvalid("Target")
			if !tt.HasText(message) {
				t.Fatal("submission did not show refusal")
			}
			// Editing an unrelated input must not dismiss the actual refusal.
			r := mustFind(t, tt, "Note")
			tt.ClickAt(568, r.Y+r.H/2)
			tt.Type(" extra")
			tt.Frame()
			if !tt.HasText(message) {
				t.Fatal("unrelated edit hid refusal")
			}
			switch scenario {
			case "edited":
				r := mustFind(t, tt, "Target")
				tt.ClickAt(568, r.Y+r.H/2)
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("fixed")
			case "removed":
				in.Params = in.Params[1:]
			}
			tt.Frame()
			tt.Frame()
			runConditionDump(t, tt, "run-"+scenario, dark)
			if tt.HasText(message) {
				t.Errorf("%s parameter left obsolete refusal visible", scenario)
			}
			if calls != 1 {
				t.Errorf("refresh submitted %d runs, want 1", calls)
			}
			if scenario == "removed" {
				in.Params = append(in.Params, Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "fresh"})
				tt.Frame()
				if tt.HasText(message) {
					t.Error("re-added parameter resurrected refusal")
				}
			}
		})
	}
}

func TestRunConditionEditedValue(t *testing.T)  { checkRunCondition(t, "edited") }
func TestRunConditionRemovedField(t *testing.T) { checkRunCondition(t, "removed") }
