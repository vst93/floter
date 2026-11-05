package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestChoiceIdentityMultiRangeFollowsOption(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			a, tt := configIdentityFrame(t, dark, &integration)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			integration.Config[0].Options = []string{"应用", "文件", "书签"}
			tt.Frame()
			if err := tt.ClickWith(ui.Shift, "书签"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			choiceIdentityDump(t, tt, "multi-range", dark)
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"文件", "书签"}) {
				t.Errorf("reordered range selected the wrong options: %v", got)
			}
		})
	}
}

func TestChoiceIdentityRemovedAnchorDoesNotSelectItsReplacement(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			a, tt := configIdentityFrame(t, dark, &integration)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			integration.Config[0].Options = []string{"应用", "书签"}
			tt.Frame()
			if err := tt.ClickWith(ui.Shift, "书签"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			choiceIdentityDump(t, tt, "removed-anchor", dark)
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"书签"}) {
				t.Errorf("removed anchor selected an unrequested replacement: %v", got)
			}
			if err := tt.ClickWith(ui.Shift, "应用"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"应用", "书签"}) {
				t.Errorf("new anchor did not extend a range: %v", got)
			}
		})
	}
}
