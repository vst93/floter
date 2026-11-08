package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settingsui"
)

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
			recovered, settled := 0, 0
			f.a.Settings.Actions.Invalidate = func() { settled++ }
			f.a.runLocalToolConnection(tool, func() { recovered++ })
			titles := noticeTitles(f.a)
			if recovered != 1 || settled != 1 {
				t.Errorf("%s recovery=%d settled=%d, want one each", scenario, recovered, settled)
			}
			if scenario == "cancel" && len(titles) != 0 {
				t.Errorf("cancelled connection published failure: %q", titles)
			}
			if scenario == "prepare" && len(titles) != 1 {
				t.Errorf("prepare failure notices=%q", titles)
			}
			if got := extensions.LoadInventory(f.a.Paths); len(got.Integrations) != 0 {
				t.Errorf("failed connection installed entries: %+v", got)
			}
		})
	}
}

func TestToolConnectionPrepareRecovery(t *testing.T) { checkToolConnectionRecovery(t, "prepare") }
func TestToolConnectionCancelRecovery(t *testing.T)  { checkToolConnectionRecovery(t, "cancel") }
