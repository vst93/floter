package terminalui

import (
	"image"
	"image/png"
	"os"
	"strconv"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P11-av · **抽屉与画布之间那条缝在抽屉的上边，窗口的底边上没有线。**
//
// The reference's drawer is a block docked under the terminal:
// `.terminal-settings-drawer { border-top: 1px solid var(--glass-control-edge);
// background: var(--glass-control) }` (`main:src/styles/terminal.css:627-632`)
// — one hairline, where the output above meets the drawer. The port wrote the
// CSS shorthand's third slot, `BorderWidth(0, 0, 1, 0)`, which is the *bottom*
// edge: the drawer is the panel's last child, so its bottom edge is the
// window's bottom edge. The hairline was therefore painted along the bottom row
// of the window (running through the 12u corner's transition) while the boundary
// the user actually reads — canvas over drawer — had no line at all. The
// comment right above it already said 「只有上边那条线」, so this is the code
// disagreeing with its own contract, not a design change.
//
// What this pin holds, per appearance and per size:
//   - the strip's first row is a **seam**: one colour across the full width,
//     different from the drawer's face;
//   - the row above it is not that seam (the hairline does not leak up into the
//     canvas);
//   - the window's bottom row is the drawer's own face — no line there.
//
// The last rule is asked only where the drawer fits (`MaxY == 0`): in a strip
// that scrolls, the bottom row is whatever row of the grid happens to be at the
// edge, and that is P11-au's business, not this seam's.
func TestTheDrawerSeamIsOnItsTopEdge(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, size := range [][2]int{
			{settings.MinTerminalWidth, settings.MinTerminalHeight},
			{720, 360},
			{720, 600},
			{900, 600},
			{1200, 800},
		} {
			tag := appearance + " " + strconv.Itoa(size[0]) + "x" + strconv.Itoa(size[1])
			a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
			withSession(a, &terminal.Terminal{})
			a.Drawer = true
			tt := render(t, a, size[0], size[1])
			tt.Frame()
			if dark {
				tt.SetDark(true)
				tt.Frame()
			}
			strip, ok := tt.Find(i18n.For(a.Store.Snapshot().Language).Settings.GroupTerminal)
			if !ok {
				t.Fatalf("%s: the drawer's strip is not on the frame: %q", tag, tt.Texts())
			}
			img := tt.Image()
			w, h := img.Rect.Dx(), img.Rect.Dy()
			top := int(strip.Y)
			if top < 1 || top+int(strip.H) > h {
				t.Fatalf("%s: the strip %+v is not inside the window %dx%d", tag, strip, w, h)
			}

			// The drawer's face, read one row below the seam: the strip's own
			// content padding (P11-au keeps the padding inside the scroll
			// content), so no control of any width reaches it — the palette
			// preview in the middle of the grid is white in the light
			// appearance and would read as the canvas.
			seam := inkAt(img, w/2, top)
			face := inkAt(img, w/2, top+1)
			if seam == face {
				t.Errorf("%s: the strip's first row %v is its own face — the drawer and the canvas "+
					"above it meet with no seam", tag, seam)
			}
			// One colour across the whole width: a hairline that stops short of
			// either edge is a line with a broken end.
			for x := 0; x < w; x++ {
				if got := inkAt(img, x, top); got != seam {
					t.Errorf("%s: the seam at y=%d is %v at x=%d but %v at the middle — "+
						"the line does not run the strip's full width", tag, top, got, x, seam)
					break
				}
			}
			if above := inkAt(img, w/2, top-1); above == seam {
				t.Errorf("%s: the row above the strip (y=%d) is the seam colour %v — the hairline "+
					"is painted outside the drawer, on the canvas", tag, top-1, above)
			}
			if a.DrawerScroll.MaxY == 0 {
				if bottom := inkAt(img, w/2, h-1); bottom != face {
					t.Errorf("%s: the window's bottom row is %v, not the drawer's face %v — "+
						"the hairline belongs on the drawer's top edge, not on the window's "+
						"bottom edge", tag, bottom, face)
				}
			}
		}
	}
}

func inkAt(img image.Image, x, y int) [3]uint8 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

// The two frames this round's report reads: the seam on the drawer's top edge,
// at a size where the whole drawer fits (so the bottom of the window is the
// drawer's face, not a scrolled row). Opt-in like the other dumps.
func TestDumpP11AVDrawerSeamPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		a.Drawer = true
		tt := render(t, a, 900, 600)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		path := "/tmp/floter-P11-av-drawer-seam-" + appearance + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
