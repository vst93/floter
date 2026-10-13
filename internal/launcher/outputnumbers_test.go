package launcher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/shortcuts"
)

// outputRowsWithActions builds the list protocol's JSON for n runnable rows,
// each inserting its own text: the action is what the numbered shortcut runs.
func outputRowsWithActions(n int) string {
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":"r%d","title":"Row %d","action":{"type":"insert","text":"picked %d"}}`, i, i, i)
	}
	b.WriteString("]")
	return b.String()
}

// resultDigit presses the select_result chord for a digit, so the key path is
// the user's binding rather than a hard-coded modifier.
func pressResultDigit(t *testing.T, tt *ui.Tester, a *App, digit int) {
	t.Helper()
	mods, key, ok := shortcuts.Parse(settings.SelectResultDigit(a.settings(), digit))
	if !ok {
		t.Fatalf("the digit %d binding did not parse", digit)
	}
	tt.Key(mods, key)
	tt.Frame()
}

// The captured-output list is the launcher's own numbered family: its runnable
// rows carry ⌘1-⌘9/⌘0 exactly as a result does, because the old build fed the
// plugin view's items through the same `resultShortcutSlots`
// (`main:src/App.tsx:1602-1611`).
func TestOutputRowsCarryTheNumberedFamily(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, outputRowsWithActions(3))
	if !a.InOutputView() {
		t.Fatal("the output view did not open")
	}
	for i, want := range []int{1, 2, 3} {
		if got := a.outputNumbers[i]; got != want {
			t.Errorf("output row %d number = %d, want %d (%v)", i, got, want, a.outputNumbers)
		}
	}
	// The badge is drawn on the row, in the field's own binding.
	for _, digit := range []int{1, 2, 3} {
		if badge := a.resultNumber(digit); !tt.HasText(badge) {
			t.Errorf("the output list does not show the %d badge %q: %q", digit, badge, tt.Texts())
		}
	}
}

// A status note is information, not a door: it takes no number, and the family
// closes over it rather than leaving a hole.
func TestOutputStatusRowsAreNotNumbered(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, `[
		{"id":"note","title":"Nothing here","kind":"status"},
		{"id":"a","title":"Alpha","action":{"type":"insert","text":"a"}},
		{"id":"b","title":"Bravo","action":{"type":"insert","text":"b"}}
	]`)
	if !a.InOutputView() {
		t.Fatal("the output view did not open")
	}
	if _, ok := a.outputNumbers[0]; ok {
		t.Errorf("the status row is numbered: %v", a.outputNumbers)
	}
	if got := a.outputNumbers[1]; got != 1 {
		t.Errorf("the first runnable row number = %d, want 1 (%v)", got, a.outputNumbers)
	}
	if got := a.outputNumbers[2]; got != 2 {
		t.Errorf("the second runnable row number = %d, want 2 (%v)", got, a.outputNumbers)
	}
	// The note is drawn, but only the two doors carry a badge.
	if !tt.HasText("Nothing here") {
		t.Errorf("the status row is not drawn: %q", tt.Texts())
	}
	if !tt.HasText(a.resultNumber(1)) || !tt.HasText(a.resultNumber(2)) {
		t.Errorf("the runnable rows are missing a badge: %q", tt.Texts())
	}
}

// Pressing the number runs the output row it names, not the launcher's own
// result behind the view: ⌘2 inserts the second row's text.
func TestTheOutputNumberRunsItsRow(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, outputRowsWithActions(3))
	if !a.InOutputView() {
		t.Fatal("the output view did not open")
	}
	pressResultDigit(t, tt, a, 2)
	if a.InOutputView() {
		t.Fatal("the insert row did not leave the output view")
	}
	if a.Query != "picked 1" {
		t.Errorf("⌘2 put %q in the field, want the second row's text", a.Query)
	}
}

// The output list follows the scroll viewport when it overflows the ten-slot
// budget (R34/R72): the digits land on the runnable rows on screen, and
// scrolling renumbers them.
func TestOutputNumbersFollowTheOutputViewport(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, outputRowsWithActions(25))
	if !a.InOutputView() {
		t.Fatal("the output view did not open")
	}
	// The first window is two pages; the family only reaches the ten slots.
	if got := len(a.output.Rows); got != 20 {
		t.Fatalf("the first window is %d rows, want 20", got)
	}
	vf, vl := a.OutputList.Visible()
	if vf != 0 {
		t.Fatalf("the output list starts at row %d, want 0", vf)
	}
	if got := a.outputNumbers[vf]; got != 1 {
		t.Errorf("the first visible output row %d number = %d, want 1 (%v)", vf, got, a.outputNumbers)
	}
	for i := range a.outputNumbers {
		if i < vf || i > vl {
			t.Errorf("output row %d is numbered but outside the viewport %d..%d: %v", i, vf, vl, a.outputNumbers)
		}
	}

	// Scroll the list: the viewport is the previous frame's, so the family
	// lands one frame after the list moves.
	tt.Scroll(340, 200, 0, 400)
	tt.Frame()
	tt.Frame()
	vf, vl = a.OutputList.Visible()
	if vf == 0 {
		t.Fatalf("the output list did not scroll: visible %d..%d", vf, vl)
	}
	if got := a.outputNumbers[vf]; got != 1 {
		t.Errorf("after scrolling, the first visible output row %d number = %d, want 1 (%v)", vf, got, a.outputNumbers)
	}
	if _, ok := a.outputNumbers[0]; ok {
		t.Errorf("output row 0 is off screen but still numbered: %v", a.outputNumbers)
	}
}
