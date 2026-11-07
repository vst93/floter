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

func TestRunExitParameterWithdrawn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "mine"}, Param{ID: "other", Label: "Other", Kind: extensions.ParamText, Default: "kept"})
			tt := runRecoveryFrame(t, a, in, dark)
			optionExitFocus(t, tt, "Target")
			in.Params = in.Params[1:]
			tt.Frame()
			tt.Frame()
			runExitDump(t, tt, "parameter-withdrawn", dark)
			if !tt.Focused(a.copy().RunCancel) {
				t.Error("withdrawn parameter lost Cancel keyboard exit")
			}
			if a.runValues[in.ID]["other"] != "kept" {
				t.Error("surviving answer changed")
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if a.runOpen[in.ID] {
				t.Error("withdrawn parameter cannot cancel with keyboard")
			}
		})
	}
}

func checkRunFormExit(t *testing.T, change string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			a.Actions.Close = func() {}
			a.Actions.Close = func() {}
			in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local", Params: []Param{{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "mine"}}}
			a.Integrations = func() []Integration { return []Integration{in} }
			calls := 0
			a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure { calls++; return nil }
			a.runOpen = map[string]bool{in.ID: true}
			tt := noticeExitPage(t, a, dark)
			optionExitFocus(t, tt, "Target")
			switch change {
			case "empty":
				in.Params = nil
			case "closed":
				a.runOpen[in.ID] = false
			case "unavailable":
				a.Actions.RunWithParams = nil
			}
			tt.Frame()
			tt.Frame()
			runExitDump(t, tt, "form-"+change, dark)
			if change != "unavailable" && !tt.Focused(a.copy().RunAction) {
				t.Errorf("%s form lost Run keyboard exit", change)
			}
			if change == "unavailable" && !tt.Focused(a.copy().Close) {
				t.Error("withdrawn Run action lost page keyboard exit")
			}
			if calls != 0 || a.runValues[in.ID]["target"] != "mine" {
				t.Error("exit changed answers or ran a command")
			}
			if change != "unavailable" {
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if change == "empty" && calls != 1 {
					t.Error("parameterless Run cannot activate")
				}
				if change == "closed" && !a.runOpen[in.ID] {
					t.Error("closed form cannot reopen")
				}
			}
		})
	}
}

func TestRunExitAllParametersWithdrawn(t *testing.T) { checkRunFormExit(t, "empty") }
func TestRunExitFormClosed(t *testing.T)             { checkRunFormExit(t, "closed") }
func TestRunExitActionWithdrawn(t *testing.T)        { checkRunFormExit(t, "unavailable") }
