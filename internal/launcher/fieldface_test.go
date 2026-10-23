package launcher

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// The field row's trailing controls are one control, and the clear button is
// one of them.
//
// The reference gives every `.collapsed-card__settings` the same three faces
// (`main:src/styles/launcher.css:382-411`): its mark in `--text-tertiary` at
// rest (`:397`), the `:hover:not(:disabled)` lift to `--text-strong` over the
// control ladder's hover fill (`:400-404`), and the `:active` press — 0.92 over
// `--glass-control-press` (`:407-411`). The port had drawn the clear button with
// a face of its own: the *muted* ink (one step darker than the shared control's
// resting tertiary), a plain hover fill, and no press at all — so the row's four
// trailing controls did not answer the pointer alike.
//
// The assertions read the mark's own darkest pixel (the ink) against the field
// block's plane, which is how the two resting registers separate: `--text-muted`
// composites ~11 levels darker than `--text-tertiary` over the same plane, an
// order of magnitude past the antialiasing tolerance. Mutation check: put the
// clear button's old `TextMuted` ink back and the resting read matches the muted
// composite instead, failing the first assertion.
func TestTheFieldRowClearControlWearsTheSharedControlFace(t *testing.T) {
	forEachAppearance(t, fieldRowClearControlFace)
}

func fieldRowClearControlFace(t *testing.T, dark bool) {
	a := testApp()
	a.Actions.OpenSessions = func() {}
	tt := renderIn(t, a, dark)
	tt.Type("x")
	tt.Frame()

	clear, ok := tt.Find("Clear")
	if !ok {
		t.Fatalf("the field's clear control was not drawn: %q", tt.Texts())
	}
	// The field block's own blank plane, sampled well left of the trailing
	// controls: what both inks are drawn over.
	field := tt.Image().RGBAAt(400, int(clear.Y+clear.H/2))
	tokens := theme.ForPreferences(a.settings(), dark, ui.Preferences{})

	// Rest: every one of the row's trailing controls rests on the shared
	// control's ink, `--text-tertiary` (`main:src/styles/launcher.css:397`).
	// The muted register is ~11 levels darker over this plane, so a tolerance
	// that absorbs the glyphs' antialiasing (a few levels) still separates the
	// two tokens. Mutation check: put the clear button's old `TextMuted` ink
	// back and its resting read lands on the muted composite instead.
	tertiary := inkOver(tokens.TextTertiary, field)
	muted := inkOver(tokens.Theme.TextMuted, field)
	for _, name := range []string{"Clear", "Sessions", "Settings"} {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("the field row has no %q control: %q", name, tt.Texts())
		}
		ink := inkOn(tt, r, field)
		if !nearRGB(ink, tertiary, 7) {
			t.Errorf("the %s mark rests at %v, want the shared control's --text-tertiary %v", name, ink, tertiary)
		}
		if nearRGB(ink, muted, 4) {
			t.Errorf("the %s mark wears the muted ink %v, not the shared control's --text-tertiary", name, muted)
		}
	}

	// Held: the shared control takes the `:active` press too — the state the
	// port's own clear button never had. The press *scales the control down*
	// (`transform: scale(0.92)`), which the drawn box reports directly and
	// antialiasing cannot blur: a control with no press face keeps its full box.
	tt.Press(clear.X+clear.W/2, clear.Y+clear.H/2)
	tt.Frame()
	pressed, ok := tt.Find("Clear")
	if !ok {
		t.Fatal("the clear control left the row while held")
	}
	if pressed.W >= clear.W {
		t.Errorf("the held clear control did not sink: box %v wide, rest %v", pressed.W, clear.W)
	}
	tt.Release(clear.X+clear.W/2, clear.Y+clear.H/2)
}

// imgPlane is the most common colour in a box: the plane the mark sits on.
func imgPlane(tt *ui.Tester, r ui.Rect) color.RGBA {
	img := tt.Image()
	counts := map[color.RGBA]int{}
	var plane color.RGBA
	best := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := img.RGBAAt(x, y)
			counts[c]++
			if counts[c] > best {
				best, plane = counts[c], c
			}
		}
	}
	return plane
}

// inkOver draws one straight-alpha token over an opaque plane.
func inkOver(over ui.Color, base color.RGBA) color.RGBA {
	a := float64(over.A) / 255
	mix := func(o, b uint8) uint8 {
		return uint8(float64(o)*a + float64(b)*(1-a) + 0.5)
	}
	return color.RGBA{mix(over.R, base.R), mix(over.G, base.G), mix(over.B, base.B), 255}
}

// channelDistance is the summed per-channel distance between two colours.
func channelDistance(a, b color.RGBA) int {
	d := 0
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		v := int(p[0]) - int(p[1])
		if v < 0 {
			v = -v
		}
		d += v
	}
	return d
}

// nearRGB reports whether two pixels agree within tol on every channel.
func nearRGB(a, b color.RGBA, tol int) bool {
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		v := int(p[0]) - int(p[1])
		if v < 0 {
			v = -v
		}
		if v > tol {
			return false
		}
	}
	return true
}
