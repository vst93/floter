package settingsui

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The reference's shared text action moves one thing per state:
// `.settings-reset:not(:disabled):hover { opacity: .78 }` and `:active
// { opacity: .6 }` (`settings.css:713-721`). P7-gk ported those two numbers
// literally; P8-d then put the ink on a rung (`TextSecondary → Text`) so the
// action would read as *lit* under the pointer instead of washed out — but it
// left both opacities in place. One state is now drawn with two knobs pulling
// opposite ways:
//
//   - hover lifts the ink a whole rung (80 levels on light) and then fades the
//     whole element by 0.78, so most of the lift never reaches the screen;
//   - press sends the ink *back* to `TextSecondary` and fades that by 0.6, so
//     the frame the user commits in is the faintest of the three — quieter than
//     the resting action it sits on.
//
// §6.59's rule is one fade per intent; §5 and `sidebarRow`'s third rung
// (`settingsui.go:1419`) say the press step changes the *face*, and a text
// action has no face (P7-gk deleted the band on purpose), so its press step is
// the geometry: `Scale(0.986)`, the same sentence the launcher's result row
// draws (`launcher.go:2511`).
//
// The pin measures the two inks against the token they are supposed to *be*
// (not against each other): an opacity-blended rung is not the rung. Then it
// requires the pressed frame to differ from the hovered one, inside the
// action's own box — "press must not be a no-op" (P13-cn's family).
func TestTheTextActionTakesOneRungPerState(t *testing.T) {
	en := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			a := New(newStore(t), Actions{})
			tt, tokens := renderIn(t, a, 720, 800, appearance.dark)
			if err := tt.Click("Shortcuts"); err != nil {
				t.Fatal(err)
			}
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			label := mustFind(t, tt, en.ShortcutsReset)
			cx, cy := label.X+label.W/2, label.Y+label.H/2

			// Resting: the pointer is nowhere near the action.
			restPlane, restInk := actionRungProbe(tt, label)

			tt.Move(cx, cy)
			tt.Frame()
			hoverPlane, hoverInk := actionRungProbe(tt, label)
			hoverFrame := copyFrame(tt.Image())

			tt.Press(cx, cy)
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
			pressPlane, pressInk := actionRungProbe(tt, label)
			pressFrame := copyFrame(tt.Image())
			tt.Release(cx, cy)

			// ① Hover is the ink rung itself, not the rung times an opacity.
			if p := actionRungPosition(hoverInk, tokens.TextSecondary, tokens.Theme.Text); p < 0.85 {
				t.Errorf("the hovered ink %v sits at %.2f of the way from TextSecondary %v to Text %v: "+
					"the ink lift is being faded back — one state, one knob",
					hoverInk, p, tokens.TextSecondary, tokens.Theme.Text)
			}
			// ② Press keeps that rung instead of falling back down the ladder. The
			// 0.986 scale costs the glyph a little coverage, so the floor sits under
			// 1 on purpose: a blend of the two rungs is what this pins away.
			pressRung := actionRungPosition(pressInk, tokens.TextSecondary, tokens.Theme.Text)
			if pressRung < 0.7 {
				t.Errorf("the pressed ink %v sits at %.2f of the way from TextSecondary %v to Text %v: "+
					"press must not step back down the ink ladder",
					pressInk, pressRung, tokens.TextSecondary, tokens.Theme.Text)
			}
			// ③ …so the frame the user commits in is never the faintest one.
			if channelDelta(pressInk, pressPlane) < channelDelta(restInk, restPlane) {
				t.Errorf("the pressed ink stands %d levels off its plane %v and the resting ink "+
					"only %d off %v: the commit frame is the quietest of the three",
					channelDelta(pressInk, pressPlane), pressPlane,
					channelDelta(restInk, restPlane), restPlane)
			}
			// ④ No band under the words in either state: the plane is the page.
			if d := channelDelta(restPlane, hoverPlane); d > 2 {
				t.Errorf("hover painted the plane: %v → %v (delta %d)", restPlane, hoverPlane, d)
			}
			if d := channelDelta(restPlane, pressPlane); d > 2 {
				t.Errorf("press painted the plane: %v → %v (delta %d)", restPlane, pressPlane, d)
			}
			// ⑤ Press is not a no-op: the two frames differ, and only inside the
			// action's own box (the geometry is a render transform on this element).
			n, box := actionRungDiff(hoverFrame, pressFrame)
			if n == 0 {
				t.Fatalf("the pressed frame is pixel-identical to the hovered one: "+
					"holding the action down says nothing (ink %v / %v)", hoverInk, pressInk)
			}
			allowed := ui.Rect{
				X: label.X - tokens.Theme.Space(2), Y: label.Y - tokens.Theme.Space(2),
				W: label.W + tokens.Theme.Space(4), H: label.H + tokens.Theme.Space(4),
			}
			if box.X < allowed.X || box.Y < allowed.Y ||
				box.X+box.W > allowed.X+allowed.W || box.Y+box.H > allowed.Y+allowed.H {
				t.Errorf("press moved %d pixels, bbox %v, outside the action's box %v",
					n, box, allowed)
			}
			t.Logf("%s: rest %v (%d levels off %v) · hover %v (%d, rung %.2f) · press %v (%d, rung %.2f) · press↔hover %d px in %v",
				appearance.name,
				restInk, channelDelta(restInk, restPlane), restPlane,
				hoverInk, channelDelta(hoverInk, hoverPlane), actionRungPosition(hoverInk, tokens.TextSecondary, tokens.Theme.Text),
				pressInk, channelDelta(pressInk, pressPlane), pressRung,
				n, box)
		})
	}
}

