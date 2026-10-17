package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// The field row spends its seams through the controls' own margins, never a
// flex `gap`.
//
// The reference's `.collapsed-card__input-row` is a plain flex row with no
// `gap` (`main:src/styles/launcher.css:251-253`): the trailing controls each
// open with `margin-left: calc(var(--u) * 8)` (`.collapsed-card__settings`,
// `:388`), so the seam between two of them is that 8u and nothing else. The
// port had a uniform `Gap(Space(1))` on the row, which added a step the design
// does not spend in front of every trailing item (12u where the reference draws
// 8u) and shortened the field by 4u per item.
//
// This walks the drawn boxes and pins every seam to the theme's own `Space(2)`
// (the codebase's spelling of 8u; `internal/theme/theme.go`: `Space(n)` is four
// reference units). Mutation check: put the row's `Gap` back and each seam
// grows by `Space(1)` past the tolerance.
func TestTheFieldRowSpendsNoFlexGapBetweenItsTrailingControls(t *testing.T) {
	a := testApp()
	a.Actions.OpenSessions = func() {}
	tt := render(t, a)
	// A query shows the clear control, so three seams are in play
	// (field → clear → sessions → settings) rather than the button pair alone.
	tt.Type("x")
	tt.Frame()

	order := []string{"Clear", "Sessions", "Settings"}
	type box struct {
		name string
		x, w float32
	}
	var boxes []box
	for _, name := range order {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("the field row has no %q control: %q", name, tt.Texts())
		}
		boxes = append(boxes, box{name, r.X, r.W})
	}
	space2 := theme.ForPreferences(a.settings(), false, ui.Preferences{}).Theme.Space(2)
	for i := 1; i < len(boxes); i++ {
		seam := boxes[i].x - (boxes[i-1].x + boxes[i-1].w)
		if diff := seam - space2; diff > 1.5 || diff < -1.5 {
			t.Errorf("the %s→%s seam is %v, want the reference's own margin %v (diff %v)",
				boxes[i-1].name, boxes[i].name, seam, space2, diff)
		}
	}
}
