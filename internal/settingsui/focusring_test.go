package settingsui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The keyboard half of「状态反馈」. P13-fi counted the pointer: fourteen
// `Disabled(...)` sites, one story about the cursor. Nothing has ever counted
// the *other* pointer — the one that lives on the keyboard. `Focusable()` is
// called nineteen times in `internal/`, and not one of those call sites draws
// a ring by hand: the ring is the framework's, painted by `paint.go:385`
// (`p.FocusRing(box, e.radius)`), and it is drawn **outside** the element box:
// `paint.go:1353-1362` builds `Rect{X-3, Y-3, W+6, H+6}` and strokes it 2px
// wide, i.e. the band lives in the three units between `X-3` and `X`.
//
// Two things follow, and both are worth a pin:
//
//  1. **The ring is not on the face, it is around it.** A control whose box is
//     flush against a clipping ancestor loses the ring entirely — the band is
//     outside the box, so a `Clip` at the box edge eats all of it. That is a
//     property of the geometry, and the only way to know it stays true is to
//     read the frame: below, the band is measurably blue and the first row
//     *inside* the box is measurably not.
//  2. **The ring's colour is the palette's** (`theme.go:480` hands
//     `p.focus` to `ui.Theme.Focus`: light `RGBA(6,74,166,0.82)`, dark
//     `RGBA(122,167,255,0.78)`, `theme.go:757`/`:880`). The framework's stock
//     value is a Tailwind blue-600 (`settingsui` would inherit it if that one
//     line were ever dropped), so the pin reads the colour off the frame rather
//     than off the struct.
//
// Measured values are in `docs/rounds/P13-ga-report.md`.

// focusBox is where `lensProbe`'s start-aligned column puts the one control the
// focus probes draw: the probe pads by 20 on both axes and the control is a
// fixed 120×40, so the box is known without searching the tree (and a round
// number keeps the 1px band off a half-covered row — the same reason P13-e put
// the probe at (20,20) rather than centring it).
var focusBox = ui.Rect{X: 20, Y: 20, W: 120, H: 40}

// focusRingBand is the ring's outer offset. `Painter.FocusRing` draws a 2px
// stroke on a rect grown by 3, so the band occupies [X-3, X-1) — two columns on
// the left, two rows on the top.
const focusRingBand = 3

func focusRingControl(c *ui.Context) {
	ui.Row(c).Focusable().Size(focusBox.W, focusBox.H).Radius(6)
}

// focusRingFrames renders the probe twice: once at rest, once after the
// keyboard has walked onto the control with Tab.
func focusRingFrames(t *testing.T, dark bool) (before, after image.Image) {
	t.Helper()
	tt, _ := lensProbe(t, dark, 200, 120, func(c *ui.Context, a *App) { focusRingControl(c) })
	before = copyImage(tt.Image())
	// Tab, not `Focus()`: `ringShown` (`inline.go:223-226`) asks for
	// `rt.focusVisible`, and that flag is only raised for keyboard-driven
	// focus. A ring that appears on a click is a ring the design does not want.
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	after = tt.Image()
	if before.Bounds() != after.Bounds() {
		t.Fatalf("frame size moved between the two reads")
	}
	return before, after
}

func copyImage(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
	return dst
}

func rgbAt(img image.Image, x, y int) (r, g, b int) {
	r16, g16, b16, _ := img.At(x, y).RGBA()
	return int(r16 >> 8), int(g16 >> 8), int(b16 >> 8)
}

// TestTheKeyboardRingRidesOutsideTheBox pins the geometry: with keyboard focus
// the frame differs from the resting frame **only** in the 3-unit band outside
// the box, and inside the box nothing moves. If the ring is ever drawn on the
// face instead (a wash, a border, a brighter background) the inside-exactly
// unchanged arm goes red; if it is ever drawn further out, or clipped by an
// ancestor, the band arm goes red.
func TestTheKeyboardRingRidesOutsideTheBox(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			before, after := focusRingFrames(t, dark)
			x0, y0 := int(focusBox.X), int(focusBox.Y)
			x1, y1 := int(focusBox.X+focusBox.W), int(focusBox.Y+focusBox.H)

			// ① The band is there: exactly the two columns of `[X-3, X-1)` on
			// the left, the two rows of `[Y-3, Y-1)` on top, and the mirror of
			// both on the far side. Blue is the whole point — the ring has to
			// read as the palette's focus, not as a brighter grey.
			cy := (y0 + y1) / 2
			cx := (x0 + x1) / 2
			for _, band := range []struct {
				what string
				x, y int
			}{
				{"left band at X-3", x0 - 3, cy},
				{"left band at X-2", x0 - 2, cy},
				{"top band at Y-3", cx, y0 - 3},
				{"top band at Y-2", cx, y0 - 2},
				{"right band at X+W+1", x1 + 1, cy},
				{"right band at X+W+2", x1 + 2, cy},
				{"bottom band at Y+H+1", cx, y1 + 1},
				{"bottom band at Y+H+2", cx, y1 + 2},
			} {
				r, _, b := rgbAt(after, band.x, band.y)
				if b-r < 60 {
					t.Errorf("%s (%d,%d): the ring is not blue (r=%d b=%d, gap %d < 60)",
						band.what, band.x, band.y, r, b, b-r)
				}
			}

			// ② Nothing inside the box moved — except the two corner vertices,
			// which the rounded band crosses on its way round (measured: the
			// arc of radius 9 centred at (X+6, Y+6) passes through (X, Y)). The
			// box is therefore read from 2 units in, where the ring cannot
			// reach. A ring drawn on the face would show here — and only then.
			for y := y0 + 2; y < y1-2; y++ {
				for x := x0 + 2; x < x1-2; x++ {
					r0, g0, b0 := rgbAt(before, x, y)
					r1, g1, b1 := rgbAt(after, x, y)
					if r0 != r1 || g0 != g1 || b0 != b1 {
						t.Fatalf("the ring was drawn inside the box at (%d,%d): %d/%d/%d → %d/%d/%d",
							x, y, r0, g0, b0, r1, g1, b1)
					}
				}
			}

			// ③ And nothing moved further out than the band: the ring's own
			// 3 units, not a shadow or a halo on top of them.
			b := after.Bounds()
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					inside := x >= x0-focusRingBand && x < x1+focusRingBand &&
						y >= y0-focusRingBand && y < y1+focusRingBand
					if inside {
						continue
					}
					r0, g0, b0 := rgbAt(before, x, y)
					r1, g1, b1 := rgbAt(after, x, y)
					if r0 != r1 || g0 != g1 || b0 != b1 {
						t.Fatalf("the focus frame changed at (%d,%d), outside the 3-unit band", x, y)
					}
				}
			}
		})
	}
}

