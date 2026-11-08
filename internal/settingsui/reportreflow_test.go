package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func reportReflowDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KT_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kt-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkHealthReflow(t *testing.T, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			label, calls := "参数 5", 0
			a.Actions.RunWithParams = func(id string, values map[string]string) *extensions.ParamFailure {
				calls++
				if id != in.ID || values["p4"] != "12" {
					t.Error("run ownership changed")
				}
				return nil
			}
			in.Params[4].Kind = kind
			switch kind {
			case "boolean":
				in.Params[4].Default = "false"
			case "select":
				in.Params[4].Default, in.Params[4].Options, label = "北方", []string{"北方", "南方"}, "北方"
			case "cancel":
				in.Params[4].Kind, label = "text", a.copy().RunCancel
			case "submit":
				in.Params[4].Kind, label = "text", a.copy().RunLabel
			}
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { c.Resolve(a.Body.Handle).Label("feedback-viewport") })
			optionExitFocus(t, tt, label)
			for step, present := range []bool{true, false, true} {
				in.HealthReport = nil
				if present {
					in.HealthReport = &HealthReport{Probes: []HealthProbe{{ID: "connection", Stderr: strings.Repeat("连接失败，请检查服务。\n", 12)}}}
				}
				tt.Frame()
				if step == 0 {
					reportReflowDump(t, tt, "health-"+kind, dark)
				}
				checkFeedbackVisible(t, tt, label)
			}
			switch kind {
			case "boolean":
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if a.runValues[in.ID]["p4"] != "true" {
					t.Error("toggle lost answer")
				}
			case "select":
				tt.Key(0, ui.KeyRight)
				tt.Frame()
				if a.runValues[in.ID]["p4"] != "南方" {
					t.Error("select lost answer")
				}
			case "cancel":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if a.runOpen[in.ID] || a.runValues[in.ID]["p4"] != "12" {
					t.Error("cancel changed answers")
				}
			case "submit":
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if calls != 1 {
					t.Errorf("submit calls=%d", calls)
				}
			default:
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("24")
				tt.Frame()
				if a.runValues[in.ID]["p4"] != "24" {
					t.Error("editor lost answer")
				}
			}
			if kind != "submit" && calls != 0 {
				t.Error("unrelated run")
			}
			runVisibilityWheel(t, a, tt)
			manual := a.Body.Y
			tt.Frame()
			tt.Frame()
			if a.Body.Y != manual {
				t.Error("report refresh trapped manual scroll")
			}
		})
	}
}

func TestHealthReflowEditors(t *testing.T) {
	for _, kind := range []string{"text", "number", "path"} {
		t.Run(kind, func(t *testing.T) { checkHealthReflow(t, kind) })
	}
}
func TestHealthReflowChoices(t *testing.T) {
	for _, kind := range []string{"boolean", "select"} {
		t.Run(kind, func(t *testing.T) { checkHealthReflow(t, kind) })
	}
}

func checkResultReflow(t *testing.T, action string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			in.Params, a.runOpen = nil, nil
			calls := map[string]int{}
			checkID := func(id, key string) {
				if id != in.ID {
					t.Error("wrong action owner")
				}
				calls[key]++
			}
			a.Actions.RunWithParams = func(id string, values map[string]string) *extensions.ParamFailure {
				checkID(id, "run")
				if len(values) != 0 {
					t.Error("unexpected arguments")
				}
				return nil
			}
			a.Actions.DiagnoseIntegration = func(id string) { checkID(id, "check") }
			a.Actions.UninstallIntegration = func(id, name, kind string, done func(error)) { checkID(id, "remove"); done(nil) }
			a.Actions.SetIntegrationEnabled = func(id string, enabled bool, done func(error)) { checkID(id, "toggle"); done(nil) }
			label := map[string]string{"run": a.copy().RunAction, "check": a.copy().IntegrationsCheck, "remove": in.removalLabel(a.copy()), "toggle": a.copy().IntegrationsEnable}[action]
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { c.Resolve(a.Body.Handle).Label("feedback-viewport") })
			optionExitFocus(t, tt, label)
			for step, open := range []bool{true, false, true} {
				a.SetRunResult(in.ID, RunResult{Stdout: strings.Repeat("天气查询结果\n", 30), Success: !open})
				a.setRunOutputOpen(in.ID, open)
				tt.Frame()
				if step == 0 {
					reportReflowDump(t, tt, "result-"+action, dark)
				}
				checkFeedbackVisible(t, tt, label)
			}
			runVisibilityWheel(t, a, tt)
			manual := a.Body.Y
			tt.Frame()
			tt.Frame()
			if a.Body.Y != manual {
				t.Error("output update trapped manual scroll")
			}
			key := ui.KeyEnter
			if action == "toggle" {
				key = ui.KeySpace
			}
			tt.Key(0, key)
			tt.Frame()
			switch action {
			case "remove":
				if !a.removeConfirm[in.ID] || len(calls) != 0 {
					t.Fatal("removal bypassed confirmation")
				}
			case "toggle":
				if !a.disableConfirm[in.ID] || len(calls) != 0 {
					t.Fatal("disable bypassed confirmation")
				}
			default:
				if calls[action] != 1 || len(calls) != 1 {
					t.Fatal(fmt.Sprintf("action dispatch=%v", calls))
				}
			}
		})
	}
}
