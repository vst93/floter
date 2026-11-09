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

func catalogReflowDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KV_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-kv-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

// Drive the same focused outlet through insertion, in-place edits, removal,
// and restoration of the content preceding it. Pointer replacement alone
// must not reclaim scrolling; neither may a hidden synthesized command.
func checkCatalogReflow(t *testing.T, scenario, kind string, lead ...string) {
	t.Helper()
	sources := []string{"approval"}
	if strings.HasPrefix(scenario, "run-") {
		sources = append(sources, "commands")
	}
	if len(lead) > 0 {
		sources = lead
	}
	for _, source := range sources {
		t.Run(source, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				t.Run(appearanceName(dark), func(t *testing.T) {
					a, in := runVisibilityApp(t, dark)
					if strings.HasPrefix(source, "param-") {
						a.Actions.ConnectLocalTool = func(CustomTool) { t.Error("unexpected local connect") }
						// Reach the downstream control before opening a TextArea,
						// whose Tab key deliberately inserts indentation.
						a.toolOpen = false
						a.toolName, a.toolCommand, a.toolProgram, a.toolVersion = "Weather", "weather", "/usr/bin/printf", "1.0.0"
						a.toolPlatforms = map[string]bool{currentPlatform(): true}
						a.toolParams = []toolParam{{ID: "city", Label: "城市", Kind: extensions.ParamSelect, Options: "北京\n上海"}}
					}
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
					label, calls := a.copy().RunLabel, 0
					a.Actions.RunWithParams = func(id string, values map[string]string) *extensions.ParamFailure {
						calls++
						if id != in.ID || values["p0"] != "12" {
							t.Error("run owner or payload changed")
						}
						return nil
					}
					config := strings.HasPrefix(scenario, "config-") || scenario == "tool-open"
					if config {
						in.Params, a.Actions.RunWithParams = nil, nil
						for i := 0; i < 8; i++ {
							in.Config = append(in.Config, ConfigField{Key: fmt.Sprintf("c%d", i), Label: fmt.Sprintf("配置 %d", i+1), Type: "text", Default: "stored"})
						}
						a.configOpen = map[string]bool{in.ID: scenario != "config-open"}
						a.draft(*in).values["c0"] = "dirty"
						transfer := func(id string, values map[string]any, done func(error)) {
							calls++
							if id != in.ID || values["c0"] != "dirty" {
								t.Error("configuration owner or payload changed")
							}
							done(nil)
						}
						a.Actions.SaveConfiguration = transfer
						a.Actions.CopyConfiguration = func(id string, values map[string]any, done func(error)) {
							if kind != "copy" {
								t.Error("export invoked copy")
							}
							transfer(id, values, done)
						}
						a.Actions.ExportConfiguration = func(id string, values map[string]any, done func(error)) {
							if kind != "export" {
								t.Error("copy invoked export")
							}
							transfer(id, values, done)
						}
						switch scenario {
						case "config-fields":
							label = "配置 5"
						case "config-defaults":
							label = a.copy().ConfigDefaults
						case "config-save":
							label = a.copy().ConfigSave
						case "config-toolbar":
							label = a.copy().ConfigCopy
							if kind == "export" {
								label = a.copy().ConfigExport
							}
						case "config-open":
							label = a.copy().ConfigOpen
						case "tool-open":
							label = a.copy().ConfigOpenConfiguration
							in.ConfigOwner, in.ConfigOpenCommand = "tool", []string{"settings"}
							a.Actions.OpenConfiguration = func(id string, command []string) {
								calls++
								if id != in.ID || len(command) != 1 || command[0] != "settings" {
									t.Error("tool editor owner changed")
								}
							}
						}
					} else if scenario == "run-controls" {
						in.Params[4].Kind, label = kind, "参数 5"
						switch kind {
						case "boolean":
							in.Params[4].Default = "false"
						case "select":
							in.Params[4].Options, in.Params[4].Default, label = []string{"北方", "南方"}, "北方", "北方"
						case "cancel":
							in.Params[4].Kind, label = "text", a.copy().RunCancel
						}
					}
					tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { c.Resolve(a.Body.Handle).Label("feedback-viewport") })
					optionExitFocus(t, tt, label)
					if strings.HasPrefix(source, "param-") {
						a.toolOpen = true
						tt.Frame()
						r := mustFind(t, tt, label)
						viewport := mustFind(t, tt, "feedback-viewport")
						a.Body.Y += r.Y + r.H + 19 - viewport.Y - viewport.H
						tt.Frame()
					}
					for step := 0; step < 4; step++ {
						if strings.HasPrefix(source, "param-") {
							a.toolParams[0].ID, a.toolParams[0].Options = "city", "北京\n上海"
							if step != 2 {
								if source == "param-refusal" {
									a.toolParams[0].ID = "bad id"
								} else {
									a.toolParams[0].Options = strings.Repeat("广州\n", 18) + "上海"
								}
							}
						} else if source == "commands" {
							switch step {
							case 0, 3:
								in.Commands = []Command{{ID: "forecast", Name: "天气预报", Declared: true, Enabled: true, Available: true}}
							case 1:
								in.Commands[0].Description = strings.Repeat("未来一周的温度、降水与出行提示。", 18)
							case 2:
								in.Commands = nil
							}
						} else if source == "recommended" || source == "detected" || source == "local" {
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
						} else {
							switch step {
							case 0, 3:
								in.Approval = &ApprovalRecord{At: 1700000000, ApprovedDigest: strings.Repeat("a", 64), CurrentDigest: strings.Repeat("a", 64)}
							case 1:
								in.Approval.CurrentDigest = strings.Repeat("b", 64)
								in.Approval.Enforced = []string{extensions.PermissionNetworkFetch}
								in.Approval.Disclosure = append([]string(nil), extensions.AllPermissions...)
							case 2:
								in.Approval = nil
							}
						}
						tt.Frame()
						if step == 0 && source == "param-options" && scenario == "run-controls" && kind == "text" {
							if tag := os.Getenv("FLOTER_P13LG_DUMP"); tag != "" {
								f, err := os.Create("/tmp/floter-P13-lg-downstream-" + appearanceName(dark) + "-" + tag + ".png")
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
						// Two representative states suffice to judge the visual change.
						if step == 0 && kind == "text" && (scenario == "run-controls" && source == "commands" || scenario == "config-fields") {
							catalogReflowDump(t, tt, scenario+"-"+source, dark)
						}
						if step == 1 && source == "recommended" && scenario == "run-controls" && kind == "text" {
							leadReflowDump(t, tt, dark)
						}
						checkFeedbackVisible(t, tt, label)
					}
					if config && a.draft(*in).values["c0"] != "dirty" {
						t.Error("approval refresh changed draft")
					}
					switch scenario {
					case "run-controls":
						switch kind {
						case "boolean":
							tt.Key(0, ui.KeySpace)
							tt.Frame()
							if a.runValues[in.ID]["p4"] != "true" {
								t.Error("wrong toggle owner")
							}
						case "select":
							tt.Key(0, ui.KeyRight)
							tt.Frame()
							if a.runValues[in.ID]["p4"] != "南方" {
								t.Error("wrong selection owner")
							}
						case "cancel":
							tt.Key(0, ui.KeyEnter)
							tt.Frame()
							if a.runOpen[in.ID] || a.runValues[in.ID]["p4"] != "12" {
								t.Error("cancel lost answers")
							}
						default:
							tt.Key(ui.Cmd, ui.KeyA)
							tt.Type("24")
							tt.Frame()
							if a.runValues[in.ID]["p4"] != "24" {
								t.Error("wrong input owner")
							}
						}
					case "config-fields":
						tt.Key(ui.Cmd, ui.KeyA)
						tt.Type("continued")
						tt.Frame()
						if a.draft(*in).values["c4"] != "continued" {
							t.Error("wrong config editor owner")
						}
					case "config-open":
						tt.Key(0, ui.KeyEnter)
						tt.Frame()
						if !a.configOpen[in.ID] {
							t.Error("configuration did not open")
						}
					case "config-defaults":
						tt.Key(0, ui.KeyEnter)
						tt.Frame()
						if a.draft(*in).values["c0"] != "stored" {
							t.Error("defaults did not restore")
						}
					default:
						tt.Key(0, ui.KeyEnter)
						tt.Frame()
						if calls != 1 {
							t.Errorf("action dispatched %d times", calls)
						}
					}
					if (scenario == "run-controls" || scenario == "config-fields" || scenario == "config-defaults" || scenario == "config-open") && calls != 0 {
						t.Errorf("unrelated operations: %d", calls)
					}
					runVisibilityWheel(t, a, tt)
					manual := a.Body.Y
					if strings.HasPrefix(source, "param-") {
						a.toolParams[0].Options = strings.ReplaceAll(a.toolParams[0].Options, "广州", "深圳")
					}
					if in.Approval != nil {
						copy := *in.Approval
						in.Approval = &copy
					}
					in.Commands = append(in.Commands, Command{ID: "hidden-editor", Description: "not declared"})
					// Equal values in fresh slices and hidden installed recommendations
					// must not claim the scroll wheel after the reveal.
					recommended = append(append([]RecommendedTool(nil), recommended...), RecommendedTool{ID: "hidden", Name: "已安装", Description: "隐藏推荐", Installed: true})
					detected = append([]DetectedTool(nil), detected...)
					tt.Frame()
					tt.Frame()
					if a.Body.Y != manual {
						t.Error("equivalent record or hidden command reclaimed manual scrolling")
					}
					if strings.HasPrefix(source, "param-") {
						// Automatic wrapping is internal to this TextArea: without
						// Lines(), its outer height only follows paragraph count.
						// Long same-paragraph edits must not reclaim manual scrolling.
						a.toolParams[0].Options = strings.ReplaceAll(a.toolParams[0].Options, "上海", strings.Repeat("上海", 100))
						tt.Frame()
						tt.Frame()
						if a.Body.Y != manual {
							t.Error("internal wrapping reclaimed manual scrolling")
						}
					}
				})
			}
		})
	}
}

