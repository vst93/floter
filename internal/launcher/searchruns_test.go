package launcher

// One frame, one walk of the catalog.
//
// The launcher used to pay a full catalog match — `Catalog()` rebuilt, every
// item scored with pinyin — two or three times per **View**, and the report it
// answers is ③ (「很多界面经常卡死」): on a machine with a large start menu the
// duplicate walked the same list again for every draw, and on the
// selection-change frame it walked it three times (the held row's presence,
// the list's length, the footer's index) after the frame's own list had
// already been built. The headless tester draws one `Frame` several times (the
// resize walk and layout settle each ask for another), so a per-draw count is
// the one that measures the cost a keystroke actually pays.
//
// Each of those questions is now answered from the slice the draw already
// holds (see View and heldRowShown), so the invariant is: one search per draw.
// A test that only pinned the row positions would keep passing with the second
// search back in place — which is how it survived this long.

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// countingRender is render() with a per-draw search count beside the tester:
// each entry is the searches one call of View ran. A key event's own handler
// also searches (the field's own act), and that cost is measured separately —
// this is the draw's share.
func countingRender(t *testing.T, a *App) (*ui.Tester, *[]int) {
	t.Helper()
	per := []int{}
	tt := ui.NewTester(func(c *ui.Context) {
		before := a.searchRuns
		a.View(c)
		per = append(per, a.searchRuns-before)
	}, InputWindowWidth, frameHeight(maxResultSlots))
	tt.Frame()
	return tt, &per
}

// TestADrawWalksTheCatalogOnce counts the searches a draw runs, on an idle
// launcher and on the draws that rebuild the list: a keystroke (the list is
// rebuilt and the selection resets) and both edges of the held modifier (the
// press selects the held row, the release hands the selection back).
func TestADrawWalksTheCatalogOnce(t *testing.T) {
	a := testApp()
	tt, per := countingRender(t, a)

	// keys is how many edits the step injects. Each edit is one draw that has
	// to search twice — once for the list it was drawn with, once because the
	// field took the text *during* that draw and the list it now means is a
	// different one (see the field's Changed branch in View). That second
	// search is the field's own price and cannot be saved; every other draw of
	// these frames is a single walk.
	type step struct {
		name string
		keys int
		run  func()
	}
	steps := []step{
		{"idle", 0, func() {}},
		{"typing", 1, func() { tt.Type("term") }},
		{"a keystroke that keeps the selection", 1, func() { tt.Type("i") }},
		{"a keystroke that empties the field", 5, func() {
			for range 5 {
				tt.Key(0, ui.KeyBackspace)
			}
		}},
		{"the modifier's press", 0, func() { tt.HoldModifiers(ui.Cmd) }},
		{"the modifier's release", 0, func() { tt.HoldModifiers(0) }},
	}
	for _, step := range steps {
		*per = (*per)[:0]
		step.run()
		tt.Frame()
		if len(*per) == 0 {
			t.Fatalf("the %s frame drew nothing", step.name)
		}
		twice := 0
		for i, searches := range *per {
			switch {
			case searches == 1:
			case searches == 2:
				twice++
			default:
				t.Errorf("the %s frame's draw %d searched %d times, want at most 2",
					step.name, i+1, searches)
			}
		}
		if twice > step.keys {
			t.Errorf("the %s frame searched twice on %d draws, but injected %d keystrokes",
				step.name, twice, step.keys)
		}
		if step.keys == 0 && twice > 0 {
			t.Errorf("the %s frame searches twice on %d draws with no keystroke to explain it",
				step.name, twice)
		}
	}
}

// TestTheModifierEdgesMoveNoRowOnTheEmptyPage is the behaviour the counting
// above must not have bought its speed from: the modifier's edges still land
// (that state is what the band reads), and on the empty page they move
// nothing — P11-bo, the held line is the band and not a row (F4-3).
func TestTheModifierEdgesMoveNoRowOnTheEmptyPage(t *testing.T) {
	a := testApp()
	recentPage(a, 3)
	tt := render(t, a)
	list := ids(a.Results())
	selected := a.Selected

	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if !a.modifierHeld {
		t.Fatal("the modifier is not held after its press frame")
	}
	if got := ids(a.Results()); !slices.Equal(got, list) {
		t.Errorf("the press edge changed the empty page's list: %v, want %v", got, list)
	}
	if a.Selected != selected {
		t.Errorf("the press edge moved the selection to %d, want %d", a.Selected, selected)
	}

	tt.HoldModifiers(0)
	tt.Frame()
	if a.modifierHeld {
		t.Fatal("the modifier is still held after its release frame")
	}
	if got := ids(a.Results()); !slices.Equal(got, list) {
		t.Errorf("the release edge changed the empty page's list: %v, want %v", got, list)
	}
	if a.Selected != selected {
		t.Errorf("the release edge moved the selection to %d, want %d", a.Selected, selected)
	}
}
