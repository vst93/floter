package settingsui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func checkCustomSaveFailure(t *testing.T, scenario string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			s := settings.Default()
			s.Language, s.Theme = "zh", appearanceName(dark)
			entries := []settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "say original"}}
			if scenario == "add" {
				entries = nil
			}
			if scenario == "mode" {
				entries[0].Action = "plugin:clipboard"
			}
			s.SetCustomShortcuts(entries)
			if err := settings.Save(path, s); err != nil {
				t.Fatal(err)
			}
			store, err := settings.OpenStore(path)
			if err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{
				CustomShortcuts: func() []settings.CustomShortcut { return settings.CustomShortcutsOf(store.Snapshot()) },
				// The shell reports OS registration failures separately; a disk
				// failure rolls back the Store and currently returns no rejection.
				SetCustomShortcuts: func(list []settings.CustomShortcut) []CustomShortcutRejection {
					_ = store.Update(func(s *settings.Settings) { s.SetCustomShortcuts(list) })
					return nil
				},
			})
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			switch scenario {
			case "add":
				if err := tt.Click(copy.ShortcutsAdd); err != nil {
					t.Fatal(err)
				}
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
				optionExitFocus(t, tt, copy.ShortcutsAction)
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnd)
				tt.Key(0, ui.KeyEnter)
				optionExitFocus(t, tt, copy.ShortcutsCommand)
				tt.Type("say unfinished")
				optionExitFocus(t, tt, copy.ShortcutsBind)
			case "mode":
				optionExitFocus(t, tt, copy.ShortcutsAction)
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnd)
			default:
				optionExitFocus(t, tt, copy.ShortcutsCommand)
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("say unfinished")
				if scenario == "remove" {
					optionExitFocus(t, tt, copy.ShortcutsRemove)
				}
				if scenario == "record" {
					optionExitFocus(t, tt, copy.ShortcutRecord)
					tt.Key(0, ui.KeyEnter)
				}
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if scenario == "record" {
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			} else {
				tt.Key(0, ui.KeyEnter)
			}
			tt.Frame()
			tt.Frame()
			residencyRefreshDump(t, tt, "shortcut-failure-"+scenario, dark)
			if store.LastError() == nil || store.Writes() != 1 {
				t.Fatalf("expected exactly one failed write, got %d / %v", store.Writes(), store.LastError())
			}
			if !a.SaveFailedVisible() {
				t.Error("shortcut disk failure has no visible save feedback")
			}
			if !slices.Equal(a.customShortcuts(), entries) {
				t.Errorf("failed %s changed the stored bindings", scenario)
			}
			switch scenario {
			case "add":
				if !a.customDraftOpen || a.customCommand != "say unfinished" || a.customKey == "" {
					t.Errorf("failed add discarded draft: open=%v key=%q command=%q", a.customDraftOpen, a.customKey, a.customCommand)
				}
			case "mode":
				if a.customDraftOpen {
					t.Error("failed command-mode switch duplicated saved binding as draft")
				}
				if !tt.Focused(copy.ShortcutsAction) {
					t.Error("failed mode switch lost original picker")
				}
			default:
				if a.customEdits["Ctrl+Shift+B"] != "say unfinished" {
					t.Errorf("failed %s discarded command draft: %v", scenario, a.customEdits)
				}
				label := copy.ShortcutsCommand
				if scenario == "remove" {
					label = copy.ShortcutsRemove
				}
				if scenario == "record" {
					label = copy.ShortcutRecord
				}
				if !tt.Focused(label) {
					t.Errorf("failed %s lost original keyboard outlet", scenario)
				}
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if scenario == "add" && a.customDraftOpen {
				optionExitFocus(t, tt, copy.ShortcutsBind)
				tt.Key(0, ui.KeyEnter)
				got := a.customShortcuts()
				if len(got) != 1 || got[0].Action != "say unfinished" || a.customDraftOpen {
					t.Error("failed add cannot retry its retained command")
				}
			} else if scenario == "command" {
				tt.Key(0, ui.KeyEnter)
				if a.customShortcuts()[0].Action != "say unfinished" {
					t.Error("failed command cannot retry its retained edit")
				}
			} else if scenario == "mode" {
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyEnd)
				tt.Key(0, ui.KeyEnter)
				if !a.customDraftOpen || len(a.customShortcuts()) != 0 {
					t.Error("failed mode switch could not retry")
				}
			} else if scenario == "remove" {
				tt.Key(0, ui.KeyEnter)
				if len(a.customShortcuts()) != 0 || !tt.Focused(copy.ShortcutsAdd) {
					t.Error("failed remove could not retry and return to Add")
				}
			} else if scenario == "record" {
				tt.Key(0, ui.KeyEnter)
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
				got := a.customShortcuts()
				if len(got) != 1 || got[0].Key == "Ctrl+Shift+B" || got[0].Action != "say original" {
					t.Error("failed recording could not retry its original action")
				}
			}
		})
	}
}

func TestCustomSaveFailureAdd(t *testing.T)     { checkCustomSaveFailure(t, "add") }
func TestCustomSaveFailureCommand(t *testing.T) { checkCustomSaveFailure(t, "command") }
