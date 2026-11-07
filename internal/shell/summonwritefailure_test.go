package shell

import (
	"floter/internal/settings"
	"os"
	"testing"
)

func TestSummonWriteFailurePreservesOldKeyAndDisplay(t *testing.T) {
	a, live, path, _ := shortcutWriteFailureApp(t)
	a.setShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+K")
	if a.Store.LastError() == nil {
		t.Fatal("expected a refused write")
	}
	if a.summonKey != "Ctrl+Alt+J" || a.Settings.Shortcut != "Ctrl+Alt+J" {
		t.Errorf("failed summon save changed active/displayed key: %q / %q", a.summonKey, a.Settings.Shortcut)
	}
	if live["Ctrl+Alt+J"] == nil || live["Ctrl+Alt+K"] != nil {
		t.Errorf("failed summon save changed system registration: %v", keysOf(live))
	}
	if a.shortcutWasRefused(settings.ShortcutToggleWindow) {
		t.Error("disk failure was mislabeled as an occupied key")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	a.setShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+K")
	if a.Store.LastError() != nil || a.summonKey != "Ctrl+Alt+K" || a.Settings.Shortcut != "Ctrl+Alt+K" {
		t.Fatal("summon retry failed")
	}
	if live["Ctrl+Alt+J"] != nil || live["Ctrl+Alt+K"] == nil {
		t.Errorf("successful retry left stale system registration: %v", keysOf(live))
	}
}
