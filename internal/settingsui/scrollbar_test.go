package settingsui

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestScrollbarHidesAfterScrollingWithPointerInContent(t *testing.T) {
	var state ui.ScrollState
	var activity scrollbarActivity
	var alpha uint8
	tt := ui.NewTester(func(c *ui.Context) {
		themed := *c.Theme()
		ui.Themed(c, &themed).Fill().Children(func() {
			scroll := ui.Scroll(c).TrackScroll(&state).KeepScrollbars(true).Fill()
			themed = activity.theme(c, scroll, &state)
			alpha = themed.Scrollbar.A
			scroll.Children(func() { ui.Box(c).FillWidth().Height(1500) })
		})
	}, 200, 200)
	tt.Frame()
	tt.Move(100, 100)
	tt.Frame()
	if alpha != 0 {
		t.Fatal("content hover reveals idle scrollbar")
	}
	tt.Scroll(100, 100, 0, 90)
	tt.Advance(300 * time.Millisecond)
	if state.Y <= 0 || alpha == 0 {
		t.Fatalf("scrolling: y=%v alpha=%v", state.Y, alpha)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if alpha != 0 {
		t.Fatalf("stopped scrollbar alpha=%v", alpha)
	}
	hidden := image.NewRGBA(tt.Image().Bounds())
	copy(hidden.Pix, tt.Image().Pix)
	before := state.Y
	tt.Move(191, 100)
	tt.Frame()
	if alpha == 0 {
		t.Fatal("gutter hover cannot reveal drag handle")
	}
	if state.Y != before {
		t.Fatal("revealing scrollbar changed offset")
	}
	visible := tt.Image()
	different := false
	for y := 5; y < 195; y++ {
		for x := 185; x < 197; x++ {
			if hidden.RGBAAt(x, y) != visible.RGBAAt(x, y) {
				different = true
			}
		}
	}
	if !different {
		t.Fatal("scrollbar theme change did not reach painted pixels")
	}
	tt.Press(194, 28)
	tt.Move(194, 90)
	tt.Release(194, 90)
	tt.Frame()
	if state.Y <= before {
		t.Fatalf("thumb drag did not move content: %v -> %v", before, state.Y)
	}
}
