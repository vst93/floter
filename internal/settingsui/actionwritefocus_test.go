package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestActionWriteFailureKeepsFullPageFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, _ := actionWriteFailureApp(t, dark)
			a.Page = PageIntegrations
			tt := render(t, a, 720, 900)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			optionExitFocus(t, tt, a.copy().BaseClipboard)
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			tt.Frame()
			actionWriteDump(t, tt, "full-page-focus", dark)
			if !a.SaveFailedVisible() || !tt.Focused(a.copy().BaseClipboard) {
				t.Error("save failure displaced the full page keyboard retry target")
			}
			a.dismissSaveFailed()
			tt.Frame()
			if !tt.Focused(a.copy().BaseClipboard) {
				t.Error("dismissing save failure displaced the keyboard target")
			}
		})
	}
}
