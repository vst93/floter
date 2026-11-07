package shell

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func queuedDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JA_DUMP"); tag != "" {
		appearance := map[bool]string{false: "light", true: "dark"}[dark]
		f, err := os.Create("/tmp/floter-P13-ja-" + name + "-" + appearance + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestQueuedCustomRemovedBindingCannotDispatch(t *testing.T) { checkQueuedCustomEdit(t, false) }
func TestQueuedCustomReplacedActionCannotDispatch(t *testing.T) { checkQueuedCustomEdit(t, true) }

func checkQueuedCustomEdit(t *testing.T, replace bool) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			commands := []string{}
			h.app.silentCommand = func(s string) error { commands = append(commands, s); return nil }
			old := h.live["Ctrl+Shift+B"]
			tt := h.frame(dark)
			copy := i18n.For("en").Settings
			name := "custom-removed"
			if replace {
				name = "custom-replaced"
				editingClick(t, tt, copy.ShortcutsCommand)
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("say replacement")
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				entries := settings.CustomShortcutsOf(h.app.Store.Snapshot())
				if len(entries) != 1 || entries[0].Action != "say replacement" {
					t.Fatalf("edit not committed: %v", entries)
				}
			} else {
				editingClick(t, tt, copy.ShortcutsRemove)
				if len(settings.CustomShortcutsOf(h.app.Store.Snapshot())) != 0 {
					t.Fatal("delete not committed")
				}
			}
			old() // native delivery was already queued when the key was released
			queuedDump(t, tt, name, dark)
			if len(commands) != 0 {
				t.Fatalf("retired custom callback dispatched: %v", commands)
			}
			if replace {
				h.live["Ctrl+Shift+B"]()
				if len(commands) != 1 || commands[0] != "say replacement" {
					t.Fatalf("current action unavailable: %v", commands)
				}
			}
		})
	}
}
