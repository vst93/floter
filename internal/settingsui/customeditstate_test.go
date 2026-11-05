package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCustomConflictFollowsRemainingBinding(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			a.customDuplicate, a.customDuplicateRow = "Ctrl+Shift+A", 1
			tt := customStateFrame(t, a, dark)
			a.removeCustomShortcut(0)
			tt.Frame()
			localFormDump(t, tt, "shortcut-conflict", dark)
			copy := a.copy()
			if a.customWarning(copy, (*entries)[0].Key, 0) != copy.ShortcutsConflict || a.customWarning(copy, (*entries)[1].Key, 1) != "" {
				t.Errorf("conflict moved to another binding: row=%d", a.customDuplicateRow)
			}
			a.removeCustomShortcut(0)
			tt.Frame()
			if a.customDuplicate != "" || tt.HasText(copy.ShortcutsConflict) {
				t.Error("removed binding left conflict behind")
			}
		})
	}
}

func TestCustomCommandFollowsRemainingBinding(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			tt := customStateFrame(t, a, dark)
			r := mustFind(t, tt, a.copy().ShortcutsCommand)
			tt.ClickAt(r.X+r.W-8, r.Y+r.H/2)
			tt.Frame()
			if !tt.Focused(a.copy().ShortcutsCommand) {
				t.Fatal("command did not take focus")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			a.removeCustomShortcut(0)
			tt.Frame()
			tt.Type("say edited")
			tt.Frame()
			localFormDump(t, tt, "shortcut-command", dark)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if (*entries)[0].Key != "Ctrl+Shift+B" || (*entries)[0].Action != "say edited" || (*entries)[1].Action != "plugin:clipboard" {
				t.Errorf("remaining command lost edit: %+v", *entries)
			}
		})
	}
}
