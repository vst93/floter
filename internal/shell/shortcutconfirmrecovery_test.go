package shell

import (
	"errors"
	"floter/internal/settings"
	"testing"
)

func TestConfirmedSummonKeyClearsPreviousRefusal(t *testing.T) {
	a, live, _, _ := shortcutWriteFailureApp(t)
	a.registerShortcut = func(string, func()) error { return errors.New("occupied") }
	a.setShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+K")
	a.setShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+J")
	if a.ShortcutErr != nil || a.shortcutWasRefused(settings.ShortcutToggleWindow) {
		t.Error("confirmation of the working key retained stale refusal")
	}
	if a.Store.Writes() != 0 || live["Ctrl+Alt+J"] == nil {
		t.Fatal("confirming current key wrote or unbound it")
	}
	// A startup registration error is not a rejected change to a working key.
	// A no-op must not claim that an unavailable system key recovered.
	a.ShortcutErr = errors.New("startup registration failed")
	a.setShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+J")
	if a.ShortcutErr == nil {
		t.Fatal("confirming configured key hid a startup registration failure")
	}
}
