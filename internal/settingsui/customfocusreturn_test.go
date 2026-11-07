package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCustomCommandConversionReturnsFocus(t *testing.T) { checkCustomDraftExit(t, "convert") }
func TestCustomDraftCancelReturnsToAdd(t *testing.T)       { checkCustomDraftExit(t, "cancel") }

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
