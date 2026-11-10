package settingsui

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// The main and terminal sliders expose actual opacity and three clickable,
// magnetic recommendations. There is no second, conflicting opacity readout.
func TestTheTransparencyGroupPrintsActualOpacity(t *testing.T) {
	for _, high := range []bool{false, true} {
		store := newStore(t)
		store.Preview(func(s *settings.Settings) { s.MainOpacity = 10; s.TerminalOpacity = 100 })
		a := New(store, Actions{})
		tt := ui.NewTester(a.View, 720, 2400)
		tt.SetPreferences(ui.Preferences{HighContrast: high, ReduceMotion: true})
		tt.Frame()
		copy := i18n.For("en").Settings
		if _, ok := tt.Find(copy.EffectiveOpacity); ok {
			t.Fatal("duplicate effective-opacity readout")
		}
		for i, label := range []string{copy.MainOpacity, copy.TerminalOpacity} {
			info, ok := tt.AccessByRole(ui.RoleSlider, label)
			if !ok {
				t.Fatalf("missing actual opacity slider %q", label)
			}
			lo, hi := glassmap.OpacityRange(high, i == 1)
			want := lo
			if i == 1 {
				want = hi
			}
			if info.Min != lo || info.Max != hi || info.Now != want || info.Step != 1 {
				t.Fatalf("incorrect actual slider range/value: %+v", info)
			}
		}
		for i, stop := range glassmap.OpacityStops(high, false) {
			label := copy.Percent(uint8(stop))
			if i == 0 {
				label += " · " + copy.TransparencyScaleLow
			}
			if i == 2 {
				label += " · " + copy.TransparencyScaleHigh
			}
			if _, ok := tt.AccessByRole(ui.RoleButton, label); !ok {
				t.Fatalf("missing recommendation %q", label)
			}
		}
	}
}

func TestOpacitySliderSnapsPointerButKeepsKeyboardPrecision(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		for _, high := range []bool{false, true} {
			a := New(newStore(t), Actions{})
			copy := i18n.For("en").Settings
			lo, hi := glassmap.OpacityRange(high, terminal)
			stops := glassmap.OpacityStops(high, terminal)
			label := copy.MainOpacity
			if terminal {
				label = copy.TerminalOpacity
			}
			value := lo
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				a.slider(c, sliderRow{label: label, low: copy.TransparencyScaleLow, high: copy.TransparencyScaleHigh, value: value, lo: lo, hi: hi, opacity: true, terminal: terminal, format: func(v float64) string { return copy.Percent(uint8(v)) }, apply: func(v float64) { value = v }})
			}, 500, 180)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true, HighContrast: high})
			tt.Frame()
			for i, stop := range stops {
				caption := copy.Percent(uint8(stop))
				if i == 0 {
					caption += " · " + copy.TransparencyScaleLow
				}
				if i == 2 {
					caption += " · " + copy.TransparencyScaleHigh
				}
				if err := tt.Click(caption); err != nil {
					t.Fatal(err)
				}
				if value != stop {
					t.Fatalf("click %s stored %v", caption, value)
				}
			}
			track, ok := tt.AccessByRole(ui.RoleSlider, label)
			if !ok {
				t.Fatal("missing slider")
			}
			r := track.Bounds
			pad := float32(9)
			x := r.X + pad + (r.W-2*pad)*float32((stops[1]-1-lo)/(hi-lo))
			y := r.Y + r.H/2
			tt.Press(x, y)
			tt.Frame()
			if value != stops[1] {
				t.Fatalf("terminal=%v high=%v pointer did not snap: %v", terminal, high, value)
			}
			tt.Release(x, y)
			tt.Frame()
			tt.Key(0, ui.KeyRight)
			tt.Frame()
			if value != stops[1]+1 {
				t.Fatalf("keyboard stuck at recommendation: %v", value)
			}
			tt.Key(0, ui.KeyLeft)
			tt.Key(0, ui.KeyLeft)
			tt.Frame()
			if value != stops[1]-1 {
				t.Fatalf("keyboard cannot reach adjacent percent: %v", value)
			}
		}
	}
}

// A window the system gives no material to cannot show the desktop at all
// (shell.windowShowsMaterial paints the card solid there), so the two sliders
// are drawn disabled with the reason instead of silently moving nothing
// (「滑块整条失效」). Whether that state is reachable depends on the platform —
// only a Windows window without a backdrop is in it — so the test reads the
// same predicate the view does.
func TestTheTransparencySlidersExplainWhenTheyCannotMove(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	var vibrancy bool
	tt := ui.NewTester(func(c *ui.Context) {
		vibrancy = c.Vibrancy()
		a.View(c)
	}, 720, 2400)
	tt.Frame()

	available := glassmap.TransparencyAvailable(runtime.GOOS, vibrancy)
	_, explained := tt.Find(copy.TransparencyUnavailable)
	if available == explained {
		t.Fatalf("available=%v but the reason %q is present=%v: the page must explain a slider it cannot honour",
			available, copy.TransparencyUnavailable, explained)
	}
	if available {
		return
	}
	// The reason is under the rows it is about, not floating at the top of the
	// page. Each row draws its own copy (the hint is the row's, drawn after the
	// track), so the question is asked in drawing order rather than of the one
	// box Find returns: the two hints and the two rows come in the same order,
	// and every hint is drawn after the row it belongs to. Comparing the first
	// hint's y to the second row's y instead read as "the reason floats above
	// the terminal slider" on a host whose hint wraps a line lower, which is the
	// font's business, not the page's.
	texts := tt.Texts()
	var reasonAt []int
	rowAt := map[string]int{}
	rows := []string{copy.MainOpacity, copy.TerminalOpacity}
	for i, text := range texts {
		if text == copy.TransparencyUnavailable {
			reasonAt = append(reasonAt, i)
		}
		for _, row := range rows {
			if text == row {
				if _, seen := rowAt[row]; !seen {
					rowAt[row] = i
				}
			}
		}
	}
	if len(reasonAt) < len(rows) {
		t.Fatalf("the reason %q is drawn %d time(s), want one under each of the %d rows: %q",
			copy.TransparencyUnavailable, len(reasonAt), len(rows), texts)
	}
	for i, row := range rows {
		at, ok := rowAt[row]
		if !ok {
			t.Fatalf("the %q row is missing", row)
		}
		if reasonAt[i] < at {
			t.Errorf("the reason is drawn before the %q row (reason #%d at %d, row at %d)", row, i, reasonAt[i], at)
		}
	}
}
