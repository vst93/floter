package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/browser"
)

// glyphName is the mark a row's glyph is, for a readable assertion message.
func glyphName(g *ui.SVG) string {
	switch g {
	case glyphClock:
		return "clock"
	case glyphSettings:
		return "settings"
	case glyphTerminal:
		return "terminal"
	case glyphGlobe:
		return "globe"
	case glyphClipboard:
		return "clipboard"
	case glyphCalculator:
		return "calculator"
	case glyphRestart:
		return "restart"
	case glyphPower:
		return "power"
	}
	return "?"
}

// Every built-in command row leads with its own mark, not the recents' clock.
//
// The port's glyph switch tested ids prefixed `cmd:` (`cmd:settings`,
// `cmd:power-restart`, …) while the rows' own ids carry no such prefix (see
// `commands()` in search.go: `settings`, `power-restart`, …). The switch
// therefore matched nothing and every built-in row fell through to the clock —
// the one glyph the reference never drew on them (`main:src/launcher/
// LauncherResults.tsx:223-262`'s `SystemActionIcon`, one shape per action).
func TestEveryBuiltInRowLeadsWithItsOwnMark(t *testing.T) {
	a := testApp()
	byID := map[string]Item{}
	for _, item := range a.Results() {
		byID[item.ID] = item
	}
	cases := []struct {
		id    string
		glyph string
	}{
		{"settings", "settings"},
		{"terminal", "terminal"},
		{"browser", "globe"},
		{"clipboard", "clipboard"},
		{"calculator", "calculator"},
		{"power-restart", "restart"},
		{"power-shutdown", "power"},
		{"quit", "power"},
	}
	for _, c := range cases {
		item, ok := byID[c.id]
		if !ok {
			t.Errorf("the built-in row %q is missing: %v", c.id, ids(a.Results()))
			continue
		}
		g, ok := rowGlyph(item)
		if !ok {
			t.Errorf("the %q row has no glyph", c.id)
			continue
		}
		if got := glyphName(g); got != c.glyph {
			t.Errorf("the %q row's glyph = %s, want %s (it must not fall through to the clock)",
				c.id, got, c.glyph)
		}
	}
	// A negative check: nothing in the shipped built-in list draws the clock —
	// the fall-through the bug produced.
	for _, item := range a.Results() {
		if g, ok := rowGlyph(item); ok && g == glyphClock {
			t.Errorf("the %q row fell through to the clock glyph", item.ID)
		}
	}
}

// The two power actions wear the warm system plane (the reference's
// `.launcher-result__icon--system`); the port's `cmd:power-` prefix test
// matched no id, so the warm plate was never drawn.
//
// The two rows are compared by *title* (a built-in row draws its own localized
// title; `Shut Down` is the confirm button's word, never a row's) and at the
// plate's own centre — the cell spans the row's 16u inset to 16u+28u, so its
// centre is the title's x less ~26px at the shipped step, not less 38 (that
// lands on the plate's rounded edge, where the lens has nothing yet painted).
func TestThePowerRowsWearTheWarmPlate(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Frame()
	img := tt.Image()
	cell := func(title string) (r, b uint32) {
		rect, ok := tt.Find(title)
		if !ok {
			t.Fatalf("the row %q was not drawn: %q", title, tt.Texts())
		}
		c := img.RGBAAt(int(rect.X)-26, int(rect.Y+rect.H/2))
		return uint32(c.R), uint32(c.B)
	}
	neutralR, neutralB := cell("Open settings")
	powerR, powerB := cell("Shut down the computer")
	if powerR <= powerB {
		t.Errorf("the power cell is not warm: r=%d b=%d", powerR, powerB)
	}
	if neutralR >= neutralB+8 {
		t.Errorf("the neutral cell reads warm too: r=%d b=%d", neutralR, neutralB)
	}
}

// A browser row's icon cell is bare: the reference's
// `.launcher-result__icon--browser` drops the plate (transparent, unshadowed,
// muted ink) so a column of links reads as text, not as eight grey controls.
func TestBrowserRowsDropTheIconPlate(t *testing.T) {
	a := testApp()
	a.Actions.SearchBrowser = func(query string, done func(BrowserResults)) {
		done(BrowserResults{Found: true, Results: []browser.Result{{
			URL: "https://go.dev", Title: "Go",
		}}})
	}
	tt := render(t, a)
	tt.Type("browser ")
	tt.Frame()
	if !a.browser {
		t.Fatalf("the browser mode did not open: %q", tt.Texts())
	}
	rowRect, ok := tt.Find("Go")
	if !ok {
		t.Fatalf("the browser row was not drawn: %q", tt.Texts())
	}
	img := tt.Image()
	y := int(rowRect.Y + rowRect.H/2)
	// The icon column starts at the row's own 16u inset; a plated cell paints
	// the control fill there, a bare one leaves the row's own fill showing.
	// Compare the cell's centre against a point inside the *same row* but past
	// the text column: the selected row's accent tint spans the whole row, so a
	// bare cell must read exactly what the row itself paints (comparing to the
	// panel's own fill would flag the selection tint as a plate).
	cell := img.RGBAAt(int(rowRect.X)-26, y)
	bare := img.RGBAAt(InputWindowWidth-40, y)
	if cell != bare {
		t.Errorf("the browser row's icon cell is plated (%v over the row's own bare fill %v)", cell, bare)
	}
}
