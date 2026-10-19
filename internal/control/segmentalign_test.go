package control

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// The base segmented track reads as a row; only the material picker centers.
//
// The reference writes the bare `.settings-option` with `text-align: left` and
// `padding: 0 calc(var(--u) * 9)` (`main:src/styles/settings.css:1055,1060`), so
// a segment in the shared `.settings-options--inline` track puts its label at
// the left inset and reads left to right. Its **only** centered variant is
// `.glass-intensity__segments .settings-option`
// (`settings.css:792-799`): `text-align: center; justify-items: center` with the
// inset tightened to `0 calc(var(--u) * 6)`. CSS's own comment names the reason
// — "A stop's label is the effect's name; centered because the segment is a
// choice, not a row" — so the base form must be the row reading, not centered.
//
// This build drew *every* segment `Justify(ui.Center)`. The test reads each
// label's own ink off the live control: for an unchosen segment the surface is
// the track colour, so the glyphs are the only ink, and the gap between the
// glyph's left edge and the segment's left edge tells the two forms apart.
// Mutation check: restore `Justify(ui.Center)` on the base form and
// TestTheBaseSegmentsAreRowsAndOnlyTheGlassTrackIsCentered goes red (the base
// labels become centered, ~half the segment wide).
func TestTheBaseSegmentsAreRowsAndOnlyTheGlassTrackIsCentered(t *testing.T) {
	labels := []string{"Follow", "Tiny", "Large"}
	tokens := theme.For(settings.Default(), true)
	inset := tokens.Theme.Space(2.25) // 9u, `.settings-option`'s padding
	if inset <= 0 {
		t.Fatalf("the theme has no unit ladder: Space(2.25) = %.2f", inset)
	}

	t.Run("base row", func(t *testing.T) {
		tt := renderSegmented(t, labels, func(c *ui.Context, tokens theme.Tokens) {
			SegmentedFill(c, tokens, labels, 1, func(int) {})
		})
		for _, label := range []string{labels[0], labels[2]} { // the unchosen segments
			seg, ok := tt.Find(label)
			if !ok {
				t.Fatalf("the segment %q is missing: %q", label, tt.Texts())
			}
			left, right, ink := segmentGlyphEdges(tt, seg)
			if left < 0 {
				t.Fatalf("no glyph ink found in segment %q", label)
			}
			// The label starts at the 9u inset: a row.
			if abs(left-inset) > 2 {
				t.Errorf("the base segment %q starts its label %.2f from the left edge, "+
					"want the reference's 9u = %.2f", label, left, inset)
			}
			// And it is nowhere near centered: the right gap must dwarf the left.
			if right < seg.W/2 {
				t.Errorf("the base segment %q looks centered (left gap %.2f, right gap %.2f, "+
					"width %.2f), want a left-aligned row", label, left, right, seg.W)
			}
			_ = ink
		}
	})

	t.Run("glass center", func(t *testing.T) {
		tt := renderSegmented(t, labels, func(c *ui.Context, tokens theme.Tokens) {
			SegmentedGlassFill(c, tokens, labels, 1, func(int) {})
		})
		for _, label := range []string{labels[0], labels[2]} {
			seg, ok := tt.Find(label)
			if !ok {
				t.Fatalf("the segment %q is missing: %q", label, tt.Texts())
			}
			left, right, _ := segmentGlyphEdges(tt, seg)
			if left < 0 {
				t.Fatalf("no glyph ink found in segment %q", label)
			}
			// A centered label has equal air on both sides of its ink — nearly.
			// What the segment centers is the text's layout box, and a glyph's
			// side bearings put the visible ink a pixel or two off that box's
			// middle ("Large" ends on an `e`, whose right bearing is the wider
			// one). The tolerance is therefore a share of the segment's width
			// rather than a fixed pixel count, which a font-size rung cannot
			// push over: a left-aligned label misses the middle by more than
			// half the segment (see the base form above), an order of magnitude
			// outside this.
			if abs(left-right) > seg.W/20 {
				t.Errorf("the glass segment %q is not centered: left gap %.2f, right gap %.2f (width %.2f)",
					label, left, right, seg.W)
			}
			// And it is clearly right of the base form's 9u inset.
			if left <= inset {
				t.Errorf("the glass segment %q starts its label %.2f from the left edge, "+
					"want it centered, past the 9u inset %.2f", label, left, inset)
			}
			// The floor still holds: centering changes alignment, not height.
			if want := tokens.Theme.Space(7.5); abs(seg.H-want) > 1 {
				t.Errorf("the glass segment %q is %.2f high, want the reference's 30u = %.2f",
					label, seg.H, want)
			}
		}
	})
}

// renderSegmented draws one segmented track and returns the tester.
func renderSegmented(t *testing.T, labels []string, draw func(*ui.Context, theme.Tokens)) *ui.Tester {
	t.Helper()
	tokens := theme.For(settings.Default(), true)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		draw(c, tokens)
	}, 600, 200)
	tt.Frame()
	return tt
}

// segmentGlyphEdges returns the air between the segment's own edges and the ink
// of its label, plus the ink width. It scans only the segment's middle rows, so
// the track's lens rim at the top and bottom is never mistaken for a glyph, and
func segmentGlyphEdges(tt *ui.Tester, r ui.Rect) (left, right, width float32) {
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
			if segmentColorDistance(c, mode) > 2500 {
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
		return -1, -1, 0
	}
	left = float32(minX) - r.X
	right = (r.X + r.W) - float32(maxX)
	return left, right, float32(maxX - minX)
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
