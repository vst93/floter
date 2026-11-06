package settingsui

import (
	"errors"
	"testing"
	"time"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestNoticeDisableSlowFailureCanRetry(t *testing.T) { checkNoticeSlowFailure(t, false) }
func TestNoticeRemoveSlowFailureCanRetry(t *testing.T)  { checkNoticeSlowFailure(t, true) }

func checkNoticeSlowFailure(t *testing.T, remove bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		for _, cancel := range []bool{false, true} {
			name := "disable-slow-failure"
			if remove {
				name = "remove-slow-failure"
			}
			if cancel {
				name += "-cancelled"
			}
			t.Run(appearanceName(dark)+"/"+name, func(t *testing.T) {
				a := noticeApp(t, dark)
				copy := i18n.For("en").Settings
				in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
				a.Integrations = func() []Integration { return []Integration{in} }
				calls := 0
				var reply func(error)
				a.Actions.SetIntegrationEnabled = func(_ string, _ bool, done func(error)) { calls++; reply = done }
				a.Actions.UninstallIntegration = func(_, _, _ string, done func(error)) { calls++; reply = done }
				tt := noticeExitPage(t, a, dark)
				opener := copy.IntegrationsEnable
				if remove {
					opener = in.removalLabel(copy)
				}
				if err := tt.Click(opener); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				tt.Frame()
				tt.Key(0, ui.KeyTab)
				tt.Frame()
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if reply == nil || calls != 1 {
					t.Fatal("confirmation did not dispatch")
				}
				tt.Advance(5 * time.Second)
				if cancel {
					tt.Key(0, ui.KeyEscape)
					tt.Frame()
				}
				message := "Permission denied; close the process and retry"
				reply(errors.New(message))
				tt.Frame()
				tt.Frame()
				noticeExitDump(t, tt, name, dark)
				if cancel {
					if a.disableConfirm[in.ID] || a.removeConfirm[in.ID] || tt.HasText(message) {
						t.Error("late failure reopened a cancelled question")
					}
					return
				}
				if !tt.HasText(message) {
					t.Error("slow failure vanished before it could be read")
				}
				tt.Advance(2 * time.Second)
				if !tt.HasText(message) {
					t.Error("failure did not get its existing three-second answer window")
				}
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if calls != 2 {
					t.Fatalf("slow failure cannot retry from keyboard: calls=%d", calls)
				}
				reply(errors.New(message))
				tt.Frame()
				tt.Advance(3*time.Second + 2*time.Millisecond)
				if a.disableConfirm[in.ID] || a.removeConfirm[in.ID] {
					t.Error("failure retry window never expires")
				}
			})
		}
	}
}
