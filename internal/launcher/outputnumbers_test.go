package launcher

import (
	"fmt"
	"os"
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

// TestATenRowOutputDump writes the frame P11-bl is about: the captured-output
// list — exactly the ten-slot budget — scrolled so its first row is mostly off
// screen. Opt-in (FLOTER_PNG=1), like the other screenshot dumps;
// FLOTER_PNG_TAG names the pair ("before"/"after").
func TestATenRowOutputDump(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P11-bl-output-<tag>-<appearance>.png")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	forEachAppearance(t, func(t *testing.T, dark bool) {
		name := "light"
		if dark {
			name = "dark"
		}
		a := testApp()
		tt := runListOutput(t, a, outputRowsWithActions(maxResultSlots))
		for _, step := range []float32{1, 3, 5, 7, 9, 11, 13, 15, 17, 19, 21, 24, 27, 30} {
			tt.Scroll(340, 200, 0, step)
			tt.Frame()
			tt.Frame()
			if got := visibleShare(a.outputRowRects[0], a.outputListRect); got > 0 && got < numberVisibleShare {
				break
			}
		}
		share := visibleShare(a.outputRowRects[0], a.outputListRect)
		t.Logf("row 0 is %.2f visible; numbers %v; badges %q", share, a.outputNumbers, tt.Texts())
		path := fmt.Sprintf("/tmp/floter-P11-bl-output-%s-%s.png", tag, name)
		writePNG(t, path, tt)
	})
}

// outputFirstMostlyVisible is the observation the numbered family is supposed
// to follow: the first row whose own box is at least `numberVisibleShare`
// inside the list's clip, and that share. It reads the boxes the frame laid
// out, not the family (P11-bl).
func outputFirstMostlyVisible(a *App) (int, float64) {
	for i := 0; i < len(a.output.Rows); i++ {
		box, ok := a.outputRowRects[i]
		if !ok {
			continue
		}
		if share := visibleShare(box, a.outputListRect); share >= numberVisibleShare {
			return i, share
		}
	}
	return -1, 0
}

// The output list follows the scroll viewport (R34/R72), and it follows it by
// the same measure the result list does: a row counts while at least seven
// tenths of it is on screen, so a row scrolled away takes no number and the
// family starts again at `1` on the first row the user can read (P11-bl).
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
	first, share := outputFirstMostlyVisible(a)
	if first != 0 {
		t.Fatalf("the first mostly visible row is %d (share %.2f), want 0", first, share)
	}
	if got := a.outputNumbers[first]; got != 1 {
		t.Errorf("the first visible output row %d number = %d, want 1 (%v)", first, got, a.outputNumbers)
	}
	for i := range a.outputNumbers {
		if i < first {
			t.Errorf("output row %d is above the first readable row but numbered: %v", i, a.outputNumbers)
		}
		if box, ok := a.outputRowRects[i]; ok && visibleShare(box, a.outputListRect) <= 0 {
			t.Errorf("output row %d is off screen but numbered: %v", i, a.outputNumbers)
		}
	}

	// Scroll the list: the viewport is the previous frame's, so the family
	// lands one frame after the list moves.
	tt.Scroll(340, 200, 0, 400)
	tt.Frame()
	tt.Frame()
	first, share = outputFirstMostlyVisible(a)
	if first <= 0 {
		t.Fatalf("the output list did not scroll: first mostly visible row %d (share %.2f)", first, share)
	}
	if got := a.outputNumbers[first]; got != 1 {
		t.Errorf("after scrolling, the first visible output row %d number = %d, want 1 (%v)", first, got, a.outputNumbers)
	}
	if _, ok := a.outputNumbers[0]; ok {
		t.Errorf("output row 0 is off screen but still numbered: %v", a.outputNumbers)
	}
	for i := range a.outputNumbers {
		if i < first {
			t.Errorf("output row %d scrolled away but kept its number: %v", i, a.outputNumbers)
		}
	}
}

// A row the list still paints a sliver of is not a row the user can read: the
// family skips it and starts on the first row that is mostly there. The
// framework's Visible() counts the sliver — that answer is the one P11-bb
// rejected for the result list, and the output list must not keep it (P11-bl).
func TestAnOutputRowMostlyScrolledAwayTakesNoNumber(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, outputRowsWithActions(25))
	pitch := a.outputRowRects[1].Y - a.outputRowRects[0].Y
	if pitch <= 0 {
		t.Fatalf("the rows have no pitch: %v", a.outputRowRects)
	}
	// Walk the list down until the row leaving through the top is still on
	// screen and less than seven tenths of it is (the wheel eases, so a single
	// nudge lands wherever the animation is that frame).
	top, topShare := -1, 0.0
	for attempt := 0; attempt < 40 && top < 0; attempt++ {
		tt.Scroll(340, 200, 0, pitch)
		tt.Frame()
		tt.Frame()
		for i := 0; i < len(a.output.Rows); i++ {
			box, ok := a.outputRowRects[i]
			if !ok {
				continue
			}
			if share := visibleShare(box, a.outputListRect); share > 0 {
				if share < numberVisibleShare {
					top, topShare = i, share
				}
				break
			}
		}
	}
	if top < 0 {
		t.Fatalf("no row ever sat mostly off screen while still drawn: %v", a.outputRowRects)
	}
	if got, ok := a.outputNumbers[top]; ok {
		t.Errorf("row %d is only %.2f on screen but carries number %d (%v)", top, topShare, got, a.outputNumbers)
	}
	first, firstShare := outputFirstMostlyVisible(a)
	if got := a.outputNumbers[first]; got != 1 {
		t.Errorf("the first readable row %d (share %.2f) number = %d, want 1 (%v)", first, firstShare, got, a.outputNumbers)
	}
}

// Exactly ten rows — the whole numbered family — do not fit: the body charges
// the status strip above the list, so a ten-row output list scrolls. The family
// must renumber there too; gating the viewport on "more rows than slots" left
// the badge on the row that scrolled away (P11-bl, P11-bi's leftover).
func TestATenRowOutputListRenumbersWhenItScrolls(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, outputRowsWithActions(maxResultSlots))
	if !a.InOutputView() {
		t.Fatal("the output view did not open")
	}
	if got := len(a.output.Rows); got != maxResultSlots {
		t.Fatalf("the list holds %d rows, want %d", got, maxResultSlots)
	}
	if got := a.outputNumbers[0]; got != 1 {
		t.Fatalf("the first frame numbers row 0 with %d, want 1 (%v)", got, a.outputNumbers)
	}
	tt.Scroll(340, 200, 0, 400)
	tt.Frame()
	tt.Frame()
	share := visibleShare(a.outputRowRects[0], a.outputListRect)
	if share >= numberVisibleShare {
		t.Fatalf("the premise failed: a ten-row list did not scroll (row 0 is %.2f visible)", share)
	}
	if got, ok := a.outputNumbers[0]; ok {
		t.Errorf("row 0 is %.2f on screen but still numbered %d (%v)", share, got, a.outputNumbers)
	}
	first, firstShare := outputFirstMostlyVisible(a)
	if got := a.outputNumbers[first]; got != 1 {
		t.Errorf("the first readable row %d (share %.2f) number = %d, want 1 (%v)", first, firstShare, got, a.outputNumbers)
	}
}
