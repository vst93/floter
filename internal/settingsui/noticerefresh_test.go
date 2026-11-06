package settingsui

import (
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestNoticeVanishedDisableRetiresQuestion(t *testing.T) { checkNoticeVanished(t, "disable") }
func TestNoticeVanishedRemoveRetiresQuestion(t *testing.T)  { checkNoticeVanished(t, "remove") }

func TestNoticeVanishedDiscardRetiresQuestion(t *testing.T) { checkNoticeVanished(t, "discard") }
func TestNoticeVanishedUninstallRetiresDialog(t *testing.T) { checkNoticeVanished(t, "uninstall") }

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
				if kind == "discard" {
					in.Config = []ConfigField{{Key: "token", Label: "Token", Type: "text"}}
					in.ConfigValues = map[string]any{"token": "stored"}
					a.configOpen = map[string]bool{in.ID: true}
					a.draft(in).values["token"] = "edited"
				}
				if kind == "uninstall" {
					in.DistributionSource = "npm"
				}
				list := []Integration{in}
				a.Integrations = func() []Integration { return list }
				dispatched, closed := 0, 0
				a.Actions.SetIntegrationEnabled = func(string, bool, func(error)) { dispatched++ }
				a.Actions.UninstallIntegration = func(string, string, string, func(error)) { dispatched++ }
				a.Actions.UninstallComponents = func(string, string, string, UninstallComponents, func(error)) { dispatched++ }
				a.Actions.Close = func() { closed++ }
				copy := i18n.For("en").Settings
				tt := noticeExitPage(t, a, dark)
				opener := copy.IntegrationsEnable
				if kind == "remove" || kind == "uninstall" {
					opener = in.removalLabel(copy)
				}
				if kind == "discard" {
					opener = copy.ConfigOpen
				}
				if err := tt.Click(opener); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				if kind == "uninstall" {
					tt.Key(0, ui.KeyTab)
					tt.Frame()
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					tt.Frame()
					if !a.uninstallOpen[in.ID] || !tt.HasText(copy.UninstallHostConfig) {
						t.Fatal("uninstall dialog did not open")
					}
				}
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
				if kind == "discard" && (!a.configOpen[in.ID] || a.draft(in).values["token"] != "edited") {
					t.Error("retiring the question discarded the draft")
				}
				if dispatched != 0 {
					t.Errorf("refresh dispatched an action: %d", dispatched)
				}
			})
		}
	}
}
