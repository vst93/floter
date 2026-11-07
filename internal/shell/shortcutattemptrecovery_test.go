package shell

import (
	"errors"
	"floter/internal/settings"
	"os"
	"testing"
)

func TestSummonDiskFailureDoesNotRepeatOldSystemRefusal(t *testing.T) {
	a, live, path, _ := shortcutWriteFailureApp(t)
	register := a.registerShortcut
	a.registerShortcut = func(key string, fn func()) error {
		if key == "Ctrl+Alt+K" {
			return errors.New("occupied")
		}
		return register(key, fn)
	}
	a.Settings.Actions.SetShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+K")
	if !a.shortcutWasRefused(settings.ShortcutToggleWindow) {
		t.Fatal("expected initial OS refusal")
	}
	a.Settings.Actions.SetShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+L")
	if a.Store.LastError() == nil {
		t.Fatal("expected disk failure")
	}
	if a.shortcutWasRefused(settings.ShortcutToggleWindow) || a.ShortcutErr != nil {
		t.Error("new disk failure retained the previous system refusal")
	}
	if live["Ctrl+Alt+J"] == nil || live["Ctrl+Alt+L"] != nil {
		t.Fatal("disk failure lost original key")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	a.Settings.Actions.SetShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+L")
	if a.Store.LastError() != nil || live["Ctrl+Alt+L"] == nil {
		t.Fatal("retry failed")
	}
}
