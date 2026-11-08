package settingsui

import (
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func toolVisibilityDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KB_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kb-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func toolVisibilityFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	a.Page = PageIntegrations
	tt, _ := renderIn(t, a, 720, 520, dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}

func checkToolRefusalVisibility(t *testing.T, field string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, editing := range []bool{false, true} {
			t.Run(appearanceName(dark)+map[bool]string{false: "/connect", true: "/edit"}[editing], func(t *testing.T) {
				a := copyLifecycleApp(t, dark)
				tool := callbackTool("weather")
				copy := a.copy()
				target, fixed := "", ""
				switch field {
				case "name":
					tool.Name, target, fixed = " ", copy.LocalToolName, "天气工具"
				case "command":
					tool.Command, target, fixed = "bad command", copy.LocalToolCommand, "forecast"
				case "version":
					tool.Version, target, fixed = "version one", copy.LocalToolVersion, "2.3.4"
				case "program":
					tool.Program, target, fixed = "", copy.LocalToolProgram, "/usr/bin/weather"
				case "script":
					tool.Mode, tool.ScriptLanguage, tool.ScriptContent = routeScript, "python", "  "
					target, fixed = copy.LocalToolScript, "print('sunny')"
				case "platform":
					tool.Platforms, target = nil, customPlatforms[0].Label
				}
				id, submit := "", copy.LocalToolConnect
				if editing {
					id, submit = "local.weather", copy.LocalToolSave
				}
				a.beginEdit(id, tool)
				calls := 0
				a.Actions.ConnectLocalTool = func(CustomTool) { calls++ }
				a.Actions.EditCustomTool = func(string, CustomTool) { calls++ }
				tt := toolVisibilityFrame(t, a, dark)
				a.Body.Y = a.Body.MaxY
				tt.Frame()
				if err := tt.Click(submit); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				toolVisibilityDump(t, tt, field+map[bool]string{false: "-connect", true: "-edit"}[editing], dark)
				if calls != 0 || !a.toolOpen || !tt.Focused(target) {
					t.Fatalf("%s refusal lost its editing target", field)
				}
				r, ok := tt.Find(target)
				if !ok || r.Y < 51 || r.Y+r.H > 520 || r.H < 14 {
					t.Fatalf("%s refusal remains outside viewport: rect=%+v scroll=%v", field, r, a.Body.Y)
				}
				// A one-shot reveal must not pin the page to its focused field.
				before := a.Body.Y
				tt.Wheel(600, 280, 0, 140)
				tt.Advance(250 * time.Millisecond)
				tt.Frame()
				if before < a.Body.MaxY && a.Body.Y <= before {
					t.Fatal("refusal reveal trapped subsequent wheel scrolling")
				}
				if field == "platform" {
					tt.Key(0, ui.KeySpace)
				} else {
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Type(fixed)
				}
				tt.Frame()
				a.Body.Y = a.Body.MaxY
				tt.Frame()
				if err := tt.Click(submit); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if calls != 1 {
					t.Fatalf("%s correction cannot resubmit: calls=%d", field, calls)
				}
			})
		}
	}
}

func TestToolRefusalVisibilityName(t *testing.T)    { checkToolRefusalVisibility(t, "name") }
func TestToolRefusalVisibilityCommand(t *testing.T) { checkToolRefusalVisibility(t, "command") }
