package settingsui

import (
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func checkConfigRecovery(t *testing.T, discard bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, calls := configFormApp(t, []ConfigField{{Key: "token", Label: "Token", Type: "text"}}, map[string]any{"token": "stored"})
			a.Store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) })
			tt := integrationPage(t, a, 900, 1400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			copy := i18n.For("en").Settings
			a.draft(a.Integrations()[0]).values["token"] = "edited"
			tt.Frame()
			if err := tt.Click(copy.ConfigOpen); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if !tt.Focused(copy.ConfigCancel) {
				t.Fatal("discard question did not focus Cancel")
			}
			name := "config-cancel"
			if discard {
				name = "config-discard"
				tt.Key(0, ui.KeyTab)
				tt.Frame()
				if !tt.Focused(copy.ConfigDiscard) {
					t.Fatal("Tab could not reach Discard")
				}
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			recoveryDump(t, tt, name, dark)
			if !tt.Focused(copy.ConfigOpen) {
				t.Errorf("%s lost focus instead of returning to Configure", name)
			}
			if a.configOpen["dev.floter.tool"] == discard || a.configDiscard["dev.floter.tool"] {
				t.Errorf("wrong close/confirmation state after %s", name)
			}
			if len(calls.saved) != 0 {
				t.Errorf("%s saved configuration", name)
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			if discard {
				if !a.configOpen["dev.floter.tool"] || a.draft(a.Integrations()[0]).values["token"] != "stored" {
					t.Error("discard could not reopen stored configuration by keyboard")
				}
			} else if !a.configDiscard["dev.floter.tool"] || a.draft(a.Integrations()[0]).values["token"] != "edited" {
				t.Error("cancel could not reopen question with draft intact")
			}
		})
	}
}

func TestConfigRecoveryCancelReturnsToOpener(t *testing.T)  { checkConfigRecovery(t, false) }
func TestConfigRecoveryDiscardReturnsToOpener(t *testing.T) { checkConfigRecovery(t, true) }
