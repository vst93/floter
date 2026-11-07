package settingsui

import (
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestActionWriteFailureIgnoresOldError(t *testing.T) {
	for _, occupied := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-write", true: "system-refusal"}[occupied], func(t *testing.T) {
			a, _ := actionWriteFailureApp(t, false)
			if err := a.Store.Update(func(s *settings.Settings) { s.HideOnBlur = !s.HideOnBlur }); err == nil {
				t.Fatal("expected a prior disk error")
			}
			a.Actions.SetShortcut = func(string, string) {}
			a.Actions.ShortcutRefused = func(string) bool { return occupied }
			tt := actionWriteFrame(a, false, "record")
			optionExitFocus(t, tt, a.copy().ShortcutRecord)
			tt.Key(0, ui.KeyEnter)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			if a.Store.Writes() != 1 || a.SaveFailedVisible() || tt.HasText(a.copy().SaveFailedAlert) {
				t.Error("a callback with no disk write replayed an unrelated old save error")
			}
			if (a.rejectedShortcut != "") != occupied {
				t.Error("system refusal lost its distinct feedback")
			}
		})
	}
}
