package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCustomCommandConversionReturnsFocus(t *testing.T) { checkCustomDraftExit(t, "convert") }
func TestCustomDraftCancelReturnsToAdd(t *testing.T)       { checkCustomDraftExit(t, "cancel") }

func TestCustomDraftBindReturnsToSavedRecorder(t *testing.T) { checkCustomStoredExit(t, "bind") }
func TestCustomSavedRemovalReturnsToAdd(t *testing.T)        { checkCustomStoredExit(t, "remove") }

func checkCustomStoredExit(t *testing.T, scenario string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = (*entries)[:1]
			if scenario == "bind" {
				*entries = nil
			}
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			if scenario == "bind" {
				if err := tt.Click(copy.ShortcutsAdd); err != nil {
					t.Fatal(err)
				}
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyR)
				optionExitFocus(t, tt, copy.ShortcutsBind)
			} else {
				optionExitFocus(t, tt, copy.ShortcutsRemove)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			recordingReturnDump(t, tt, "saved-"+scenario, dark)
			if scenario == "bind" {
				if len(*entries) != 1 || (*entries)[0].Key != "Ctrl+Shift+R" || (*entries)[0].Action != "plugin:clipboard" {
					t.Fatalf("binding failed: %+v", *entries)
				}
				if !tt.Focused(copy.ShortcutRecord) {
					t.Error("bound draft lost the saved recorder focus")
				}
			} else {
				if len(*entries) != 0 {
					t.Fatal("saved binding was not removed")
				}
				if !tt.Focused(copy.ShortcutsAdd) {
					t.Error("saved removal lost the Add outlet")
				}
			}
			tt.Key(0, ui.KeyEnter)
			if !a.customRecording {
				t.Error("Enter cannot continue after binding exit")
			}
			if scenario == "bind" && a.customRecordRow != 0 {
				t.Error("bound draft restarted the wrong recorder")
			}
		})
	}
}

func checkCustomDraftExit(t *testing.T, scenario string) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = (*entries)[:1]
			(*entries)[0].Action = "plugin:clipboard"
			if scenario == "cancel" {
				*entries = nil
			}
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			if scenario == "convert" {
				if err := tt.Click(copy.ShortcutsAction); err != nil {
					t.Fatal(err)
				}
				if err := tt.Click(copy.ShortcutsActions[len(copy.ShortcutsActions)-1].Label); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if !a.customDraftOpen || len(*entries) != 0 {
					t.Fatalf("command conversion did not create a draft: open=%v entries=%+v texts=%v", a.customDraftOpen, *entries, tt.Texts())
				}
			} else {
				if err := tt.Click(copy.ShortcutsAdd); err != nil {
					t.Fatal(err)
				}
				tt.Key(ui.Ctrl|ui.Shift, ui.KeyR)
				optionExitFocus(t, tt, copy.ShortcutsRemove)
				tt.Key(0, ui.KeyEnter)
				if a.customDraftOpen {
					t.Fatal("draft cancel did not close the draft")
				}
			}
			tt.Frame()
			recordingReturnDump(t, tt, "draft-"+scenario, dark)
			if scenario == "convert" {
				if !tt.Focused(copy.ShortcutsCommand) {
					t.Error("command conversion lost its input focus")
				}
				tt.Type("say converted")
				if a.customCommand != "say converted" {
					t.Errorf("converted command cannot type: %q", a.customCommand)
				}
			} else {
				if !tt.Focused(copy.ShortcutsAdd) {
					t.Error("draft cancel lost the Add outlet")
				}
				tt.Key(0, ui.KeyEnter)
				if !a.customDraftOpen || !a.customRecording {
					t.Error("Enter cannot add again after cancel")
				}
			}
			if len(*entries) != 0 {
				t.Fatal("exit unexpectedly saved a binding")
			}
		})
	}
}
