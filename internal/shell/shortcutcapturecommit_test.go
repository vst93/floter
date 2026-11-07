package shell

import (
	"testing"

	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestShortcutEditingConversionKeepsCapture(t *testing.T) { checkShortcutEditing(t, "convert") }
func TestShortcutEditingBindKeepsCapture(t *testing.T)       { checkShortcutEditing(t, "bind") }

func TestShortcutDraftBindKeepsAcceptedClaim(t *testing.T) {
	h := newShortcutLifecycle(t, "Ctrl+Alt+J")
	tt := h.frame(false)
	copy := i18n.For("en").Settings
	editingClick(t, tt, copy.ShortcutsAdd)
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
	tt.Frame()
	lifecycleRecord(t, h, tt, "Ctrl+Shift+Z")
	before := h.claims["Ctrl+Shift+Z"]
	h.limit["Ctrl+Shift+Z"] = before + 1
	editingClick(t, tt, copy.ShortcutsBind)
	editingDump(t, tt, "draft-bind-single-claim", false)
	if h.claims["Ctrl+Shift+Z"] != before+1 || h.live["Ctrl+Shift+Z"] == nil {
		t.Errorf("Bind released accepted chord and lost it on reclaim: claims=%d live=%v", h.claims["Ctrl+Shift+Z"]-before, h.live["Ctrl+Shift+Z"] != nil)
	}
	if h.app.shortcutsSuspended || tt.HasText(copy.ShortcutRecording) || tt.HasText(copy.ShortcutsOccupied) {
		t.Error("accepted Bind did not finish cleanly")
	}
}

func TestShortcutEditingDoesNotDispatchDuringValidation(t *testing.T) {
	h := newShortcutLifecycle(t, "Ctrl+Alt+J")
	tt := h.frame(false)
	copy := i18n.For("en").Settings
	var commands []string
	h.app.silentCommand = func(command string) error { commands = append(commands, command); return nil }
	lifecycleRecord(t, h, tt, "Ctrl+Alt+J")
	register := h.app.registerShortcut
	h.app.registerShortcut = func(key string, fn func()) error {
		err := register(key, fn)
		// Deliver the native callback in the registration/save interval, before
		// the explicit editor releases temporary validation claims.
		if err == nil && key == "Ctrl+Shift+B" && h.app.Settings.RecordingShortcut() {
			fn()
		}
		return err
	}
	// Binding a draft is an actual page event; the existing saved command
	// must be validated too while the summon recorder still owns keys.
	editingClick(t, tt, copy.ShortcutsAdd)
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyZ)
	tt.Frame()
	lifecycleRecord(t, h, tt, "Ctrl+Alt+J")
	editingClick(t, tt, copy.ShortcutsBind)
	if len(commands) != 0 {
		t.Errorf("validation dispatched command under recorder: %v", commands)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.Frame()
	if fn := h.live["Ctrl+Shift+B"]; fn != nil {
		fn()
	} else {
		t.Fatal("confirmed command not restored")
	}
	if len(commands) != 1 || commands[0] != "say original" {
		t.Errorf("resume did not restore command dispatch: %v", commands)
	}
}
