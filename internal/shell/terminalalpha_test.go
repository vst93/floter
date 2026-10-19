package shell

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/glassmap"
	"floter/internal/settings"
)

// newTerminalApp is an app whose terminal sessions are stubs. A stub session
// cannot be drawn (the plugin's emulator needs its native library), so these
// frames are the terminal page *without* a session: the empty page still paints
// the two layers the transparency is split between — the card's face and the
// terminal panel's own background — which is what the numbers below read.
func newTerminalApp(t *testing.T, mutate func(*settings.Settings)) *App {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if mutate != nil {
		if err := store.Update(mutate); err != nil {
			t.Fatal(err)
		}
	}
	return New(Options{
		Store: store,
		Paths: extensions.FromRoot(t.TempDir()),
		NewTerminal: func(terminal.Options) (*terminal.Terminal, error) {
			return &terminal.Terminal{}, nil
		},
		OpenAtLogin:    func() bool { return false },
		SetOpenAtLogin: func(bool) error { return nil },
	})
}

// renderTerminalPage renders one frame of the terminal surface, empty.
func renderTerminalPage(t *testing.T, mutate func(*settings.Settings)) *ui.Tester {
	t.Helper()
	a := newTerminalApp(t, mutate)
	a.Surf = SurfaceTerminal
	s := a.Store.Snapshot()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, int(s.TerminalWidth), int(s.TerminalHeight))
	tt.Frame()
	return tt
}

// The terminal's transparency is the terminal slider's, once.
//
// The page is two layers by design — the card's face, which the title bar and
// the panel's margin sit on, and the terminal panel's own background, which is
// the colour the user picked — and both used to paint the slider, so 46%
// composited to `1-(1-0.46)^2` = 71% and the knob lied by half its range
// (「终端透明度滚轴说的和看到的不一致」). The face now carries
// glassmap.TerminalFrameShare of the alpha and the panel the rest, so the
// rendered page is the slider's own alpha; painting either layer at the full
// slider again puts the page back over 0.6 and this goes red.
func TestTheTerminalsVisibleAlphaIsTheSlider(t *testing.T) {
	for _, opacity := range []uint8{settings.DefaultTerminalOpacity, 70} {
		tt := renderTerminalPage(t, func(s *settings.Settings) {
			s.TerminalOpacity = opacity
		})
		// A band in the middle of the panel, clear of the bar and the drawer.
		got := meanBandAlpha(tt, 300, 340)
		want := float32(opacity) / 100
		if d := got - want; d > 0.03 || d < -0.03 {
			t.Errorf("the terminal at %d%% renders at %.3f, want the slider's %.3f (a second full tint would read %.3f)",
				opacity, got, want, 1-(1-want)*(1-want))
		}
	}
	// The two layers are the split the spec names, not a number the view made
	// up: the face's share and the panel's share composite to the slider.
	spec := glassmap.SpecForOS(runtime.GOOS, "regular", settings.DefaultTerminalOpacity)
	if got, want := spec.TerminalFrameAlpha(), spec.TintAlpha*glassmap.TerminalFrameShare; got != want {
		t.Errorf("the terminal's frame share = %v, want %v", got, want)
	}
	if spec.CanvasAlpha() <= spec.TerminalFrameAlpha() {
		t.Errorf("the panel's share %v is not the larger one", spec.CanvasAlpha())
	}
}
