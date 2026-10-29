package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// P11-bq · A line that only changes when its clock runs out asks for that one
// frame, not for a rebuild of the whole view per refresh.
//
// The footer toast lives `feedbackDuration` (4.5s) and nothing about it moves
// meanwhile: before this round the launcher asked `Context.AnimationFrame` on
// every frame it was up, and that is a *rebuild* of the whole view
// (`ui/runtime.go:744-748`) — 120 a second on a 120 Hz panel for a sentence that
// stayed the same. The fix arms one wake at the deadline (`internal/deadline`),
// the same shape the settings page's "copied" confirmation already had
// (`settingsui.go:3917`).
//
// Three things are pinned: inside the window the view is built not once, past
// the window the toast leaves by itself (so the wake really was armed), and it
// is still up one second before its own deadline (so the wake is not an early
// rebuild in disguise).
//
// Mutation M1 (`deadline.Wake` back to `c.AnimationFrame()` in the launcher's
// toast) turns the first arm red with 60 builds a second.
func TestTheTimedToastWakesAtItsDeadline(t *testing.T) {
	a := testApp()
	builds := 0
	tt := ui.NewTester(func(c *ui.Context) {
		builds++
		a.View(c)
	}, InputWindowWidth, frameHeight(maxResultSlots))
	// The launcher's clock is the tester's, so a wait the test cannot see is a
	// wait the launcher cannot take either.
	a.clock = tt.Now
	tt.Frame()

	// The page's own frames, with nothing raised. The caret blinks only while the
	// window has focus, so the window is left unfocused here: with nothing else
	// animating, the number the toast must not add to is zero.
	tt.SetFocused(false)
	built := builds
	tt.Advance(time.Second)
	baseline := builds - built

	sentence := "Saved to the clipboard"
	a.Feedback(sentence)
	tt.Frame()
	if !tt.HasText(sentence) {
		t.Fatalf("the toast is not drawn: %q", tt.Texts())
	}

	// One second of a still window: not one build of its own beyond the ones the
	// page asks for anyway, and the sentence is still there, so the deadline is
	// not being reached early. The band's arrival changes the list's geometry,
	// and whatever settles around that is given half a second to do it before the
	// count starts — the clock is what is being measured, not the layout.
	tt.Advance(500 * time.Millisecond)
	built = builds
	tt.Advance(time.Second)
	if grew := builds - built; grew != baseline {
		t.Fatalf("the toast built the view %d times in a second, %d more than the "+
			"page's own %d: the frames are rebuilds of a line that does not change",
			grew, grew-baseline, baseline)
	}
	if !tt.HasText(sentence) {
		t.Fatalf("the toast left inside its window: the wake fired early")
	}

	// Past it: the wake fires, the line clears itself, and the page is quiet
	// again.
	built = builds
	tt.Advance(feedbackDuration)
	if tt.HasText(sentence) {
		t.Errorf("the toast outlived its window: %q", tt.Texts())
	}
	if grew := builds - built; grew > baseline+1 {
		t.Errorf("the toast's deadline took %d builds, want at most one more than the page's own %d",
			grew, baseline)
	}
}

// TestDumpTheTimedToastPNG writes the round's visual evidence: the band the
// launcher's toast draws, before and after the wake changed. The change is a
// wake-up, not ink, so the two pictures are expected to be byte for byte the
// same. Opt-in, as every dump in this package is (`FLOTER_PNG=1`, and
// `FLOTER_PNG_SUFFIX` to keep the two rounds' files apart).
func TestDumpTheTimedToastPNG(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	suffix := os.Getenv("FLOTER_PNG_SUFFIX")
	sentence := "Saved to the clipboard"
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		tt := ui.NewTester(func(c *ui.Context) {
			a.View(c)
		}, InputWindowWidth, frameHeight(maxResultSlots))
		tt.SetDark(dark)
		a.Feedback(sentence)
		tt.Frame()
		tt.Frame()
		if !tt.HasText(sentence) {
			t.Fatalf("%s: the toast is not drawn", appearance)
		}
		path := filepath.Join("/tmp", "floter-P11-bq-toast-"+appearance+suffix+".png")
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
