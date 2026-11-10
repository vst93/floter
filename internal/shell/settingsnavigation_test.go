package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Control regressions enter the new management surface through its real UI.
func openSettingsToolForTest(t *testing.T, a *App, tt *ui.Tester) {
	t.Helper()
	copy := a.SettingsCopy()
	if tt.HasText(copy.IntegrationsBack) {
		return
	}
	a.Settings.Body.Y = 0
	tt.Frame()
	if err := tt.Click(copy.IntegrationsMine); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	items := a.Settings.Integrations()
	if len(items) == 0 {
		return
	}
	if err := tt.Click(copy.IntegrationsManage + " " + items[0].Name); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
}

func settingsSectionForTest(t *testing.T, a *App, tt *ui.Tester, labels ...string) {
	t.Helper()
	a.Settings.Body.Y = 0
	tt.Frame()
	if tt.HasText(a.SettingsCopy().IntegrationsBack) {
		if err := tt.Click(a.SettingsCopy().IntegrationsBack); err != nil {
			t.Fatal(err)
		}
		a.Settings.Body.Y = 0
		tt.Frame()
	}
	for _, label := range labels {
		if err := tt.Click(label); err != nil {
			// A queued toast may cover the top of this small viewport. Navigate the
			// same segmented control with Tab/Right, without dismissing the evidence.
			copy := a.SettingsCopy()
			group := []string{copy.IntegrationsMine, copy.IntegrationsAdd, copy.IntegrationsTransfer}
			if label == copy.IntegrationsDiscover || label == copy.IntegrationsLocal || label == copy.IntegrationsPackages {
				group = []string{copy.IntegrationsDiscover, copy.IntegrationsLocal, copy.IntegrationsPackages}
			}
			for i := 0; i < 60; i++ {
				inGroup := false
				for _, name := range group {
					if tt.Focused(name) {
						inGroup = true
					}
				}
				if inGroup {
					break
				}
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			for i := 0; i < 3 && !tt.Focused(label); i++ {
				tt.Key(0, ui.KeyRight)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatal(err)
			}
		}
		tt.Frame()
	}
}