// TestNoOneTakesTheKeyboardRingAway is the census. The framework paints the
// ring for free, and every `FocusRing(false)` in the tree turns that off — the
// framework's own widgets that use it always draw one by hand right after
// (`collapsible.go:203` → `p.FocusRing(...)`, `list.go:735`, `tabs.go:37`, …).
// This port has nineteen `Focusable()` sites and, as of this round, **zero**
// `FocusRing(false)`: every one of them rides the automatic ring. The registry
// below is by file so that a new focusable site is a deliberate act — adding one
// means opening this table and saying which surface it belongs to.
func TestNoOneTakesTheKeyboardRingAway(t *testing.T) {
	want := map[string]int{
		"power.go":      1,
		"filters.go":    1,
		"launcher.go":   4,
		"output.go":     1,
		"custom.go":     1,
		"savealert.go":  1,
		"settingsui.go": 3,
		"rows.go":       1,
		// P13-id S: no group focus -> one temporary recovery target per
		// surface, only while a removed option's original group owns focus.
		"config.go":      1, // integration multiselect's existing list box
		"pluginsheet.go": 1, // plugin checkbox group's existing row
		"notices.go":     2,
		"pinned.go":      1,
		"detach.go":      1,
		"terminalui.go":  2,
	}
	got := map[string]int{}
	unpaired := []string{}
	for _, dir := range []string{".", "..", "../launcher", "../shell", "../terminalui"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".go" ||
				len(e.Name()) > 8 && e.Name()[len(e.Name())-8:] == "_test.go" {
				continue
			}
			path := filepath.Join(dir, e.Name())
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			// The key is the file name alone: no two of these names collide
			// across the five directories, and the table then does not care
			// which directory a file sits in.
			key := e.Name()
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "Focusable":
					got[key]++
				case "FocusRing":
					// `FocusRing(false)` without a hand-drawn ring in the same
					// function is a control the keyboard cannot find.
					if len(call.Args) != 1 {
						return true
					}
					id, ok := call.Args[0].(*ast.Ident)
					if !ok || id.Name != "false" {
						return true
					}
					if !drawsOwnRing(f, call.Pos()) {
						unpaired = append(unpaired, key+":"+strconv.Itoa(1+len(unpaired)))
					}
				}
				return true
			})
		}
	}
	for file, n := range want {
		if got[file] != n {
			t.Errorf("%s: registry says %d focusable sites, the file has %d", file, n, got[file])
		}
	}
	for file, n := range got {
		if _, ok := want[file]; !ok && n > 0 {
			t.Errorf("%s: %d focusable sites and nobody said which surface they belong to", file, n)
		}
	}
	for _, where := range unpaired {
		t.Errorf("%s: `FocusRing(false)` with no hand-drawn ring", where)
	}
}

// drawsOwnRing reports whether the function holding pos calls `FocusRing` with
// a rectangle (two or more arguments) — the framework's rule for taking the
// ring away: draw your own with `Painter.FocusRing` while `FocusVisible`.
func drawsOwnRing(f *ast.File, pos token.Pos) bool {
	found := false
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || pos < fn.Pos() || pos > fn.End() {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "FocusRing" {
				found = true
			}
			return true
		})
	}
	return found
}

// TestDumpP13GaFocusRing writes the two frames the probe reads, so the band can
// be looked at rather than only counted. Opt-in: `FLOTER_P13GA_DUMP=1`.
func TestDumpP13GaFocusRing(t *testing.T) {
	if os.Getenv("FLOTER_P13GA_DUMP") == "" {
		t.Skip("opt-in: FLOTER_P13GA_DUMP=1 to write the focus-ring frames")
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		_, after := focusRingFrames(t, dark)
		path := filepath.Join(os.TempDir(), "floter-P13-ga-focusring-"+name+".png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, after); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
