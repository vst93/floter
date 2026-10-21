package shell

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The launcher card's face follows the keyboard.
//
// The reference's `.collapsed-card:focus-within` rule swaps **two** things at
// once (`main:src/styles/launcher.css:136-144`): the 1px edge
// (`border-color: var(--input-stroke-active)`) and the lit face
// (`background: linear-gradient(180deg, var(--sheen-top-active),
// var(--sheen-bottom-active)), var(--launcher-tint)`). The port carried the edge
// and not the face, so the card's own fill never moved when it took the
// keyboard — half of the one focus cue the design spends on the whole panel, and
// it was missing on all three platforms alike (which is why no per-platform
// suite ever caught it).
//
// Where the lift is *visible* is the dark appearance: the light one's sheen
// (0.66) is already above the frame's alpha at the shipped opacity, so both
// pairs cap to the frame and the face is the same either way — the compensation
// in `theme.Tokens.FaceFill` deliberately keeps the frame's alpha the slider's.
// The dark pair (0.046/0.01) sits under it, so the raise reads as a lit face.
// That asymmetry is base.css's own, not this build's.
func TestTheCardLiftsItsFaceWhenItHasTheKeyboard(t *testing.T) {
	render := func(focused bool) *image.RGBA {
		a := appKeysApp()
		s := a.Store.Snapshot()
		s.Theme = "dark"
		a.Store = settings.NewStore(s)
		a.cardFocus = func() bool { return focused }
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, launcher.InputWindowWidth, 420)
		tt.Frame()
		tt.Type("ls")
		tt.Frame()
		return tt.Image()
	}
	resting, lit := render(false), render(true)

	// The card's fill is read inside its top band, a few pixels in from the
	// card's own edge: that area is the face and nothing else.
	x, y := 400, 20
	if got, want := resting.RGBAAt(x, y), lit.RGBAAt(x, y); got == want {
		t.Errorf("the focused card's face is the resting one: %v at (%d,%d) both ways", got, x, y)
	}
	// The lit face is the *raised* one, not a dimmer: the reference's pair goes
	// up on focus in both appearances (light 0.66→0.78 / 0.22→0.28, dark
	// 0.046→0.056 / 0.01→0.012, `base.css:630-631,908-909`).
	r, l := resting.RGBAAt(x, y), lit.RGBAAt(x, y)
	if l.R < r.R || l.G < r.G || l.B < r.B {
		t.Errorf("the focused face is not lit: resting %v, lit %v", r, l)
	}
}

// The face and the stroke are one rule: a card with the keyboard wears both, so
// neither half can be reverted without the other going with it.
func TestTheFocusFaceAndTheFocusStrokeMoveTogether(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tk := theme.ForPreferences(appKeysApp().Store.Snapshot(), dark, ui.Preferences{})
		if tk.InputStrokeActive == tk.InputStroke {
			t.Errorf("dark=%v: the focused stroke is the resting one", dark)
		}
		if tk.SheenTopActive == tk.SheenTop && tk.SheenBottomActive == tk.SheenBottom {
			t.Errorf("dark=%v: the focused sheen is the resting one: %v / %v",
				dark, tk.SheenTopActive, tk.SheenBottomActive)
		}
		// Both are capped at the frame's own alpha, so a raised face never makes
		// the frame denser than the slider asked for (see SheenAtFocus).
		if top, bottom := tk.SheenAtFocus(0.2); float32(top.A)/255 > 0.2+0.004 || float32(bottom.A)/255 > 0.2+0.004 {
			t.Errorf("dark=%v: the focused sheen is not capped to the face: %v / %v", dark, top, bottom)
		}
		// ...and the fill under the raised pair still composites back to the
		// frame's alpha, exactly as the resting one does.
		top, bottom := tk.FaceFillAt(0.47, true)
		sheenTop, sheenBottom := tk.SheenAtFocus(0.47)
		for _, pair := range [][2]ui.Color{{top, sheenTop}, {bottom, sheenBottom}} {
			composite := 1 - (1-float32(pair[0].A)/255)*(1-float32(pair[1].A)/255)
			if diff := composite - 0.47; diff > 0.005 || diff < -0.005 {
				t.Errorf("dark=%v: the focused face composites to %.3f, want the frame's 0.47", dark, composite)
			}
		}
	}
}
