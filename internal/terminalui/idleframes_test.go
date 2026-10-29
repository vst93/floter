package terminalui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
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
