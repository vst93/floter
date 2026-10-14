package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The launcher's result plate and its action-bar plate are one draw point in
// this build (`iconPlate`, the 28u cell every row leads with), and the
// reference's `glass-reaxis` census names it twice: `.launcher-result__icon`
// and `.launcher-action-bar__icon`. The reference's rule is the raised lens
// (`--elev-0`), so the cell must ride the stop's `--glass-lens-scale`; the
// P7-ej measurement was ≤0.4 RGB — invisible. On a constant background this is
// the plate's own light budget.
func renderPlate(t *testing.T, step string) *ui.Tester {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.GlassStep = step }); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(ui.DarkTheme())
		ui.Column(c).FillWidth().FillHeight().Background(ui.RGB(28, 29, 33)).
			Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			a.iconPlate(c, false)
		})
	}, 120, 80)
	tt.Frame()
	return tt
}

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

func TestTheResultPlateCarriesTheStopLens(t *testing.T) {
	sums := make(map[string]float64)
	for _, step := range []string{"off", "frosted", "regular", "liquid"} {
		sums[step] = frameRGBSum(renderPlate(t, step))
		t.Logf("%-8s plate rgb sum %.1f", step, sums[step])
	}
	if !(sums["off"] < sums["frosted"] && sums["frosted"] < sums["regular"] && sums["regular"] < sums["liquid"]) {
		t.Errorf("the plate's lens did not rise with the stop: off %.0f, frosted %.0f, regular %.0f, liquid %.0f",
			sums["off"], sums["frosted"], sums["regular"], sums["liquid"])
	}
	// regular→liquid is the top rung; it must be a visible spread, not a
	// rounding error (the P7-ej failure was under one unit of colour).
	if d := sums["liquid"] - sums["regular"]; d < 50 {
		t.Errorf("regular→liquid moved %.0f of colour on the plate, want a visible spread", d)
	}
}
