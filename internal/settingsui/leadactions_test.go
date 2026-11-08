package settingsui

import (
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func TestLeadActionsAlias(t *testing.T)    { checkLeadAction(t, "alias") }
func TestLeadActionsCommand(t *testing.T)  { checkLeadAction(t, "command") }
func TestLeadActionsRun(t *testing.T)      { checkLeadAction(t, "run") }
func TestLeadActionsDiagnose(t *testing.T) { checkLeadAction(t, "diagnose") }
func TestLeadActionsRemove(t *testing.T)   { checkLeadAction(t, "remove") }
func TestLeadActionsToggle(t *testing.T)   { checkLeadAction(t, "toggle") }
func TestLeadActionsReprobe(t *testing.T)  { checkLeadAction(t, "reprobe") }
func TestLeadActionsEdit(t *testing.T)     { checkLeadAction(t, "edit") }

func checkLeadAction(t *testing.T, action string) {
	t.Helper()
	for _, source := range []string{"local", "recommended", "detected"} {
		t.Run(source, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				t.Run(appearanceName(dark), func(t *testing.T) {
					a, in := runVisibilityApp(t, dark)
					in.Params, a.Actions.RunWithParams = nil, nil
					in.Description = strings.Repeat("天气查询工具的运行方式与使用说明。\n", 8)
					var recommended []RecommendedTool
					var detected []DetectedTool
					if source == "recommended" {
						a.Recommended = func() []RecommendedTool { return recommended }
						a.Actions.ConnectRecommended = func(string) { t.Error("unexpected install") }
					}
					if source == "detected" {
						a.Actions.Detected = func() []DetectedTool { return detected }
						a.Actions.ConnectDetected = func(DetectedTool) { t.Error("unexpected connect") }
					}
					calls := 0
					called := func(id string) {
						calls++
						if id != in.ID {
							t.Errorf("wrong owner: %s", id)
						}
					}
					copy := a.copy()
					label := ""
					switch action {
					case "alias", "command":
						in.Commands = []Command{{ID: "forecast", Name: "天气预报", Declared: true, Enabled: true, Available: true, Alias: "stored"}}
						label = copy.IntegrationsCommandAliasFor("天气预报")
						a.Actions.SetCommandAlias = func(id, command, value string) {
							called(id)
							if command != "forecast" || value != "stored-next" {
								t.Errorf("alias payload: %s %s", command, value)
							}
						}
						a.Actions.SetCommandEnabled = func(id, command string, enabled bool) {
							called(id)
							if command != "forecast" || enabled {
								t.Error("wrong command toggle")
							}
						}
						if action == "command" {
							label = copy.IntegrationsCommandEnable("天气预报")
						}
					case "run":
						label = copy.RunAction
						a.Actions.RunWithParams = func(id string, values map[string]string) *extensions.ParamFailure {
							called(id)
							if len(values) != 0 {
								t.Error("unexpected run values")
							}
							return nil
						}
					case "diagnose":
						label, a.Actions.DiagnoseIntegration = copy.IntegrationsCheck, called
					case "remove":
						label = in.removalLabel(copy)
						a.Actions.UninstallIntegration = func(string, string, string, func(error)) { t.Error("removed before confirmation") }
					case "toggle":
						label, in.Enabled = copy.IntegrationsEnable, false
						a.Actions.SetIntegrationEnabled = func(id string, enabled bool, done func(error)) {
							called(id)
							if !enabled {
								t.Error("wrong enable value")
							}
							done(nil)
						}
					case "reprobe":
						in.Generated = true
						label, a.Actions.ReprobeCommands = copy.IntegrationsReprobe, called
					case "edit":
						in.Generated = true
						label = copy.EditLabel
						a.Actions.EditCustomTool = func(string, CustomTool) { t.Error("saved before editing") }
						a.Actions.ToolForEdit = func(id string) (CustomTool, bool) { called(id); return callbackTool("weather"), true }
					}
					tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { c.Resolve(a.Body.Handle).Label("feedback-viewport") })
					a.Now = tt.Now
					optionExitFocus(t, tt, label)
					if action == "alias" {
						tt.Key(0, ui.KeyEnd)
						tt.Type("-next")
						tt.Frame()
					}
					for step := 0; step < 4; step++ {
						description := "查询附近城市天气"
						if step == 1 {
							description = strings.Repeat("未来一周的温度、降水与出行提示。\n", 12)
						}
						recommended = []RecommendedTool{{ID: "nearby", Name: "周边天气", Description: description}}
						detected = []DetectedTool{{Name: "周边天气", Path: "/usr/bin/nearby", Description: description, RuntimeAvailable: true}}
						if source == "local" {
							a.Actions.ConnectLocalTool = func(CustomTool) { t.Error("unexpected local connect") }
						}
						if step == 2 {
							recommended, detected = nil, nil
							a.Actions.ConnectLocalTool = nil
						}
						tt.Frame()
						if step == 0 && source == "recommended" && action == "alias" {
							if tag := os.Getenv("FLOTER_P13LB_DUMP"); tag != "" {
								f, err := os.Create("/tmp/floter-P13-lb-command-" + appearanceName(dark) + "-" + tag + ".png")
								if err != nil {
									t.Fatal(err)
								}
								if err = png.Encode(f, tt.Image()); err != nil {
									t.Fatal(err)
								}
								if err = f.Close(); err != nil {
									t.Fatal(err)
								}
							}
						}
						checkFeedbackVisible(t, tt, label)
					}
					if action == "alias" {
						if calls != 0 {
							t.Fatal("alias wrote before debounce")
						}
						tt.Advance(181 * time.Millisecond)
					} else if action == "command" || action == "toggle" {
						tt.Key(0, ui.KeySpace)
						tt.Frame()
					} else {
						tt.Key(0, ui.KeyEnter)
						tt.Frame()
					}
					if action == "remove" {
						if !a.removeConfirm[in.ID] || calls != 0 {
							t.Error("removal confirmation not preserved")
						}
					} else if calls != 1 {
						t.Errorf("calls=%d, want 1", calls)
					}
					if action == "edit" && a.toolEditing != in.ID {
						t.Error("wrong editor owner")
					}
					// Equivalent data and invisible entries may not reclaim a manual scroll.
					a.Body.Y = 0
					tt.Frame()
					before := a.Body.Y
					recommended = append([]RecommendedTool(nil), recommended...)
					recommended = append(recommended, RecommendedTool{ID: "hidden", Name: "已安装", Installed: true})
					detected = append([]DetectedTool(nil), detected...)
					tt.Frame()
					if a.Body.Y != before {
						t.Errorf("stable layout stole scroll: %v -> %v", before, a.Body.Y)
					}
				})
			}
		})
	}
}
