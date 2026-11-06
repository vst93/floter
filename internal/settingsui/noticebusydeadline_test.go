package settingsui

import (
	"errors"
	"image/png"
	"os"
	"testing"
	"time"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func noticeRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HW_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hw-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNoticeBusyDisableDoesNotStarveDiscardDeadline(t *testing.T) {
	checkNoticeBusyDeadline(t, false)
}
func TestNoticeBusyRemoveDoesNotStarveDiscardDeadline(t *testing.T) { checkNoticeBusyDeadline(t, true) }

func checkNoticeBusyDeadline(t *testing.T, remove bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, calls := configFormApp(t, []ConfigField{{Key: "token", Label: "Token", Type: "text"}}, map[string]any{"token": "stored"})
			if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
				t.Fatal(err)
			}
			in := a.Integrations()[0]
			copy := i18n.For("en").Settings
			var reply func(error)
			dispatched := 0
			a.Actions.SetIntegrationEnabled = func(_ string, _ bool, done func(error)) { dispatched++; reply = done }
			a.Actions.UninstallIntegration = func(_, _, _ string, done func(error)) { dispatched++; reply = done }
			tt := noticeExitPage(t, a, dark)
			a.draft(in).values["token"] = "edited"
			tt.Frame()
			opener, name := copy.IntegrationsEnable, "busy-disable-deadline"
			if remove {
				opener, name = in.removalLabel(copy), "busy-remove-deadline"
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
			if dispatched != 1 || reply == nil {
				t.Fatal("busy operation did not dispatch")
			}
			tt.Advance(4 * time.Second)
			if err := tt.Click(copy.ConfigOpen); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			if !a.configDiscard[in.ID] {
				t.Fatal("later discard did not open")
			}
			// Advance only: no forced Frame, pointer movement or key to hide a
			// missed wake. Reduced motion is a supported production preference.
			tt.Advance(2 * time.Second)
			if !a.configDiscard[in.ID] {
				t.Error("discard expired early")
			}
			tt.Advance(time.Second + 2*time.Millisecond)
			noticeRefreshDump(t, tt, name, dark)
			if a.configDiscard[in.ID] {
				t.Error("later discard missed its deadline behind a busy confirmation")
			}
			if !a.disableBusy[in.ID] && !a.removeBusy[in.ID] {
				t.Error("expiry cancelled the running operation")
			}
			if a.draft(in).values["token"] != "edited" || len(calls.saved) != 0 {
				t.Error("expiry changed the draft or saved it")
			}
			reply(errors.New("Still running; retry later"))
			tt.Frame()
			if !tt.HasText("Still running; retry later") {
				t.Error("busy result lost its own retry question")
			}
		})
	}
}
