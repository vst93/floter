package settingsui

import (
	"testing"
	"time"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestNoticeRemoveSurvivesEarlierQuestionExpiry(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			copy := i18n.For("en").Settings
			in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
			a.Integrations = func() []Integration { return []Integration{in} }
			disabled, removed := 0, 0
			a.Actions.SetIntegrationEnabled = func(string, bool, func(error)) { disabled++ }
			a.Actions.UninstallIntegration = func(string, string, string, func(error)) { removed++ }
			tt := noticeExitPage(t, a, dark)
			if err := tt.Click(copy.IntegrationsEnable); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Advance(2 * time.Second)
			if err := tt.Click(in.removalLabel(copy)); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			if !tt.Focused(in.removalLabel(copy)) {
				t.Fatal("removal answer did not take focus")
			}
			tt.Advance(time.Second + 2*time.Millisecond)
			tt.Frame()
			tt.Frame()
			noticeExitDump(t, tt, "remove-survives-expiry", dark)
			if a.disableConfirm[in.ID] || !a.removeConfirm[in.ID] {
				t.Fatal("confirmation clocks not independent")
			}
			if !tt.Focused(in.removalLabel(copy)) {
				t.Error("earlier question expiry lost removal answer focus")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if disabled != 0 || removed != 1 {
				t.Errorf("remaining answer no longer executes removal: disable=%d remove=%d", disabled, removed)
			}
		})
	}
}

func TestNoticeNewDisableDoesNotInheritRemovalAnswer(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := noticeApp(t, dark)
			copy := i18n.For("en").Settings
			in := Integration{ID: "local", Name: "Workspace Tools", Enabled: true, Running: true, RuntimeAvailable: true, DistributionSource: "local"}
			a.Integrations = func() []Integration { return []Integration{in} }
			disabled, removed := 0, 0
			a.Actions.SetIntegrationEnabled = func(string, bool, func(error)) { disabled++ }
			a.Actions.UninstallIntegration = func(string, string, string, func(error)) { removed++ }
			tt := noticeExitPage(t, a, dark)
			if err := tt.Click(in.removalLabel(copy)); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			// Open the earlier sibling through its real switch. The new question
			// must focus its own Cancel, never a reused answer on the old band.
			if err := tt.Click(copy.IntegrationsEnable); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			noticeExitDump(t, tt, "disable-inserted", dark)
			if a.disableConfirm[in.ID] || !a.removeConfirm[in.ID] || disabled != 0 || removed != 0 {
				t.Errorf("new question default cancels the wrong band: disableOpen=%v removeOpen=%v disable=%d remove=%d", a.disableConfirm[in.ID], a.removeConfirm[in.ID], disabled, removed)
			}
		})
	}
}
