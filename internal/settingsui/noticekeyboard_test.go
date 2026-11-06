package settingsui

import (
	"errors"
	"image/png"
	"os"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func noticeDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HU_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hu-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func noticeApp(t *testing.T, dark bool) *App {
	t.Helper()
	a := newTestApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestNoticeDisableKeyboardConfirmAndRetry(t *testing.T) {
	checkNoticeKeyboard(t, false)
}

func TestNoticeRemoveKeyboardConfirmAndRetry(t *testing.T) {
	checkNoticeKeyboard(t, true)
}

func checkNoticeKeyboard(t *testing.T, remove bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, lang := range []string{"en", "zh"} {
			for _, delayed := range []bool{false, true} {
				name := "disable"
				if remove {
					name = "remove"
				}
				t.Run(appearanceName(dark)+"/"+lang+"/"+map[bool]string{false: "sync", true: "delayed"}[delayed], func(t *testing.T) {
					a := noticeApp(t, dark)
					copy := i18n.For(lang).Settings
					integration := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, DistributionSource: "local"}
					calls := 0
					var answer func(error)
					dispatch := func(done func(error)) {
						calls++
						if delayed {
							answer = done
						} else {
							done(errors.New("Permission denied; close the process and retry"))
						}
					}
					a.Actions.SetIntegrationEnabled = func(_ string, enabled bool, done func(error)) {
						if enabled {
							t.Error("disable dispatched enable")
						}
						dispatch(done)
					}
					a.Actions.UninstallIntegration = func(_, _, _ string, done func(error)) { dispatch(done) }
					confirm := copy.DisableConfirm
					if remove {
						a.requestRemove(integration.ID)
						confirm = integration.removalLabel(copy)
					} else {
						a.requestDisable(integration.ID)
					}
					tt := ui.NewTester(func(c *ui.Context) {
						c.SetTheme(a.tokens(c).Theme)
						c.Root().Background(c.Theme().Background)
						ui.Column(c).FillWidth().Padding(24).Children(func() {
							if remove {
								a.removeConfirmation(c, copy, integration)
							} else {
								a.disableConfirmation(c, copy, integration)
							}
						})
					}, 720, 260)
					tt.SetDark(dark)
					tt.SetPreferences(ui.Preferences{ReduceMotion: true})
					tt.Frame()
					tt.Frame()
					if !tt.Focused(copy.IntegrationsCancel) {
						t.Fatal("fresh confirmation did not focus cancel")
					}
					tt.Key(0, ui.KeyTab)
					tt.Frame()
					tt.Frame()
					noticeDump(t, tt, name+"-tab-"+lang, dark)
					if !tt.Focused(confirm) {
						t.Fatal("Tab cannot leave cancel for the confirmation action")
					}
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					if delayed {
						if answer == nil {
							t.Fatal("confirmation did not reach callback")
						}
						// Repeated activation while busy must not dispatch twice.
						tt.Key(0, ui.KeySpace)
						tt.Frame()
						answer(errors.New("Permission denied; close the process and retry"))
					}
					tt.Frame()
					tt.Frame()
					if calls != 1 || !tt.Focused(confirm) {
						t.Fatalf("failure lost retry: calls=%d focused=%v", calls, tt.Focused(confirm))
					}
					tt.Key(0, ui.KeySpace)
					tt.Frame()
					if calls != 2 {
						t.Errorf("keyboard retry dispatched %d calls, want 2", calls)
					}
				})
			}
		}
	}
}
