package settingsui

import (
	"errors"
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestUninstallRecoveryFailureKeepsKeyboardRetry(t *testing.T) {
	checkUninstallRecovery(t, false)
}

func TestUninstallRecoveryDelayedFailureKeepsKeyboardRetry(t *testing.T) {
	checkUninstallRecovery(t, true)
}

func checkUninstallRecovery(t *testing.T, delayed bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := uninstallTestApp()
			a.uninstallOpen = map[string]bool{}
			copy := i18n.For("en").Settings
			calls := 0
			var answer func(error)
			a.Actions.UninstallComponents = func(_, _, _ string, _ UninstallComponents, done func(error)) {
				calls++
				if delayed {
					answer = done
				} else {
					done(errors.New("Removal failed; try again"))
				}
			}
			tt := ui.NewTester(func(c *ui.Context) {
				if ui.Button(c, "Open uninstall").Clicked() {
					a.uninstallOpen["npm"] = true
				}
				if a.uninstallOpen["npm"] {
					a.uninstallDialog(c, copy, npmTool())
				}
			}, 720, 640)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			if err := tt.Click("Open uninstall"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if !tt.Focused(copy.PermissionsCancel) {
				t.Fatal("dialog did not initially focus cancel")
			}
			for i := 0; i < 8 && !tt.Focused(copy.IntegrationsUninstall); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(copy.IntegrationsUninstall) {
				recoveryDump(t, tt, "uninstall-retry", dark)
				t.Fatal("Tab did not reach uninstall")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			if delayed {
				if answer == nil || !a.uninstallBusy["npm"] {
					t.Fatal("uninstall did not stay busy")
				}
				answer(errors.New("Removal failed; try again"))
				tt.Frame()
				tt.Frame()
			}
			recoveryDump(t, tt, "uninstall-retry", dark)
			if !tt.Focused(copy.IntegrationsUninstall) {
				t.Error("failure moved keyboard focus off retry")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			if calls != 2 {
				t.Errorf("keyboard retry dispatched %d calls, want 2", calls)
			}
		})
	}
}
