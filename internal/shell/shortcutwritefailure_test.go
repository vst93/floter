package shell

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settings"
)

func shortcutWriteFailureApp(t *testing.T) (*App, map[string]func(), string, *[]string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	s := settings.Default()
	s.SetCustomShortcuts([]settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "say original"}})
	s.SetExtra("hotkey", "Ctrl+Alt+J")
	s.SetShortcut(settings.ShortcutToggleWindow, "Ctrl+Alt+J")
	if err := settings.Save(path, s); err != nil {
		t.Fatal(err)
	}
	store, err := settings.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	live := map[string]func(){}
	commands := []string{}
	a := New(Options{Store: store, Paths: extensions.FromRoot(t.TempDir()),
		RegisterShortcut:   func(key string, fn func()) error { live[key] = fn; return nil },
		UnregisterShortcut: func(key string) { delete(live, key) },
		RunSilentCommand:   func(command string) error { commands = append(commands, command); return nil },
	})
	a.ApplyCustomShortcuts()
	a.summonKey = ""
	a.ApplySummonShortcut()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	return a, live, path, &commands
}

func TestCustomWriteFailureRestoresRegisteredKeys(t *testing.T) {
	for _, scenario := range []string{"add", "remove", "rerecord"} {
		t.Run(scenario, func(t *testing.T) {
			a, live, path, _ := shortcutWriteFailureApp(t)
			old := settings.CustomShortcutsOf(a.Store.Snapshot())
			next := append([]settings.CustomShortcut{}, old...)
			switch scenario {
			case "add":
				next = append(next, settings.CustomShortcut{Key: "Ctrl+Shift+Z", Action: "say new"})
			case "remove":
				next = nil
			case "rerecord":
				next[0].Key = "Ctrl+Shift+Z"
			}
			a.setCustomShortcuts(next)
			if a.Store.LastError() == nil || a.Store.Writes() != 1 {
				t.Fatal("expected one refused write")
			}
			if !slices.Equal(settings.CustomShortcutsOf(a.Store.Snapshot()), old) {
				t.Fatal("store did not roll back")
			}
			if got := keysOf(live); !slices.Equal(got, []string{"Ctrl+Alt+J", "Ctrl+Shift+B"}) {
				t.Errorf("failed %s left uncommitted system keys: %v", scenario, got)
			}
			if a.customSignature != customShortcutSignature(a.Store.Snapshot()) {
				t.Error("failed write left an uncommitted registration signature")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			a.setCustomShortcuts(next)
			if a.Store.LastError() != nil || !slices.Equal(settings.CustomShortcutsOf(a.Store.Snapshot()), next) {
				t.Fatal("retry did not store requested keys")
			}
			want := []string{"Ctrl+Alt+J"}
			for _, entry := range next {
				want = append(want, entry.Key)
			}
			slices.Sort(want)
			if !slices.Equal(keysOf(live), want) {
				t.Errorf("retry system keys = %v, want %v", keysOf(live), want)
			}
		})
	}
}

func TestCustomWriteFailureRestoresActionDispatch(t *testing.T) {
	a, live, path, commands := shortcutWriteFailureApp(t)
	a.setCustomShortcuts([]settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "say uncommitted"}})
	if a.Store.LastError() == nil {
		t.Fatal("expected a refused write")
	}
	if live["Ctrl+Shift+B"] == nil {
		t.Fatal("original command key disappeared")
	}
	live["Ctrl+Shift+B"]()
	if !slices.Equal(*commands, []string{"say original"}) {
		t.Errorf("failed command update dispatched uncommitted action: %v", *commands)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	a.setCustomShortcuts([]settings.CustomShortcut{{Key: "Ctrl+Shift+B", Action: "say accepted"}})
	if got := settings.CustomShortcutsOf(a.Store.Snapshot()); len(got) != 1 || got[0].Action != "say accepted" {
		t.Errorf("retry action = %v", got)
	}
}
