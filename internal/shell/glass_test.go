package shell

import (
	"fmt"
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

// The glass stop is the effect ladder, and its visible rungs are the window's
// OS material (see vibrancyForStop — the blur ladder 10/22/28) and the
// **control** lens (`--glass-lens-scale` 0.5/1/1.6, base.css:784/791/798). The
// frame's own lit face is *not* a rung: base.css's `--sheen-top` /
// `--sheen-bottom` are fixed tokens that do not vary with `[data-glass]`
// (:628-629 dark 0.046/0.01, :906-907 light 0.66/0.22), so every effect stop
// paints the same sheen over the same tint.
//
// This is the F3b acceptance, and it replaces an earlier test that pinned the
// opposite. That test was written when the frame's sheen was graded by the
// stop (`sheen = tint * lens`) because on a machine with no window material
// the three stops otherwise read as one tint (「3 档效果也没看出区别」). P7-em
// then gave the stops a rung that is really theirs — the control lens, graded
// by the per-package pixel tests (launcher's plate, settingsui's card and
// switch, control's segmented track) — so the frame's face can go back to the
// reference's fixed token. The pixels below are the launcher's top band: off
// has no sheen, and frosted / regular / liquid are the **same** colour there,
// while the frame's alpha stays the slider's on every stop — turning the glass
// on does not make the window denser (「开启时又没有透明了」).
func TestTheFrameSheenIsFixedAcrossStops(t *testing.T) {
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

		offR, offG, offB := meanBandRGB(off, topY0, topY1)
		fR, fG, fB := meanBandRGB(frosted, topY0, topY1)
		rR, rG, rB := meanBandRGB(regular, topY0, topY1)
		lR, lG, lB := meanBandRGB(liquid, topY0, topY1)

		t.Logf("%s appearance — mean alpha / mean colour in the frame's top band:", theme)
		t.Logf("  off     alpha %.3f  rgb %.1f/%.1f/%.1f", offA, offR, offG, offB)
		t.Logf("  frosted alpha %.3f  rgb %.1f/%.1f/%.1f", fA, fR, fG, fB)
		t.Logf("  regular alpha %.3f  rgb %.1f/%.1f/%.1f", rA, rR, rG, rB)
		t.Logf("  liquid  alpha %.3f  rgb %.1f/%.1f/%.1f", lA, lR, lG, lB)
		// The material parameters the pixels come from, for the record.
		for _, step := range []string{"frosted", "regular", "liquid"} {
			spec := glassmap.SpecFor(step, settings.DefaultMainOpacity)
			t.Logf("  %-8s material: blur=%.0f saturate=%.0f%% lens=%.1f tint=%.3f haze=%.2f", step, spec.Blur, spec.Saturate, spec.LensScale, spec.TintAlpha, spec.HazeAlpha)
		}

		// Off is the plain translucent frame: no glass effect, but the
		// window's own transparency still paints. It must not be the opaque
		// stand-in that made the app's transparent bottom fail when the glass
		// was turned off (「关闭玻璃效果的时候，应用的透明底是失效的」).
		if offA >= 0.99 {
			t.Errorf("%s: the off stop is %.3f, want the frame's translucent tint", theme, offA)
		}
		// Turning the glass on must not make the frame's *fill* denser: the
		// tint is the slider's own alpha on every stop (see the glassmap test),
		// so what the stop adds is only the sheen — a lit highlight, a few
		// hundredths — never the haze's lift. This is the pixel-level check of
		// 「开启时又没有透明了」: the effect stops sit on the off frame's
		// transparency, not above it.
		for _, row := range []struct {
			name string
			a    float32
		}{{"frosted", fA}, {"regular", rA}, {"liquid", lA}} {
			if row.a-offA > 0.2 {
				t.Errorf("%s: %s is %.3f, %.3f denser than the off frame %.3f — the glass must not cost transparency",
					theme, row.name, row.a, row.a-offA, offA)
			}
		}
		// The sheen is a lit highlight, so the effect stops are clearly
		// lighter than the off frame in this band.
		luminance := func(r, g, b float64) float64 { return r + g + b }
		offL := luminance(offR, offG, offB)
		fL := luminance(fR, fG, fB)
		rL := luminance(rR, rG, rB)
		lL := luminance(lR, lG, lB)
		if fL-offL < 5 {
			t.Errorf("%s: the effect stops only lit the frame by %.1f of colour (off %.1f, frosted %.1f); want a visible sheen",
				theme, fL-offL, offL, fL)
		}
		// And the stop does **not** grade the frame's face: the reference's
		// sheen is the same fixed token on every effect stop, so the three
		// stops must be the same colour in this band (the residual few tenths
		// are the control lens bleeding into the band mean). The stops' own
		// difference is the control lens, graded by the per-package pixel
		// tests — see the census in glasscensus_test.go.
		if spread := max(lL, max(fL, rL)) - min(lL, min(fL, rL)); spread > 1 {
			t.Errorf("%s: the stops moved the frame's own face by %.2f of colour (frosted %.1f, regular %.1f, liquid %.1f); the sheen is a fixed token, not a rung",
				theme, spread, fL, rL, lL)
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

		// The off stop and the effect stops share the transparency floor: the
		// stop adds the sheen (a lit highlight), never the haze's lift, so the
		// two are within the sheen's few hundredths at the same slider.
		offMid := meanBandAlpha(renderStopOpacity(t, theme, "off", 47), botY0, botY1)
		regularMid := meanBandAlpha(renderStopOpacity(t, theme, "regular", 47), botY0, botY1)
		if regularMid-offMid > 0.2 {
			t.Errorf("%s: regular %.3f is %.3f denser than off %.3f at the same slider", theme, regularMid, regularMid-offMid, offMid)
		}
	}
}

// Turning the glass **on** must not cost the transparency either. The off stop
// was fixed first, but the effect stops raised the frame's alpha by the stop's
// haze, so a glass-on frame was denser than the off one and the slider's low
// end went inert — exactly the report 「关闭液态玻璃效果时透明底可以了，但是开启时
// 又没有透明了」. This is the pixel-level acceptance: at the slider's bottom
// every stop is as thin as the off frame, and the slider still owns the bottom
// band on every stop.
func TestTurningTheGlassOnKeepsTheTransparency(t *testing.T) {
	const botY0, botY1 = 350, 400

	for _, theme := range []string{"dark", "light"} {
		offLow := meanBandAlpha(renderStopOpacity(t, theme, "off", settings.MinWindowOpacity), botY0, botY1)
		t.Logf("%s appearance — mean alpha at the window's bottom, transparency %d%%:", theme, settings.MinWindowOpacity)
		t.Logf("  off     %.3f", offLow)
		for _, step := range []string{"frosted", "regular", "liquid"} {
			low := meanBandAlpha(renderStopOpacity(t, theme, step, settings.MinWindowOpacity), botY0, botY1)
			high := meanBandAlpha(renderStopOpacity(t, theme, step, settings.MaxWindowOpacity), botY0, botY1)
			t.Logf("  %-8s low %.3f  high %.3f", step, low, high)
			// The glass-on frame is translucent at the slider's bottom, and the
			// slider still moves it: the stop cannot hold the window open.
			if low >= 0.5 {
				t.Errorf("%s: %s at %d%% is %.3f, want a translucent frame", theme, step, settings.MinWindowOpacity, low)
			}
			if low-offLow > 0.15 {
				t.Errorf("%s: %s at %d%% is %.3f, %.3f denser than the off frame %.3f", theme, step, settings.MinWindowOpacity, low, low-offLow, offLow)
			}
			if high <= low {
				t.Errorf("%s: %s's bottom did not rise with the slider: %.3f -> %.3f", theme, step, low, high)
			}
		}
	}
}

// The frame's alpha is the transparency slider's on **every** effect stop, and
// the three stops agree with each other band for band: the material's own
// weight is compensated out of the tint (glassmap.MaterialVeil), so one slider
// position is one solidity whether the stop is `frosted`, `regular` or
// `liquid`. That is the pixel-level acceptance of the report 「同一个透明度在三
// 档下观感不同」.
//
// The `off` stop is a **separate** reading, and the one place the two ladders
// are allowed to differ: it carries no material, so it carries a readability
// floor instead (glassmap.OffFrameFloor) — at the slider's bottom end a plain
// tint with no blur behind it would put the text on the wallpaper. Every other
// slider position is the slider's own value on every stop.
//
// The layer that used to break this was the **frame's sheen**, painted as a
// separate translucent layer over the fill. The sheen is a stop-graded
// highlight, so only the glass-on frames carried it: it composites its own
// alpha over the fill, and the light appearance added up to 0.11 of alpha in
// the result band at the default 47%. `theme.Tokens.FaceFill` bakes the sheen
// into the fill's own alpha budget, so the stop grades the lit face's colour,
// never the frame's thickness.
func TestTheFrameAlphaIsTheSlidersOnEveryStop(t *testing.T) {
	// The window's own background bands, all below the field block: the top of
	// the list, its middle and its bottom.
	bands := []struct {
		name   string
		y0, y1 int
	}{
		{"top", 90, 120},
		{"middle", 200, 260},
		{"bottom", 350, 400},
	}
	stops := []string{"frosted", "regular", "liquid"}

	for _, theme := range []string{"dark", "light"} {
		for _, opacity := range []uint8{settings.MinWindowOpacity, settings.DefaultMainOpacity, settings.MaxWindowOpacity} {
			off := renderStopOpacity(t, theme, "off", opacity)
			offBottom := meanBandAlpha(off, bands[2].y0, bands[2].y1)
			t.Logf("%s appearance, transparency %d%% — mean alpha per band:", theme, opacity)
			for _, band := range bands {
				offA := meanBandAlpha(off, band.y0, band.y1)
				line := fmt.Sprintf("  %-7s off %.3f", band.name, offA)
				first := meanBandAlpha(renderStopOpacity(t, theme, stops[0], opacity), band.y0, band.y1)
				for _, step := range stops[1:] {
					a := meanBandAlpha(renderStopOpacity(t, theme, step, opacity), band.y0, band.y1)
					line += fmt.Sprintf("  %s %.3f (%+.3f)", step, a, a-first)
					// One alpha truth: the stops are the same thickness, so a
					// stop switch cannot change how solid the window is.
					if d := a - first; d > 0.01 || d < -0.01 {
						t.Errorf("%s %d%% %s: %s is %.3f, %+.3f from %s %.3f — the stops must share one alpha",
							theme, opacity, band.name, step, a, d, stops[0], first)
					}
				}
				t.Log(line)
			}
			// The `off` stop's floor is the one rung of its own: never thinner
			// than the floor, and (with no material behind it) the densest
			// reading at the slider's bottom end.
			if offBottom < glassmap.OffFrameFloor-0.02 {
				t.Errorf("%s %d%%: the off frame's bottom is %.3f, want at least the readability floor %.2f",
					theme, opacity, offBottom, glassmap.OffFrameFloor)
			}
			if opacity == settings.MinWindowOpacity {
				thin := meanBandAlpha(renderStopOpacity(t, theme, "regular", opacity), bands[2].y0, bands[2].y1)
				if offBottom <= thin {
					t.Errorf("%s: the off frame (%.3f) is not denser than the blurred regular frame (%.3f) at the slider's bottom — the floor is inert",
						theme, offBottom, thin)
				}
			}
		}
	}
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
		// `off` removes the material on every platform, not only on macOS:
		// the early return this used to take for Windows and Linux kept
		// Windows' Acrylic, so a glass-off Windows window still blurred what
		// was behind it (「关了玻璃还是有磨砂」).
		{"windows", "off", mygo.VibrancyNone},
		{"linux", "regular", mygo.VibrancyNone},
		{"linux", "off", mygo.VibrancyNone},
	}
	for _, tc := range cases {
		if got := vibrancyForStop(tc.goos, tc.step); got != tc.want {
			t.Errorf("vibrancyForStop(%q, %q) = %q, want %q", tc.goos, tc.step, got, tc.want)
		}
	}
}
