package control

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// Every short choice centers its label and keeps the same usable hit area.
func TestSegmentLabelsAreCentered(t *testing.T) {
	labels := []string{"Follow", "Tiny", "Large"}
	tokens := theme.For(settings.Default(), true)
	for _, glass := range []bool{false, true} {
		tt := renderSegmented(t, labels, func(c *ui.Context, tokens theme.Tokens) {
			if glass {
				SegmentedGlassFill(c, tokens, labels, 1, func(int) {})
			} else {
				SegmentedFill(c, tokens, labels, 1, func(int) {})
			}
		})
		for _, label := range []string{labels[0], labels[2]} {
			seg, ok := tt.Find(label)
			if !ok {
				t.Fatalf("missing segment %q", label)
			}
			left, right, _ := segmentGlyphEdges(tt, seg)
			// Allow glyph side bearings, but reject leading alignment.
			if left < 0 || abs(left-right) > seg.W/20 {
				t.Errorf("glass=%v %q is not centered: left %.2f right %.2f width %.2f", glass, label, left, right, seg.W)
			}
			if want := max(float32(36), tokens.Theme.Space(9)); abs(seg.H-want) > 1 {
				t.Errorf("%q height %.2f, want %.2f", label, seg.H, want)
			}
		}
	}
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
