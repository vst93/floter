package settingsui

import (
	"image"
	"image/png"
	"math"
	"os"
	"testing"
	"time"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

// Compare occupied raster cells, not two independently accumulated float32
// origins. No margin is allowed: losing one painted pixel fails the pin.
func toolVisiblePixels(r ui.Rect) image.Rectangle {
	return image.Rect(int(math.Floor(float64(r.X))), int(math.Floor(float64(r.Y))),
		int(math.Ceil(float64(r.X+r.W))), int(math.Ceil(float64(r.Y+r.H))))
}

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

func toolVisibilityFrame(t *testing.T, a *App, dark bool, probes ...func(*ui.Context)) *ui.Tester {
	t.Helper()
	a.Page = PageIntegrations
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
		for _, probe := range probes {
			probe(c)
		}
	}, 720, 520)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
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
				if field != "name" && field != "command" {
					// Real declarations lengthen the form enough that even the
					// platform row is above the viewport at submission time.
					a.toolParams = []toolParam{
						{ID: "city", Label: "城市", Kind: extensions.ParamText},
						{ID: "day", Label: "日期", Kind: extensions.ParamText},
						{ID: "unit", Label: "单位", Kind: extensions.ParamText},
					}
				}
				calls := 0
				a.Actions.ConnectLocalTool = func(CustomTool) { calls++ }
				a.Actions.EditCustomTool = func(string, CustomTool) { calls++ }
				var targetBox, viewport ui.Rect
				key := map[string]string{"name": "name", "command": "command", "version": "version", "program": "plan", "script": "script", "platform": "platforms"}[field]
				tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
					targetBox = c.Resolve(a.contentFocus(c, key).source).Bounds()
					viewport = c.Resolve(a.Body.Handle).Bounds()
				})
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
				r := targetBox
				if !toolVisiblePixels(r).In(toolVisiblePixels(viewport)) || r.H <= 0 || r.W <= 0 {
					t.Fatalf("%s refusal remains outside viewport: rect=%+v viewport=%+v scroll=%v", field, r, viewport, a.Body.Y)
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

func TestToolRefusalVisibilityName(t *testing.T)     { checkToolRefusalVisibility(t, "name") }
func TestToolRefusalVisibilityCommand(t *testing.T)  { checkToolRefusalVisibility(t, "command") }
func TestToolRefusalVisibilityVersion(t *testing.T)  { checkToolRefusalVisibility(t, "version") }
func TestToolRefusalVisibilityProgram(t *testing.T)  { checkToolRefusalVisibility(t, "program") }
func TestToolRefusalVisibilityScript(t *testing.T)   { checkToolRefusalVisibility(t, "script") }
func TestToolRefusalVisibilityPlatform(t *testing.T) { checkToolRefusalVisibility(t, "platform") }
