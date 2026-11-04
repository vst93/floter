package settingsui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The keyboard half of「禁用」. P13-fi counted the pointer (a disabled control
// wears `CursorDefault`), P13-s/t/u counted the material (the light goes out),
// P13-ge counted the state inside a disabled groove. The framework's own
// promise is the third leg — `api_gen.go:3481-3482`: a disabled element
// "report[s] no clicks, take[s] no focus" — and nothing in this repo has ever
// read it off a frame.
//
// Two things are measurably true and worth a pin:
//
//  1. **Tab steps over a disabled control.** A three-box probe: with the middle
//     box disabled from the start, Tab walks left → right → wrap, and the
//     disabled box's ring band never leaves the ground.
//  2. **A disabled pill has nowhere for the ring to go.** On a page whose only
//     control is disabled, Tab is a pixel no-op; on the same page with the
//     enabled pill, Tab moves the ring. Without the second arm the first would
//     be vacuously true (a control that is not keyboard-focusable at all also
//     has a still frame).
//
// Measured values are in `docs/rounds/P13-gf-report.md`.

// disabledRingBoxes is the three-box probe's geometry: `lensProbe` pads by 20
// and the boxes are a start-aligned row of 60×40 with an 8-unit gap, so the
// boxes are known without searching the tree — the same reason `focusBox` is a
// constant.
var disabledRingBoxes = [3]ui.Rect{
	{X: 20, Y: 20, W: 60, H: 40},
	{X: 88, Y: 20, W: 60, H: 40},
	{X: 156, Y: 20, W: 60, H: 40},
}

// disabledRingBand sums the ring band outside a box: the two columns of
// `[X-3, X-1)`, inset by four rows at each end so the box's rounded corners are
// not in the reading (a corner's antialiasing is not the ring).
func disabledRingBand(img *image.RGBA, b ui.Rect) int {
	c := lensMean(img, int(b.X)-focusRingBand, int(b.Y)+4, int(b.X)-focusRingBand+1, int(b.Y)+int(b.H)-5)
	return int(c.R) + int(c.G) + int(c.B)
}

func disabledRingBands(tt *ui.Tester) [3]int {
	img := tt.Image()
	var out [3]int
	for i, b := range disabledRingBoxes {
		out[i] = disabledRingBand(img, b)
	}
	return out
}

// TestTabStepsOverADisabledControl pins the traversal: the disabled middle box
// is never the one wearing the ring, and the order is A → C → A, i.e. the
// keyboard does not spend a step on a control that refuses it.
func TestTabStepsOverADisabledControl(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tt, _ := lensProbe(t, dark, 260, 100, func(c *ui.Context, a *App) {
				ui.Row(c).Gap(8).Children(func() {
					for i := 0; i < 3; i++ {
						box := ui.Row(c).Focusable().Size(60, 40).Radius(6)
						if i == 1 {
							box.Disabled(true)
						}
					}
				})
			})
			rest := disabledRingBands(tt)
			// A ring is a visible change: if the band does not move, the probe
			// cannot see a ring at all and everything below is vacuous.
			var walked [3][3]int
			for step := 0; step < 3; step++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
				walked[step] = disabledRingBands(tt)
			}
			t.Logf("%s rest %v; tab %v %v %v", name, rest, walked[0], walked[1], walked[2])

			for step := 0; step < 3; step++ {
				if got := walked[step][1]; got != rest[1] {
					t.Errorf("%s step %d: the disabled box's band is %d but at rest it is %d — the ring was drawn on a control the pointer refuses",
						name, step+1, got, rest[1])
				}
			}
			want := [3]int{0, 2, 0}
			for step, which := range want {
				focused := -1
				for i := 0; i < 3; i++ {
					if walked[step][i] != rest[i] {
						focused = i
					}
				}
				if focused != which {
					t.Errorf("%s step %d: the ring is on box %d, want box %d (the disabled one is box 1, and it must be stepped over)",
						name, step+1, focused, which)
				}
			}
		})
	}
}

// stillAfterTab renders a one-control page, reads the frame, presses Tab and
// reads it again; it reports how many pixels moved.
func stillAfterTab(t *testing.T, dark bool, build func(c *ui.Context, a *App)) (moved int) {
	t.Helper()
	tt, _ := lensProbe(t, dark, 260, 100, build)
	before := tt.Image()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	after := tt.Image()
	b := before.Bounds()
	if after.Bounds() != b {
		t.Fatalf("frame size moved between the two reads")
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if before.RGBAAt(x, y) != after.RGBAAt(x, y) {
				moved++
			}
		}
	}
	return moved
}

// TestADisabledControlHasNowhereForTheRingToGo pins the product's own two
// disabled faces — the action pill (`disabledActionButton`) and the groove
// (`disabledSwitch`) — against the enabled arms of the same two builders. A
// disabled control must not be able to wear the keyboard's ring, and the
// enabled arm proves the ring exists on this page.
func TestADisabledControlHasNowhereForTheRingToGo(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			on := true
			pill := map[bool]func(c *ui.Context, a *App){
				false: func(c *ui.Context, a *App) { a.actionButton(c, "Run") },
				true:  func(c *ui.Context, a *App) { a.disabledActionButton(c, "Run") },
			}
			groove := map[bool]func(c *ui.Context, a *App){
				false: func(c *ui.Context, a *App) { a.switchControl(c, &on) },
				true:  func(c *ui.Context, a *App) { disabledSwitch(a.switchControl(c, &on)) },
			}
			for _, family := range []struct {
				label string
				arms  map[bool]func(c *ui.Context, a *App)
			}{{"pill", pill}, {"groove", groove}} {
				enabledMoved := stillAfterTab(t, dark, family.arms[false])
				disabledMoved := stillAfterTab(t, dark, family.arms[true])
				t.Logf("%s %s: enabled arm moved %d px, disabled arm moved %d px", name, family.label, enabledMoved, disabledMoved)
				if enabledMoved == 0 {
					t.Errorf("%s %s: Tab moved nothing on the enabled arm — this page has no keyboard focus to give, so the disabled arm proves nothing",
						name, family.label)
				}
				if disabledMoved != 0 {
					t.Errorf("%s %s: Tab moved %d px on the disabled arm — a control the pointer refuses is wearing the keyboard's ring",
						name, family.label, disabledMoved)
				}
			}
		})
	}
}
