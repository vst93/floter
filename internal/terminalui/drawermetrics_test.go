package terminalui

import (
	"fmt"
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// drawerMetricsFrame draws just the drawer's strip, over a black root, with the
// reference's own metrics resolved from the frame it is drawn in (the tester's
// preferences round `Space`, so a theme built beside it would not agree).
type drawerMetricsFrame struct {
	frame  *ui.Tester
	tokens theme.Tokens
}

func drawDrawerMetrics(t *testing.T, a *App) drawerMetricsFrame {
	t.Helper()
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.drawer(c, i18n.For("en").Terminal)
	}, 900, 400)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return drawerMetricsFrame{frame: tt, tokens: tok}
}

func (f drawerMetricsFrame) mustFind(t *testing.T, text string) ui.Rect {
	t.Helper()
	box, ok := f.frame.Find(text)
	if !ok {
		t.Fatalf("the drawer does not draw %q: %v", text, f.frame.Texts())
	}
	return box
}

// inkRun is the extent of the pixels brighter than a threshold within a window
// of the frame: the knob's own white lens over the strip's grey track, or the
// accent ink of a value.
type inkRun struct {
	min, max int
}

func (r inkRun) empty() bool     { return r.max < 0 }
func (r inkRun) center() float32 { return float32(r.min+r.max) / 2 }

// brightRun scans a window of the last frame for the pixels a threshold
// catches.
func brightRun(tt *ui.Tester, x0, x1, y0, y1, threshold int) inkRun {
	img := tt.Image()
	out := inkRun{min: 1 << 30, max: -1}
	for y := max(0, y0); y < min(img.Rect.Dy(), y1); y++ {
		for x := max(0, x0); x < min(img.Rect.Dx(), x1); x++ {
			i := y*img.Stride + x*4
			if int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2]) > threshold {
				if x < out.min {
					out.min = x
				}
				if x > out.max {
					out.max = x
				}
			}
		}
	}
	return out
}

// liveKnobCenter renders the real drawer with the font-size setting at one end
// of its range and returns the centre of the range's knob ink — the slider the
// production code draws, moved by the setting it edits, between the label and
// its readout.
func liveKnobCenter(t *testing.T, a *App, size int) float32 {
	t.Helper()
	if err := a.Store.Update(func(s *settings.Settings) { s.FontSize = size }); err != nil {
		t.Fatal(err)
	}
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.drawer(c, i18n.For("en").Terminal)
	}, 900, 400)
	tt.Frame()
	en := i18n.For("en").Settings
	label, ok := tt.Find(en.FontSize)
	if !ok {
		t.Fatalf("the drawer does not draw the range's label %q: %v", en.FontSize, tt.Texts())
	}
	readout, ok := tt.Find(fmt.Sprintf("%d px", size))
	if !ok {
		t.Fatalf("the drawer does not draw the readout %q: %v", fmt.Sprintf("%d px", size), tt.Texts())
	}
	// The knob is the one bright lens in the range: hunt between the label and
	// the readout for the whitest pixels.
	run := brightRun(tt, int(label.X+label.W)+2, int(readout.X), int(label.Y)-8, int(label.Y+label.H)+8, 600)
	if run.empty() {
		t.Fatal("the drawer's range drew no knob")
	}
	return run.center()
}

// probeKnobCenter lays the same range at one width step on a bare root and
// returns its knob's centre; the pair of probes is the ruler a live reading is
// held against, at the reference's width and at the old port's.
func probeKnobCenter(t *testing.T, tok theme.Tokens, step float32, size float64) float32 {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		ui.Row(c).FillWidth().Padding(tok.Theme.Space(2), tok.Theme.Space(2)).Children(func() {
			ui.Slider(c, &size, 8, 48).Width(tok.Theme.Space(step))
		})
	}, 400, 60)
	tt.Frame()
	run := brightRun(tt, 0, 400, 0, 60, 600)
	if run.empty() {
		t.Fatal("the probe slider drew no knob")
	}
	return run.center()
}

// The drawer's range slider is the reference's `.terminal-setting-range`
// (`terminal.css:677-692`): a fixed `width: 110px` track — 27.5 units, not the
// 22 (88px) the old port wrote. The live drawer is read: the font-size setting
// is moved to both ends of its range and the distance the range's own knob
// travels is held against the same slider laid out at the reference's width and
// at the old port's. The knob travels the track minus its own two-radius inset,
// so the two widths are what the reading distinguishes.
func TestTheDrawerSliderIsTheReferenceWidth(t *testing.T) {
	if drawerRangeWidth != 27.5 {
		t.Fatalf("drawerRangeWidth = %v, want the reference's 110px = 27.5u", drawerRangeWidth)
	}
	a := drawerApp(t)
	tok := drawDrawerMetrics(t, a).tokens

	live := liveKnobCenter(t, a, 8) - liveKnobCenter(t, a, 48)
	reference := probeKnobCenter(t, tok, drawerRangeWidth, 8) - probeKnobCenter(t, tok, drawerRangeWidth, 48)
	old := probeKnobCenter(t, tok, 22, 8) - probeKnobCenter(t, tok, 22, 48)
	if reference == old {
		t.Fatalf("the probe cannot tell the two widths apart (both travel %v)", reference)
	}
	if absFloat32(live-reference) > absFloat32(live-old) {
		t.Errorf("the live range's knob travels %v, closer to the old port's 22u width (%v) than the reference's 110px (%v)",
			live, old, reference)
	}
}

