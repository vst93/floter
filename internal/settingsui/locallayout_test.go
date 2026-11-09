package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestLocalLayoutOpen(t *testing.T)     { checkLocalLayout(t, "open") }
func TestLocalLayoutMode(t *testing.T)     { checkLocalLayout(t, "mode") }
func TestLocalLayoutAdvanced(t *testing.T) { checkLocalLayout(t, "advanced") }
func TestLocalLayoutEditing(t *testing.T)  { checkLocalLayout(t, "editing") }

// The host can restore a failed local connection while the user has already
// continued into another tool. Layout feedback must follow the drawn content,
// without consuming that tool's focus, draft, or later manual scrolling.
func checkLocalLayout(t *testing.T, change string) {
	t.Helper()
	for _, config := range []bool{false, true} {
		name := "run"
		if config {
			name = "config"
		}
		t.Run(name, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				t.Run(appearanceName(dark), func(t *testing.T) {
					a, in := runVisibilityApp(t, dark)
					a.Actions.ConnectLocalTool = func(CustomTool) { t.Error("unexpected connect") }
					a.toolOpen = change != "open"
					a.toolName, a.toolCommand, a.toolProgram, a.toolVersion = "Weather", "weather", "/usr/bin/printf", "1.0.0"
					a.toolPlatforms = map[string]bool{currentPlatform(): true}
					if change == "kind" {
						a.toolParams = []toolParam{{ID: "target", Label: "城市", Kind: "text"}}
					}
					if change == "issues" {
						a.toolName, a.toolCommand, a.toolVersion = "", "bad command", "bad version"
					}
					label := "参数 5"
					if config {
						in.Params, a.Actions.RunWithParams = nil, nil
						in.Config = []ConfigField{{Key: "city", Label: "查询城市", Type: "text", Default: "stored"}}
						a.configOpen = map[string]bool{in.ID: true}
						label = "查询城市"
					}
					var control ui.Rect
					tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) {
						c.Resolve(a.Body.Handle).Label("feedback-viewport")
						form := c.Resolve(a.noticeFocus(c, in.ID, "run").panel)
						if form.Valid() {
							focus := ui.Local(form, "focused-parameter", func() ui.Handle { return ui.Handle{} })
							control = c.Resolve(*focus).Bounds()
						}
					})
					optionExitFocus(t, tt, label)
					for step, on := range []bool{true, false, true} {
						switch change {
						case "open":
							if on {
								recover, retired := a.localToolRecovery()
								a.toolFormKey = retired
								recover()
							} else {
								a.toolOpen = false
							}
						case "mode":
							a.toolMode = ""
							if on {
								a.toolMode = routeScript
							}
						case "advanced":
							a.toolAdvanced = on
						case "editing":
							a.toolEditing = ""
							if on {
								a.toolEditing = "weather"
							}
						case "params":
							a.toolParams = nil
							if on {
								a.toolParams = []toolParam{{ID: "city", Label: "城市", Kind: "text"}, {ID: "days", Label: "天数", Kind: "number"}}
							}
						case "kind":
							a.toolParams[0].Kind = "text"
							if on {
								a.toolParams[0].Kind = "select"
							}
						case "issues":
							a.toolIssue = on
						case "platforms":
							a.toolPlatforms = map[string]bool{currentPlatform(): true}
							if on {
								a.toolPlatforms = map[string]bool{}
							}
						}
						tt.Frame()
						if step == 0 && change == "open" && !config {
							if tag := os.Getenv("FLOTER_P13LD_DUMP"); tag != "" {
								f, err := os.Create("/tmp/floter-P13-ld-local-" + appearanceName(dark) + "-" + tag + ".png")
								if err != nil {
									t.Fatal(err)
								}
								err = png.Encode(f, tt.Image())
								f.Close()
								if err != nil {
									t.Fatal(err)
								}
							}
						}
						if config {
							checkFeedbackVisible(t, tt, label)
						} else {
							viewport := mustFind(t, tt, "feedback-viewport")
							if !tt.Focused(label) || control.W <= 0 || control.H <= 0 || control.Y-3 < viewport.Y+16 || control.Y+control.H+3 > viewport.Y+viewport.H-16 {
								t.Errorf("local layout hides active editor: target=%+v viewport=%+v focused=%v", control, viewport, tt.Focused(label))
							}
						}
					}
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Type("continued")
					tt.Frame()
					if config {
						if a.draft(*in).values["city"] != "continued" {
							t.Error("wrong configuration owner")
						}
					} else if a.runValues[in.ID]["p4"] != "continued" {
						t.Error("wrong run owner")
					}
					runVisibilityWheel(t, a, tt)
					manual := a.Body.Y
					a.toolDescription = "same fixed-height input, different draft"
					tt.Frame()
					tt.Frame()
					if a.Body.Y != manual {
						t.Error("unchanged local layout reclaimed manual scroll")
					}
				})
			}
		})
	}
}
