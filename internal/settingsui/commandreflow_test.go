package settingsui

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func TestCommandAliasApprovalReflow(t *testing.T)  { checkCommandReflow(t, "alias", "approval") }
func TestCommandSwitchApprovalReflow(t *testing.T) { checkCommandReflow(t, "switch", "approval") }

func TestCommandAliasPermissionReflow(t *testing.T)  { checkCommandReflow(t, "alias", "permissions") }
func TestCommandSwitchPermissionReflow(t *testing.T) { checkCommandReflow(t, "switch", "permissions") }

func checkCommandReflow(t *testing.T, control, source string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			in.Params, a.Actions.RunWithParams = nil, nil
			active := Command{ID: "forecast", Name: "天气预报", Declared: true, Enabled: true, Available: true, Alias: "stored"}
			in.Commands = []Command{active}
			for i := 0; i < 8; i++ {
				in.Commands = append(in.Commands, Command{ID: fmt.Sprintf("other%d", i), Name: fmt.Sprintf("其他命令 %d", i+1), Declared: true, Enabled: true})
			}
			original := append([]Command(nil), in.Commands...)
			freshness := extensions.Freshness{}
			hasFreshness := source == "freshness-source"
			if source == "freshness-source" || source == "config-presence" || source == "config-schema" {
				in.Description = strings.Repeat("查询城市天气并返回温度与降雨信息。\n", 8)
			}
			a.Actions.Freshness = func(string) (extensions.Freshness, bool) { return freshness, hasFreshness }
			if source == "config-schema" || source == "config-error" {
				in.Config = []ConfigField{{Key: "city", Label: "城市", Type: "text"}}
				a.configOpen = map[string]bool{in.ID: true}
			}
			label := a.copy().IntegrationsCommandAliasFor(active.Name)
			if control == "switch" {
				label = a.copy().IntegrationsCommandEnable(active.Name)
			}
			writes := 0
			a.Actions.SetCommandAlias = func(ext, id, value string) {
				writes++
				if ext != in.ID || id != active.ID || value != "stored-next" {
					t.Errorf("alias owner/payload: %s %s %q", ext, id, value)
				}
			}
			a.Actions.SetCommandEnabled = func(ext, id string, enabled bool) {
				writes++
				if ext != in.ID || id != active.ID || enabled {
					t.Errorf("switch owner/payload: %s %s %v", ext, id, enabled)
				}
			}
			tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { c.Resolve(a.Body.Handle).Label("feedback-viewport") })
			a.Now = tt.Now
			optionExitFocus(t, tt, label)
			if source == "freshness-source" && control == "switch" || strings.HasPrefix(source, "transfer-") || source == "highlight" {
				// A manually scrolled, still visible switch can sit at the lower
				// safe edge before an asynchronous freshness update arrives.
				r, viewport := mustFind(t, tt, label), mustFind(t, tt, "feedback-viewport")
				a.Body.Y += r.Y + r.H + 19 - (viewport.Y + viewport.H)
				tt.Frame()
				checkFeedbackVisible(t, tt, label)
			}
			if control == "alias" {
				tt.Key(0, ui.KeyEnd)
				tt.Type("-next")
				tt.Frame()
			}
			for step := 0; step < 4; step++ {
				switch source {
				case "catalog":
					in.Commands = append([]Command(nil), original...)
					if step%2 == 0 {
						in.Commands = append([]Command{{ID: "new", Name: "新增命令", Declared: true, Enabled: true}}, in.Commands...)
					}
				case "order":
					in.Commands = append([]Command(nil), original...)
					if step%2 == 0 {
						in.Commands = append(in.Commands[1:], active)
					}
				case "approval":
					switch step {
					case 0, 3:
						in.Approval = &ApprovalRecord{At: 1700000000, ApprovedDigest: strings.Repeat("a", 64), CurrentDigest: strings.Repeat("a", 64)}
					case 1:
						in.Approval.CurrentDigest = strings.Repeat("b", 64)
						in.Approval.Disclosure = append([]string(nil), extensions.AllPermissions...)
					case 2:
						in.Approval = nil
					}
				case "permissions":
					in.Permissions = []string{extensions.PermissionNetworkFetch}
					a.auditOpen = map[string]bool{in.ID: true}
					if step%2 == 0 {
						in.Permissions = append([]string(nil), extensions.AllPermissions...)
					}
				case "error":
					in.Error = ""
					if step%2 == 0 {
						in.Error = "无法连接天气服务，请检查网络后重试"
					}
				case "description":
					in.Description = ""
					if step%2 == 0 {
						in.Description = strings.Repeat("查询城市天气并返回温度与降雨信息。\n", 10)
					}
				case "config-error":
					a.configError[in.ID] = ""
					if step%2 == 0 {
						a.finishConfigSave(in.ID, errors.New(strings.Repeat("无法保存配置，请检查目录权限后重试。\n", 8)), a.copy())
					}
				case "highlight":
					in.Highlighted = step%2 == 0
				case "transfer-report":
					a.TransferReport = nil
					if step%2 == 0 {
						a.TransferReport = &ImportReport{Path: "/tmp/weather.json", Failed: []TransferFailure{{ID: in.ID, Reason: strings.Repeat("导入失败，请检查网络后重试。\n", 8)}}}
					}
				case "freshness":
					hasFreshness = step%2 == 0
				case "freshness-source":
					freshness.Source = extensions.FreshnessNone
					if step%2 == 0 {
						freshness.Source = extensions.FreshnessHealth
					}
				case "config-presence":
					in.Config = nil
					if step%2 == 0 {
						in.Config = []ConfigField{{Key: "city", Label: "城市", Type: "text"}}
					}
				case "config-schema":
					in.Config = []ConfigField{{Key: "city", Label: "城市", Type: "text"}}
					if step%2 == 0 {
						in.Config[0].Description = strings.Repeat("请选择要查询的城市。\n", 5)
						in.Config = append(in.Config, ConfigField{Key: "unit", Label: "温度单位", Type: "text"})
					}
				case "failure":
					in.FailureCode, in.FailureDetail = "", ""
					if step%2 == 0 {
						in.FailureCode = extensions.BindingCodeMissing
						in.FailureDetail = "/usr/bin/weather"
					}
				case "health":
					switch step {
					case 0, 3:
						in.Health, in.HealthReport = "Degraded", testHealthReport()
					case 1:
						in.HealthReport.Probes = append(in.HealthReport.Probes, HealthProbe{ID: "network", Stderr: strings.Repeat("连接失败，请检查服务。\n", 12)})
					case 2:
						in.Health, in.HealthReport = "", nil
					}
				case "diagnosis":
					switch step {
					case 0, 3:
						in.Diagnosis = "天气服务不可用，请检查网络后重试"
					case 1:
						in.DiagnosisChecks = []DiagnosisCheck{{ID: "network", Status: "error", Message: strings.Repeat("连接失败，请检查服务。\n", 12)}}
					case 2:
						in.Diagnosis, in.DiagnosisChecks = "", nil
					}
				}
				tt.Frame()
				if step == 0 && source == "description" && control == "alias" {
					if tag := os.Getenv("FLOTER_P13KZ_DUMP"); tag != "" {
						f, err := os.Create("/tmp/floter-P13-kz-command-" + appearanceName(dark) + "-" + tag + ".png")
						if err != nil {
							t.Fatal(err)
						}
						if err := png.Encode(f, tt.Image()); err != nil {
							t.Fatal(err)
						}
						if err := f.Close(); err != nil {
							t.Fatal(err)
						}
					}
				}
				if step == 0 && source == "freshness" && control == "alias" {
					if tag := os.Getenv("FLOTER_P13KY_DUMP"); tag != "" {
						f, err := os.Create("/tmp/floter-P13-ky-command-" + appearanceName(dark) + "-" + tag + ".png")
						if err != nil {
							t.Fatal(err)
						}
						if err := png.Encode(f, tt.Image()); err != nil {
							t.Fatal(err)
						}
						if err := f.Close(); err != nil {
							t.Fatal(err)
						}
					}
				}
				if step == 0 && source == "error" && control == "alias" {
					if tag := os.Getenv("FLOTER_P13KX_DUMP"); tag != "" {
						f, err := os.Create("/tmp/floter-P13-kx-command-" + appearanceName(dark) + "-" + tag + ".png")
						if err != nil {
							t.Fatal(err)
						}
						if err := png.Encode(f, tt.Image()); err != nil {
							t.Fatal(err)
						}
						f.Close()
					}
				}
				if step == 0 && source == "catalog" && control == "alias" {
					if tag := os.Getenv("FLOTER_P13KW_DUMP"); tag != "" {
						f, err := os.Create("/tmp/floter-P13-kw-command-" + appearanceName(dark) + "-" + tag + ".png")
						if err != nil {
							t.Fatal(err)
						}
						if err := png.Encode(f, tt.Image()); err != nil {
							t.Fatal(err)
						}
						f.Close()
					}
				}
				checkFeedbackVisible(t, tt, label)
			}
			if control == "switch" {
				tt.Key(0, ui.KeySpace)
				tt.Frame()
			} else {
				if writes != 0 {
					t.Fatal("alias wrote before debounce")
				}
				tt.Advance(181 * time.Millisecond)
			}
			if writes != 1 {
				t.Errorf("writes=%d, want exactly one", writes)
			}
			// Same layout and unrelated changes must leave manual scrolling alone.
			a.Body.Y = 0
			tt.Frame()
			before := a.Body.Y
			freshness.AtSeconds++
			in.Name = strings.Repeat("更新后的天气查询工具名称", 20)
			count := 2
			freshness.CommandCount = &count
			in.Config = append([]ConfigField(nil), in.Config...)
			if len(in.Config) > 0 {
				in.Config[0].Default = "新默认值"
			}
			in.Commands[len(in.Commands)-1].Description = "更新描述，不改变行高"
			if in.Approval != nil {
				copy := *in.Approval
				in.Approval = &copy
			}
			if in.HealthReport != nil {
				copy := *in.HealthReport
				in.HealthReport = &copy
			}
			tt.Frame()
			if a.Body.Y != before {
				t.Errorf("stable layout stole scroll: %v -> %v", before, a.Body.Y)
			}
			if source == "config-error" {
				a.configOpen[in.ID] = false
				tt.Frame()
				a.Body.Y = 0
				tt.Frame()
				before = a.Body.Y
				a.configError[in.ID] = "隐藏表单的保存失败"
				tt.Frame()
				if a.Body.Y != before {
					t.Errorf("hidden configuration error stole scroll: %v -> %v", before, a.Body.Y)
				}
			}
		})
	}
}

