package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// The live General page routes only the material pick through the centered
// segmented form.
//
// `control.SegmentedGlassFill` is the reference's one centered track
// (`.glass-intensity__segments`, `main:src/styles/settings.css:792-799`); every
// other segmented row — theme, language, interface size, the app-icon pick — is
// a `.settings-option` row with `text-align: left` (`:1060`). The control's own
// test pins the two forms; this one pins the App's routing, so a later change
// that sends the material pick back through the base `pick` (or the reverse)
// cannot land silently.
//
// It measures the label's ink inside an unchosen segment: the theme/language
// rows have transparent segment faces, so the glyphs are the only ink, and the
// air on each side tells a row from a centered choice.
func TestTheMaterialPickCentersWhereTheBasePickIsARow(t *testing.T) {
	a := New(newStore(t), Actions{})
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
		tokens = a.tokens(c)
	}, 720, 1800)
	tt.Frame()

	inset := tokens.Theme.Space(2.25) // 9u, `.settings-option`'s padding
	if inset <= 0 {
		t.Fatalf("the theme has no unit ladder: Space(2.25) = %.2f", inset)
	}

	// A language segment (a base row) and the material pick's widest stop (the
	// one centered track). Both are unchosen at the shipped defaults, so neither
	// segment wears the raised pane that would obscure its ink.
	base, ok := tt.Find("中文")
	if !ok {
		t.Fatalf("the language segment is missing: %q", tt.Texts())
	}
	left, right := settingsGlyphEdges(tt, base)
	if left < 0 {
		t.Fatalf("no glyph ink found in the language segment")
	}
	if absFloat(left-inset) > 2 {
		t.Errorf("the base segmented row starts its label %.2f from the left edge, "+
			"want the reference's 9u = %.2f", left, inset)
	}
	if right < base.W/2 {
		t.Errorf("the base segmented row looks centered (left gap %.2f, right gap %.2f), "+
			"want a left-aligned row", left, right)
	}

	glass, ok := tt.Find("Liquid Max")
	if !ok {
		t.Fatalf("the material segment is missing: %q", tt.Texts())
	}
	gLeft, gRight := settingsGlyphEdges(tt, glass)
	if gLeft < 0 {
		t.Fatalf("no glyph ink found in the material segment")
	}
	if absFloat(gLeft-gRight) > 5 {
		t.Errorf("the material pick is not centered: left gap %.2f, right gap %.2f",
			gLeft, gRight)
	}
	// And it must sit clearly past the base row's 9u inset — a centered choice,
	// not a left-aligned row whose glyphs happen to have uneven bearings.
	if gLeft <= inset+4 {
		t.Errorf("the material pick starts its label %.2f from the left edge, want it "+
			"centered well past the base row's 9u inset %.2f", gLeft, inset)
	}
}

// settingsGlyphEdges returns the air between a segment's own edges and its
// label's ink, scanning only the middle rows so the track's lens rim is never
// mistaken for a glyph.
func settingsGlyphEdges(tt *ui.Tester, r ui.Rect) (left, right float32) {
	img := tt.Image()
	x0, x1 := int(r.X+0.5), int(r.X+r.W-0.5)
	cy := int(r.Y + r.H/2)
	y0, y1 := cy-3, cy+3

	counts := map[ui.Color]int{}
	var mode ui.Color
	best := 0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			counts[c]++
			if counts[c] > best {
				best = counts[c]
				mode = c
			}
		}
	}

	minX, maxX := -1, -1
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			if settingsColorDistance(c, mode) > 2500 {
				if minX < 0 || x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
			}
		}
	}
	if minX < 0 {
		return -1, -1
	}
	return float32(minX) - r.X, (r.X + r.W) - float32(maxX)
}

func settingsColorDistance(a, b ui.Color) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	return dr*dr + dg*dg + db*db
}
