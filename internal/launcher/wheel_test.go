package launcher

import (
	"strings"
	"testing"
	"time"
)

// longOutput is a text-mode capture: n lines, each wider than a word, so the
// block is much taller than the window and the page has somewhere to scroll.
func longOutput(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("line ")
		b.WriteString(strings.Repeat("x", 20))
		b.WriteString("\n")
	}
	return b.String()
}

// A captured run's text page is a scroll container (output.go, `ui.Scroll` with
// `TrackScroll(&a.Output)`), and a wheel over it is how the user reads the end
// of a long output: the pointer's notches move the text, the container clamps
// at both ends, and the field keeps the keyboard while it happens.
//
// Nothing pinned this before: the page's `ui.Scroll` had no test at all, so a
// wheel that never reached it (an ancestor that took the pointer, a body that
// grew the window instead of scrolling) would have shipped silently. The
// framework's own `ui.Tester.Wheel` (mygo v0.6.3, `ui/headless.go:658`) is the
// real path a mouse wheel takes, unlike `Tester.Scroll`'s touchpad glide.
func TestTheOutputPageScrollsWithTheWheel(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, longOutput(300))
	if !a.InOutputView() {
		t.Fatal("the output view did not open")
	}
	if a.output.IsList {
		t.Fatalf("the capture opened as a list, not the text page: %+v", a.output)
	}
	if a.Output.MaxY <= 0 {
		t.Fatalf("the output page does not overflow its frame (MaxY = %v), so there is nothing to scroll: "+
			"the pin below would pass on a page that cannot scroll at all", a.Output.MaxY)
	}
	if a.Output.Y != 0 {
		t.Fatalf("the output page starts scrolled to %v, want the start", a.Output.Y)
	}

	// The pointer sits in the middle of the page, away from the status line and
	// the field: a notch there is the text's.
	bounds := tt.Image().Bounds()
	x, y := float32(bounds.Max.X/2), float32(bounds.Max.Y*3/4)
	tt.Wheel(x, y, 0, 120)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	afterNotch := a.Output.Y
	if afterNotch <= 0 {
		t.Fatalf("a wheel notch left the page at Y = %v: the wheel does not reach the output's scroll container", afterNotch)
	}
	if afterNotch > a.Output.MaxY {
		t.Fatalf("the page scrolled past its end: Y = %v > MaxY = %v", afterNotch, a.Output.MaxY)
	}

	// The page stays the page: a wheel is a read, not a way out of the output
	// (Esc is), and it must not walk the list's selection either.
	if !a.InOutputView() {
		t.Errorf("the wheel closed the output view")
	}

	// The far end clamps exactly at MaxY, and a wheel back up returns to the
	// very start — the two readings the scroll state promises.
	tt.Wheel(x, y, 0, a.Output.MaxY+2000)
	tt.Advance(800 * time.Millisecond)
	tt.Frame()
	if a.Output.Y != a.Output.MaxY {
		t.Errorf("after scrolling far past the end the page is at Y = %v, want the clamp at MaxY = %v",
			a.Output.Y, a.Output.MaxY)
	}
	tt.Wheel(x, y, 0, -(a.Output.MaxY + 2000))
	tt.Advance(800 * time.Millisecond)
	tt.Frame()
	if a.Output.Y != 0 {
		t.Errorf("after scrolling far past the start the page is at Y = %v, want 0", a.Output.Y)
	}
}

// The other arm of the same page: a list-mode capture scrolls its `ui.List`, and
// a wheel over it must walk the rows rather than leave the list where it was.
// The list is windowed in memory (see TestCapturedListWindowsInMemory), so this
// reads the frame: the rows that show before the notch are not the rows that
// show after it.
func TestTheOutputListScrollsWithTheWheel(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, listOutput(60))
	if !a.InOutputView() || !a.output.IsList {
		t.Fatalf("the capture did not open as the list page: %+v", a.output)
	}
	before := tt.Texts()
	bounds := tt.Image().Bounds()
	tt.Wheel(float32(bounds.Max.X/2), float32(bounds.Max.Y*3/4), 0, 400)
	tt.Advance(800 * time.Millisecond)
	tt.Frame()
	after := tt.Texts()

	// A row that was drawn before the notch and is not drawn after it proves
	// the list moved. (The texts of a `ui.List` are only the rows it draws.)
	moved := false
	for _, text := range before {
		if !strings.HasPrefix(text, "Row ") {
			continue
		}
		if !containsText(after, text) {
			moved = true
			break
		}
	}
	if !moved {
		t.Fatalf("a wheel over the captured list drew the same rows: the list does not scroll\nbefore: %q\nafter:  %q",
			before, after)
	}
}

func containsText(texts []string, want string) bool {
	for _, text := range texts {
		if text == want {
			return true
		}
	}
	return false
}
