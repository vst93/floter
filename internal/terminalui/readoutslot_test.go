package terminalui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-co · `docs/liquid-glass.md` §6.58.
//
// The drawer's three "track + readout" rows end in a trailing slot that is
// right-justified (`drawerRow`), so a readout that is exactly as wide as the
// number it happens to show pushes the track beside it leftwards by whatever
// the number gained. The font-size readout prints `8 px` … `48 px`, and the
// digit that appears at 10 is a whole glyph: the track jumps under the finger
// that is dragging it. The fix is a slot: a fixed minimum width plus the
// readout's own right alignment, so the number grows *inside* its box.
//
// The pin reads the real drawer, not a mock: the readout is the box the tester
// registered for its own string, and the trailing cluster is right-justified
// with fixed-width siblings (`drawerRow`: a 27.5u track, a 2u gap, the readout),
// so the readout's own left edge **is** the track's position — one number pins
// both. (The knob itself is not a ruler here: its centre encodes the value, so
// it legitimately moves 75px between the two ends of the range.)

// drawerReadoutFrame renders the real drawer with the terminal font size set to
// one value and returns the range's knob centre together with the readout's own
// box.
func drawerReadoutFrame(t *testing.T, a *App, size int) ui.Rect {
	t.Helper()
	if err := a.Store.Update(func(s *settings.Settings) { s.FontSize = size }); err != nil {
		t.Fatal(err)
	}
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.drawer(c, i18n.For("en").Terminal)
	}, 900, 400)
	tt.Frame()
	readout, ok := tt.Find(fmt.Sprintf("%d px", size))
	if !ok {
		t.Fatalf("the drawer does not draw the readout %q: %v", fmt.Sprintf("%d px", size), tt.Texts())
	}
	return readout
}

// TestTheDrawerReadoutHasItsOwnSlot: the widest value the range can print must
// not move the track it belongs to.
func TestTheDrawerReadoutHasItsOwnSlot(t *testing.T) {
	a := drawerApp(t)
	narrowBox := drawerReadoutFrame(t, a, settings.MinFontSize)
	wideBox := drawerReadoutFrame(t, a, settings.MaxFontSize)

	t.Logf("readout slot: %d px → %v, %d px → %v", settings.MinFontSize, narrowBox, settings.MaxFontSize, wideBox)
	if narrowBox.X != wideBox.X || narrowBox.W != wideBox.W {
		t.Errorf("the readout is not in a slot: its box goes %v → %v (x %.1f → %.1f, w %.1f → %.1f)",
			narrowBox, wideBox, narrowBox.X, wideBox.X, narrowBox.W, wideBox.W)
	}
	// A slot that is too narrow would wrap the widest value instead of moving
	// anything: the box keeps its width and doubles its height.
	if narrowBox.H != wideBox.H {
		t.Errorf("the widest readout wrapped instead of fitting its slot: box height %.1f → %.1f",
			narrowBox.H, wideBox.H)
	}
}