// textWidthAt lays one line at a register and reads its natural width — the
// only handle a frame gives a test on a font size (the framework's text boxes
// are the ruler internal/settingsui's register tests use too).
func textWidthAt(t *testing.T, tok theme.Tokens, text string, mutate func(ui.Element) ui.Element) float32 {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		ui.Row(c).Children(func() {
			mutate(ui.Text(c, text).SingleLine().Shrink(0))
		})
	}, 400, 60)
	tt.Frame()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the register probe did not draw %q", text)
	}
	return box.W
}

// The range's live readout is the reference's `.terminal-setting-control__value`
// (`settings.css:854-865`): `--text-body` in monospace with `tabular-nums` at
// 600, in the accent — not the muted emphasis step one size down the old port
// wrote. The live strip is read: the ink's own box against the same string laid
// out at the body and emphasis registers, and its colour against the frame's
// accent.
func TestTheDrawerReadoutIsTheAccentBodyRegister(t *testing.T) {
	a := drawerApp(t)
	f := drawDrawerMetrics(t, a)
	tok := f.tokens

	const readout = "14 px"
	rect := f.mustFind(t, readout)

	// The mono body register, with the reference's features and weight.
	want := textWidthAt(t, tok, readout, func(e ui.Element) ui.Element {
		return e.Font(terminalSettingValueFont).FontSize(tok.Body).FontWeight(600).FontFeatures("tnum")
	})
	emphasis := textWidthAt(t, tok, readout, func(e ui.Element) ui.Element {
		return e.FontSize(tok.Emphasis).TextColor(tok.Theme.TextMuted)
	})
	if absFloat32(rect.W-want) > absFloat32(rect.W-emphasis) {
		t.Errorf("the readout is not the body step (box %v, body probe %v, emphasis probe %v)", rect.W, want, emphasis)
	}

	// Its ink is the accent: an antialiased accent-only ink over the black root
	// is `c * {6, 74, 166}`, so its blue-minus-red gap is far wider than a grey
	// ink's (the muted step is near-neutral, its gap a few units).
	img := f.frame.Image()
	bestBR, bestGR := -1, -1
	for y := int(rect.Y); y < int(rect.Y+rect.H)+2; y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			i := y*img.Stride + x*4
			r, g, b := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
			if b-r > bestBR {
				bestBR, bestGR = b-r, g-r
			}
		}
	}
	if bestBR < 40 || bestGR < 15 {
		t.Errorf("the readout's ink is not the accent: its strongest pixel is blue-minus-red %d, green-minus-red %d (want the accent's ~160/~68 scaled by coverage)", bestBR, bestGR)
	}
}

// The strip is a wrapping grid whose cells have the reference's floor,
// `minmax(230px, 1fr)` (`terminal.css:654-658`): 57.5 units. The row count of
// the live grid is the observable: rendering the real `drawerGrid` at both the
// reference's 57.5u floor and the old port's 60u (240px) one, and bisecting for
// the window width at which the grid drops from three row-lines to two, shows
// the shelf moves with the floor. Between the two breakpoints the reference's
// grid folds onto two lines and the old port's still needs a third.
func TestTheDrawerGridCellsKeepTheReferenceFloor(t *testing.T) {
	if drawerCellFloor != 57.5 {
		t.Fatalf("drawerCellFloor = %v, want the reference's 230px = 57.5u", drawerCellFloor)
	}
	en := i18n.For("en").Settings
	labels := []string{
		en.FontFamily, en.FontSize, en.LineHeight, en.Padding,
		en.CursorShape, en.CursorBlink, en.Palette, en.SelectCopy, en.PasteSafe,
	}
	rowsAt := func(floorStep float32, w int) int {
		a := drawerApp(t)
		tt := ui.NewTester(func(c *ui.Context) {
			tok := tokensOf(a, c)
			c.SetTheme(tok.Theme)
			c.Root().Background(ui.RGBA(0, 0, 0, 1))
			a.drawerGrid(c, tok.Theme.Space(floorStep))
		}, w, 600)
		tt.Frame()
		var ys []float32
		for _, label := range labels {
			if r, ok := tt.Find(label); ok {
				ys = append(ys, r.Y)
			}
		}
		sort.Slice(ys, func(i, j int) bool { return ys[i] < ys[j] })
		n := 0
		var last float32 = -100
		for _, y := range ys {
			if y-last > 3 {
				n++
			}
			last = y
		}
		return n
	}
	// The width at which the grid stops needing three row-lines.
	breakpoint := func(floorStep float32) int {
		lo, hi := 300, 1400
		for lo < hi {
			mid := (lo + hi) / 2
			if rowsAt(floorStep, mid) <= 2 {
				hi = mid
			} else {
				lo = mid + 1
			}
		}
		return lo
	}
	reference, old := breakpoint(drawerCellFloor), breakpoint(60)
	if reference >= old {
		t.Fatalf("the three-line breakpoint at the reference's floor (%d) is not below the old 60u floor's (%d): the floor did not move the shelf", reference, old)
	}
	mid := (reference + old) / 2
	if got, want := rowsAt(drawerCellFloor, mid), rowsAt(60, mid); got >= want {
		t.Errorf("the %dpx window holds %d row-lines at the reference's floor and %d at the old 60u one: want fewer at the reference's", mid, got, want)
	}
}
