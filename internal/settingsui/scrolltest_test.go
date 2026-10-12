package settingsui

import (
	"math"
	"testing"
)

// The settings body scrolls with the wheel: the General page is taller than
// the window, so a wheel event moves the body under the pointer.
func TestScrollingTheSettingsBody(t *testing.T) {
	a := newTestApp()
	tt := render(t, a, 720, 560)
	tt.Frame()
	if a.Body.MaxY <= 0 {
		t.Fatalf("the body has nowhere to scroll (max %v)", a.Body.MaxY)
	}
	tt.Scroll(360, 300, 0, 240)
	tt.Frame()
	if a.Body.Y == 0 {
		t.Error("the wheel did not move the settings body")
	}
	// The scroll stays within the content.
	if a.Body.Y > a.Body.MaxY {
		t.Errorf("Y = %v past max %v", a.Body.Y, a.Body.MaxY)
	}
}

// The settings body scrolls to its true bottom: the scroller's viewport is the
// card's body, not the whole window, so the page's last block must come to
// rest fully inside the window at the end of the scroll.
//
// The body row used to be a `Fill` (100% of the card) inside the column that
// also holds the 56u header, so it kept the header's own height below the
// window. The scroller then believed it had that much more viewport than the
// user can see, and the last ~57u of every page sat below the card's bottom
// edge and could not be scrolled to (「设置页面的列表，无法滚动到底部」). The
// row now grows into the space the header leaves, so the viewport is the body
// the user sees. This is the pixel-level acceptance: the page's last block is
// not the clipped stub the old viewport drew, and it rests above the window's
// bottom edge.
func TestTheSettingsBodyReachesItsBottom(t *testing.T) {
	// The page's last block is the terminal palette preview. At a window tall
	// enough that the whole page fits, it is drawn at its full height.
	full := render(t, newTestApp(), 720, 2000)
	full.Frame()
	whole, ok := full.Find("Palette preview")
	if !ok {
		t.Fatal("the page's last block is not built")
	}

	for _, h := range []int{522, 700, 900} {
		a := newTestApp()
		tt := render(t, a, 720, h)
		tt.Frame()
		if a.Body.MaxY <= 0 {
			t.Fatalf("h=%d: the body has nowhere to scroll (max %v)", h, a.Body.MaxY)
		}
		a.Body.Y = math.MaxFloat32
		tt.Frame()
		last, ok := tt.Find("Palette preview")
		if !ok {
			t.Fatalf("h=%d: the page's last block is not built", h)
		}
		// The block is not the clipped stub: its full height shows.
		if last.H < whole.H {
			t.Errorf("h=%d: the last block is %.1f high, want its full %.1f (the viewport was taller than the body)",
				h, last.H, whole.H)
		}
		// And it rests inside the window, not at or past its bottom edge.
		if last.Y+last.H >= float32(h) {
			t.Errorf("h=%d: the last block ends at %.1f, at the window's bottom edge %.1f", h, last.Y+last.H, float32(h))
		}
	}
}
