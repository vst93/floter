package control

import (
	"bytes"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// A material stop changes the window glass, leaving choice controls quiet.
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

func TestMaterialStopsKeepSegmentDecorationStable(t *testing.T) {
	for _, current := range []int{0, 1} {
		baseline := renderSegmentedStop(t, "off", current).Image()
		for _, step := range []string{"frosted", "regular", "liquid"} {
			img := renderSegmentedStop(t, step, current).Image()
			if !bytes.Equal(baseline.Pix, img.Pix) {
				t.Errorf("chosen=%d: %s adds decoration to the segmented control", current, step)
			}
		}
	}
}
