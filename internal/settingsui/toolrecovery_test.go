package settingsui

import (
	"image/png"
	"os"
	"reflect"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func toolRecoveryDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JY_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jy-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func toolRecoveryFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = "zh"; s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	return localDraftFrame(t, a, dark)
}

func TestToolRecoveryRestoresCompleteDraft(t *testing.T) {
	for _, mode := range []string{"", routeScript} {
		for _, dark := range []bool{false, true} {
			t.Run(mode+appearanceName(dark), func(t *testing.T) {
				a := localDraftApp()
				tool := callbackTool("weather")
				tool.Name, tool.Mode = "  工作区工具  ", mode
				tool.ScriptLanguage, tool.ScriptContent = "python", "print('sunny')\n"
				tool.Output, tool.Description, tool.VersionArgs = routeBackground, "  本地天气  ", " --version "
				tool.Permissions = []string{extensions.PermissionNetworkFetch}
				a.beginEdit("", tool)
				a.toolParams = []toolParam{{ID: "city", Label: "城市", Kind: extensions.ParamText, Default: "上海", Required: true}}
				a.toolAdvanced = true
				var recoverDraft func()
				var requests []CustomTool
				a.Actions.ConnectLocalTool = func(tool CustomTool) {
					requests = append(requests, tool)
					recoverDraft = a.LocalToolRecovery()
				}
				tt := toolRecoveryFrame(t, a, dark)
				params := append([]toolParam(nil), a.toolParams...)
				if err := tt.Click(a.copy().LocalToolConnect); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if recoverDraft == nil || a.toolOpen || a.toolName != "" {
					t.Fatal("submission did not retire its form")
				}
				recoverDraft()
				tt.Frame()
				toolRecoveryDump(t, tt, "draft-"+map[bool]string{true: "script", false: "program"}[mode == routeScript], dark)
				if !a.toolOpen || a.toolName != tool.Name || a.toolProgram != tool.Program || a.toolScript != tool.ScriptContent || a.toolVersionArgs != tool.VersionArgs || a.toolDescription != tool.Description || !a.toolAdvanced || !reflect.DeepEqual(a.toolParams, params) {
					t.Errorf("failed connection lost complete draft: open=%v name=%q program=%q script=%q params=%+v", a.toolOpen, a.toolName, a.toolProgram, a.toolScript, a.toolParams)
					return
				}
				if err := tt.Click(a.copy().LocalToolConnect); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if len(requests) != 2 || !reflect.DeepEqual(requests[0], requests[1]) {
					t.Errorf("retry request changed: %+v", requests)
				}
			})
		}
	}
}

func TestToolRecoveryKeepsNewerForm(t *testing.T) {
	for _, newer := range []string{"typed", "cleared", "folded", "edit"} {
		t.Run(newer, func(t *testing.T) {
			a := localDraftApp()
			a.beginEdit("", callbackTool("weather"))
			var recoverDraft func()
			a.Actions.ConnectLocalTool = func(CustomTool) { recoverDraft = a.LocalToolRecovery() }
			tt := toolCallbackFrame(t, a, false)
			if err := tt.Click(a.copy().LocalToolConnect); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if newer == "edit" {
				a.beginEdit("next", callbackTool("next"))
			} else {
				if err := tt.Click(a.copy().LocalToolAdd); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				localFormFocus(t, tt, a.copy().LocalToolName)
				tt.Type("新工具")
				tt.Frame()
				if newer == "cleared" {
					tt.Key(ui.Cmd, ui.KeyA)
					tt.Key(0, ui.KeyBackspace)
					tt.Frame()
				}
				if newer == "folded" {
					if err := tt.Click(a.copy().LocalToolAdd); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
				}
			}
			name, program, open, key := a.toolName, a.toolProgram, a.toolOpen, a.toolFormKey
			recoverDraft()
			tt.Frame()
			if a.toolName != name || a.toolProgram != program || a.toolOpen != open || a.toolFormKey != key {
				t.Error("old failure overwrote a newer form")
			}
		})
	}
}

func TestToolRecoveryBusyConnection(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := localDraftApp()
			a.beginEdit("", callbackTool("weather"))
			calls := 0
			a.Actions.ConnectLocalTool = func(CustomTool) { calls++; a.BeginPackageInstall("custom") }
			tt := toolRecoveryFrame(t, a, dark)
			if err := tt.Click(a.copy().LocalToolConnect); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText(a.copy().LocalToolConnect) {
				t.Error("pending connection has no visible progress")
			}
			toolRecoveryDump(t, tt, "connect-wait", dark)
			if err := tt.Click(a.copy().LocalToolAdd); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			localFormFocus(t, tt, a.copy().LocalToolName)
			tt.Type("新工具")
			tt.Frame()
			if err := tt.Click(a.copy().LocalToolConnect); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 || !a.toolOpen || a.toolName != "新工具" {
				t.Errorf("busy connection consumed next draft: calls=%d open=%v name=%q", calls, a.toolOpen, a.toolName)
			}
			// A second complete draft also cannot submit while the first waits.
			a.beginEdit("", callbackTool("next"))
			tt.Frame()
			if err := tt.Click(a.copy().LocalToolConnect); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 {
				t.Errorf("busy connection dispatched %d requests", calls)
			}
			a.FinishPackageInstall("custom")
			tt.Frame()
			if err := tt.Click(a.copy().LocalToolConnect); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 2 {
				t.Errorf("unlocked connection calls=%d", calls)
			}
		})
	}
}
