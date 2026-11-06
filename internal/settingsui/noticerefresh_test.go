package settingsui

import (
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestNoticeVanishedDisableRetiresQuestion(t *testing.T) { checkNoticeVanished(t, "disable") }
func TestNoticeVanishedRemoveRetiresQuestion(t *testing.T)  { checkNoticeVanished(t, "remove") }

func checkNoticeVanished(t *testing.T, kind string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, escape := range []bool{false, true} {
			name := kind + "-return"
			if escape {
				name = kind + "-missing-escape"
			}
			t.Run(appearanceName(dark)+"/"+name, func(t *testing.T) {
				a := noticeApp(t, dark)
				in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
				list := []Integration{in}
				a.Integrations = func() []Integration { return list }
				dispatched, closed := 0, 0
				a.Actions.SetIntegrationEnabled = func(string, bool, func(error)) { dispatched++ }
				a.Actions.UninstallIntegration = func(string, string, string, func(error)) { dispatched++ }
				a.Actions.Close = func() { closed++ }
				copy := i18n.For("en").Settings
				tt := noticeExitPage(t, a, dark)
				opener := copy.IntegrationsEnable
				if kind == "remove" {
					opener = in.removalLabel(copy)
				}
				if err := tt.Click(opener); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				if !a.anyConfirmOpen() {
					t.Fatal("question did not open")
				}
				list = nil
				tt.Frame()
				tt.Frame()
				if !tt.HasText(copy.IntegrationsEmpty) {
					t.Fatal("list did not refresh")
				}
				if escape {
					tt.Key(0, ui.KeyEscape)
					tt.Frame()
					if closed != 1 {
						t.Errorf("invisible %s question swallowed Escape: close=%d", kind, closed)
					}
				} else {
					list = []Integration{in}
					tt.Frame()
					tt.Frame()
					if a.anyConfirmOpen() || tt.HasText(copy.IntegrationsCancel) {
						t.Errorf("stale %s question revived after list refresh", kind)
					}
				}
				noticeRefreshDump(t, tt, name, dark)
				if dispatched != 0 {
					t.Errorf("refresh dispatched an action: %d", dispatched)
				}
			})
		}
	}
}
