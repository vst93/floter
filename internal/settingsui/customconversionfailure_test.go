package settingsui

import (
	"os"
	"path/filepath"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestCustomFailedConversionDoesNotStealDraftFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			s := settings.Default()
			s.Language = "zh"
			s.SetCustomShortcuts([]settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "plugin:browser"}})
			if err := settings.Save(path, s); err != nil {
				t.Fatal(err)
			}
			store, err := settings.OpenStore(path)
			if err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{CustomShortcuts: func() []settings.CustomShortcut { return settings.CustomShortcutsOf(store.Snapshot()) }, SetCustomShortcuts: func(list []settings.CustomShortcut) []CustomShortcutRejection {
				_ = store.Update(func(s *settings.Settings) { s.SetCustomShortcuts(list) })
				return nil
			}})
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			ownershipClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			ownershipChoose(t, tt, copy.ShortcutsActions[0].Label, true)
			optionExitFocus(t, tt, copy.ShortcutsCommand)
			tt.Type("say kept")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			ownershipChoose(t, tt, copy.ShortcutsActions[1].Label, true)
			ownershipDump(t, tt, "failed-conversion-focus", dark)
			if store.LastError() == nil {
				t.Fatal("conversion did not hit disk failure")
			}
			if len(a.customShortcuts()) != 1 || a.customCommand != "say kept" {
				t.Fatal("conversion changed state")
			}
			if tt.Focused(copy.ShortcutsCommand) {
				t.Error("failed conversion stole focus into unrelated command draft")
			}
			if !tt.Focused(copy.ShortcutsAction) {
				t.Error("failed conversion lost original picker outlet")
			}
		})
	}
}
