package shell

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/extensions"
	"floter/internal/settings"
)

// P9-k: a chord the system refuses (another app holds it) used to leave
// nothing behind but a log line: the old key stayed registered, nothing was
// written, and both ShortcutErr and the page said all was well.
func TestASystemRefusedChordIsReportedAndChangesNothing(t *testing.T) {
	registered := []string{}
	failing := true
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:       store,
		Paths:       extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
		RegisterShortcut: func(accelerator string, fn func()) error {
			if failing {
				return errors.New("taken")
			}
			registered = append(registered, accelerator)
			return nil
		},
	})
	a.summonKey = "Ctrl+Space"
	a.Settings.ShortcutID = shortcutToggleWindow

	a.setShortcut(shortcutToggleWindow, "Cmd+Shift+G")
	if a.ShortcutErr == nil {
		t.Fatal("a refused chord left ShortcutErr nil")
	}
	if !a.shortcutWasRefused(shortcutToggleWindow) {
		t.Fatalf("refusal = %q, want the summon action", a.shortcutRefusal)
	}
	if a.summonKey != "Ctrl+Space" {
		t.Errorf("summon key = %q, want the old one", a.summonKey)
	}
	if got := store.Snapshot().Extra()["hotkey"]; got != nil {
		t.Errorf("hotkey = %v, want nothing written", got)
	}
	if len(registered) != 0 {
		t.Errorf("registered = %v", registered)
	}

	// The same chord once the other app lets go: registered, stored, and the
	// refusal gone.
	failing = false
	a.setShortcut(shortcutToggleWindow, "Cmd+Shift+G")
	if a.ShortcutErr != nil || a.shortcutWasRefused(shortcutToggleWindow) {
		t.Fatalf("after success: err = %v, refusal = %q", a.ShortcutErr, a.shortcutRefusal)
	}
	if got := store.Snapshot().Extra()["hotkey"]; got != "Cmd+Shift+G" {
		t.Errorf("hotkey = %v", got)
	}
}

// ApplySummonShortcut is the other door (start, and the settings coming back):
// it names the summon action itself rather than whatever id it was handed.
func TestASystemRefusedSummonShortcutNamesTheSummonAction(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(Options{
		Store:       store,
		Paths:       extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) { return nil, errors.New("no library in tests") },
		OpenAtLogin: func() bool { return false }, SetOpenAtLogin: func(bool) error { return nil },
		RegisterShortcut: func(string, func()) error { return errors.New("taken") },
	})
	// A key other than the one the settings name, so the call has work to do.
	a.summonKey = "Ctrl+Alt+F9"
	a.ApplySummonShortcut()
	if !a.shortcutWasRefused(settings.ShortcutToggleWindow) {
		t.Fatalf("refusal = %q, want %q", a.shortcutRefusal, settings.ShortcutToggleWindow)
	}
	if a.ShortcutErr == nil {
		t.Error("ShortcutErr is nil after a refused summon key")
	}
}
