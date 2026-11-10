package shell

import (
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func TestShortcutDraftCancelKeepsSavedRecorder(t *testing.T) { checkDraftRecorderExit(t, false) }
func TestShortcutDraftBindEndsItsRecorder(t *testing.T)      { checkDraftRecorderExit(t, true) }

func checkDraftRecorderExit(t *testing.T, bind bool) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			tt := h.frame(dark)
			copy := i18n.For("en").Settings
			editingClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			// Both resting keycaps have the same width. Capture the trash column
			// before the saved recorder expands into its "Press keys" prompt.
			trash, trashOK := tt.Find(copy.ShortcutsRemove)
			key, name := "Ctrl+Shift+B", "draft-cancel-neighbor"
			if bind {
				key, name = "Ctrl+Shift+Z", "draft-bind-recording"
			}
			lifecycleRecord(t, h, tt, key)
			if bind {
				editingClick(t, tt, copy.ShortcutsBind)
			} else {
				// The draft and saved rows share a trash label; click the real
				// draft trash at its resting column and the draft Bind row Y.
				if !trashOK {
					t.Fatal("no trash")
				}
				row, ok := tt.Find(copy.ShortcutsBind)
				if !ok {
					t.Fatal("no draft")
				}
				tt.ClickAt(trash.X+trash.W/2, row.Y+row.H/2)
				tt.Frame()
				tt.Frame()
			}
			editingDump(t, tt, name, dark)
			if tt.HasText(copy.ShortcutsBind) {
				t.Fatal("draft exit did not close draft")
			}
			entries := settings.CustomShortcutsOf(h.app.Store.Snapshot())
			if bind {
				if len(entries) != 2 || entries[1].Key != "Ctrl+Shift+Z" {
					t.Fatalf("bound wrong draft: %v", entries)
				}
				if h.app.shortcutsSuspended {
					t.Error("bound draft left an invisible recorder suspending all keys")
				}
				if h.live["Ctrl+Shift+Z"] == nil || h.live["Ctrl+Shift+B"] == nil || h.live["Ctrl+Alt+J"] == nil {
					t.Error("bound draft did not restore confirmed system keys")
				}
				if !tt.Focused(copy.ShortcutRecord) {
					t.Error("bound draft lost saved recorder keyboard exit")
				}
			} else {
				if !h.app.shortcutsSuspended || !tt.HasText(copy.ShortcutRecording) {
					t.Error("canceling draft stopped saved binding recorder")
				}
				if len(h.live) != 0 {
					t.Error("canceling draft enabled system keys under saved recorder")
				}
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyR)
				tt.Frame()
				tt.Frame()
				entries = settings.CustomShortcutsOf(h.app.Store.Snapshot())
				if len(entries) != 1 || entries[0].Key != "Ctrl+Shift+R" || entries[0].Action != "say original" {
					t.Errorf("saved capture no longer reaches its target: %v", entries)
				}
				if h.app.shortcutsSuspended || h.live["Ctrl+Shift+R"] == nil {
					t.Error("saved recording cannot finish after draft cancel")
				}
			}
		})
	}
}

func TestShortcutDraftOccupiedRetainsRetry(t *testing.T)     { checkDraftRejection(t, false) }
func TestShortcutDraftDuplicateRetainsOriginal(t *testing.T) { checkDraftRejection(t, true) }

func checkDraftRejection(t *testing.T, duplicate bool) {
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			h := newShortcutLifecycle(t, "Ctrl+Alt+J")
			tt := h.frame(dark)
			copy := i18n.For("en").Settings
			editingClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			name, warning := "draft-occupied", copy.ShortcutsOccupied
			if duplicate {
				name, warning = "draft-duplicate", copy.ShortcutsConflict
				if err := h.app.Store.Update(func(s *settings.Settings) {
					list := settings.CustomShortcutsOf(*s)
					s.SetCustomShortcuts(append(list, settings.CustomShortcut{Key: "Ctrl+Shift+Z", Action: "say external"}))
				}); err != nil {
					t.Fatal(err)
				}
				h.app.ApplyCustomShortcuts()
				tt.Frame()
			} else {
				h.blocked["Ctrl+Shift+Z"] = true
			}
			writes := h.app.Store.Writes()
			editingClick(t, tt, copy.ShortcutsBind)
			editingDump(t, tt, name, dark)
			if !tt.HasText(copy.ShortcutsBind) || !tt.HasText(warning) {
				t.Errorf("%s discarded draft or hid refusal", name)
			}
			if duplicate {
				entries := settings.CustomShortcutsOf(h.app.Store.Snapshot())
				if len(entries) != 2 || entries[1].Action != "say external" || h.app.Store.Writes() != writes {
					t.Errorf("duplicate Bind rewrote existing binding: %v writes=%d", entries, h.app.Store.Writes()-writes)
				}
			}
			if !tt.HasText(copy.ShortcutsBind) {
				return
			}
			// Reach the retained draft recorder by its row geometry, even when
			// another saved row now displays the very same chord.
			row, _ := tt.Find(copy.ShortcutsBind)
			recorder, _ := tt.Find(shortcuts.Display("Ctrl+Shift+B"))
			tt.ClickAt(recorder.X+recorder.W/2, row.Y+row.H/2)
			tt.Frame()
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyR)
			tt.Frame()
			editingClick(t, tt, copy.ShortcutsBind)
			editingDump(t, tt, name+"-recovered", dark)
			if tt.HasText(copy.ShortcutsBind) || tt.HasText(warning) || h.live["Ctrl+Shift+R"] == nil {
				t.Error("retained draft cannot record a replacement and bind")
			}
		})
	}
}
