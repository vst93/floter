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

func prefixDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13KS_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ks-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkFormPrefix(t *testing.T, scenario, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			label := a.copy().RunLabel
			calls := 0
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
				a.Actions.SaveConfiguration = func(id string, values map[string]any, done func(error)) {
					calls++
					if id != in.ID || values["c0"] != "dirty" {
						t.Error("save owner or payload changed")
					}
					done(nil)
				}
				transfer := func(id string, values map[string]any, done func(error)) {
					calls++
					if id != in.ID || values["c0"] != "dirty" {
						t.Error("transfer owner or payload changed")
					}
					done(nil)
				}
				a.Actions.CopyConfiguration, a.Actions.ExportConfiguration = transfer, transfer
				switch scenario {
				case "config-open":
					label = a.copy().ConfigOpen
				case "config-defaults":
					label = a.copy().ConfigDefaults
				case "config-save":
					label = a.copy().ConfigSave
				case "config-toolbar":
					label = a.copy().ConfigCopy
					if kind == "export" {
						label = a.copy().ConfigExport
					}
				case "tool-open":
					in.ConfigOwner, in.ConfigOpenCommand = "tool", []string{"settings"}
					label = a.copy().ConfigOpenConfiguration
					a.Actions.OpenConfiguration = func(id string, command []string) {
						calls++
						if id != in.ID || len(command) != 1 || command[0] != "settings" {
							t.Error("tool editor owner changed")
						}
					}
				case "config-fields":
					label = "配置 5"
				}
			} else if scenario == "run-controls" {
				in.Params[4].Kind, label = kind, "参数 5"
				if kind == "boolean" {
					in.Params[4].Default = "false"
				}
				if kind == "select" {
					in.Params[4].Options, in.Params[4].Default, label = []string{"北方", "南方"}, "北方", "北方"
				}
				if kind == "cancel" {
					in.Params[4].Kind, label = "text", a.copy().RunCancel
				}
			}
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { c.Resolve(a.Body.Handle).Label("feedback-viewport") })
			optionExitFocus(t, tt, label)
			for step, present := range []bool{true, false, true} {
				message := ""
				if present {
					message = strings.Repeat("服务连接失败，请检查连接。\n", 9)
				}
				switch scenario {
				case "run-submit":
					in.Params[0].Label = "参数 1" + message
				case "config-toolbar":
					in.Description = message
				default:
					in.Error = message
				}
				tt.Frame()
				if step == 0 {
					prefixDump(t, tt, scenario+"-"+kind, dark)
				}
				checkFeedbackVisible(t, tt, label)
			}
			if config && a.draft(*in).values["c0"] != "dirty" {
				t.Fatal("diagnostic refresh changed draft")
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
			runVisibilityWheel(t, a, tt)
			manual := a.Body.Y
			tt.Frame()
			tt.Frame()
			if a.Body.Y != manual {
				t.Error("unchanged prefix reclaimed manual scrolling")
			}
		})
	}
}

func TestFormPrefixRunSubmit(t *testing.T)      { checkFormPrefix(t, "run-submit", "text") }
func TestFormPrefixToolOpen(t *testing.T)       { checkFormPrefix(t, "tool-open", "") }
func TestFormPrefixConfigFields(t *testing.T)   { checkFormPrefix(t, "config-fields", "text") }
func TestFormPrefixConfigDefaults(t *testing.T) { checkFormPrefix(t, "config-defaults", "") }
func TestFormPrefixConfigSave(t *testing.T)     { checkFormPrefix(t, "config-save", "") }
func TestFormPrefixRunControls(t *testing.T) {
	for _, kind := range []string{"text", "number", "path", "boolean", "select", "cancel"} {
		t.Run(kind, func(t *testing.T) { checkFormPrefix(t, "run-controls", kind) })
	}
}
