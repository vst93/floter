package terminalui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P11-bq · The panel's failure note is a line on a clock, not an animation: it
// changes once, when its 4.5s window runs out. Before this round the note asked
// `Context.AnimationFrame` on every frame it was up — a rebuild of the whole
// terminal view per display refresh (`ui/runtime.go:744-748`), for a sentence
// that did not change. The fix arms one wake at the deadline
// (`internal/deadline`).
//
// The pointer rests on the canvas so the bar's breathing status dot is not
// drawing: this test measures the note alone (P11-bm pinned the same rest state).
//
// Mutation M2 (`deadline.Wake` back to `c.AnimationFrame()` in `feedbackNote`)
// turns the first arm red.
func TestTheTimedFeedbackWakesAtItsDeadline(t *testing.T) {
	a := New(settings.NewStore(settings.Default()), Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	builds := 0
	tt := ui.NewTester(func(c *ui.Context) {
		builds++
		c.SetTheme(tokensOf(a, c).Theme)
		a.View(c)
	}, 900, 600)
	tt.Move(450, 400)
	tt.Frame()

	a.ShowFeedback("Could not copy the selection")
	tt.Frame()
	if !a.FeedbackVisible() {
		t.Fatal("the note is not up")
	}

	built := builds
	tt.Advance(time.Second)
	if grew := builds - built; grew != 0 {
		t.Fatalf("the note built the terminal view %d times in a second: "+
			"the frames are rebuilds of a line that does not change", grew)
	}
	if !a.FeedbackVisible() {
		t.Fatalf("the note left inside its window: the wake fired early")
	}

	built = builds
	tt.Advance(terminalFeedbackDuration)
	if grew := builds - built; grew != 1 {
		t.Errorf("the note's deadline woke the view %d times, want exactly one", grew)
	}
	if a.FeedbackVisible() {
		t.Errorf("the note outlived its window")
	}
}

// TestDumpTheTimedFeedbackPNG writes the round's picture of the note the panel
// shows when an action it owns fails: the band under the title bar. The change
// is a wake-up, not ink, so the two rounds' pictures are expected to be byte for
// byte the same. Opt-in, as every dump in this package is.
func TestDumpTheTimedFeedbackPNG(t *testing.T) {
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
		tt.Move(450, 520)
		a.ShowFeedback("Could not copy the selection")
		tt.Frame()
		tt.Frame()
		if !a.FeedbackVisible() {
			t.Fatalf("%s: the note is not up", appearance)
		}
		path := filepath.Join("/tmp", "floter-P11-bq-note-"+appearance+suffix+".png")
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
