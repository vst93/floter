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

// `select_result` is a family: the stored binding is reduced to its head (the
// user's modifiers and the digit `1`) when the map is read, so every digit in
// the family is drawn from the same modifiers. A stored binding with no
// modifier at all is refused and the shipped default stands, so a bare digit
// can never launch a row on ordinary number entry — the old build's
// `withShortcutDefaults` + `normalizeResultShortcut`.
func TestSelectResultStoredBindingIsReducedToItsHead(t *testing.T) {
	s, err := Parse([]byte(`{"shortcuts": {"select_result": "Alt+Shift+7"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := Shortcut(s, ShortcutSelectResult); got != "Alt+Shift+1" {
		t.Errorf("a stored family member = %q, want Alt+Shift+1", got)
	}
	if got := SelectResultDigit(s, 4); got != "Alt+Shift+4" {
		t.Errorf("the fourth digit = %q", got)
	}

	// A modifier-less binding is refused, so the shipped default stands.
	s, err = Parse([]byte(`{"shortcuts": {"select_result": "7"}}`))
	if err != nil {
		t.Fatal(err)
	}
	app := "Ctrl"
	if runtime.GOOS == "darwin" {
		app = "Cmd"
	}
	if got := Shortcut(s, ShortcutSelectResult); got != app+"+1" {
		t.Errorf("a modifier-less binding = %q, want the default", got)
	}
	if got := SelectResultDigit(s, 3); got != app+"+3" {
		t.Errorf("the third digit after a refusal = %q", got)
	}

	// SetShortcut reduces on the way in, and refuses a modifier-less write.
	if !s.SetShortcut(ShortcutSelectResult, "Cmd+Shift+5") {
		t.Fatal("a valid family member was refused")
	}
	if got := Shortcut(s, ShortcutSelectResult); got != "Cmd+Shift+1" {
		t.Errorf("a written family member = %q, want Cmd+Shift+1", got)
	}
	if s.SetShortcut(ShortcutSelectResult, "5") {
		t.Error("a modifier-less binding was accepted")
	}
	if got := Shortcut(s, ShortcutSelectResult); got != "Cmd+Shift+1" {
		t.Errorf("a refused write changed the binding: %q", got)
	}
}
