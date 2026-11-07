package settingsui

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func checkToolModeExit(t *testing.T, script bool, field string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tool := callbackTool("weather")
			tool.VersionArgs, tool.Description = "--version", "Weather provider"
			tool.ScriptLanguage, tool.ScriptContent = "javascript", "console.log('weather');"
			if script {
				tool.Mode = routeScript
			}
			a.beginEdit("local.weather", tool)
			a.Actions.ChooseProgram = func() (string, error) { return "", nil }
			a.Actions.CopyText = func(string) {}
			a.Actions.ExportScript = func(string, string) {}
			tt := toolCallbackFrame(t, a, dark)
			copy := a.copy()
			name := "program-exit-" + field
			if script {
				name = "script-exit-" + field
				if field == "editor" {
					hint := mustFind(t, tt, copy.LocalToolScriptHint)
					tt.ClickAt(hint.X+100, hint.Y+hint.H+15)
					tt.Key(0, ui.KeyLeft)
				} else if field == "language" {
					optionExitFocus(t, tt, copy.LocalToolLanguage)
				} else {
					labels := map[string]string{"language": copy.LocalToolLanguage, "copy": copy.LocalToolCopyScript, "export": copy.LocalToolExportScript}
					// Click the action without inserting a Tab into the code editor.
					if err := tt.Click(labels[field]); err != nil {
						t.Fatal(err)
					}
				}
				a.toolMode = ""
			} else {
				labels := map[string]string{"input": copy.LocalToolProgram, "args": copy.LocalToolArgs, "version": copy.LocalToolVersionArgs, "description": copy.LocalToolDescription, "choose": copy.LocalToolChoose, "copy": copy.LocalToolCopyCommand, "plan": copy.LocalToolCopyPlan}
				if field == "input" {
					// In Chinese the Program segment and field share a label.
					// The Choose button's preceding Tab stop is the actual input.
					optionExitFocus(t, tt, copy.LocalToolChoose)
					tt.Key(ui.Shift, ui.KeyTab)
					tt.Frame()
				} else {
					optionExitFocus(t, tt, labels[field])
				}
				a.toolMode = routeScript
			}
			tt.Frame()
			tt.Frame()
			runExitDump(t, tt, name, dark)
			if !tt.Focused(copy.LocalToolMode) {
				t.Error("retired mode content lost mode keyboard exit")
			}
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			// The existing neutral group origin enters its first segment on
			// Tab; only Space or an arrow chooses, just like withdrawn options.
			if !tt.Focused(copy.LocalToolModeProgram) {
				t.Error("mode exit cannot Tab into mode group")
			}
			if script {
				tt.Key(0, ui.KeyRight)
			} else {
				tt.Key(0, ui.KeySpace)
			}
			tt.Frame()
			if script && a.toolMode != routeScript || !script && a.toolMode == routeScript {
				t.Error("mode exit cannot switch back with keyboard")
			}
			if a.toolProgram != tool.Program || a.toolScript != tool.ScriptContent {
				t.Errorf("mode exit lost drafts: program=%q script=%q", a.toolProgram, a.toolScript)
			}
		})
	}
}

func TestToolModeExitProgram(t *testing.T) {
	for _, field := range []string{"input", "args", "version", "description", "choose", "copy", "plan"} {
		t.Run(field, func(t *testing.T) { checkToolModeExit(t, false, field) })
	}
}
func TestToolModeExitScript(t *testing.T) {
	for _, field := range []string{"editor", "language", "copy", "export"} {
		t.Run(field, func(t *testing.T) { checkToolModeExit(t, true, field) })
	}
}

func TestToolModeExitDoesNotStealFromSurvivingInput(t *testing.T) {
	for _, script := range []bool{false, true} {
		t.Run(map[bool]string{false: "program", true: "script"}[script], func(t *testing.T) {
			a := copyLifecycleApp(t, true)
			tool := callbackTool("weather")
			if script {
				tool.Mode, tool.ScriptLanguage, tool.ScriptContent = routeScript, "javascript", "console.log('weather');"
			}
			a.beginEdit("local.weather", tool)
			tt := toolCallbackFrame(t, a, true)
			optionExitFocus(t, tt, a.copy().LocalToolName)
			if script {
				a.toolMode = ""
			} else {
				a.toolMode = routeScript
			}
			tt.Frame()
			tt.Frame()
			if !tt.Focused(a.copy().LocalToolName) {
				t.Error("mode refresh stole surviving name focus")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("Still editing")
			tt.Frame()
			if a.toolName != "Still editing" {
				t.Error("surviving name lost keyboard input")
			}
		})
	}
}
