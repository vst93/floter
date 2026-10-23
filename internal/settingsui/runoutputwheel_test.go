package settingsui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"floter/internal/settings"
)

// runOutputScrollApp is the fixture: one connected integration whose last
// background run produced far more text than the block's fixed height shows,
// with its output block open. The block is `ui.Scroll(...).MaxHeight(Space(45))`
// (settingsui.go), so ~11 caption lines fill it and 60 lines overflow it.
func runOutputScrollApp(t *testing.T) (*App, string) {
	t.Helper()
	var b strings.Builder
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&b, "line %02d\n", i)
	}
	text := b.String()
	a := oneIntegrationApp(Integration{
		ID: "dev.floter.tool", Name: "V Tools", Version: "1.2.3",
		Enabled: true, Running: true, RuntimeAvailable: true,
		Permissions: []string{"environment"},
		Enforced:    map[string]bool{"environment": true},
		OutputMode:  "background",
	}, Actions{})
	a.SetRunResult("dev.floter.tool", RunResult{Stdout: text, Success: true})
	a.setRunOutputOpen("dev.floter.tool", true)
	return a, text
}

// The integration run output block is its own fixed-height scroll container
// (settingsui.go, `ui.Scroll("settings.runoutput."+id).TrackScroll(a.runOutputScroll(id))
// .MaxHeight(Space(45))`), and a wheel over it is how the user reaches the end
// of a long captured run. P9-h pinned the page body and the two launcher
// scrollers, but this one — the innermost of the three — still had no wheel
// test, so a block that never took the wheel (an ancestor that ate the
// pointer, a MaxHeight that grew instead of scrolling) would have shipped
// silently. `ui.Tester.Wheel` (mygo v0.6.3, ui/headless.go:668) is the real
// path a mouse wheel takes; the block's own ScrollState is where the
// container writes how far it went.
func TestTheRunOutputBlockScrollsWithTheWheel(t *testing.T) {
	a, text := runOutputScrollApp(t)
	// A tall window so the page body does not overflow: the block is then the
	// only scroll container under the pointer, and a scroll can only have come
	// from it. That is the guard that makes the assertion below about the
	// block and not about its ancestor.
	tt := integrationPage(t, a, 720, 2400)
	if a.Body.MaxY != 0 {
		t.Fatalf("the page body overflows its frame (MaxY = %v), so the wheel could be taken by the body: "+
			"render a taller window so only the output block can scroll", a.Body.MaxY)
	}
	scroll := a.runOutputScroll("dev.floter.tool")
	if scroll.MaxY <= 0 {
		t.Fatalf("the output block does not overflow its frame (MaxY = %v), so there is nothing to scroll: "+
			"the pin below would pass on a block that cannot scroll at all", scroll.MaxY)
	}
	if scroll.Y != 0 {
		t.Fatalf("the block starts scrolled to %v, want the start", scroll.Y)
	}
	// The block's viewport on screen: `Find` returns the drawn box of the
	// captured text, which is the scroll viewport (MaxHeight-clipped), and it
	// does not move as the content scrolls — only the content does.
	viewport, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the captured text was not drawn: %q", tt.Texts())
	}
	x, y := viewport.X+viewport.W/2, viewport.Y+viewport.H/2

	tt.Wheel(x, y, 0, 120)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if scroll.Y <= 0 {
		t.Fatalf("a wheel notch left the block at Y = %v: the wheel does not reach the block's scroll container",
			scroll.Y)
	}
	if scroll.Y > scroll.MaxY {
		t.Fatalf("the block scrolled past its end: Y = %v > MaxY = %v", scroll.Y, scroll.MaxY)
	}

	// The far end clamps exactly at MaxY, and a wheel back up returns to the
	// very start — the same two-end contract the body and the launcher
	// scrollers are pinned to.
	tt.Wheel(x, y, 0, scroll.MaxY+2000)
	tt.Advance(800 * time.Millisecond)
	tt.Frame()
	if scroll.Y != scroll.MaxY {
		t.Errorf("after scrolling far past the end the block is at Y = %v, want the clamp at MaxY = %v",
			scroll.Y, scroll.MaxY)
	}
	tt.Wheel(x, y, 0, -(scroll.MaxY + 2000))
	tt.Advance(800 * time.Millisecond)
	tt.Frame()
	if scroll.Y != 0 {
		t.Errorf("after scrolling far past the start the block is at Y = %v, want 0", scroll.Y)
	}
}

// The place the user scrolled to inside the block is the app's, not the
// frame's: a repaint (a setting written back, a frame for any other reason)
// must not send the captured output back to its first line. That is what
// `TrackScroll(a.runOutputScroll(id))` buys, and the reason the offset lives
// on the app rather than inside the element.
func TestTheRunOutputBlockKeepsItsPlaceAcrossARepaint(t *testing.T) {
	a, text := runOutputScrollApp(t)
	tt := integrationPage(t, a, 720, 2400)
	scroll := a.runOutputScroll("dev.floter.tool")
	if scroll.MaxY <= 0 {
		t.Fatalf("the output block does not overflow (MaxY = %v), so there is no place to keep", scroll.MaxY)
	}
	viewport, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the captured text was not drawn: %q", tt.Texts())
	}
	tt.Wheel(viewport.X+viewport.W/2, viewport.Y+viewport.H/2, 0, 300)
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	scrolled := scroll.Y
	if scrolled <= 0 {
		t.Fatalf("the block did not scroll (Y = %v), so there is no place to keep", scrolled)
	}

	// A settings writeback repaints the page, as any control on it does.
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = "en" }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if scroll.Y != scrolled {
		t.Errorf("a repaint moved the block from Y = %v to %v: the captured output jumped under the user",
			scrolled, scroll.Y)
	}
}
