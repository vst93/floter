package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

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

// The nudge is dropped when the box the row leaves it falls under six cells
// of its own font — not when the query field falls under a fixed pixel floor.
// The box is what the row gives: the text's own width, capped by the room.
func TestTriggerHintFits(t *testing.T) {
	// The floor is six cells, in the nudge's own measurement: one cell
	// short of it goes, and a nudge whose own text is shorter than the floor
	// goes too (the box is the text's width, capped by the room).
	if !triggerHintFits(60, 1000, 10) {
		t.Error("a box of exactly six cells was dropped")
	}
	if triggerHintFits(59.5, 1000, 10) {
		t.Error("a box under six cells was kept")
	}
	if triggerHintFits(1000, 59.5, 10) {
		t.Error("a nudge shorter than six cells was kept")
	}
	// A font that cannot be measured keeps the nudge: a wrong hide is the
	// only way to lose it for good.
	if !triggerHintFits(0, 0, 0) || !triggerHintFits(0, 100, 0) {
		t.Error("an unmeasurable font dropped the nudge")
	}

	// In the real row the room is the row's inner width less the controls
	// that keep their size; the nudge's box in a window the old 160px field
	// floor hid it in is far above six cells, so it stays.
	text := "Clipboard history \u00b7 enter"
	var room, natural, cell float32

	// One App per window: a bound Handle cannot be focused from two windows.
	wideApp := hintApp()
	wideApp.Query = "clip"
	var line float32
	wide := ui.NewTester(func(c *ui.Context) {
		t := c.Theme()
		room = wideApp.triggerHintRoom(c, t)
		natural, line = c.MeasureText(0, ui.Span{Text: text, Size: t.FontSize - 1})
		cell, _ = c.MeasureText(0, ui.Span{Text: "0", Size: t.FontSize - 1})
		wideApp.View(c)
	}, 300, int(WindowHeight("small")))
	wide.Frame()
	if room < cell*triggerHintMinCells {
		t.Fatalf("the test window has no room to show the nudge: %v < %v", room, cell*triggerHintMinCells)
	}
	if natural <= room {
		t.Fatalf("the nudge was not squeezed in the test window: %v <= %v", natural, room)
	}
	box, ok := wide.Find(text)
	if !ok {
		t.Fatalf("the nudge is missing at 200px: %q", wide.Texts())
	}
	// The row squeezed the nudge to its own leftover, and it is one line:
	// the formula and the layout agree.
	if box.W != room {
		t.Errorf("the nudge's box = %v, want the row's room %v", box.W, room)
	}
	if box.H > line {
		t.Errorf("the nudge wrapped instead of ellipsising: %v, one line is %v", box.H, line)
	}

	// A window too narrow for the floor drops the nudge rather than leaving
	// an ellipsis with no words left.
	narrowApp := hintApp()
	narrowApp.Query = "clip"
	narrow := ui.NewTester(func(c *ui.Context) {
		room = narrowApp.triggerHintRoom(c, c.Theme())
		narrowApp.View(c)
	}, 120, int(WindowHeight("small")))
	narrow.Frame()
	if room >= cell*triggerHintMinCells {
		t.Fatalf("the narrow window still has room: %v", room)
	}
	if narrow.HasText(text) {
		t.Errorf("the nudge was kept without room to read it: %q", narrow.Texts())
	}
}

// The nudge's other half: the transition it promises. A whole trigger word
// followed by a space enters the command's mode — the word is consumed, and
// whatever followed it is the argv the field starts with. The vocabulary is
// the same list the nudge reads (id, name, alias), so the two cannot disagree.
func TestTriggerWordEntersTheCommandMode(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID: "x", IntegrationName: "X",
		Command: extensions.Command{
			ID: "jv", Name: "JSON Viewer", Aliases: []string{"json"},
		},
		Program: "/bin/echo", Mode: "pty",
	}})
	for _, typed := range []string{"jv ", "JSON ", "json ", "jv -f "} {
		a.mode = nil
		a.Query = typed
		a.syncTypedMode()
		if a.mode == nil {
			t.Errorf("typing %q did not enter the mode", typed)
			continue
		}
		if a.mode.Command.ID != "jv" {
			t.Errorf("typing %q entered %q, want jv", typed, a.mode.Command.ID)
		}
	}
	// The rest of the line is the argv: the word itself is consumed.
	a.mode = nil
	a.Query = "jv -f "
	a.syncTypedMode()
	if a.mode == nil {
		t.Fatal("jv -f  did not enter the mode")
	}
	if a.Query != "-f" {
		t.Errorf("the mode's argv = %q, want -f", a.Query)
	}
	// A bare word is still an ordinary query: entering takes the space.
	a.mode = nil
	a.Query = "jv"
	a.syncTypedMode()
	if a.mode != nil {
		t.Error("a bare trigger word entered the mode")
	}
}

// The built-ins win the namespace, exactly as their parsers do: a command
// named after one cannot shadow its mode.
func TestBuiltInTriggerWordsWinTheNamespace(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		Command: extensions.Command{ID: "clipboard", Name: "Clipboard thing"},
		Program: "/bin/echo", Mode: "pty",
	}})
	a.Query = "clipboard "
	a.syncTypedMode()
	if a.mode != nil {
		t.Errorf("a command named after the built-in shadowed its mode: %q", a.mode.Command.ID)
	}
	if !a.clipboard {
		t.Error("the built-in clipboard mode did not open")
	}
}
