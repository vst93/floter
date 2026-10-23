package launcher

import (
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// renderIn renders the launcher on one appearance.
//
// The appearance switch is the tester's dark flag, not the settings store: the
// app's own tokens follow `c.Theme().Dark` (`launcher.go:1026`), so `SetDark` is
// what moves every face the pins below read. `Advance` is mandatory — the rows
// and the feedback band carry colour transitions, and the first frame after
// `SetDark` is still the previous palette easing out (the P9-c lesson: a pin
// that reads that frame measures a blend, not a face).
func renderIn(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
	}, InputWindowWidth, frameHeight(maxResultSlots))
	if dark {
		tt.SetDark(true)
		tt.Advance(500 * time.Millisecond)
	}
	tt.Frame()
	return tt
}

// forEachAppearance runs one pin on both appearances, each on its own App.
//
// The fixtures accumulate state as they are driven (results, row transitions,
// a held modifier), so a second appearance reusing the same App would read the
// first one's leftovers. Each subtest therefore builds its own app and only
// takes the appearance as a parameter.
func forEachAppearance(t *testing.T, pin func(t *testing.T, dark bool)) {
	t.Helper()
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) { pin(t, appearance.dark) })
	}
}

// inkOn reads a glyph's own ink off the plane it is drawn on: the pixel
// farthest (summed channels) from that plane.
//
// Reading the *darkest* pixel is the light-appearance half of this probe —
// there the ink is darker than its plane. In the dark appearance the same ink is *lighter* than
// its plane, so "darkest pixel" silently reads the plane (or the plate's own
// edge) and an assertion built on it passes on a glyph that was never drawn.
func inkOn(tt *ui.Tester, r ui.Rect, plane color.RGBA) color.RGBA {
	img := tt.Image()
	far := -1
	var ink color.RGBA
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := img.RGBAAt(x, y)
			if d := channelDistance(c, plane); d > far {
				far, ink = d, c
			}
		}
	}
	return ink
}
