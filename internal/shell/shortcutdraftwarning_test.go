package shell

import (
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestShortcutDraftCancelKeepsSavedRefusal(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			if err := h.app.Store.Update(func(s *settings.Settings) {
				s.SetCustomShortcuts([]settings.CustomShortcut{{Key: "Ctrl+Shift+A", Action: "plugin:clipboard"}, {Key: "Ctrl+Shift+B", Action: "say original"}})
			}); err != nil {
				t.Fatal(err)
			}
			h.app.ApplyCustomShortcuts()
			tt := h.frame(dark)
			copy := i18n.For("en").Settings
			editingClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			lifecycleRecord(t, h, tt, "Ctrl+Shift+B")
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyA)
			tt.Frame()
			tt.Frame()
			if !tt.HasText(copy.ShortcutsConflict) {
				t.Fatal("duplicate capture did not reach saved-row refusal")
			}
			writes := h.app.Store.Writes()
			trash, _ := tt.Find(copy.ShortcutsRemove)
			row, _ := tt.Find(copy.ShortcutsBind)
			tt.ClickAt(trash.X+trash.W/2, row.Y+row.H/2)
			tt.Frame()
			tt.Frame()
			editingDump(t, tt, "draft-cancel-refusal", dark)
			if tt.HasText(copy.ShortcutsBind) {
				t.Fatal("draft was not canceled")
			}
			if !tt.HasText(copy.ShortcutsConflict) {
				t.Error("canceling unrelated draft erased saved binding refusal")
			}
			entries := settings.CustomShortcutsOf(h.app.Store.Snapshot())
			if len(entries) != 2 || entries[1].Key != "Ctrl+Shift+B" || h.app.Store.Writes() != writes {
				t.Error("draft cancel changed saved bindings")
			}
			lifecycleRecord(t, h, tt, "Ctrl+Shift+B")
			if tt.HasText(copy.ShortcutsConflict) {
				t.Error("fresh recording did not clear its own refusal")
			}
		})
	}
}
