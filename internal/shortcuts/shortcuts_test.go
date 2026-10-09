package shortcuts

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"Alt+Space":             "Alt+Space",
		"Cmd+Comma":             "Cmd+,",
		"CmdOrCtrl+Shift+Space": "CmdOrCtrl+Shift+Space",
		"CommandOrControl+N":    "CmdOrCtrl+N",
		"Ctrl+Shift+Space":      "Ctrl+Shift+Space",
		"Option+Space":          "Alt+Space",
		"Super+Period":          "Cmd+.",
		"cmdorctrl+shift+t":     "CmdOrCtrl+Shift+T",
		"Cmd+Shift+Space":       "Cmd+Shift+Space",
		"Ctrl+Alt+Delete":       "Ctrl+Alt+Delete",
		"F5":                    "F5",
		"Cmd+F12":               "Cmd+F12",
		"Ctrl+BracketLeft":      "Ctrl+[",
		"Cmd+Semicolon":         "Cmd+;",
		"Alt+Quote":             "Alt+'",
	}
	for input, want := range cases {
		got, ok := Normalize(input)
		if !ok {
			t.Errorf("Normalize(%q) failed", input)
			continue
		}
		if got != want {
			t.Errorf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
	// The normalization is idempotent, and the modifier order is canonical.
	if got, _ := Normalize("Shift+Super+Space"); got != "Cmd+Shift+Space" {
		t.Errorf("modifier order = %q", got)
	}
	if got, _ := Normalize("Cmd+,"); got != "Cmd+," {
		t.Errorf("idempotent = %q", got)
	}

	for _, input := range []string{"", "   ", "Cmd", "Cmd+", "Cmd+A+B", "Hyper+X", "Cmd+Unknown"} {
		if got, ok := Normalize(input); ok {
			t.Errorf("Normalize(%q) = %q, want a rejection", input, got)
		}
	}
}

func TestFromKey(t *testing.T) {
	cases := []struct {
		mods ui.Modifiers
		key  ui.Key
		want string
	}{
		{ui.Super, ui.KeySpace, "Cmd+Space"},
		{ui.Ctrl, ui.KeySpace, "Ctrl+Space"},
		{ui.Super | ui.Shift, ui.KeyS, "Cmd+Shift+S"},
		{ui.Ctrl | ui.Alt, ui.KeyComma, "Ctrl+Alt+,"},
		{ui.Super, ui.KeyF5, "Cmd+F5"},
		{ui.Alt, ui.KeyEscape, "Alt+Esc"},
		{ui.Super, ui.Key0, "Cmd+0"},
	}
	for _, tc := range cases {
		got, ok := FromKey(tc.mods, tc.key)
		if !ok {
			t.Errorf("FromKey(%v, %v) failed", tc.mods, tc.key)
			continue
		}
		if got != tc.want {
			t.Errorf("FromKey(%v, %v) = %q, want %q", tc.mods, tc.key, got, tc.want)
		}
		// What the recorder produces normalizes to itself.
		if normalized, ok := Normalize(got); !ok || normalized != got {
			t.Errorf("FromKey(%v, %v) = %q, which does not normalize: %q", tc.mods, tc.key, got, normalized)
		}
	}

	// A bare key, or Shift alone, is not a shortcut.
	if _, ok := FromKey(0, ui.KeyS); ok {
		t.Error("a bare key was accepted")
	}
	if _, ok := FromKey(ui.Shift, ui.KeyS); ok {
		t.Error("shift alone was accepted")
	}
	if _, ok := FromKey(ui.Super, ui.KeyUnknown); ok {
		t.Error("an unknown key was accepted")
	}
}

func TestDisplay(t *testing.T) {
	if got := Display("Cmd+Comma"); got != "Cmd + ," {
		t.Errorf("Display = %q", got)
	}
	if got := Display("not a shortcut"); got != "not a shortcut" {
		t.Errorf("Display of junk = %q", got)
	}
}

func TestEqualAndDuplicate(t *testing.T) {
	// The two spellings of one binding are the same key.
	for _, pair := range [][2]string{
		{"Cmd+Comma", "Cmd+,"},
		{"Alt+Space", "Alt+Space"},
	} {
		if !Equal(pair[0], pair[1]) {
			t.Errorf("Equal(%q, %q) = false", pair[0], pair[1])
		}
	}
	// `CmdOrCtrl` is the modifier the platform binds: the same key as Cmd on
	// macOS and as Ctrl elsewhere.
	portable := "CmdOrCtrl+Shift+P"
	if runtime.GOOS == "darwin" {
		if !Equal(portable, "Cmd+Shift+P") || Equal(portable, "Ctrl+Shift+P") {
			t.Error("CmdOrCtrl did not resolve to Cmd on macOS")
		}
	} else {
		if !Equal(portable, "Ctrl+Shift+P") || Equal(portable, "Cmd+Shift+P") {
			t.Error("CmdOrCtrl did not resolve to Ctrl off macOS")
		}
	}
	if Equal("Cmd+A", "Cmd+B") {
		t.Error("two different keys compared equal")
	}
	// A key neither side can parse compares as trimmed text.
	if !Equal("not a key", " NOT A KEY ") {
		t.Error("unparseable keys did not compare by text")
	}

	keys := []string{"Cmd+Comma", "Alt+Space"}
	if got := Duplicate("Cmd+,", keys, -1); got != "Cmd+Comma" {
		t.Errorf("Duplicate = %q", got)
	}
	if got := Duplicate("Cmd+,", keys, 0); got != "" {
		t.Errorf("Duplicate ignoring its own row = %q", got)
	}
	if got := Duplicate("", keys, -1); got != "" {
		t.Errorf("an empty key = %q", got)
	}
}

func TestParseAndMatch(t *testing.T) {
	cases := []struct {
		accelerator string
		mods        ui.Modifiers
		key         ui.Key
	}{
		{"Cmd+G", ui.Super, ui.KeyG},
		{"Ctrl+Shift+C", ui.Ctrl | ui.Shift, ui.KeyC},
		{"Alt+Space", ui.Alt, ui.KeySpace},
		{"Cmd+,", ui.Super, ui.KeyComma},
		{"Cmd+1", ui.Super, ui.Key1},
		{"F5", 0, ui.KeyF5},
		{"Cmd+Up", ui.Super, ui.KeyUp},
	}
	for _, tc := range cases {
		mods, key, ok := Parse(tc.accelerator)
		if !ok || mods != tc.mods || key != tc.key {
			t.Errorf("Parse(%q) = %v, %v, %v", tc.accelerator, mods, key, ok)
		}
		if !Match(tc.accelerator, tc.mods, tc.key) {
			t.Errorf("Match(%q) = false", tc.accelerator)
		}
	}
	// The portable modifier resolves to the platform's.
	mods, key, ok := Parse("CmdOrCtrl+1")
	if !ok || key != ui.Key1 {
		t.Fatalf("Parse(CmdOrCtrl+1) = %v, %v, %v", mods, key, ok)
	}
	if runtime.GOOS == "darwin" && mods != ui.Super {
		t.Errorf("CmdOrCtrl = %v on macOS", mods)
	}
	if runtime.GOOS != "darwin" && mods != ui.Ctrl {
		t.Errorf("CmdOrCtrl = %v off macOS", mods)
	}

	// A binding without Shift accepts the letter either way; one with Shift
	// requires it.
	if !Match("Cmd+G", ui.Super|ui.Shift, ui.KeyG) {
		t.Error("a shifted letter did not match a binding without Shift")
	}
	if Match("Cmd+Shift+G", ui.Super, ui.KeyG) {
		t.Error("a binding with Shift matched an unshifted letter")
	}
	if Match("Cmd+G", ui.Ctrl, ui.KeyG) {
		t.Error("the wrong modifier matched")
	}
	if Match("Cmd+G", ui.Super, ui.KeyH) {
		t.Error("the wrong key matched")
	}
	if _, _, ok := Parse("not an accelerator"); ok {
		t.Error("junk parsed")
	}
}
