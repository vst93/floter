package settingsui

import (
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
				case "failure":
					in.FailureCode, in.FailureDetail = "", ""
					if step%2 == 0 {
						in.FailureCode = extensions.BindingCodeMissing
						in.FailureDetail = "/usr/bin/weather"
					}
				case "health":
					in.Health, in.HealthReport = "", nil
					if step%2 == 0 {
						in.Health, in.HealthReport = "Degraded", testHealthReport()
					}
				case "diagnosis":
					in.Diagnosis = ""
					if step%2 == 0 {
						in.Diagnosis = "天气服务不可用，请检查网络后重试"
					}
				}
				tt.Frame()
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
			in.Commands[len(in.Commands)-1].Description = "更新描述，不改变行高"
			if in.Approval != nil {
				copy := *in.Approval
				in.Approval = &copy
			}
			tt.Frame()
			if a.Body.Y != before {
				t.Errorf("stable layout stole scroll: %v -> %v", before, a.Body.Y)
			}
		})
	}
}

func TestCommandAliasCatalogReflow(t *testing.T)  { checkCommandReflow(t, "alias", "catalog") }
func TestCommandAliasOrderReflow(t *testing.T)    { checkCommandReflow(t, "alias", "order") }
func TestCommandSwitchCatalogReflow(t *testing.T) { checkCommandReflow(t, "switch", "catalog") }
func TestCommandSwitchOrderReflow(t *testing.T)   { checkCommandReflow(t, "switch", "order") }

func TestCommandAliasErrorReflow(t *testing.T)  { checkCommandReflow(t, "alias", "error") }
func TestCommandSwitchErrorReflow(t *testing.T) { checkCommandReflow(t, "switch", "error") }