// actionRungProbe reads the action's own ink and the plane it sits on: 1u left
// of the first glyph is inside the action's 2u padding (plane, no ink), and the
// label band's farthest pixel from that plane is the glyph core (antialiased
// edges sit between the two, so "farthest" is the ink).
func actionRungProbe(tt *ui.Tester, label ui.Rect) (plane, ink ui.Color) {
	// Read this immutable frame once. Tester.Image converts the whole raster;
	// repeating it for each glyph pixel made this probe take several minutes.
	img := tt.Image()
	pixel := func(x, y float32) ui.Color {
		i := int(y)*img.Stride + int(x)*4
		return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
	}
	midY := label.Y + label.H/2
	plane = pixel(label.X-1, midY)
	far := -1
	for x := int(label.X); x < int(label.X+label.W); x++ {
		for y := int(label.Y); y < int(label.Y+label.H); y++ {
			p := pixel(float32(x), float32(y))
			if d := colorDistance(p, plane); d > far {
				far, ink = d, p
			}
		}
	}
	return plane, ink
}

// actionRungPosition says where an ink sits on the action's own two-rung
// ladder: 0 is `TextSecondary` (the resting action), 1 is `Text` (the lit one).
// A rung drawn through an opacity lands between them — a third ink the ladder
// does not have, which is exactly what P13-cq removed.
func actionRungPosition(ink, secondary, lit ui.Color) float64 {
	step := channelDelta(lit, secondary)
	if step == 0 {
		return 1
	}
	return 1 - float64(channelDelta(ink, lit))/float64(step)
}

// actionRungDiff counts the pixels that moved by more than 8 levels in any
// channel and returns the box they occupy.
func actionRungDiff(before, after *image.RGBA) (int, ui.Rect) {
	b := after.Bounds()
	x0, y0, x1, y1 := 1<<30, 1<<30, -1, -1
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p, q := imagePixel(before, x, y), imagePixel(after, x, y)
			if diff8(p.R, q.R)+diff8(p.G, q.G)+diff8(p.B, q.B) <= 8 {
				continue
			}
			n++
			if x < x0 {
				x0 = x
			}
			if y < y0 {
				y0 = y
			}
			if x > x1 {
				x1 = x
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	if n == 0 {
		return 0, ui.Rect{}
	}
	return n, ui.Rect{X: float32(x0), Y: float32(y0), W: float32(x1 - x0 + 1), H: float32(y1 - y0 + 1)}
}

// The round's visual evidence: the action in its three states, both
// appearances. Opt-in so a normal run does not litter /tmp.
func TestDumpP13CQTextActionRungs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	en := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		for _, state := range []string{"rest", "hover", "press"} {
			a := New(newStore(t), Actions{})
			tt, _ := renderIn(t, a, 720, 800, appearance.dark)
			if err := tt.Click("Shortcuts"); err != nil {
				t.Fatal(err)
			}
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			label := mustFind(t, tt, en.ShortcutsReset)
			cx, cy := label.X+label.W/2, label.Y+label.H/2
			if state != "rest" {
				tt.Move(cx, cy)
				tt.Frame()
			}
			if state == "press" {
				tt.Press(cx, cy)
				tt.Advance(300 * time.Millisecond)
				tt.Frame()
			}
			writePNG(t, filepath.Join("/tmp",
				"floter-P13-cq-textaction-"+appearance.name+"-"+state+".png"), tt)
		}
	}
}
