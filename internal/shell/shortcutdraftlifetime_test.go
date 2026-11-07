package shell

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestShortcutCanceledDraftDoesNotInheritSystemRefusal(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			tt := h.frame(dark)
			copy := i18n.For("en").Settings
			editingClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			h.blocked["Ctrl+Shift+Z"] = true
			editingClick(t, tt, copy.ShortcutsBind)
			if !tt.HasText(copy.ShortcutsOccupied) {
				t.Fatal("no system refusal")
			}
			trash, _ := tt.Find(copy.ShortcutsRemove)
			row, _ := tt.Find(copy.ShortcutsBind)
			tt.ClickAt(trash.X+trash.W/2, row.Y+row.H/2)
			tt.Frame()
			tt.Frame()
			if tt.HasText(copy.ShortcutsBind) {
				t.Fatal("draft not canceled")
			}
			h.blocked["Ctrl+Shift+Z"] = false
			editingClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			tt.Frame()
			if tag := os.Getenv("FLOTER_P13IZ_DUMP"); tag != "" {
				appearance := map[bool]string{false: "light", true: "dark"}[dark]
				f, err := os.Create("/tmp/floter-P13-iz/host-draft-lifetime-" + appearance + "-" + tag + ".png")
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
			if tt.HasText(copy.ShortcutsOccupied) {
				t.Error("fresh draft inherited old host refusal")
			}
			editingClick(t, tt, copy.ShortcutsBind)
			entries := settings.CustomShortcutsOf(h.app.Store.Snapshot())
			if len(entries) != 2 || entries[1].Key != "Ctrl+Shift+Z" || h.live["Ctrl+Shift+Z"] == nil || h.app.shortcutsSuspended {
				t.Fatalf("fresh draft did not register and resume: %v", entries)
			}
		})
	}
}
