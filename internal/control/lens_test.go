package control

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// The segmented control is two of the reference's named lens landings:
// `--glass-reaxis`'s census (`src/tests/glass-reaxis.test.ts:47-90`) names
// `.settings-options--inline` (the track, the *recessed* lens) and
// `.settings-option--active` (the chosen slot, the *raised* lens). The user's
// third hard requirement was 「当前的液态玻璃效果还是不够，特别是各种控件按钮」, and
// this is the pixel acceptance: with the frame held constant, the control's own
// pixels must ride the stop's `--glass-lens-scale` (0.5/1/1.6), so `frosted`,
// `regular` and `liquid` cannot read as one control (the P7-ej measurement was
// ≤0.4 RGB — invisible).
//
// The frame is held constant by construction: the test draws the control on a
// fixed opaque background, so every pixel that moves between two renders is
// the control's own. (The frame's sheen is a separate rung; F3b returns it to
// the reference's fixed value once the controls carry the difference.)
func renderSegmentedStop(t *testing.T, step string, current int) *ui.Tester {
	t.Helper()
	s := settings.Default()
	s.GlassStep = step
	tokens := theme.For(s, true)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		ui.Column(c).FillWidth().FillHeight().Background(ui.RGB(28, 29, 33)).
			Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			Segmented(c, tokens, []string{"Beam", "Block", "Underline"}, current, func(int) {})
		})
	}, 420, 90)
	tt.Frame()
	return tt
}

// frameRGBSum is the sum of every pixel's three channels: with a constant
// background it is the control's own light budget.
func frameRGBSum(tt *ui.Tester) float64 {
	img := tt.Image()
	var sum float64
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			i := y*img.Stride + x*4
			sum += float64(img.Pix[i]) + float64(img.Pix[i+1]) + float64(img.Pix[i+2])
		}
	}
	return sum
}

func TestTheSegmentedControlCarriesTheStopLens(t *testing.T) {
	// Both states of the control: the chosen slot carries the raised lens, and
	// the rest of the track carries the recessed one.
	for _, current := range []int{0, 1} {
		sums := make(map[string]float64)
		for _, step := range []string{"off", "frosted", "regular", "liquid"} {
			sums[step] = frameRGBSum(renderSegmentedStop(t, step, current))
			t.Logf("chosen=%d %-8s rgb sum %.1f", current, step, sums[step])
		}
		// The `off` stop has no glass effect, so `--glass-lens-scale` is 0 and
		// the control is the bare track; the effect stops must light it, and
		// strictly more with each rung.
		if !(sums["off"] < sums["frosted"] && sums["frosted"] < sums["regular"] && sums["regular"] < sums["liquid"]) {
			t.Errorf("chosen=%d: the control's lens did not rise with the stop: off %.0f, frosted %.0f, regular %.0f, liquid %.0f",
				current, sums["off"], sums["frosted"], sums["regular"], sums["liquid"])
		}
		// And the top rung is a *visible* spread, not a rounding error: the
		// regular→liquid step alone must move several thousand units of colour
		// across the control (the P7-ej failure was under a single unit).
		if d := sums["liquid"] - sums["regular"]; d < 3000 {
			t.Errorf("chosen=%d: regular→liquid moved %.0f of colour, want a visible spread", current, d)
		}
	}
}
