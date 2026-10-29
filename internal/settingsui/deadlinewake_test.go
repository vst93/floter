package settingsui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P11-bq · The write-failure banner's own clock is a deadline, not an
// animation. Its entrance moves for a fraction of a second (`AnimateTo` drives
// those frames); the remaining five seconds of its life are a sentence that does
// not change, and before this round the page rebuilt the whole settings view on
// every display refresh to keep it up (`c.AnimationFrame`, a rebuild by
// `ui/runtime.go:744-748`). The fix arms one wake at the deadline
// (`internal/deadline`), the shape the page's "copied" confirmations already
// used (`settingsui.go:3917`).
//
// The measurement starts once the entrance has settled, so the number it pins is
// the clock's and not the animation's.
//
// Mutation M3 (dropping `wakeSettle` from `deadline.Wake`) turns the last arm
// red: a wake at exactly the deadline reads as "waited > window" being false, so
// the banner would never clear.
func TestTheSaveFailureBannerWakesAtItsDeadline(t *testing.T) {
	store, _ := settings.OpenStore(t.TempDir())
	a := New(store, Actions{})
	builds := 0
	tt := ui.NewTester(func(c *ui.Context) {
		builds++
		a.View(c)
	}, 720, 580)
	a.Now = tt.Now
	tt.Frame()

	a.set(func(s *settings.Settings) { s.HideOnBlur = !s.HideOnBlur })
	tt.Frame()
	if !a.SaveFailedVisible() {
		t.Fatal("a refused write showed no banner")
	}
	// The entrance is the banner's one motion: let it settle before counting.
	tt.Advance(time.Second)
	if got := a.SaveAlertEnterProgress(); got != 1 {
		t.Fatalf("the entrance did not settle: progress %v", got)
	}
	if !a.SaveFailedVisible() {
		t.Fatalf("the banner left inside its window")
	}

	built := builds
	tt.Advance(time.Second)
	if grew := builds - built; grew != 0 {
		t.Fatalf("the banner built the settings view %d times in a second: "+
			"the frames are rebuilds of a sentence that does not change", grew)
	}
	if !a.SaveFailedVisible() {
		t.Fatalf("the banner left before its window ran out")
	}

	built = builds
	tt.Advance(saveFailedWindow)
	if grew := builds - built; grew != 1 {
		t.Errorf("the banner's deadline woke the view %d times, want exactly one", grew)
	}
	if a.SaveFailedVisible() {
		t.Errorf("the banner outlived its window")
	}
}
