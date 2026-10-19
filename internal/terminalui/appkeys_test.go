package terminalui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/shortcuts"
)

// capturedOptions starts one session against a fake constructor and returns
// the options the app handed the plugin.
func capturedOptions(t *testing.T, store *settings.Store) terminal.Options {
	t.Helper()
	var opts terminal.Options
	a := New(store, Actions{}, func(o terminal.Options) (*terminal.Terminal, error) {
		opts = o
		return &terminal.Terminal{}, nil
	})
	if _, err := a.startSession("", nil); err != nil {
		t.Fatalf("startSession: %v", err)
	}
	return opts
}

// findBind returns the binding of a key.
func findBind(binds []terminal.Keybind, mods ui.Modifiers, key ui.Key) (terminal.Keybind, bool) {
	for _, b := range binds {
		if b.Mods == mods && b.Key == key {
			return b, true
		}
	}
	return terminal.Keybind{}, false
}

// The app's own copy and paste shortcuts are bound on the terminal, which is
// the only seam that can reach the selection: the port shipped both settings
// with no reader, so a rebind did nothing at all.
func TestTheTerminalAnswersTheAppsCopyAndPaste(t *testing.T) {
	store := settings.NewStore(settings.Default())
	s := store.Snapshot()
	opts := capturedOptions(t, store)

	mods, key, ok := shortcuts.Parse(settings.Shortcut(s, settings.ShortcutCopySelection))
	if !ok {
		t.Fatalf("the copy default does not parse: %q", settings.Shortcut(s, settings.ShortcutCopySelection))
	}
	copyBind, ok := findBind(opts.Keybinds, mods, key)
	if !ok {
		t.Fatalf("copy_selection %q is not bound on the terminal", settings.Shortcut(s, settings.ShortcutCopySelection))
	}
	if copyBind.Action != terminal.ActionCopy {
		t.Errorf("copy_selection is bound to action %v, want ActionCopy", copyBind.Action)
	}
	// The reference claims the copy key only when there is a selection, so a
	// Ctrl+C binding still interrupts the foreground process.
	if !copyBind.Performable {
		t.Error("copy_selection must be performable, or Ctrl+C never reaches the program")
	}

	mods, key, ok = shortcuts.Parse(settings.Shortcut(s, settings.ShortcutPaste))
	if !ok {
		t.Fatalf("the paste default does not parse: %q", settings.Shortcut(s, settings.ShortcutPaste))
	}
	pasteBind, ok := findBind(opts.Keybinds, mods, key)
	if !ok {
		t.Fatalf("paste %q is not bound on the terminal", settings.Shortcut(s, settings.ShortcutPaste))
	}
	if pasteBind.Action != terminal.ActionPaste {
		t.Errorf("paste is bound to action %v, want ActionPaste", pasteBind.Action)
	}
	if pasteBind.Performable {
		t.Error("the reference claimed paste unconditionally, so it is not performable")
	}
}

// A rebind in the settings page reaches the terminal, and the key it replaced
// is gone: the plugin's own defaults for that key are replaced, not shadowed.
func TestAReboundCopyReachesTheTerminal(t *testing.T) {
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) {
		s.SetExtra("shortcuts", map[string]any{settings.ShortcutCopySelection: "Alt+J"})
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	opts := capturedOptions(t, store)

	bind, ok := findBind(opts.Keybinds, ui.Alt, ui.KeyJ)
	if !ok {
		t.Fatal("the rebound copy shortcut is not bound on the terminal")
	}
	if bind.Action != terminal.ActionCopy || !bind.Performable {
		t.Errorf("Alt+J = %v performable=%v, want ActionCopy performable", bind.Action, bind.Performable)
	}
	if _, ok := findBind(opts.Keybinds, ui.Cmd|ui.Shift, ui.KeyC); ok {
		t.Error("the old copy default is still bound")
	}
}

// A binding the platform cannot parse falls back to the shipped one rather
// than to nothing: the terminal always has a way to copy.
func TestAnUnparseableCopyKeepsTheShippedKey(t *testing.T) {
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) {
		s.SetExtra("shortcuts", map[string]any{settings.ShortcutCopySelection: "Hyper+Q"})
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	opts := capturedOptions(t, store)
	wantMods, wantKey, ok := shortcuts.Parse(settings.Shortcut(settings.Default(), settings.ShortcutCopySelection))
	if !ok {
		t.Fatal("the shipped copy default does not parse")
	}
	if _, ok := findBind(opts.Keybinds, wantMods, wantKey); !ok {
		t.Errorf("no copy binding for the shipped key %q",
			settings.Shortcut(settings.Default(), settings.ShortcutCopySelection))
	}
}
