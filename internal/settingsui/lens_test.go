package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The settings surface's own lens landings, from the reference's
// `glass-reaxis` census (`src/tests/glass-reaxis.test.ts:47-90`):
// `.extensions-action-button` and `.settings-section__card` (the raised lens)
// and `.settings-switch` (the recessed groove). P7-ej measured that the
// controls contributed ≤0.4 RGB between stops — effectively invisible — so the
// user's 「特别是各种控件按钮」 was still unanswered. These controls now wrap
// the framework's flat pane in `theme.Tokens.Lens`/`LensInset`, and this is the
// pixel acceptance: on a constant background, the control's own pixels must
// ride `--glass-lens-scale` (0.5/1/1.6) with the stop.
func renderLensControl(t *testing.T, step string, w, h int, draw func(c *ui.Context, a *App)) *ui.Tester {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.GlassStep = step }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(ui.DarkTheme())
		ui.Column(c).FillWidth().FillHeight().Background(ui.RGB(28, 29, 33)).Padding(24).Children(func() {
			draw(c, a)
		})
	}, w, h)
	tt.Frame()
	return tt
}

// frameRGBSum is the sum of every pixel's three channels: with a constant
// background it is the controls' own light budget.
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

// rowMeanRGB is the mean colour of one scanline over x0..x1.
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

// The raised lens: the button and the card get strictly brighter with the
// stop (a lit rim and a broad sheen, under a dark shade that is the smaller
// term), so the whole frame's light budget rises.
func TestTheSettingsCardControlsCarryTheStopLens(t *testing.T) {
	landings := []struct {
		selector string
		draw     func(c *ui.Context, a *App)
	}{
		// `.extensions-action-button` / `.settings-save-alert button`.
		{".extensions-action-button", func(c *ui.Context, a *App) {
			a.actionButton(c, "Check")
		}},
		// `.settings-section__card`: the raised pane holding a row.
		{".settings-section__card", func(c *ui.Context, a *App) {
			a.card(c, func() {
				a.row(c, "Row", "hint", func() { a.actionButton(c, "Go") })
			})
		}},
	}
	for _, landing := range landings {
		landing := landing
		t.Run(landing.selector, func(t *testing.T) {
			sums := make(map[string]float64)
			for _, step := range []string{"off", "frosted", "regular", "liquid"} {
				sums[step] = frameRGBSum(renderLensControl(t, step, 400, 120, landing.draw))
				t.Logf("%-26s %-8s rgb sum %.1f", landing.selector, step, sums[step])
			}
			if !(sums["off"] < sums["frosted"] && sums["frosted"] < sums["regular"] && sums["regular"] < sums["liquid"]) {
				t.Errorf("%s: the control's lens did not rise with the stop: off %.0f, frosted %.0f, regular %.0f, liquid %.0f",
					landing.selector, sums["off"], sums["frosted"], sums["regular"], sums["liquid"])
			}
			if d := sums["liquid"] - sums["regular"]; d < 300 {
				t.Errorf("%s: regular→liquid moved %.0f of colour, want a visible spread", landing.selector, d)
			}
		})
	}
}

// The custom-tool form's mode pickers (the reference's
// `.extension-custom-mode`): this build uses the framework's `Segmented` inside
// the recessed lens, so the control must ride the stop like the switch does.
func TestTheCustomModeControlCarriesTheStopLens(t *testing.T) {
	draw := func(c *ui.Context, a *App) {
		index := 0
		a.segmentedControl(c, &index, "Program", "Script")
	}
	sums := make(map[string]float64)
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		sums[step] = frameRGBSum(renderLensControl(t, step, 400, 120, draw))
		t.Logf("%-8s custom-mode rgb sum %.1f", step, sums[step])
	}
	if !(sums["off"] < sums["frosted"] && sums["frosted"] < sums["regular"] && sums["regular"] < sums["liquid"]) {
		t.Errorf("the control's lens did not rise with the stop: off %.0f, frosted %.0f, regular %.0f, liquid %.0f",
			sums["off"], sums["frosted"], sums["regular"], sums["liquid"])
	}
	if d := sums["liquid"] - sums["regular"]; d < 300 {
		t.Errorf("regular→liquid moved %.0f of colour, want a visible spread", d)
	}
}

// The recessed lens: the switch's track is a groove whose *spread* — lit
// bottom edge minus dark top edge — grows with the stop. The reference pins
// this direction too (`the recessed track lens is the raised lens read
// backwards`, the same test file), so a flat sum is not the metric here: the
// two edges move in opposite directions and nearly cancel.
func TestTheSettingsSwitchCarriesTheRecessedStopLens(t *testing.T) {
	// The switch is laid out at the padding's top-left; it is 20 DIPs tall, so
	// its top inner edge and bottom inner edge are the scanlines below the
	// padding. The rows are read across the switch's width, clear of its pill
	// corners.
	const x0, x1 = 30, 80
	draw := func(c *ui.Context, a *App) {
		on := true
		a.switchControl(c, &on)
	}
	spread := make(map[string]float64)
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		tt := renderLensControl(t, step, 200, 80, draw)
		top, bottom := rowMeanRGB(tt, 24, x0, x1), rowMeanRGB(tt, 43, x0, x1)
		spread[step] = bottom - top
		t.Logf("%-8s top %.1f bottom %.1f (spread %.1f)", step, top, bottom, spread[step])
		if bottom <= top {
			t.Errorf("%s: the switch's bottom edge (%.1f) is not lit over its top edge (%.1f)", step, bottom, top)
		}
	}
	if !(spread["off"] < spread["frosted"] && spread["frosted"] < spread["regular"] && spread["regular"] < spread["liquid"]) {
		t.Errorf("the switch's recessed lens did not rise with the stop: off %.1f, frosted %.1f, regular %.1f, liquid %.1f",
			spread["off"], spread["frosted"], spread["regular"], spread["liquid"])
	}
}
