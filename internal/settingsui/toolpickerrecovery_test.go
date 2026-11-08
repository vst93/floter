package settingsui

import (
	"github.com/egoist/mygo/ui"
	"os"
	"testing"
)

func checkToolPickerRecovery(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := localDraftApp()
			a.beginEdit("", callbackTool("weather"))
			var warnings []string
			a.Actions.NotifyWarning = func(s string) { warnings = append(warnings, s) }
			calls := 0
			a.Actions.ChooseProgram = func() (string, error) {
				calls++
				if scenario == "error" {
					return "", os.ErrPermission
				}
				if scenario == "picked" {
					return "/usr/bin/forecast", nil
				}
				return "", nil
			}
			tt := toolRecoveryFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().LocalToolChoose)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			tt.Frame()
			if scenario != "error" {
				toolRecoveryDump(t, tt, "picker-"+scenario, dark)
			}
			if calls != 1 {
				t.Fatalf("picker calls=%d", calls)
			}
			if scenario == "error" {
				if len(warnings) != 1 || warnings[0] != a.copy().LocalToolFailed {
					t.Errorf("program picker error warnings=%q", warnings)
				}
				if a.toolProgram != "/usr/bin/weather" {
					t.Errorf("picker error changed path=%q", a.toolProgram)
				}
				return
			}
			if len(warnings) != 0 {
				t.Errorf("ordinary picker exit warned: %q", warnings)
			}
			if !tt.Focused(a.copy().LocalToolProgram) {
				t.Errorf("%s picker did not return program focus", scenario)
			}
			want := "/usr/bin/weather"
			if scenario == "picked" {
				want = "/usr/bin/forecast"
			}
			if a.toolProgram != want {
				t.Errorf("%s picker path=%q", scenario, a.toolProgram)
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("/usr/bin/retry")
			tt.Frame()
			if a.toolProgram != "/usr/bin/retry" {
				t.Errorf("%s picker lost immediate keyboard edit: %q", scenario, a.toolProgram)
			}
		})
	}
}

func TestToolPickerRecoveryError(t *testing.T)  { checkToolPickerRecovery(t, "error") }
func TestToolPickerRecoveryCancel(t *testing.T) { checkToolPickerRecovery(t, "cancel") }

func TestToolPickerRecoveryStaleError(t *testing.T) {
	a := localDraftApp()
	a.beginEdit("", callbackTool("weather"))
	warnings := 0
	a.Actions.NotifyWarning = func(string) { warnings++ }
	a.Actions.ChooseProgram = func() (string, error) { a.beginEdit("new", callbackTool("next")); return "", os.ErrPermission }
	tt := toolRecoveryFrame(t, a, false)
	if err := tt.Click(a.copy().LocalToolChoose); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if warnings != 0 || a.toolName != "next" || a.toolProgram != "/usr/bin/next" {
		t.Errorf("stale picker touched new form: warnings=%d name=%q program=%q", warnings, a.toolName, a.toolProgram)
	}
}
