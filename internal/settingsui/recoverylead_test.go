package settingsui

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestRecoveryLeadRecheck(t *testing.T)     { checkRecoveryLead(t, "recheck") }
func TestRecoveryLeadReconnect(t *testing.T)   { checkRecoveryLead(t, "reconnect") }
func TestRecoveryLeadHomepage(t *testing.T)    { checkRecoveryLead(t, "homepage") }
func TestRecoveryLeadHealth(t *testing.T)      { checkRecoveryLead(t, "health") }
func TestRecoveryLeadAdopt(t *testing.T)       { checkRecoveryLead(t, "adopt") }
func TestRecoveryLeadDelete(t *testing.T)      { checkRecoveryLead(t, "delete") }
func TestRecoveryLeadPermissions(t *testing.T) { checkRecoveryLead(t, "permissions") }
func TestRecoveryLeadSearch(t *testing.T)      { checkRecoveryLead(t, "search") }

// Exercise actual focus and activation, including pointer press/release. A
// layout refresh must reveal the same owner without stealing later scrolling.
func checkRecoveryLead(t *testing.T, action string) {
	t.Helper()
	for _, source := range []string{"local", "recommended", "detected"} {
		t.Run(source, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				t.Run(appearanceName(dark), func(t *testing.T) {
					for _, pointer := range []bool{false, true} {
						t.Run(map[bool]string{false: "keyboard", true: "pointer"}[pointer], func(t *testing.T) {
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
							beforeSearch := settings.ShowCommandsInSearch(a.settings())
							switch action {
							case "recheck":
								in.Broken = true
								label = copy.IntegrationsRecheck
								a.Actions.RepairIntegration = func(id string, done func(error)) { called(id); done(nil) }
							case "reconnect":
								in.RuntimeAvailable, in.RuntimeOwnership = false, extensions.RuntimeOwnershipSystem
								label = copy.IntegrationsReconnect
								a.Actions.ReconnectIntegration = func(id string, done func(error)) { called(id); done(nil) }
							case "homepage":
								in.RuntimeAvailable, in.Homepage = false, "https://example.org/weather"
								label = copy.IntegrationsInstallTool
								a.Actions.OpenHomepage = func(url string) {
									calls++
									if url != in.Homepage {
										t.Errorf("wrong homepage: %s", url)
									}
								}
							case "health":
								in.Health, in.HealthReport = "Degraded", testHealthReport()
								label, a.Actions.ReprobeHealth = copy.IntegrationsHealthReprobe, called
							case "permissions":
								in.Permissions = []string{"environment"}
								label = copy.IntegrationsPermissions
							case "adopt":
								in.Orphan = true
								label, a.Actions.AdoptIntegration = copy.IntegrationsAdopt, called
							case "delete":
								in.Orphan = true
								label, a.Actions.DeleteOrphan = copy.IntegrationsDeleteOrphan, called
							case "search":
								label = copy.ShowCommandsInSearch
							}
							tt := toolVisibilityFrame(t, a, dark, func(c *ui.Context) { c.Resolve(a.Body.Handle).Label("feedback-viewport") })
							optionExitFocus(t, tt, label)
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
								if action == "recheck" && source == "recommended" && step == 1 && !pointer {
									if tag := os.Getenv("FLOTER_P13LC_DUMP"); tag != "" {
										f, err := os.Create("/tmp/floter-P13-lc-recovery-" + appearanceName(dark) + "-" + tag + ".png")
										if err != nil {
											t.Fatal(err)
										}
										err = png.Encode(f, tt.Image())
										closeErr := f.Close()
										if err != nil || closeErr != nil {
											t.Fatalf("dump: %v %v", err, closeErr)
										}
									}
								}
								checkFeedbackVisible(t, tt, label)
							}
							if pointer {
								// Drop keyboard focus before a real click, without moving the body.
								tt.Key(ui.Shift, ui.KeyTab)
								tt.Frame()
								if err := tt.Click(label); err != nil {
									t.Fatal(err)
								}
							} else if action == "search" {
								tt.Key(0, ui.KeySpace)
							} else {
								tt.Key(0, ui.KeyEnter)
							}
							tt.Frame()
							switch action {
							case "permissions":
								if !a.auditOpen[in.ID] {
									t.Error("permission disclosure did not open")
								}
							case "search":
								if settings.ShowCommandsInSearch(a.settings()) == beforeSearch {
									t.Error("search switch did not change")
								}
							default:
								if calls != 1 {
									t.Errorf("activation calls=%d, want 1", calls)
								}
							}
							a.Body.Y = 0
							tt.Frame()
							before := a.Body.Y
							recommended = append(append([]RecommendedTool(nil), recommended...), RecommendedTool{ID: "hidden", Name: "已安装", Installed: true})
							detected = append([]DetectedTool(nil), detected...)
							tt.Frame()
							if a.Body.Y != before {
								t.Errorf("stable layout stole scroll: %v -> %v", before, a.Body.Y)
							}
						})
					}
				})
			}
		})
	}
}
