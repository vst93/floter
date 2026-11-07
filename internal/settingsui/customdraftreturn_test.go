package settingsui

import (
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestCustomDraftRetiresWithRemovedBinding(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("say obsolete")
			first, target, last := (*entries)[0], (*entries)[1], (*entries)[2]
			*entries = []settings.CustomShortcut{first, last}
			tt.Frame()
			target.Action = "say new"
			*entries = []settings.CustomShortcut{first, target, last}
			tt.Frame()
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			feedbackDump(t, tt, "command-removed-return", dark)
			if got := refreshRead(tt); got != "say new" {
				t.Errorf("recreated binding resurrected deleted command draft: %q", got)
			}
			tt.Key(0, ui.KeyEnter)
			if (*entries)[1].Action != "say new" {
				t.Error("retired draft overwrote recreated binding")
			}
		})
	}
}

func TestCustomDraftSurvivesRecordingTheSameKey(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("say unfinished")
			for i := 0; i < 3; i++ {
				tt.Key(ui.Shift, ui.KeyTab)
			}
			tt.Key(0, ui.KeyEnter)
			if !a.customRecording || a.customRecordRow != 1 {
				t.Fatal("second recorder not armed")
			}
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyB)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			feedbackDump(t, tt, "command-same-key", dark)
			if got := refreshRead(tt); got != "say unfinished" {
				t.Errorf("same-key recording discarded unfinished command: %q", got)
			}
			if (*entries)[1].Action != "say original" {
				t.Fatal("recording committed an unfinished command")
			}
			tt.Key(0, ui.KeyEnter)
			if (*entries)[1].Action != "say unfinished" {
				t.Error("retained command cannot be committed")
			}
		})
	}
}
