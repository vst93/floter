package settingsui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The settings page's body is one scroll container (settingsui.go,
// `ui.Scroll(...).TrackScroll(&a.Body)`), and a wheel over it is how the user
// reaches the bottom of a page taller than the window. Nothing pinned it: the
// body's `ui.Scroll` had no test, so a wheel that never reached it — an
// ancestor that took the pointer, a body that grew instead of scrolling —
// would have shipped silently. `ui.Tester.Wheel` (mygo v0.6.3,
// `ui/headless.go:658`) is the real path a mouse wheel takes.
func TestTheSettingsBodyScrollsWithTheWheel(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	tt.Frame()
	if a.Body.MaxY <= 0 {
		t.Fatalf("the body does not overflow its frame (MaxY = %v), so there is nothing to scroll: "+
			"the pin below would pass on a page that cannot scroll at all", a.Body.MaxY)
	}
	if a.Body.Y != 0 {
		t.Fatalf("the body starts scrolled to %v, want the start", a.Body.Y)
	}
	page := a.Page

	// A notch in the middle of the body: the pointer is over the page, away
	// from the sidebar and the field rows' own controls.
	tt.Wheel(400, 300, 0, 120)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if a.Body.Y <= 0 {
		t.Fatalf("a wheel notch left the body at Y = %v: the wheel does not reach the body's scroll container",
			a.Body.Y)
	}
	if a.Body.Y > a.Body.MaxY {
		t.Fatalf("the body scrolled past its end: Y = %v > MaxY = %v", a.Body.Y, a.Body.MaxY)
	}
	// Scrolling is not navigating: the page the user is on stays.
	if a.Page != page {
		t.Errorf("the wheel switched the page from %d to %d", page, a.Page)
	}

	// The far end clamps exactly at MaxY, and a wheel back up returns to the
	// very start.
	tt.Wheel(400, 300, 0, a.Body.MaxY+2000)
	tt.Advance(800 * time.Millisecond)
	tt.Frame()
	if a.Body.Y != a.Body.MaxY {
		t.Errorf("after scrolling far past the end the body is at Y = %v, want the clamp at MaxY = %v",
			a.Body.Y, a.Body.MaxY)
	}
	tt.Wheel(400, 300, 0, -(a.Body.MaxY + 2000))
	tt.Advance(800 * time.Millisecond)
	tt.Frame()
	if a.Body.Y != 0 {
		t.Errorf("after scrolling far past the start the body is at Y = %v, want 0", a.Body.Y)
	}
}

// The place the user scrolled to is the app's, not the frame's: a repaint (a
// setting written back, a frame for any other reason) must not send the page
// back to the top. That is what `TrackScroll(&a.Body)` buys, and it is the
// reason the body's offset lives on the app rather than inside the element.
func TestTheSettingsBodyKeepsItsPlaceAcrossARepaint(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	tt.Frame()
	tt.Wheel(400, 300, 0, 300)
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	scrolled := a.Body.Y
	if scrolled <= 0 {
		t.Fatalf("the body did not scroll (Y = %v), so there is no place to keep", scrolled)
	}

	// A settings writeback repaints the page, as any control on it does.
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = "en" }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Body.Y != scrolled {
		t.Errorf("a repaint moved the body from Y = %v to %v: the page jumped under the user",
			scrolled, a.Body.Y)
	}
}
