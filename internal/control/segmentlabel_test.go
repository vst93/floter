package control

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// The segmented control's labels all wear one register.
//
// The reference (`main:src/styles/settings.css:1111-1120`) writes
// `.settings-option__label { color: var(--text-strong); font-weight: 580 }` for
// *every* segment. The chosen state is carried by the raised pane and the accent
// keyline (`.settings-option--active`, `:1095-1098`); `color` and `font-weight`
// never follow the selection, and the bare `.settings-option` label colour
// (`:1060`) is `--text-primary`. This build dimmed the unchosen labels to
// `--text-muted` at weight 500 — a difference the reference does not have.
//
// The test reads each label's own ink off the live control: the pixel farthest
// from the segment's surface is the glyph's colour, and it must be
// `--text-strong` composited over that same surface. Mutation check: restore the
// `TextMuted`/500 branch and the unchosen labels fail both assertions.
func TestEverySegmentLabelIsTextStrong(t *testing.T) {
	labels := []string{"Follow", "Long interface", "Tiny"}
	tt, _ := renderMeasured(t, labels, 1)
	tokens := theme.For(settings.Default(), true)

	for i, label := range labels {
		rect, ok := tt.Find(label)
		if !ok {
			t.Fatalf("the segment %q is missing: %q", label, tt.Texts())
		}
		surface, _ := segmentModal(tt, rect)
		got := segmentInk(tt, rect)
		want := segmentOver(tokens.TextStrong, surface)
		if !segmentColorClose(got, want, 20) {
			t.Errorf("segment %d (%q) label ink = %v, want --text-strong over its surface (%v)",
				i, label, got, want)
		}
		if muted := segmentOver(tokens.Theme.TextMuted, surface); segmentColorClose(got, muted, 20) {
			t.Errorf("segment %d (%q) label = %v wears --text-muted, not --text-strong", i, label, got)
		}
	}
}

// segmentInk is the colour a text rect is actually drawn in: the pixel of the
// rect farthest from its background. Antialiased glyph edges sit between the
// two, so the farthest pixel is the glyph's own colour.
func segmentInk(tt *ui.Tester, r ui.Rect) ui.Color {
	img := tt.Image()
	background, _ := segmentModal(tt, r)
	var ink ui.Color
	best := -1
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			if d := segmentColorDistance(c, background); d > best {
				best = d
				ink = c
			}
		}
	}
	return ink
}

// segmentModal returns the most common colour in a rect, which for a text box
// is the surface the glyphs sit on.
func segmentModal(tt *ui.Tester, r ui.Rect) (ui.Color, int) {
	img := tt.Image()
	counts := map[ui.Color]int{}
	var mode ui.Color
	best := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			counts[c]++
			if counts[c] > best {
				best = counts[c]
				mode = c
			}
		}
	}
	return mode, best
}

// segmentOver is one straight-alpha color drawn over an opaque one, the way the
// renderer paints a token text color onto its surface.
func segmentOver(over, base ui.Color) ui.Color {
	a := float32(over.A) / 255
	mix := func(o, b uint8) uint8 {
		return uint8(float32(o)*a + float32(b)*(1-a) + 0.5)
	}
	return ui.Color{R: mix(over.R, base.R), G: mix(over.G, base.G), B: mix(over.B, base.B), A: 255}
}

func segmentColorDistance(a, b ui.Color) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	return dr*dr + dg*dg + db*db
}

func segmentColorClose(a, b ui.Color, tol int) bool {
	return segmentColorDistance(a, b) <= tol*tol*3
}
