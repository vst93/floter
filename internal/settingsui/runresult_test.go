package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func resultDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IF_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-if-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkRunResult(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamSelect, Default: "old", Options: []string{"old", "next"}})
			calls := 0
			a.Actions.RunWithParams = func(_ string, sent map[string]string) *extensions.ParamFailure {
				calls++
				if sent["target"] != "old" {
					t.Errorf("submitted %q, want old", sent["target"])
				}
				if scenario == "options" {
					in.Params[0].Options[1] = "fresh"
				}
				if scenario == "value" {
					a.runValues[in.ID]["target"] = "next"
				}
				// The callback owns its argument; mutation must not rewrite the refusal snapshot.
				sent["target"] = "callback-owned"
				return &extensions.ParamFailure{Key: extensions.RunErrorParamInvalid, Param: "target"}
			}
			tt := runRecoveryFrame(t, a, in, dark)
			copy := i18n.For("en").Settings
			if err := tt.Click(copy.RunLabel); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			resultDump(t, tt, "run-"+scenario, dark)
			if tt.HasText(copy.RunParamInvalid("Target")) {
				t.Errorf("%s callback left obsolete refusal on current answer", scenario)
			}
			if calls != 1 {
				t.Errorf("refresh dispatched %d runs, want 1", calls)
			}
			if scenario == "value" && a.runValues[in.ID]["target"] != "next" {
				t.Error("callback's current answer was overwritten")
			}
		})
	}
}

func TestRunResultOptionsChangedInsideCallback(t *testing.T) { checkRunResult(t, "options") }
func TestRunResultValueChangedInsideCallback(t *testing.T)   { checkRunResult(t, "value") }
