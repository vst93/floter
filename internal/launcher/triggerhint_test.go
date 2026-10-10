package launcher

import (
	"testing"

	"floter/internal/extensions"
)

func hintApp() *App {
	a := testApp()
	a.Commands = []extensions.CommandEntry{
		{Command: extensions.Command{ID: "browser", Name: "Browser history"}},
		{Command: extensions.Command{ID: "clipboard", Name: "Clipboard history", Aliases: []string{"clip"}}},
	}
	return a
}

// The nudge appears only for a whole single word that prefixes an enabled
// command's trigger, and it names the command; a space then enters the mode.
func TestTriggerHintWord(t *testing.T) {
	a := hintApp()
	cases := []struct {
		query string
		ok    bool
		name  string
		count int
	}{
		{"b", true, "Browser history", 1},
		{"br", true, "Browser history", 1},
		{"clip", true, "Clipboard history", 1},
		{"browser", true, "Browser history", 1},
		{"", false, "", 0},
		{"b ", false, "", 0},        // whitespace is the entry, not a nudge
		{"browser x", false, "", 0}, // a phrase
		{"zz", false, "", 0},
	}
	for _, c := range cases {
		a.Query = c.query
		hint, ok := a.externalTriggerHint(c.query)
		if ok != c.ok {
			t.Errorf("hint(%q) ok = %v, want %v", c.query, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if got := commandDisplayName(hint.entry); got != c.name {
			t.Errorf("hint(%q) names %q, want %q", c.query, got, c.name)
		}
		if hint.count != c.count {
			t.Errorf("hint(%q) count = %d, want %d", c.query, hint.count, c.count)
		}
	}
	// A word two commands share counts both, and the text says "…and more".
	a.Commands = append(a.Commands, extensions.CommandEntry{
		Command: extensions.Command{ID: "browser-tabs", Name: "Browser tabs", Aliases: []string{"br"}},
	})
	a.Query = "b"
	hint, ok := a.externalTriggerHint("b")
	if !ok || hint.count != 2 {
		t.Fatalf("shared prefix: %+v ok=%v", hint, ok)
	}
	single := a.copy().TriggerHint(commandDisplayName(hint.entry))
	if text := a.triggerHintText(); text == "" || text == single {
		t.Errorf("the two-command nudge did not say more: %q (single %q)", text, single)
	}
}

// The nudge is the ordinary page's alone: a mode that owns the field has no
// word left to enter with.
func TestTriggerHintIsTheOrdinaryPageOnly(t *testing.T) {
	a := hintApp()
	a.Query = "b"
	if a.triggerHintText() == "" {
		t.Fatal("no nudge on the ordinary page")
	}
	a.browser = true
	if got := a.triggerHintText(); got != "" {
		t.Errorf("the browser mode nudged: %q", got)
	}
	a.browser = false
	a.mode = &a.Commands[0]
	if got := a.triggerHintText(); got != "" {
		t.Errorf("a command mode nudged: %q", got)
	}
}
