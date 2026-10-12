package shell

import (
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// bandAlpha is the mean alpha, 0..1, of a horizontal band of the rendered
// frame: the pixels of the window's own background, read straight off the
// production surface frame (ui.Tester renders through the same engine the
// window uses).
func bandAlpha(tt *ui.Tester, y0, y1 int) float32 {
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

// renderWithOpacity renders one launcher frame at a stored transparency.
func renderWithOpacity(t *testing.T, theme string, opacity uint8) *ui.Tester {
	t.Helper()
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
	return tt
}

// The transparency slider moves the app's own background — every band of it,
// including the bottom the user reads. This is the pixel-level acceptance the
// previous rounds did not have: it reads the alpha of the rendered frame at
// each slider stop, not an internal token.
//
// Two earlier rounds failed here. The first changed the window material but
// the content pane (painted at a fixed 0.62-0.80 band) covered it, so the
// visible bottom barely moved. The second made the pane ride the slider but
// painted it at the *frame's* own alpha, stacking two full tints: the visible
// bottom was `1-(1-t)^2`, 0.76 at the slider's bottom, and the slider moved it
// over half the band it moves the frame. The pane is now the over-frame fill
// that composites to the frame tint plus a small recess, so the bottom is one
// sheet the slider owns.
func TestTheSliderMovesTheRenderedBackground(t *testing.T) {
	// The slider's stops, and the opacity each stores. 0 is "unset" to the
	// loader (the reference's normalize_window_opacity rule) and lands on the
	// shipped 47; the shipped band is 10-100.
	stops := []struct {
		label  string
		stored uint8
		want   uint8
	}{
		{"0%", 0, settings.DefaultMainOpacity},
		{"10%", 10, 10},
		{"25%", 25, 25},
		{"50%", 50, 50},
		{"75%", 75, 75},
		{"100%", 100, 100},
	}
	// The bands of the window's own background, all below the field block:
	// the top of the list, its middle, and its bottom. The field block itself
	// (the top ~56px) is deliberately near-solid - the reference's R21 - so the
	// "top" band starts under it.
	const topY0, topY1 = 90, 120
	const midY0, midY1 = 200, 260
	const botY0, botY1 = 350, 400

	for _, theme := range []string{"dark", "light"} {
		type row struct {
			label         string
			eff           uint8
			top, mid, bot float32
		}
		rows := make([]row, 0, len(stops))
		for _, s := range stops {
			if eff := settings.NormalizeOpacity(s.stored, settings.DefaultMainOpacity); eff != s.want {
				t.Fatalf("%s: NormalizeOpacity(%d) = %d, want %d", s.label, s.stored, eff, s.want)
			}
			tt := renderWithOpacity(t, theme, s.stored)
			rows = append(rows, row{
				label: s.label,
				eff:   s.want,
				top:   bandAlpha(tt, topY0, topY1),
				mid:   bandAlpha(tt, midY0, midY1),
				bot:   bandAlpha(tt, botY0, botY1),
			})
		}

		// A stored 0 is "unset" to the loader and lands on the shipped 47, so
		// the rows are sorted by their effective value before the monotonic
		// checks: the slider's real band is 10-100.
		sort.Slice(rows, func(i, j int) bool { return rows[i].eff < rows[j].eff })

		t.Logf("%s appearance — mean alpha per band:", theme)
		for _, r := range rows {
			t.Logf("  opacity %3d%%: top %.3f  middle %.3f  bottom %.3f", r.eff, r.top, r.mid, r.bot)
		}

		// The bottom must rise with the slider, stop by stop.
		for i := 1; i < len(rows); i++ {
			if rows[i].bot <= rows[i-1].bot {
				t.Errorf("%s: the bottom did not rise from %s (%.3f) to %s (%.3f)",
					theme, rows[i-1].label, rows[i-1].bot, rows[i].label, rows[i].bot)
			}
		}
		low, high := rows[0], rows[len(rows)-1]
		// A visible change over the slider's whole range.
		if high.bot-low.bot < 0.25 {
			t.Errorf("%s: the bottom moved only %.3f (%.3f -> %.3f); want a visible change",
				theme, high.bot-low.bot, low.bot, high.bot)
		}
		// The bottom end is genuinely translucent, not the near-opaque bar the
		// double-stacked pane drew (0.76 dark / 0.81 light).
		if low.bot > 0.68 {
			t.Errorf("%s: the bottom at %s is %.3f, want translucent", theme, low.label, low.bot)
		}
		// The top end reaches the near-solid face the slider owns.
		if high.bot < 0.95 {
			t.Errorf("%s: the bottom at %s is %.3f, want near-solid", theme, high.label, high.bot)
		}
		// Every band moves, not just one: the middle and the frame tint above
		// the field ride the same slider.
		if high.mid-low.mid < 0.25 {
			t.Errorf("%s: the middle moved only %.3f (%.3f -> %.3f)", theme, high.mid-low.mid, low.mid, high.mid)
		}
		if high.top-low.top < 0.25 {
			t.Errorf("%s: the top moved only %.3f (%.3f -> %.3f)", theme, high.top-low.top, low.top, high.top)
		}
	}
}

// The content pane is one sheet, not a second full tint: the pane's alpha is
// chosen so the visible recess composites to the frame tint plus the glassmap
// step. This is the mechanism behind the pixels above, pinned where a later
// round could quietly reintroduce the double stack.
func TestTheContentPaneIsNotASecondFullTint(t *testing.T) {
	tt := renderWithOpacity(t, "dark", settings.MinWindowOpacity)
	// With the frame tint at ~0.505 and the pane's own alpha ~0.12, the
	// visible bottom is ~0.565 - not the 0.755 a frame-alpha pane would draw.
	bot := bandAlpha(tt, 350, 400)
	if bot > 0.65 {
		t.Errorf("the low-stop bottom = %.3f, want the single-sheet ~0.57 not the 0.76 double stack", bot)
	}
}

// The framework's own bottom: what the window paints before the view does, the
// layer the user reads as「整个框架的最底层背景」. There are exactly two layers
// below the app's view, and this pins both:
//
//  1. the scene's clear color — mygo ui/paint.go resets every frame to
//     scene.Color{} (fully transparent); the rasterizer and the GPU renderers
//     both honor its alpha;
//  2. the root element — the context gives it the theme's background, an
//     OPAQUE fill by default (the stock root renders at alpha 255), which the
//     view must explicitly clear.
//
// So the framework's bottom is the root's own background, not a hidden
// compositor layer: a themed transparent root leaves the whole window at
// alpha 0, which is what the app's View does (shell.go: c.Root().Background(
// ui.Transparent)). If this test ever fails the opaque way, a transparent
// root no longer clears the window — and the transparency slider stops owning
// the frame.
func TestTheFrameworkPaintsNoBottomFill(t *testing.T) {
	maxAlpha := func(tt *ui.Tester) uint8 {
		img := tt.Image()
		max := uint8(0)
		for y := 0; y < img.Rect.Dy(); y++ {
			for x := 0; x < img.Rect.Dx(); x++ {
				if a := img.Pix[y*img.Stride+x*4+3]; a > max {
					max = a
				}
			}
		}
		return max
	}
	// A view that draws nothing: the framework's own layer alone — the root
	// with the stock theme's opaque background, which is exactly the bottom
	// an app that never touches the root gets.
	empty := ui.NewTester(func(c *ui.Context) {}, 200, 120)
	empty.Frame()
	if a := maxAlpha(empty); a != 255 {
		t.Errorf("the stock root is not the opaque default: max alpha %d, want 255", a)
	}
	// Our own root shape: SetTheme then a transparent root — what View does.
	app := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(ui.LightTheme())
		c.Root().Background(ui.Transparent)
	}, 200, 120)
	app.Frame()
	if a := maxAlpha(app); a != 0 {
		t.Errorf("a themed transparent root left a bottom: max alpha %d, want 0", a)
	}
}
