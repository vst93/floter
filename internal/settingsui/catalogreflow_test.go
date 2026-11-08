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
func checkCatalogReflow(t *testing.T, scenario, kind string) {
	t.Helper()
	sources := []string{"approval"}
	if strings.HasPrefix(scenario, "run-") {
		sources = append(sources, "commands")
	}
	for _, source := range sources {
		t.Run(source, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				t.Run(appearanceName(dark), func(t *testing.T) {
					a, in := runVisibilityApp(t, dark)
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
					for step := 0; step < 4; step++ {
						if source == "commands" {
							switch step {
							case 0, 3:
								in.Commands = []Command{{ID: "forecast", Name: "天气预报", Declared: true, Enabled: true, Available: true}}
							case 1:
								in.Commands[0].Description = strings.Repeat("未来一周的温度、降水与出行提示。", 18)
							case 2:
								in.Commands = nil
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
						// Two representative states suffice to judge the visual change.
						if step == 0 && kind == "text" && (scenario == "run-controls" && source == "commands" || scenario == "config-fields") {
							catalogReflowDump(t, tt, scenario+"-"+source, dark)
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
					if in.Approval != nil {
						copy := *in.Approval
						in.Approval = &copy
					}
					in.Commands = append(in.Commands, Command{ID: "hidden-editor", Description: "not declared"})
					tt.Frame()
					tt.Frame()
					if a.Body.Y != manual {
						t.Error("equivalent record or hidden command reclaimed manual scrolling")
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
