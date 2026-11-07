package shell

import (
	"errors"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settings"
)

func TestResetSystemRefusalKeepsConfirmedBindings(t *testing.T) {
	s := settings.Default()
	s.SetExtra("hotkey", "Ctrl+Alt+J")
	s.SetShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+J")
	s.SetShortcut(settings.ShortcutNewCommand, "Ctrl+Alt+N")
	live := map[string]func(){"Ctrl+Alt+J": func() {}}
	refuse := true
	a := New(Options{Store: settings.NewStore(s), Paths: extensions.FromRoot(t.TempDir()),
		RegisterShortcut: func(key string, fn func()) error {
			if refuse && key == settings.DefaultSummonShortcut {
				return errors.New("occupied")
			}
			live[key] = fn
			return nil
		},
		UnregisterShortcut: func(key string) { delete(live, key) },
	})
	a.Settings.Actions.ResetShortcuts()
	if SummonShortcut(a.Store.Snapshot()) != "Ctrl+Alt+J" || settings.Shortcut(a.Store.Snapshot(), settings.ShortcutNewCommand) != "Ctrl+Alt+N" {
		t.Errorf("system-refused reset persisted unregistered defaults: %v", a.Store.Snapshot().Extra())
	}
	if a.Store.Writes() != 0 || live["Ctrl+Alt+J"] == nil || live[settings.DefaultSummonShortcut] != nil || a.Settings.Shortcut != "Ctrl+Alt+J" {
		t.Errorf("system-refused reset changed confirmed registration: writes=%d live=%v display=%s", a.Store.Writes(), keysOf(live), a.Settings.Shortcut)
	}
	refuse = false
	a.Settings.Actions.ResetShortcuts()
	if SummonShortcut(a.Store.Snapshot()) != settings.DefaultSummonShortcut || a.ShortcutErr != nil || live[settings.DefaultSummonShortcut] == nil || live["Ctrl+Alt+J"] != nil {
		t.Fatal("reset could not retry after system recovery")
	}

	// The newly claimed default must be released if the subsequent file write
	// fails. The original key and the custom command remain usable.
	broken, held, _, _ := shortcutWriteFailureApp(t)
	broken.Settings.Actions.ResetShortcuts()
	if broken.Store.LastError() == nil || broken.Store.Writes() != 1 || held[settings.DefaultSummonShortcut] != nil || held["Ctrl+Alt+J"] == nil || held["Ctrl+Shift+B"] == nil || broken.Settings.Shortcut != "Ctrl+Alt+J" {
		t.Fatalf("disk-refused reset lost confirmed keys or leaked reserved default: held=%v writes=%d display=%s", keysOf(held), broken.Store.Writes(), broken.Settings.Shortcut)
	}
}
