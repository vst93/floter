package shell

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func editingDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	tag := os.Getenv("FLOTER_P13IX_DUMP")
	if tag == "" {
		return
	}
	appearance := map[bool]string{false: "light", true: "dark"}[dark]
	f, err := os.Create("/tmp/floter-P13-ix-" + name + "-" + appearance + "-" + tag + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func editingClick(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
}

func TestShortcutEditingActionKeepsCapture(t *testing.T)  { checkShortcutEditing(t, "action") }
func TestShortcutEditingRemovalKeepsCapture(t *testing.T) { checkShortcutEditing(t, "remove") }

func checkShortcutEditing(t *testing.T, scenario string) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			entries := []settings.CustomShortcut{
				{Key: "Ctrl+Shift+A", Action: "plugin:clipboard"},
				{Key: "Ctrl+Shift+B", Action: "say original"},
			}
			if scenario == "convert" {
				// Keep the command option unique so Click targets the popup
				// item rather than another saved row's command picker.
				entries[1].Action = "plugin:browser"
			}
			if err := h.app.Store.Update(func(s *settings.Settings) { s.SetCustomShortcuts(entries) }); err != nil {
				t.Fatal(err)
			}
			h.app.ApplyCustomShortcuts()
			tt := h.frame(dark)
			copy := i18n.For("en").Settings
			recorder := "Ctrl+Alt+J"
			if scenario == "remove" || scenario == "convert" {
				recorder = "Ctrl+Shift+B"
			}
			if scenario == "bind" {
				editingClick(t, tt, copy.ShortcutsAdd)
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
				tt.Frame()
			}
			lifecycleRecord(t, h, tt, recorder)
			switch scenario {
			case "action", "convert":
				editingClick(t, tt, copy.ShortcutsActions[0].Label)
				option := copy.ShortcutsActions[1].Label
				if scenario == "convert" {
					option = copy.ShortcutsActions[len(copy.ShortcutsActions)-1].Label
				}
				editingClick(t, tt, option)
			case "remove":
				editingClick(t, tt, copy.ShortcutsRemove)
			case "bind":
				editingClick(t, tt, copy.ShortcutsBind)
			}
			editingDump(t, tt, "editing-"+scenario, dark)
			got := settings.CustomShortcutsOf(h.app.Store.Snapshot())
			switch scenario {
			case "action":
				if len(got) != 2 || got[0].Action != copy.ShortcutsActions[1].ID {
					t.Fatalf("action not committed: %v", got)
				}
			case "remove", "convert":
				if len(got) != 1 || got[0] != entries[1] {
					t.Fatalf("neighbor removal not committed: %v", got)
				}
			case "bind":
				if len(got) != 3 || got[2].Key != "Ctrl+Shift+Z" {
					t.Fatalf("draft not bound: %v", got)
				}
			}
			if !h.app.shortcutsSuspended || !tt.HasText(copy.ShortcutRecording) {
				t.Error("edit ended unrelated recording")
			}
			if len(h.live) != 0 {
				t.Errorf("%s activated system keys during recording: %v", scenario, keysOf(h.live))
			}
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			tt.Frame()
			if h.app.shortcutsSuspended || h.live["Ctrl+Alt+J"] == nil {
				t.Error("cancel failed to restore summon")
			}
			for _, entry := range got {
				if h.live[entry.Key] == nil {
					t.Errorf("confirmed key not restored: %s", entry.Key)
				}
			}
			if (scenario == "remove" || scenario == "convert") && h.live["Ctrl+Shift+A"] != nil {
				t.Error("removed key revived")
			}
		})
	}
}
