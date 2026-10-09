package settings

import (
	"runtime"
	"testing"
)

func TestShortcutMapDefaultsAndOverrides(t *testing.T) {
	m := ShortcutsOf(Default())
	for _, action := range ShortcutActions {
		if m[action] == "" {
			t.Errorf("%s has no default binding", action)
		}
	}
	app := "Ctrl"
	if runtime.GOOS == "darwin" {
		app = "Cmd"
	}
	if got := m[ShortcutNewCommand]; got != app+"+W" {
		t.Errorf("new command = %q, want %q", got, app+"+W")
	}
	if got := m[ShortcutOpenSettings]; got != app+"+," {
		t.Errorf("open settings = %q", got)
	}
	// Off macOS the terminal's copy and paste keep the shifted keys, so
	// Ctrl+C stays the interrupt.
	if runtime.GOOS != "darwin" {
		if m[ShortcutCopySelection] != "Ctrl+Shift+C" || m[ShortcutPaste] != "Ctrl+Shift+V" {
			t.Errorf("copy/paste = %q / %q", m[ShortcutCopySelection], m[ShortcutPaste])
		}
	}

	// A stored binding overrides the default; the old build's spellings are
	// normalized on the way in.
	s, err := Parse([]byte(`{"shortcuts": {"new_command": "CmdOrCtrl+Shift+N", "open_settings": "Cmd+Comma", "nonsense": "Cmd+X"}}`))
	if err != nil {
		t.Fatal(err)
	}
	m = ShortcutsOf(s)
	if m[ShortcutNewCommand] != "CmdOrCtrl+Shift+N" {
		t.Errorf("stored new command = %q", m[ShortcutNewCommand])
	}
	if m[ShortcutOpenSettings] != "Cmd+," {
		t.Errorf("stored open settings = %q", m[ShortcutOpenSettings])
	}
	if _, ok := m["nonsense"]; ok {
		t.Error("an unknown action was kept")
	}

	// A binding this build cannot parse falls back to the default rather
	// than leaving the action with no key.
	s, err = Parse([]byte(`{"shortcuts": {"new_command": "Hyper+Q"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := ShortcutsOf(s)[ShortcutNewCommand]; got != app+"+W" {
		t.Errorf("an unparseable binding = %q", got)
	}
}

func TestSetShortcutRefusesJunk(t *testing.T) {
	s := Default()
	if !s.SetShortcut(ShortcutNewCommand, "Cmd+Shift+J") {
		t.Fatal("a valid binding was refused")
	}
	if got := Shortcut(s, ShortcutNewCommand); got != "Cmd+Shift+J" {
		t.Errorf("binding = %q", got)
	}
	if s.SetShortcut("nonsense", "Cmd+J") {
		t.Error("an unknown action was accepted")
	}
	if s.SetShortcut(ShortcutNewCommand, "not a key") {
		t.Error("an unparseable accelerator was accepted")
	}
	if got := Shortcut(s, ShortcutNewCommand); got != "Cmd+Shift+J" {
		t.Errorf("a refused write changed the binding: %q", got)
	}
}

func TestSelectResultDigit(t *testing.T) {
	s := Default()
	app := "Ctrl"
	if runtime.GOOS == "darwin" {
		app = "Cmd"
	}
	if got := SelectResultDigit(s, 0); got != app+"+0" {
		t.Errorf("the tenth digit = %q", got)
	}
	if got := SelectResultDigit(s, 7); got != app+"+7" {
		t.Errorf("the seventh digit = %q", got)
	}
	// Rebinding the family moves every digit with it.
	s.SetShortcut(ShortcutSelectResult, "Alt+Shift+1")
	if got := SelectResultDigit(s, 3); got != "Alt+Shift+3" {
		t.Errorf("after rebinding = %q", got)
	}
	if got := SelectResultDigit(s, 42); got != "" {
		t.Errorf("an out-of-range digit = %q", got)
	}
}
