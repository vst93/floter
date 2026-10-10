package settingsui

import (
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