func TestCatalogReflowRunEditors(t *testing.T) {
	for _, kind := range []string{"text", "number", "path"} {
		t.Run(kind, func(t *testing.T) { checkCatalogReflow(t, "run-controls", kind) })
	}
}

func TestCatalogReflowRunChoices(t *testing.T) {
	for _, kind := range []string{"boolean", "select"} {
		t.Run(kind, func(t *testing.T) { checkCatalogReflow(t, "run-controls", kind) })
	}
}

func TestCatalogReflowRunCancel(t *testing.T) { checkCatalogReflow(t, "run-controls", "cancel") }
func TestCatalogReflowRunSubmit(t *testing.T) { checkCatalogReflow(t, "run-submit", "text") }

func TestCatalogReflowConfigFields(t *testing.T) { checkCatalogReflow(t, "config-fields", "text") }
func TestCatalogReflowConfigActions(t *testing.T) {
	for _, scenario := range []string{"config-defaults", "config-save"} {
		t.Run(scenario, func(t *testing.T) { checkCatalogReflow(t, scenario, "") })
	}
}

func TestCatalogReflowConfigTransfers(t *testing.T) {
	for _, kind := range []string{"copy", "export"} {
		t.Run(kind, func(t *testing.T) { checkCatalogReflow(t, "config-toolbar", kind) })
	}
}

func TestCatalogReflowConfigOpen(t *testing.T) {
	for _, scenario := range []string{"config-open", "tool-open"} {
		t.Run(scenario, func(t *testing.T) { checkCatalogReflow(t, scenario, "") })
	}
}
