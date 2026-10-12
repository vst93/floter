package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// sample reads one pixel of the last frame, at a logical coordinate.
func sample(tt *ui.Tester, x, y float32) ui.Color {
	img := tt.Image()
	i := int(y)*img.Stride + int(x)*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// The transparency slider moves the app's own background, not just a tint
// hidden under a near-opaque pane. This renders the launcher at the two ends
// of the slider and reads the content pane's composited alpha: the reference's
// fixed `--surface-sunken` band left it at ~0.86 -> ~1.0 in the light
// appearance and ~0.83 -> ~1.0 in the dark one (a change the user read as
// 「不起作用」), while a pane that rides the frame tint moves it over a far wider
// band. The material the previous round changed sits *behind* this fill, so no
// window material can move this number.
func TestTheSliderMovesTheRenderedBackground(t *testing.T) {
	alphaAt := func(theme string, opacity uint8) float32 {
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) {
			s.Theme = theme
			s.MainOpacity = opacity
		}); err != nil {
			t.Fatal(err)
		}
		a := New(Options{Store: store})
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
		tt.Frame()
		// A point in the launcher's result pane, below the field row and
		// clear of the rows' text.
		return float32(sample(tt, 360, 400).A) / 255
	}

	for _, theme := range []string{"dark", "light"} {
		thin := alphaAt(theme, settings.MinWindowOpacity)
		thick := alphaAt(theme, settings.MaxWindowOpacity)
		if thick-thin < 0.15 {
			t.Errorf("%s: the slider moved the background by %v (%.3f -> %.3f); want a visible change",
				theme, thick-thin, thin, thick)
		}
		if thin > 0.85 {
			t.Errorf("%s: the background at %d%% is %.3f, want translucent", theme, settings.MinWindowOpacity, thin)
		}
		if thick < 0.97 {
			t.Errorf("%s: the background at %d%% is %.3f, want near-solid", theme, settings.MaxWindowOpacity, thick)
		}
	}
}
