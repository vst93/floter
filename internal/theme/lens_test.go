package theme

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The recessed lens is the raised lens read backwards (base.css's
// `--glass-lens-stack-inset`, the value the `glass-reaxis` census pins): the
// light pools along the *bottom* edge of a groove and its top edge is the dark
// one — the opposite of `Lens`. This draws a groove and reads the two edges,
// so a stack that regressed to "the same stack as a raised pane" is caught.
func renderGroove(t *testing.T, step string) *ui.Tester {
	t.Helper()
	s := settings.Default()
	s.GlassStep = step
	tokens := For(s, true)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(ui.DarkTheme())
		ui.Column(c).FillWidth().FillHeight().Background(ui.RGB(24, 25, 28)).Padding(20).Children(func() {
			box := ui.Box(c).FillWidth().Height(60).Radius(12).Background(ui.RGB(40, 41, 45))
			tokens.LensInset(box)
		})
	}, 240, 100)
	tt.Frame()
	return tt
}

// rowMeanRGB is the mean of one scanline's three channels, over x0..x1.
func rowMeanRGB(tt *ui.Tester, y, x0, x1 int) float64 {
	img := tt.Image()
	if y < 0 || y >= img.Rect.Dy() {
		return 0
	}
	var sum float64
	var n int
	for x := x0; x < x1 && x < img.Rect.Dx(); x++ {
		i := y*img.Stride + x*4
		sum += float64(img.Pix[i]) + float64(img.Pix[i+1]) + float64(img.Pix[i+2])
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func TestTheRecessedLensLightsTheGroovesBottomEdge(t *testing.T) {
	// The groove sits at y=20..79 with a 12px radius, so the middle of each
	// edge (x=60..180) is inside the shape and clear of the corners. The lit
	// rim is the bottom scanline (the shadow's 1px line); the dark top edge and
	// its inner slot shadow darken the top one.
	const topY, bottomY, x0, x1 = 20, 79, 60, 180

	spread := make(map[string]float64)
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		tt := renderGroove(t, step)
		top := rowMeanRGB(tt, topY, x0, x1)
		bottom := rowMeanRGB(tt, bottomY, x0, x1)
		spread[step] = bottom - top
		t.Logf("%-8s top %.1f bottom %.1f (bottom-top %.1f)", step, top, bottom, spread[step])
		// The groove's lit edge is its bottom; its top edge is the dark one.
		if bottom <= top {
			t.Errorf("%s: the groove's bottom edge (%.1f) is not lit over its top edge (%.1f)", step, bottom, top)
		}
	}
	// The lit rim is graded by `--glass-lens-scale`, so the spread grows with
	// the stop: a fixed stack (the F3b regression) would hold it flat.
	if !(spread["off"] < spread["frosted"] && spread["frosted"] < spread["regular"] && spread["regular"] < spread["liquid"]) {
		t.Errorf("the groove's lens did not rise with the stop: off %.1f, frosted %.1f, regular %.1f, liquid %.1f",
			spread["off"], spread["frosted"], spread["regular"], spread["liquid"])
	}
}
