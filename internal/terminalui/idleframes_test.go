package terminalui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P11-bm · A status dot nobody can see asks for no frames.
//
// The dot is the identity row's leading mark, and the identity row is drawn at
// `Opacity(0)` until the bar is revealed (`terminalui.go`'s `revealed`). Before
// this round the dot read `Element.Loop` in every frame anyway, and `Loop` asks
// the window to *rebuild* the view (`ui/widgets.go:69-84` →
// `Context.AnimationFrame` → `runtime.go:744-748`). A terminal page left open
// with the pointer off the bar therefore rebuilt the whole terminal view on
// every display refresh — 120 times a second on a 120 Hz panel — for an
// animation at zero opacity.
//
// The assertion is the frame the page asks for next, the same measure R161 used
// for the reduce-motion arrival: a page with nothing visible moving must ask
// for none.
//
// Mutation M1 (drop the `if !drawn { return }` guard) turns the first arm red;
// M2 (pass `true` at the call site) turns it red the same way.
func TestTheAtRestStatusDotAsksForNoFrames(t *testing.T) {
	a := New(settings.NewStore(settings.Default()), Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokensOf(a, c).Theme)
		a.View(c)
	}, 900, 600)

	// The pointer rests on the canvas, clear of the bar: the bar is not
	// revealed, so the identity row and the dot in it are at zero opacity.
	tt.Move(450, 400)
	tt.Frame()
	if tt.FramePending() {
		t.Errorf("the at-rest terminal page asked for another frame: the invisible status dot still reads its loop")
	}

	// The arm above means something only because the same page does ask for
	// frames while the dot is drawn: reaching for the bar reveals it, and the
	// breath is the page's one ambient loop.
	tt.Move(150, 14)
	tt.Frame()
	if !tt.FramePending() {
		t.Errorf("the revealed bar's breathing dot asked for no frames at all: the loop is gone, not just gated")
	}

	// And under the desktop's reduce-motion preference the revealed frame asks
	// for none either — which is what makes the dot's loop, rather than some
	// other transition, the thing the two arms above are measuring.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	tt.Frame()
	if tt.FramePending() {
		t.Errorf("a reduce-motion terminal frame asked for another frame: something besides the dot's loop is animating")
	}
}

// TestDumpTheAtRestTerminalPNGs writes the evidence for the round's pixel
// claim: the at-rest page is the same picture before and after the fix, because
// the dot it stopped animating was drawn at zero opacity all along. Opt-in, as
// every dump in this package is.
func TestDumpTheAtRestTerminalPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	suffix := os.Getenv("FLOTER_PNG_SUFFIX")
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{}, nil)
		withSession(a, &terminal.Terminal{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokensOf(a, c).Theme)
			a.View(c)
		}, 900, 620)
		tt.SetDark(dark)
		// The pointer rests on the canvas, clear of the bar: the at-rest page.
		tt.Move(450, 520)
		tt.Frame()
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P11-bm-rest-"+appearance+suffix+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}

// P11-bn · The revealed dot's breath paints again instead of building the view.
//
// The window has two kinds of frame for what moves: `Context.AnimationFrame`
// (what `Element.Loop` asks for) *builds* the view again, and
// `Painter.AnimationFrameRate` asks for a frame that paints the elements of the
// last one again, the view neither built nor laid out (`ui/runtime.go`'s
// repaintFrame). The breath needs the second: it is one 1.75u circle fading,
// and it used to cost a whole terminal view rebuild on every display refresh.
//
// The measure is the view function's own call count across `Tester.Advance`,
// whose frames take the paint-again path: a breath that moves while that count
// stands still is drawn in the paint phase, and one that moves by rebuilding
// makes the count grow. The ink's mass is the second half — a paint-phase breath
// has to move the pixels without the view being asked to build.
//
// Mutation M1 (ask with `c.AnimationFrame()` instead, the build-again path)
// turns the first arm red; M2 (ask for nothing at all) turns it red at
// `FramePending`.
func TestTheRevealedDotsBreathPaintsAgainInsteadOfBuilding(t *testing.T) {
	a := New(settings.NewStore(settings.Default()), Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	builds := 0
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		builds++
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.titleBar(c, copy)
	}, 900, 40)
	// The pointer over the bar: the identity row, and the dot in it, are drawn.
	tt.Move(200, 14)
	tt.Frame()

	built := builds
	_, _, rest := barInkBox(tt, 0, 22, 40)

	tt.Advance(dotBreathPeriod / 2) // the breath's 50% keyframe
	if grew := builds - built; grew != 0 {
		t.Fatalf("the raised bar's breath built the view %d times over one half period: "+
			"the frames are builds, not paints", grew)
	}
	if !tt.FramePending() {
		t.Fatalf("the raised bar's breathing dot asked for no frame at all: nothing is driving it")
	}
	midX0, midX1, later := barInkBox(tt, 0, 22, 40)
	if diff := absInt(later - rest); diff < 500 {
		t.Errorf("the dot's ink mass is %d at the 0%% keyframe and %d at the 50%% one "+
			"(a difference of %d): the phase is not read while painting",
			rest, later, diff)
	}
	// The 50% keyframe is the old stylesheet's second frame, read off the paint:
	// the dot at half opacity, at 0.88 of its size. The rest frame is the first
	// (full opacity, full size) and is byte-identical to the frame before this
	// round — its centre is the flat tertiary step.
	if got, want := dotCenter(tt, midX0, midX1), flatColor(tok, tok.TextTertiary, 0.5); !sameRGB(got, want) {
		t.Errorf("the dot's centre at the 50%% keyframe = %v, want --text-tertiary at half opacity (%v)", got, want)
	}
}

// TestDumpTheDotBreathPNGs writes this round's visual evidence: the bar at the
// breath's two keyframes, in both appearances. Opt-in, as every dump here is.
func TestDumpTheDotBreathPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{}, nil)
		withSession(a, &terminal.Terminal{})
		copy := i18n.For(a.Store.Snapshot().Language).Terminal
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokensOf(a, c).Theme)
			c.Root().Background(ui.RGBA(0, 0, 0, 1))
			a.titleBar(c, copy)
		}, 900, 40)
		tt.SetDark(dark)
		tt.Move(200, 14)
		tt.Frame()
		for _, frame := range []struct {
			name  string
			after time.Duration
		}{{"rest", 0}, {"breath", dotBreathPeriod / 2}} {
			if frame.after > 0 {
				tt.Advance(frame.after)
			}
			path := filepath.Join("/tmp", "floter-P11-bn-dot-"+appearance+"-"+frame.name+".png")
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
