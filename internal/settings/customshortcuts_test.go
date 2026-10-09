package settings

import (
	"runtime"
	"testing"
)

func TestCustomShortcutsNormalize(t *testing.T) {
	// A draft (no key or no action) is not a binding, and of two entries
	// claiming one key the first wins — including across spellings.
	// `CmdOrCtrl` is the modifier this platform binds, so which of these two
	// collides depends on the host: on macOS it is the same key as Cmd.
	entries := NormalizeCustomShortcuts([]CustomShortcut{
		{Key: "CmdOrCtrl+Shift+P", Action: "plugin:clipboard"},
		{Key: "Cmd+Shift+P", Action: "action:toggle_window"},
		{Key: "  ", Action: "plugin:browser"},
		{Key: "Alt+X", Action: "   "},
		{Key: "Alt+Y", Action: "say done"},
	})
	want := 3
	if runtime.GOOS == "darwin" {
		// CmdOrCtrl is Cmd there, so the second entry collides with the first.
		want = 2
	}
	if len(entries) != want {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Key != "CmdOrCtrl+Shift+P" || entries[0].Action != "plugin:clipboard" {
		t.Errorf("first = %+v", entries[0])
	}
	if entries[len(entries)-1].Action != "say done" {
		t.Errorf("last = %+v", entries[len(entries)-1])
	}
}

func TestCustomShortcutsRoundTrip(t *testing.T) {
	s := Default()
	if got := CustomShortcutsOf(s); got != nil {
		t.Errorf("defaults = %+v", got)
	}

	s.SetCustomShortcuts([]CustomShortcut{
		{Key: "Cmd+Shift+P", Action: "plugin:clipboard"},
		{Key: "Cmd+Alt+T", Action: "action:open_external_terminal"},
		{Key: "Alt+R", Action: "rg --files | head"},
	})
	encoded := mustEncode(t, s)
	parsed := mustParse(t, encoded)
	got := CustomShortcutsOf(parsed)
	if len(got) != 3 || got[2].Action != "rg --files | head" {
		t.Fatalf("round trip = %+v", got)
	}

	// A hand-written list of the wrong shape is ignored, not fatal.
	broken, err := Parse([]byte(`{"custom_shortcuts": {"key": "x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := CustomShortcutsOf(broken); got != nil {
		t.Errorf("a broken list = %+v", got)
	}
}

func TestClassifyCustomShortcut(t *testing.T) {
	cases := map[string]struct{ kind, value string }{
		"plugin:clipboard":         {"plugin", "clipboard"},
		"action:toggle_window":     {"action", "toggle_window"},
		"  action:open_settings  ": {"action", "open_settings"},
		"say done":                 {"command", "say done"},
		"":                         {"command", ""},
	}
	for action, want := range cases {
		kind, value := ClassifyCustomShortcut(action)
		if kind != want.kind || value != want.value {
			t.Errorf("ClassifyCustomShortcut(%q) = %q, %q; want %q, %q", action, kind, value, want.kind, want.value)
		}
	}
}
