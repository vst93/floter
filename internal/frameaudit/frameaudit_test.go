package frameaudit

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The ruler's own ruler. `Violations` is the rule without a tester, so the four
// questions that decide whether this audit is worth anything can be asked with
// hand-made boxes instead of a painted frame:
//
//  1. a **full-width** run painted over another run is a violation — the case the
//     old `r.W >= w-1 { continue }` in three test files threw away (P11-bx M4:
//     the settings banner at six times its font size stayed green);
//  2. an **ancestor** (a panel's own label spanning the card) is not — that is
//     what `Encloses` is for, and the reason the skip was never needed;
//  3. the one-DIP tolerance is still one DIP (a hairline of overlap is a row's
//     baseline, not two words on the same pixels);
//  4. a run outside the window is a violation.
//
// The first arm is the one that fails if somebody puts the skip back.
func TestTheFrameRuleMeasuresFullWidthRuns(t *testing.T) {
	const w, h = 200, 100
	box := func(x, y, bw, bh float32) ui.Rect { return ui.Rect{X: x, Y: y, W: bw, H: bh} }

	t.Run("a full-width run painted over another", func(t *testing.T) {
		runs := []Run{
			{S: "the banner", R: box(0, 0, w, 60)}, // W == w: the old skip
			{S: "under it", R: box(20, 40, 100, 30)},
		}
		got := Violations(w, h, runs)
		if len(got) != 1 || !strings.Contains(got[0], "is painted over") {
			t.Fatalf("a full-width run on another run's pixels went unreported: %v", got)
		}
		if !strings.Contains(got[0], "the banner") {
			t.Fatalf("the violation should name the full-width run, got %q", got[0])
		}
	})

	t.Run("an ancestor is still exempt", func(t *testing.T) {
		runs := []Run{
			{S: "the panel's own label", R: box(0, 0, w, h)},
			{S: "a row inside it", R: box(20, 20, 50, 20)},
		}
		if got := Violations(w, h, runs); len(got) != 0 {
			t.Fatalf("a full-window ancestor is exempt by construction, got %v", got)
		}
	})

	t.Run("the tolerance is one DIP", func(t *testing.T) {
		runs := []Run{
			{S: "the row above", R: box(0, 0, 50, 20)},
			{S: "the row below", R: box(0, 19, 50, 20)},
		}
		if got := Violations(w, h, runs); len(got) != 0 {
			t.Fatalf("one DIP of shared edge is not two words on one pixel, got %v", got)
		}
	})

	t.Run("a run outside the window", func(t *testing.T) {
		runs := []Run{{S: "pushed right", R: box(150, 90, 80, 20)}}
		got := Violations(w, h, runs)
		if len(got) != 1 || !strings.Contains(got[0], "leaves the window") {
			t.Fatalf("a run outside the window went unreported: %v", got)
		}
	})
}
