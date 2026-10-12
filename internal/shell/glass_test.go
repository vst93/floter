package shell

import (
	"testing"

	"github.com/egoist/mygo"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/settings"
)

// renderStop renders one launcher frame at a stored glass stop.
func renderStop(t *testing.T, theme, step string) *ui.Tester {
	t.Helper()
	return renderStopOpacity(t, theme, step, settings.DefaultMainOpacity)
}

// renderStopOpacity renders one launcher frame at a stored glass stop and
// transparency.
func renderStopOpacity(t *testing.T, theme, step string, opacity uint8) *ui.Tester {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) {
		s.Theme = theme
		s.GlassStep = step
		s.MainOpacity = opacity
	}); err != nil {
		t.Fatal(err)
	}
	a := New(Options{Store: store})
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()
	return tt
}

// meanBandAlpha is the mean alpha, 0..1, of a horizontal band of the frame.
func meanBandAlpha(tt *ui.Tester, y0, y1 int) float32 {
	img := tt.Image()
	if y1 > img.Rect.Dy() {
		y1 = img.Rect.Dy()
	}
	var sum float64
	var n int
	for y := y0; y < y1; y++ {
		for x := 20; x < img.Rect.Dx()-20; x++ {
			sum += float64(img.Pix[y*img.Stride+x*4+3])
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return float32(sum / float64(n) / 255)
}

// meanBandRGB is the mean color, 0..255 per channel, of a horizontal band: it
// carries the control lens, which the alpha does not.
func meanBandRGB(tt *ui.Tester, y0, y1 int) (r, g, b float64) {
	img := tt.Image()
	if y1 > img.Rect.Dy() {
		y1 = img.Rect.Dy()
	}
	var n int
	for y := y0; y < y1; y++ {
		for x := 20; x < img.Rect.Dx()-20; x++ {
			i := y*img.Stride + x*4
			r += float64(img.Pix[i])
			g += float64(img.Pix[i+1])
			b += float64(img.Pix[i+2])
			n++
		}
	}
	if n == 0 {
		return 0, 0, 0
	}
	return r / float64(n), g / float64(n), b / float64(n)
}

// The glass stop is the effect ladder, and it must be visible in the rendered
// frame — not only in an internal token. The old build's `--glass-step-blur`
// (10/22/28) is the OS material (see vibrancyForStop) and its
// `--glass-lens-scale` (0.5/1/1.6) is the control lens; the tint carries the
// haze. This is the pixel-level acceptance: the frame's own alpha rises from
// the thinnest stop to the heaviest, and the off stop is the plain translucent
// frame (no glass effect, but no opaque stand-in either).
//
// The earlier rewrite mapped only the haze, so `frosted` and `regular` were
// 0.03 of alpha apart and the user read the three stops as one
// (「玻璃效果…看起来好像没什么区别」).
func TestTheGlassStopsAreVisiblyDifferent(t *testing.T) {
	const topY0, topY1 = 90, 120

	for _, theme := range []string{"dark", "light"} {
		off := renderStop(t, theme, "off")
		frosted := renderStop(t, theme, "frosted")
		regular := renderStop(t, theme, "regular")
		liquid := renderStop(t, theme, "liquid")

		offA := meanBandAlpha(off, topY0, topY1)
		fA := meanBandAlpha(frosted, topY0, topY1)
		rA := meanBandAlpha(regular, topY0, topY1)
		lA := meanBandAlpha(liquid, topY0, topY1)

		t.Logf("%s appearance — mean alpha in the frame's top band:", theme)
		t.Logf("  off     %.3f", offA)
		t.Logf("  frosted %.3f", fA)
		t.Logf("  regular %.3f", rA)
		t.Logf("  liquid  %.3f", lA)
		// The material parameters the pixels come from, for the record.
		for _, step := range []string{"frosted", "regular", "liquid"} {
			spec := glassmap.SpecFor(step, settings.DefaultMainOpacity)
			t.Logf("  %-8s material: blur=%.0f saturate=%.0f%% lens=%.1f tint=%.3f", step, spec.Blur, spec.Saturate, spec.LensScale, spec.TintAlpha)
		}

		// Off is the plain translucent frame: no glass effect, but the
		// window's own transparency still paints. It must not be the opaque
		// stand-in that made the app's transparent bottom fail when the glass
		// was turned off (「关闭玻璃效果的时候，应用的透明底是失效的」).
		if offA >= 0.99 {
			t.Errorf("%s: the off stop is %.3f, want the frame's translucent tint", theme, offA)
		}
		if offA >= fA {
			t.Errorf("%s: the off stop (%.3f) is not thinner than frosted (%.3f)", theme, offA, fA)
		}
		// The three effect stops are translucent, and each heavier stop lets
		// more desktop through: the tint ladder is monotonic.
		if !(fA > rA && rA > lA) {
			t.Errorf("%s: the stop ladder did not fall: frosted %.3f, regular %.3f, liquid %.3f", theme, fA, rA, lA)
		}
		if fA-lA < 0.08 {
			t.Errorf("%s: the stops moved only %.3f of alpha; want a visible spread", theme, fA-lA)
		}
		// The lens rides the same stop: the frame's colour, not only its
		// alpha, changes with the stop (the old rewrite had no lens at all).
		fR, fG, fB := meanBandRGB(frosted, topY0, topY1)
		lR, lG, lB := meanBandRGB(liquid, topY0, topY1)
		if absf(fR-lR)+absf(fG-lG)+absf(fB-lB) < 1 {
			t.Errorf("%s: the lens did not change the frame's colour (frosted %.1f/%.1f/%.1f, liquid %.1f/%.1f/%.1f)",
				theme, fR, fG, fB, lR, lG, lB)
		}
	}
}

// Turning the glass effect off must not turn the window opaque. The off stop
// drops the blur, the haze and the lens, but the frame's own transparency —
// the opacity slider — still paints; the old `off` branch took the near-solid
// stand-in instead, which is what read as the app's transparent bottom
// failing (「关闭玻璃效果的时候，应用的透明底是失效的」).
func TestTurningTheGlassOffKeepsTheTransparency(t *testing.T) {
	const botY0, botY1 = 350, 400

	for _, theme := range []string{"dark", "light"} {
		low := renderStopOpacity(t, theme, "off", settings.MinWindowOpacity)
		high := renderStopOpacity(t, theme, "off", settings.MaxWindowOpacity)
		lowA := meanBandAlpha(low, botY0, botY1)
		highA := meanBandAlpha(high, botY0, botY1)

		t.Logf("%s appearance — off stop, mean alpha at the window's bottom:", theme)
		t.Logf("  transparency %d%%: %.3f", settings.MinWindowOpacity, lowA)
		t.Logf("  transparency %d%%: %.3f", settings.MaxWindowOpacity, highA)

		// At the slider's bottom the off frame is clearly translucent: the
		// desktop shows through it.
		if lowA >= 0.9 {
			t.Errorf("%s: the off stop at %d%% is %.3f, want a translucent frame", theme, settings.MinWindowOpacity, lowA)
		}
		// And the transparency slider still owns it.
		if highA <= lowA {
			t.Errorf("%s: the off stop's bottom did not rise with the slider: %.3f -> %.3f", theme, lowA, highA)
		}

		// The off stop is thinner than the glass stops at the same slider:
		// it has no haze to raise the tint, only the frame's own alpha.
		offMid := meanBandAlpha(renderStopOpacity(t, theme, "off", 47), botY0, botY1)
		regularMid := meanBandAlpha(renderStopOpacity(t, theme, "regular", 47), botY0, botY1)
		if offMid >= regularMid {
			t.Errorf("%s: off %.3f is not thinner than regular %.3f at the same slider", theme, offMid, regularMid)
		}
	}
}

// absf is the absolute value of a float64, for the colour-sum checks.
func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// The stop chooses the window's own material, which is the blur the panel
// cannot run itself. macOS gets a material per stop; Windows keeps the one
// backdrop that blurs; Linux has none. The off stop removes the material.
func TestVibrancyFollowsTheGlassStop(t *testing.T) {
	cases := []struct {
		goos, step string
		want       mygo.Vibrancy
	}{
		{"darwin", "off", mygo.VibrancyNone},
		{"darwin", "frosted", mygo.VibrancyUnderPage},
		{"darwin", "regular", mygo.VibrancyUnderWindow},
		{"darwin", "liquid", mygo.VibrancySidebar},
		{"darwin", "jelly", mygo.VibrancySidebar}, // a legacy id migrates
		{"windows", "frosted", mygo.VibrancyAcrylic},
		{"windows", "liquid", mygo.VibrancyAcrylic},
		{"linux", "regular", mygo.VibrancyNone},
	}
	for _, tc := range cases {
		if got := vibrancyForStop(tc.goos, tc.step); got != tc.want {
			t.Errorf("vibrancyForStop(%q, %q) = %q, want %q", tc.goos, tc.step, got, tc.want)
		}
	}
}
