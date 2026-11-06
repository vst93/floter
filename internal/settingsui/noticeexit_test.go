package settingsui

import (
	"image/png"
	"os"
	"testing"
	"time"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func noticeExitDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HV_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hv-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func noticeExitPage(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	a.Page, a.entered = PageIntegrations, true
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		a.View(c)
	}, 720, 900)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	a.Now = tt.Now
	tt.Frame()
	tt.Frame()
	return tt
}

func TestNoticeDisableExternalExitKeepsKeyboard(t *testing.T) { checkNoticeExternalExit(t, false) }
func TestNoticeRemoveExternalExitKeepsKeyboard(t *testing.T)  { checkNoticeExternalExit(t, true) }

func checkNoticeExternalExit(t *testing.T, remove bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, timeout := range []bool{false, true} {
			for _, elsewhere := range []bool{false, true} {
				name := "disable"
				if remove {
					name = "remove"
				}
				if timeout {
					name += "-timeout"
				} else {
					name += "-escape"
				}
				if elsewhere {
					name += "-elsewhere"
				}
				t.Run(appearanceName(dark)+"/"+name, func(t *testing.T) {
					a := noticeApp(t, dark)
					copy := i18n.For("en").Settings
					integration := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
					a.Integrations = func() []Integration { return []Integration{integration} }
					calls, closed := 0, 0
					a.Actions.SetIntegrationEnabled = func(string, bool, func(error)) { calls++ }
					a.Actions.UninstallIntegration = func(string, string, string, func(error)) { calls++ }
					a.Actions.Close = func() { closed++ }
					a.Actions.DiagnoseIntegration = func(string) {}
					tt := noticeExitPage(t, a, dark)
					opener := copy.IntegrationsEnable
					if remove {
						opener = integration.removalLabel(copy)
					}
					if err := tt.Click(opener); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
					tt.Frame()
					if !tt.Focused(copy.IntegrationsCancel) {
						t.Fatal("confirmation not focused on entry")
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
					if a.disableConfirm[integration.ID] || a.removeConfirm[integration.ID] || calls != 0 || closed != 0 {
						t.Fatalf("exit acted or stayed armed: calls=%d closed=%d", calls, closed)
					}
					if elsewhere {
						if !tt.Focused(copy.IntegrationsCheck) {
							t.Error("exit stole focus from another control")
						}
						return
					}
					if !tt.Focused(opener) {
						t.Error("external exit lost its keyboard opener")
					}
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					tt.Frame()
					open := a.disableConfirm[integration.ID]
					if remove {
						open = a.removeConfirm[integration.ID]
					}
					if !open || !tt.Focused(copy.IntegrationsCancel) || calls != 0 {
						t.Error("Space cannot reopen the cancelled question")
					}
				})
			}
		}
	}
}
