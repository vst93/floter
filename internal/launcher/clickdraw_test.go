package launcher

import (
	"testing"

	"floter/internal/apps"
	"github.com/egoist/mygo/ui"
)

// P11-cl: a pointer click runs the row the frame drew.
//
// The click branch fires *inside* the draw (App.row's `row.Clicked()`), after
// the frame has already built its list. It used to ask the catalog again —
// `a.activate(a.Results())` — so one draw paid two full matches with no
// keystroke to explain the second, the same duplicate walk P11-bw's
// `TestADrawWalksTheCatalogOnce` counts for the keyboard. It now reads the
// slice the draw holds (`App.drawn`).
//
// The count alone would pass with the app reading a *stale* slice, so this
// pin reads both halves: the frame searches once, and the row that runs is
// the row the pointer landed on.

func TestAClickRunsTheRowTheDrawBuilt(t *testing.T) {
	a := testApp()
	recentPage(a, 5)
	var launched []string
	a.Actions.OpenApp = func(app apps.App) { launched = append(launched, app.Name) }

	searches := []int{}
	tt := ui.NewTester(func(c *ui.Context) {
		before := a.searchRuns
		a.View(c)
		searches = append(searches, a.searchRuns-before)
	}, InputWindowWidth, frameHeight(maxResultSlots))
	tt.Frame()

	// Narrow the list first: the click must run the row of *this* frame's
	// list, not one of the five the previous frame held.
	tt.Type("Recent 02")
	tt.Frame()
	if got := ids(a.Results()); len(got) == 0 || got[0] != "app:/Applications/Recent02.app" {
		t.Fatalf("the narrowed list is %v, want the Recent 02 row first", got)
	}
	searches = searches[:0]
	if err := tt.Click("Recent 02"); err != nil {
		t.Fatalf("the click cannot reach its row: %v", err)
	}
	tt.Frame()

	if len(launched) != 1 || launched[0] != "Recent 02" {
		t.Errorf("the click ran %v, want [Recent 02]", launched)
	}
	for i, n := range searches {
		if n != 1 {
			t.Errorf("the click frame's draw %d searched %d times, want 1", i+1, n)
		}
	}
}
