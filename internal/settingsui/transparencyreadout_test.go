package settingsui

import (
	"math"
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/glassmap"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// The transparency group carries one readout line: what the window actually
// shows, once the glass material's own weight is counted
// (glassmap.Spec.Composite). The slider is the ask; this is the answer, and the
// two differ exactly where the material has a floor of its own — the `off`
// stop's readability floor, or a material denser than the slider's ask.
//
// This is the visible half of the coupling fix: the report was that the same
// 47% read differently on different stops, and a number on the page that says
// what the pair resolves to is how a user can tell.
func TestTheTransparencyGroupPrintsItsEffectiveOpacity(t *testing.T) {
	copy := i18n.For("en").Settings

	cases := []struct {
		name  string
		step  string
		alpha uint8
		want  string
	}{
		// The shipped pair: the readout is the slider's own number.
		{"shipped", settings.DefaultGlassStep, settings.DefaultMainOpacity, copy.EffectiveOpacityValue(settings.DefaultMainOpacity)},
		// The `off` stop's floor: a 10% slider with no blur behind it shows the
		// floor instead, and the readout says so rather than repeating the knob.
		// The `off` stop carries no material, so this number is the same on
		// every platform.
		{"off floor", "off", settings.MinWindowOpacity, copy.EffectiveOpacityValue(35)},
		// A material heavier than the ask: on macOS the `liquid` material alone
		// is denser than a 10% slider, so the composite is the material's own.
		// The platform's own number comes from the spec, so this case is the
		// wiring (does the readout print what the spec says) on any host, and
		// the concrete per-platform numbers are glassmap's own tests.
		{"material floor", "liquid", settings.MinWindowOpacity, copy.EffectiveOpacityValue(
			uint8(math.Round(float64(glassmap.SpecForOS(runtime.GOOS, "liquid", settings.MinWindowOpacity).Composite()) * 100)))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newStore(t)
			if err := store.Update(func(s *settings.Settings) {
				s.GlassStep = tc.step
				s.MainOpacity = tc.alpha
			}); err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{})
			tt := render(t, a, 720, 2400)

			label, ok := tt.Find(copy.EffectiveOpacity)
			if !ok {
				t.Fatalf("the effective-opacity readout is missing: %q", tt.Texts())
			}
			// It sits under the two sliders it reads, not above them.
			for _, row := range []string{copy.MainOpacity, copy.TerminalOpacity} {
				slider, ok := tt.Find(row)
				if !ok {
					t.Fatalf("the %q row is missing", row)
				}
				if label.Y <= slider.Y {
					t.Errorf("the readout at %v is not under the %q row at %v", label, row, slider)
				}
			}
			value, ok := tt.Find(tc.want)
			if !ok {
				t.Errorf("the readout does not print %q: %q", tc.want, tt.Texts())
			}
			if ok && value.Y != label.Y {
				t.Errorf("the value at %v is not on the label's line at %v", value, label)
			}
		})
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
	// page.
	reason, _ := tt.Find(copy.TransparencyUnavailable)
	for _, row := range []string{copy.MainOpacity, copy.TerminalOpacity} {
		slider, ok := tt.Find(row)
		if !ok {
			t.Fatalf("the %q row is missing", row)
		}
		if reason.Y <= slider.Y {
			t.Errorf("the reason at %v is not under the %q row at %v", reason, row, slider)
		}
	}
}
