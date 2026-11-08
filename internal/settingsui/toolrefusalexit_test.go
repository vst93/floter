package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func toolExitDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KA_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ka-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkToolRefusalExit(t *testing.T, field string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, editing := range []bool{false, true} {
			t.Run(appearanceName(dark)+map[bool]string{false: "/connect", true: "/edit"}[editing], func(t *testing.T) {
				a := copyLifecycleApp(t, dark)
				tool := callbackTool("weather")
				id := ""
				if editing {
					id = "local.weather"
				}
				switch field {
				case "name":
					tool.Name = " "
				case "command":
					tool.Command = "bad command"
				case "version":
					tool.Version = "version one"
				case "program":
					tool.Program = ""
				case "script":
					tool.Mode, tool.ScriptLanguage, tool.ScriptContent = routeScript, "python", "  "
				case "platform":
					tool.Platforms = nil
				}
				a.beginEdit(id, tool)
				var submitted []CustomTool
				a.Actions.ConnectLocalTool = func(v CustomTool) { submitted = append(submitted, v) }
				a.Actions.EditCustomTool = func(got string, v CustomTool) {
					if got != id {
						t.Errorf("edit target=%q", got)
					}
					submitted = append(submitted, v)
				}
				tt := toolRecoveryFrame(t, a, dark)
				copy := a.copy()
				label := copy.LocalToolConnect
				if editing {
					label = copy.LocalToolSave
				}
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				toolExitDump(t, tt, "refusal-"+field+map[bool]string{false: "-connect", true: "-edit"}[editing], dark)
				if len(submitted) != 0 || !a.toolOpen {
					t.Fatal("invalid form was submitted")
				}
				target := map[string]string{"name": copy.LocalToolName, "command": copy.LocalToolCommand, "version": copy.LocalToolVersion, "program": copy.LocalToolProgram, "script": copy.LocalToolScript, "platform": customPlatforms[0].Label}[field]
				if !tt.Focused(target) {
					t.Errorf("%s refusal lost editable keyboard exit", field)
				}
				if field == "platform" {
					tt.Key(0, ui.KeySpace)
				} else {
					tt.Key(ui.Cmd, ui.KeyA)
					value := map[string]string{"name": "天气工具", "command": "forecast", "version": "2.3.4", "program": "/usr/bin/weather", "script": "print('sunny')"}[field]
					tt.Type(value)
				}
				tt.Frame()
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if len(submitted) != 1 {
					t.Errorf("%s refused direct keyboard correction: submissions=%d", field, len(submitted))
				}
			})
		}
	}
}

func TestToolRefusalExitName(t *testing.T)     { checkToolRefusalExit(t, "name") }
func TestToolRefusalExitCommand(t *testing.T)  { checkToolRefusalExit(t, "command") }
func TestToolRefusalExitVersion(t *testing.T)  { checkToolRefusalExit(t, "version") }
func TestToolRefusalExitProgram(t *testing.T)  { checkToolRefusalExit(t, "program") }
func TestToolRefusalExitScript(t *testing.T)   { checkToolRefusalExit(t, "script") }
func TestToolRefusalExitPlatform(t *testing.T) { checkToolRefusalExit(t, "platform") }
