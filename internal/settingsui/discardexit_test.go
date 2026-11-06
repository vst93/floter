package settingsui

import (
	"testing"
	"time"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestConfigDiscardEscapeKeepsDraftAndKeyboard(t *testing.T) { checkDiscardExit(t, false) }
func TestConfigDiscardExpiryKeepsDraftAndKeyboard(t *testing.T) { checkDiscardExit(t, true) }

func checkDiscardExit(t *testing.T, timeout bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, lang := range []string{"en", "zh"} {
			for _, elsewhere := range []bool{false, true} {
				name := "discard-escape"
				if timeout {
					name = "discard-timeout"
				}
				if elsewhere {
					name += "-elsewhere"
				}
				name += "-" + lang
				t.Run(appearanceName(dark)+"/"+name, func(t *testing.T) {
					a, calls := configFormApp(t, []ConfigField{{Key: "token", Label: "Token", Type: "text"}}, map[string]any{"token": "stored"})
					if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark); s.Language = lang }); err != nil {
						t.Fatal(err)
					}
					closed := 0
					a.Actions.Close = func() { closed++ }
					a.Actions.DiagnoseIntegration = func(string) {}
					tt := noticeExitPage(t, a, dark)
					copy := i18n.For(lang).Settings
					in := a.Integrations()[0]
					a.draft(in).values["token"] = "edited"
					tt.Frame()
					if err := tt.Click(copy.ConfigOpen); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
					tt.Frame()
					if !tt.Focused(copy.ConfigCancel) {
						t.Fatal("discard did not focus Cancel")
					}
					if elsewhere {
						if err := tt.Click(copy.IntegrationsCheck); err != nil {
							t.Fatal(err)
						}
						tt.Frame()
					}
					if timeout {
						tt.Advance(3*time.Second + 2*time.Millisecond)
					} else {
						tt.Key(0, ui.KeyEscape)
					}
					tt.Frame()
					tt.Frame()
					noticeExitDump(t, tt, name, dark)
					if closed != 0 || a.configDiscard[in.ID] || !a.configOpen[in.ID] || a.draft(in).values["token"] != "edited" || len(calls.saved) != 0 {
						t.Errorf("discard exit closed settings or lost draft: closed=%d armed=%v open=%v value=%v saved=%d", closed, a.configDiscard[in.ID], a.configOpen[in.ID], a.draft(in).values["token"], len(calls.saved))
					}
					if elsewhere {
						if !tt.Focused(copy.IntegrationsCheck) {
							t.Error("discard exit stole unrelated focus")
						}
						return
					}
					if !tt.Focused(copy.ConfigOpen) {
						t.Error("discard exit lost Configure focus")
					}
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					tt.Frame()
					if !a.configDiscard[in.ID] || !tt.Focused(copy.ConfigCancel) {
						t.Error("discard question cannot reopen by keyboard")
					}
				})
			}
		}
	}
}
