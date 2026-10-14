package launcher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/settings"
	"floter/internal/shortcuts"
)

// firstNumberable is the first row at or after `from` the numbered family can
// land on: the action bar's rows take no number (the old build's
// `resultShortcutSlots` ran over `displayedResults`, which excludes the bar),
// so the family starts on the first catalogue row behind it.
func firstNumberable(a *App, from int) int {
	results := a.Results()
	for i := from; i < len(results); i++ {
		if !strings.HasPrefix(results[i].ID, "bar:") && results[i].Run != nil {
			return i
		}
	}
	return from
}

// The numbered family follows the scroll viewport, not the top of the list.
// The old build's `resultShortcutSlots` (src/launcher/result-budget.ts) numbers
// the first nine runnable rows *inside the viewport* and the tenth `0`, so
// scrolling renumbers the list to what is on screen ("what you see is what you
// select", R34); `resultIndexForSlot` runs the row whose slot matches. The
// rewrite numbered the first ten rows of the whole list, so the digits were
// pinned to the top and scrolling did not move them — the report
// 「固定打到前几项上」.
//
// R72 gates the viewport on the list overflowing the ten-slot budget: a list
// that fits is numbered whole, because a list that fits cannot be scrolled and
// the viewport measurement adds nothing but risk.
func TestResultShortcutsFollowTheScrollViewport(t *testing.T) {
	a := testApp()
	found := make([]apps.App, 0, 30)
	for i := 0; i < 30; i++ {
		found = append(found, apps.App{
			Name: fmt.Sprintf("App %02d", i),
			Path: fmt.Sprintf("/Applications/App %02d.app", i),
		})
	}
	a.SetApps(found)
	a.Query = "app"
	tt := render(t, a)
	tt.Frame()

	// At the top of the list the first visible *numbered* row is row 1 (the
	// action bar's shell row heads the matched list and takes no number), so
	// it takes `1`.
	first, _ := a.List.Visible()
	if first != 0 {
		t.Fatalf("the list starts at row %d, want 0", first)
	}
	n1 := firstNumberable(a, first)
	if got := a.numbers[n1]; got != 1 {
		t.Fatalf("the first numbered row %d has number %d, want 1 (%v)", n1, got, a.numbers)
	}

	// Scroll down. The viewport is the previous frame's (the old build's key
	// handler read the last render's slots), so the family lands one frame
	// after the list moves.
	tt.Scroll(340, 200, 0, 600)
	tt.Frame()
	tt.Frame()
	vf, vl := a.List.Visible()
	if vf == 0 {
		t.Fatalf("the list did not scroll: visible %d..%d", vf, vl)
	}
	vn := firstNumberable(a, vf)
	if got := a.numbers[vn]; got != 1 {
		t.Errorf("after scrolling, the first numbered row %d has number %d, want 1 (%v)", vn, got, a.numbers)
	}
	if _, ok := a.numbers[0]; ok {
		t.Errorf("row 0 (the action bar) is numbered: %v", a.numbers)
	}
	// Every numbered row is inside the viewport, and the numbers are in order.
	for i := range a.numbers {
		if i < vf || i > vl {
			t.Errorf("row %d is numbered but outside the viewport %d..%d: %v", i, vf, vl, a.numbers)
		}
	}

	// The key runs the row the viewport numbers, not the list's first row.
	ran := ""
	a.Actions.OpenApp = func(app apps.App) { ran = app.Name }
	mods, _, ok := shortcuts.Parse(settings.SelectResultDigit(a.settings(), 1))
	if !ok {
		t.Fatal("the first result's key did not parse")
	}
	tt.Key(mods, ui.Key1)
	tt.Frame()
	want := a.Results()[vn].Title
	if ran != want {
		t.Errorf("the modifier+1 ran %q, want the first numbered row %q", ran, want)
	}
	if ran == found[0].Name {
		t.Errorf("the modifier+1 ran the list's first row %q, not the visible one", ran)
	}
}

// A list that fits the ten-slot budget is numbered whole and in order: its
// rows are all visible by construction, and the R72 gate keeps a viewport
// measured mid-walk from leaving badges off (a ten-row list showed one badge
// and nine rows with nothing).
func TestAShortListIsNumberedWhole(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{
		{Name: "Alpha", Path: "/Applications/Alpha.app"},
		{Name: "Alpine", Path: "/Applications/Alpine.app"},
	})
	a.Query = "al"
	tt := render(t, a)
	tt.Frame()
	if got := a.numbers[1]; got != 1 {
		t.Errorf("row 1 number = %d, want 1 (%v)", got, a.numbers)
	}
	if got := a.numbers[2]; got != 2 {
		t.Errorf("row 2 number = %d, want 2 (%v)", got, a.numbers)
	}
	if _, ok := a.numbers[0]; ok {
		t.Errorf("the action bar was numbered: %v", a.numbers)
	}
}
