package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/shortcuts"
	"github.com/egoist/mygo/ui"
)

func ownershipDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IY_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-iy/" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func ownershipClick(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Frame()
}

func ownershipChoose(t *testing.T, tt *ui.Tester, label string, last bool) {
	t.Helper()
	ownershipClick(t, tt, label)
	tt.Key(0, ui.KeyDown)
	if last {
		tt.Key(0, ui.KeyEnd)
	} else {
		tt.Key(0, ui.KeyHome)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
}

func TestCustomCommandLiteralDoesNotBecomeMode(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = (*entries)[1:2]
			tt := customStateFrame(t, a, dark)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("command")
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			ownershipDump(t, tt, "literal-command", dark)
			if len(*entries) != 1 || (*entries)[0].Action != "command" || a.customDraftOpen {
				t.Errorf("literal command became a mode switch: entries=%v draft=%v", *entries, a.customDraftOpen)
			}
		})
	}
}

func TestCustomDraftCommandModeKeepsBindFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = nil
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			ownershipClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			ownershipChoose(t, tt, copy.ShortcutsAction, true)
			optionExitFocus(t, tt, copy.ShortcutsCommand)
			tt.Type("say draft")
			optionExitFocus(t, tt, copy.ShortcutsBind)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			tt.Frame()
			ownershipDump(t, tt, "command-bind-focus", dark)
			if len(*entries) != 1 || (*entries)[0].Action != "say draft" {
				t.Fatalf("bad binding: %v", *entries)
			}
			if !tt.Focused(copy.ShortcutRecord) {
				t.Error("command draft Bind lost saved recorder focus")
			}
		})
	}
}

func TestCustomDraftModeReturnKeepsText(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = nil
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			ownershipClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			ownershipChoose(t, tt, copy.ShortcutsAction, true)
			optionExitFocus(t, tt, copy.ShortcutsCommand)
			tt.Type("say draft")
			ownershipChoose(t, tt, copy.ShortcutsAction, false)
			ownershipChoose(t, tt, copy.ShortcutsAction, true)
			optionExitFocus(t, tt, copy.ShortcutsCommand)
			ownershipDump(t, tt, "mode-return", dark)
			if got := refreshRead(tt); got != "say draft" {
				t.Errorf("return lost draft: %q", got)
			}
		})
	}
}

func TestCustomRekeyKeepsRecorderFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = (*entries)[1:2]
			tt := customStateFrame(t, a, dark)
			ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+B"))
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			tt.Frame()
			tt.Frame()
			ownershipDump(t, tt, "rekey-focus", dark)
			if (*entries)[0].Key != "Ctrl+Shift+Z" {
				t.Fatal("key not changed")
			}
			if !tt.Focused(a.copy().ShortcutRecord) {
				t.Error("successful rekey moved focus away from recorder")
			}
		})
	}
}

func TestCustomSubmittedTextMatchesStoredCommand(t *testing.T) {
	for _, text := range []string{"  say changed  ", "  say original  ", ""} {
		t.Run(text, func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = (*entries)[1:2]
			tt := customStateFrame(t, a, false)
			optionExitFocus(t, tt, a.copy().ShortcutsCommand)
			tt.Key(ui.Cmd, ui.KeyA)
			if text == "" {
				tt.Key(0, ui.KeyBackspace)
			} else {
				tt.Type(text)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := refreshRead(tt); got != (*entries)[0].Action {
				t.Errorf("submitted display=%q stored=%q", got, (*entries)[0].Action)
			}
		})
	}
}

func TestCustomDuplicateCaptureKeepsRetryFocus(t *testing.T) {
	for _, draft := range []bool{false, true} {
		t.Run(map[bool]string{false: "saved", true: "draft"}[draft], func(t *testing.T) {
			a, _ := customStateApp(t)
			tt := customStateFrame(t, a, false)
			if draft {
				ownershipClick(t, tt, a.copy().ShortcutsAdd)
			} else {
				ownershipClick(t, tt, shortcuts.Display("Ctrl+Shift+B"))
			}
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyA)
			tt.Frame()
			tt.Frame()
			ownershipDump(t, tt, "duplicate-"+map[bool]string{false: "saved", true: "draft"}[draft], false)
			if !tt.HasText(a.copy().ShortcutsConflict) {
				t.Fatal("no duplicate")
			}
			if !tt.Focused(a.copy().ShortcutRecord) {
				t.Error("duplicate recording lost keyboard retry")
			}
			tt.Key(0, ui.KeyEnter)
			if !a.customRecording {
				t.Error("Enter cannot retry rejected recording")
			}
		})
	}
}

func TestCustomDraftRecreateDoesNotUndoOldCommand(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = nil
			tt := customStateFrame(t, a, dark)
			copy := a.copy()
			ownershipClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
			ownershipChoose(t, tt, copy.ShortcutsAction, true)
			optionExitFocus(t, tt, copy.ShortcutsCommand)
			tt.Type("say private")
			ownershipClick(t, tt, copy.ShortcutsRemove)
			ownershipClick(t, tt, copy.ShortcutsAdd)
			tt.Key(ui.Ctrl|ui.Shift, ui.KeyR)
			optionExitFocus(t, tt, copy.ShortcutsCommand)
			tt.Type("say new")
			tt.Key(ui.Cmd, ui.KeyZ)
			ownershipDump(t, tt, "recreate-undo", dark)
			if got := refreshRead(tt); got != "" {
				t.Errorf("fresh draft undo revived old command: %q", got)
			}
		})
	}
}

func TestCustomSavedPickerSelectionKeepsFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, entries := customStateApp(t)
			*entries = (*entries)[1:2]
			tt := customStateFrame(t, a, dark)
			ownershipChoose(t, tt, a.copy().ShortcutsAction, false)
			ownershipDump(t, tt, "saved-action-focus", dark)
			if (*entries)[0].Action != a.copy().ShortcutsActions[0].ID {
				t.Fatal("action not changed")
			}
			if !tt.Focused(a.copy().ShortcutsAction) {
				t.Error("saved action selection lost picker focus")
			}
		})
	}
}