func TestCommandAliasFreshnessReflow(t *testing.T)  { checkCommandReflow(t, "alias", "freshness") }
func TestCommandSwitchFreshnessReflow(t *testing.T) { checkCommandReflow(t, "switch", "freshness") }

func TestCommandAliasFreshnessSourceReflow(t *testing.T) {
	checkCommandReflow(t, "alias", "freshness-source")
}
func TestCommandSwitchFreshnessSourceReflow(t *testing.T) {
	checkCommandReflow(t, "switch", "freshness-source")
}

func TestCommandAliasConfigPresenceReflow(t *testing.T) {
	checkCommandReflow(t, "alias", "config-presence")
}
func TestCommandSwitchConfigPresenceReflow(t *testing.T) {
	checkCommandReflow(t, "switch", "config-presence")
}

func TestCommandAliasConfigSchemaReflow(t *testing.T) {
	checkCommandReflow(t, "alias", "config-schema")
}
func TestCommandSwitchConfigSchemaReflow(t *testing.T) {
	checkCommandReflow(t, "switch", "config-schema")
}

func TestCommandAliasCatalogReflow(t *testing.T)  { checkCommandReflow(t, "alias", "catalog") }
func TestCommandAliasOrderReflow(t *testing.T)    { checkCommandReflow(t, "alias", "order") }
func TestCommandSwitchCatalogReflow(t *testing.T) { checkCommandReflow(t, "switch", "catalog") }
func TestCommandSwitchOrderReflow(t *testing.T)   { checkCommandReflow(t, "switch", "order") }

