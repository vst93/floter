package launcher

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
)

// P11-ar · **一份新的捕获输出从它自己的开头开始。**
//
// The output view's scroll offset (`App.Output`) and the output list's place
// (`App.OutputList`) are app-level because the containers need them to survive
// across frames. They are also reused across *views*: `showOutput` swaps in a
// brand-new `*OutputView` but never reset them, so a second long capture opened
// at the first one's offset — close a 300-line page scrolled 400u down, run
// another command, and read its output from ~line 15 instead of line 1.
//
// This is the same rule P11-ao set for the settings pages (a new page starts at
// the top), applied to the launcher's output band. P11-ar resets both states in
// `showOutput`; the reset is deliberately *not* on the per-frame rebuild path, or
// the wheel would be zeroed every frame (`TestTheOutputPageScrollsWithTheWheel`
// guards that).
func TestAFreshOutputStartsAtItsBeginning(t *testing.T) {
	// §1 the text page: scroll one long capture, close it, open another and
	// check the new one starts at Y = 0 with the first line on screen.
	t.Run("text", func(t *testing.T) {
		a := testApp()
		tt := runListOutput(t, a, longOutput(300))
		if a.output == nil || a.output.IsList {
			t.Fatalf("the capture did not open as the text page: %+v", a.output)
		}
		if a.Output.MaxY <= 0 {
			t.Fatalf("the page cannot scroll (MaxY = %v); the pin would pass on a page that never scrolled", a.Output.MaxY)
		}
		bounds := tt.Image().Bounds()
		x, y := float32(bounds.Max.X/2), float32(bounds.Max.Y*3/4)
		tt.Wheel(x, y, 0, 400)
		tt.Advance(500 * time.Millisecond)
		tt.Frame()
		if a.Output.Y <= 0 {
			t.Fatalf("the wheel did not scroll the first page: Y = %v", a.Output.Y)
		}
		scrolled := a.Output.Y

		// Close with Escape (the page's own way out), then show a *different*
		// long capture through the same app.
		tt.Key(0, ui.KeyEscape)
		tt.Frame()
		if a.InOutputView() {
			t.Fatal("Escape did not close the output view")
		}
		a.showOutput(extensions.CapturedRun{
			Command: []string{"tool"}, Success: true, Stdout: longOutput(300),
		}, nil, nil, nil)
		tt.Frame()

		if a.Output.Y != 0 {
			t.Errorf("a fresh text output opened at Y = %v (the previous page was at %v), want 0: "+
				"the new capture starts mid-way instead of at its first line", a.Output.Y, scrolled)
		}
	})

	// §2 the list page: the same bug on `OutputList`. Scroll a long list, close
	// it, open a second long list and check its first visible row is row 0.
	t.Run("list", func(t *testing.T) {
		a := testApp()
		tt := runListOutput(t, a, listOutput(300))
		if a.output == nil || !a.output.IsList {
			t.Fatalf("the capture did not open as the list page: %+v", a.output)
		}
		bounds := tt.Image().Bounds()
		x, y := float32(bounds.Max.X/2), float32(bounds.Max.Y*3/4)
		tt.Wheel(x, y, 0, 400)
		tt.Advance(500 * time.Millisecond)
		tt.Frame()
		first, last := a.OutputList.Visible()
		if last < first || first == 0 {
			t.Fatalf("the wheel did not scroll the first list: visible = %d..%d", first, last)
		}
		scrolledFirst := first

		tt.Key(0, ui.KeyEscape)
		tt.Frame()
		if a.InOutputView() {
			t.Fatal("Escape did not close the output view")
		}
		a.showOutput(extensions.CapturedRun{
			Command: []string{"tool"}, Success: true, Stdout: listOutput(300),
		}, nil, nil, nil)
		tt.Frame()

		first, last = a.OutputList.Visible()
		if last < first {
			t.Fatalf("the fresh list has no visible rows: %d..%d", first, last)
		}
		if first != 0 {
			t.Errorf("a fresh list output starts at visible row %d (the previous list was at %d), want 0: "+
				"the new capture opens scrolled down", first, scrolledFirst)
		}
	})
}
