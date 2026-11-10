package shell

import (
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

func toolConnectionField(t *testing.T, tt *ui.Tester, label, value string) {
	t.Helper()
	toolConnectionFocus(t, tt, label)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(value)
	tt.Frame()
}

func toolConnectionFocus(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for i := 0; !tt.Focused(label) && i < 80; i++ {
		tt.Key(0, ui.KeyTab)
		tt.Frame()
	}
	if !tt.Focused(label) {
		t.Fatalf("cannot focus %s", label)
	}
}

func toolConnectionSubmit(t *testing.T, tt *ui.Tester) {
	t.Helper()
	copy := i18n.For("zh").Settings
	// Several recommendations are also named Connect. Traverse from this
	// form's program input so Enter targets its own final action.
	toolConnectionFocus(t, tt, copy.LocalToolProgram)
	toolConnectionFocus(t, tt, copy.LocalToolConnect)
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
}

func checkToolConnectionRecovery(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newInstallCurrentFixture(t, dark)
			program, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			tool := settingsui.CustomTool{Name: "工作区工具", Program: program, Command: "weather", Platforms: []string{runtime.GOOS}, Permissions: []string{extensions.PermissionNetworkFetch}}
			if scenario == "prepare" {
				tool.Program = filepath.Join(t.TempDir(), "missing-program")
			}
			f.a.confirmPermissions = func(extensions.PermissionApproval) bool { return false }
			if scenario == "commit" {
				f.a.confirmPermissions = func(extensions.PermissionApproval) bool {
					files, err := filepath.Glob(filepath.Join(f.a.Paths.Data, "*", "integration", "floter.extension.json"))
					if err != nil || len(files) != 1 {
						t.Errorf("prepared manifest: %v / %v", files, err)
						return false
					}
					if err := os.Remove(files[0]); err != nil {
						t.Error(err)
						return false
					}
					return true
				}
			}
			copy := i18n.For("zh").Settings
			tt := ui.NewTester(f.a.View, 960, 2400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			settingsSectionForTest(t, f.a, tt, copy.IntegrationsAdd, copy.IntegrationsLocal)
			if err := tt.Click(copy.LocalToolAdd); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			toolConnectionField(t, tt, copy.LocalToolName, tool.Name)
			f.a.Settings.Actions.ChooseProgram = func() (string, error) { return tool.Program, nil }
			if err := tt.Click(copy.LocalToolChoose); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			toolConnectionField(t, tt, copy.LocalToolCommand, tool.Command)
			if err := tt.Click(copy.LocalToolAdvanced); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click(i18n.PermissionLabel("zh", extensions.PermissionNetworkFetch)); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click(copy.LocalToolAdvanced); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			var recovery func()
			var requests []settingsui.CustomTool
			f.a.Settings.Actions.ConnectLocalTool = func(draft settingsui.CustomTool) {
				requests = append(requests, draft)
				recovery = f.a.Settings.LocalToolRecovery()
			}
			toolConnectionSubmit(t, tt)
			if len(requests) != 1 || recovery == nil {
				t.Fatal("form never dispatched connection")
			}
			recovered, settled := 0, 0
			f.a.Settings.Actions.Invalidate = func() { settled++ }
			f.a.runLocalToolConnection(requests[0], func() { recovered++; recovery() })
			titles := noticeTitles(f.a)
			tt.Frame()
			tt.Frame()
			if tag := os.Getenv("FLOTER_P13JY_DUMP"); tag != "" {
				file, err := os.Create("/tmp/floter-P13-jy-connect-" + scenario + "-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(file, tt.Image()); err != nil {
					t.Fatal(err)
				}
				file.Close()
			}
			if recovered != 1 || settled != 1 {
				t.Errorf("%s recovery=%d settled=%d, want one each", scenario, recovered, settled)
			}
			if scenario == "cancel" && len(titles) != 0 {
				t.Errorf("cancelled connection published failure: %q", titles)
			}
			if scenario != "cancel" && len(titles) != 1 {
				t.Errorf("prepare failure notices=%q", titles)
			}
			if got := extensions.LoadInventory(f.a.Paths); len(got.Integrations) != 0 {
				t.Errorf("failed connection installed entries: %+v", got)
			}
			if !tt.HasText(copy.LocalToolProgram) {
				t.Error("failed connection did not reopen editable form")
				return
			}
			toolConnectionSubmit(t, tt)
			if len(requests) != 2 || !reflect.DeepEqual(requests[0], requests[1]) {
				t.Errorf("%s retry lost input: %+v", scenario, requests)
			}
		})
	}
}

func TestToolConnectionPrepareRecovery(t *testing.T) { checkToolConnectionRecovery(t, "prepare") }
func TestToolConnectionCancelRecovery(t *testing.T)  { checkToolConnectionRecovery(t, "cancel") }

func TestToolConnectionCommitRecovery(t *testing.T) { checkToolConnectionRecovery(t, "commit") }

func TestToolConnectionPickerFailure(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := newInstallCurrentFixture(t, dark)
			copy := i18n.For("zh").Settings
			f.a.Settings.Actions.ChooseProgram = func() (string, error) { return "", os.ErrPermission }
			tt := ui.NewTester(f.a.View, 960, 2400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			settingsSectionForTest(t, f.a, tt, copy.IntegrationsAdd, copy.IntegrationsLocal)
			if err := tt.Click(copy.LocalToolAdd); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click(copy.LocalToolChoose); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Advance(200 * time.Millisecond)
			tt.Frame()
			if tag := os.Getenv("FLOTER_P13JY_DUMP"); tag != "" {
				file, err := os.Create("/tmp/floter-P13-jy-picker-error-" + map[bool]string{false: "light", true: "dark"}[dark] + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(file, tt.Image()); err != nil {
					t.Fatal(err)
				}
				file.Close()
			}
			if !tt.HasText(copy.LocalToolFailed) {
				t.Error("program picker failure never reached host notice")
			}
		})
	}
}