func TestCommandAliasErrorReflow(t *testing.T)  { checkCommandReflow(t, "alias", "error") }
func TestCommandSwitchErrorReflow(t *testing.T) { checkCommandReflow(t, "switch", "error") }

func TestCommandAliasFailureReflow(t *testing.T)  { checkCommandReflow(t, "alias", "failure") }
func TestCommandSwitchFailureReflow(t *testing.T) { checkCommandReflow(t, "switch", "failure") }

func TestCommandAliasHealthReflow(t *testing.T)  { checkCommandReflow(t, "alias", "health") }
func TestCommandSwitchHealthReflow(t *testing.T) { checkCommandReflow(t, "switch", "health") }

func TestCommandAliasDiagnosisReflow(t *testing.T)  { checkCommandReflow(t, "alias", "diagnosis") }
func TestCommandSwitchDiagnosisReflow(t *testing.T) { checkCommandReflow(t, "switch", "diagnosis") }

func TestCommandAliasDescriptionReflow(t *testing.T)  { checkCommandReflow(t, "alias", "description") }
func TestCommandSwitchDescriptionReflow(t *testing.T) { checkCommandReflow(t, "switch", "description") }

func TestCommandAliasConfigErrorReflow(t *testing.T) { checkCommandReflow(t, "alias", "config-error") }
func TestCommandSwitchConfigErrorReflow(t *testing.T) {
	checkCommandReflow(t, "switch", "config-error")
}

func TestCommandAliasHighlightReflow(t *testing.T)  { checkCommandReflow(t, "alias", "highlight") }
func TestCommandSwitchHighlightReflow(t *testing.T) { checkCommandReflow(t, "switch", "highlight") }
