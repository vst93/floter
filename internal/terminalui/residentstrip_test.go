package terminalui

import (
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/i18n"
)

// P11-bc · **「进程已退出」这条便条是画布底边的一条带，不是压在输出上的玻璃片。**
//
// 用户实测 F3 的原话是「不要 floating/overlay 观感，元素要 inline、要贴进布局」。
// F3 修掉了终端页那圈双层框（P11-z），但这一页上最后一处真正的浮层就是这条
// 便条：它以前是 `Absolute().Bottom(3).Left(3).Right(3)` 的圆角浮卡——`Float`
// 面、`ElevThrow` 投影、`LensRim` 受光边，再加一段从上方 2u 落下的 160ms 入场。
// 它还是**常驻**的（会话退出后一直在，直到用户自己关掉），所以在纯白画布上
// 读成一块玻璃片，而不是一条状态。
//
// 两条 pin：
//
//  1. 像素：这条带贴画布底边、整宽、无圆角，上边只有**一行**发丝，发丝上面
//     那一行与更上面的画布逐字节相同（浮卡的投影会在这里留一圈更暗的像素）。
//  2. 源码契约：它 `PassThrough()`（参考的 `pointer-events: none`），并且不再
//     带投影、内高光、圆角与入场动画。
func TestTheResidentNoteIsADockedStrip(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, size := range [][2]int{{860, 600}, {720, 488}} {
			tag := appearance + " " + strconv.Itoa(size[0]) + "x" + strconv.Itoa(size[1])
			a := New(newStore(), Actions{}, nil)
			withSession(a, &terminal.Terminal{})
			a.ResidentAt(0, 3)
			tt := render(t, a, size[0], size[1])
			if dark {
				tt.SetDark(true)
				tt.Frame()
			}
			copy := i18n.For(a.Store.Snapshot().Language).Terminal
			title, ok := tt.Find(copy.ProcessExited(3))
			if !ok {
				t.Fatalf("%s: the resident note is not on the frame: %q", tag, tt.Texts())
			}
			img := tt.Image()
			w, h := img.Rect.Dx(), img.Rect.Dy()

			// The strip's own top: the hairline is the first row above the title
			// whose ink differs from the row under it. The title's box starts
			// one padding step (7u) below it, so the scan starts above the words.
			seam := -1
			for y := int(title.Y) - 1; y > h-120 && y > 0; y-- {
				if inkAt(img, 2, y) != inkAt(img, 2, y+1) {
					seam = y
					break
				}
			}
			if seam < 0 {
				t.Fatalf("%s: the note has no top hairline above its title %+v", tag, title)
			}

			// 1. One hairline, and only one: the row above it is the canvas, the
			//    same colour it is twenty rows higher. A floater's `ElevThrow`
			//    darkens exactly this band, and a rim would light the row under it.
			above, canvas := inkAt(img, 2, seam-1), inkAt(img, 2, seam-20)
			if above != canvas {
				t.Errorf("%s: the row above the strip's hairline is %v but the canvas is %v — "+
					"a shadow or a rim is lying on the output", tag, above, canvas)
			}
			// 2. The hairline is one colour across the full width (the strip is
			//    edge to edge, not a 12u-inset card).
			if left, right := inkAt(img, 2, seam), inkAt(img, w-3, seam); left != right {
				t.Errorf("%s: the strip's top edge runs %v at x=2 and %v at x=%d — "+
					"it is inset like a floating card", tag, left, right, w-3)
			}
			// 3. The bottom row is the strip's face, on both edges and in the
			//    middle: flush with the panel's bottom, square corners.
			face := inkAt(img, w/2, h-1)
			for _, x := range []int{0, 2, w - 3, w - 1} {
				if got := inkAt(img, x, h-1); got != face {
					t.Errorf("%s: the bottom row at x=%d is %v, not the strip's face %v — "+
						"the strip does not reach the panel's bottom edge square", tag, x, got, face)
				}
			}
			// 4. And it is not the canvas: the face has to read as a band, not as
			//    more output.
			if face == canvas {
				t.Errorf("%s: the strip's face %v is the canvas itself: nothing marks the band", tag, face)
			}
		}
	}
}

// The source contract: the note passes the pointer through (the reference's
// `pointer-events: none`), and none of the floater's four chrome pieces are
// back on it.
func TestTheResidentNoteLetsThePointerThrough(t *testing.T) {
	body := funcBody(t, "residentNote")
	for _, gone := range []string{"ElevThrow(", "InsetShadow(", "Radius(", "Transition("} {
		if strings.Contains(body, gone) {
			t.Errorf("residentNote still carries %q: the note is a floating card again", gone)
		}
	}
	if !strings.Contains(body, ".PassThrough()") {
		t.Error("residentNote is not PassThrough: the band eats the pointer over the last lines of output")
	}
	if !strings.Contains(body, "Bottom(0)") {
		t.Error("residentNote is not flush with the canvas bottom")
	}
	if !strings.Contains(body, "Background(tokens.Control)") {
		t.Error("residentNote does not wear the drawer's own band face")
	}
	if !strings.Contains(body, "BorderWidth(1, 0, 0, 0)") {
		t.Error("residentNote's hairline is not on its top edge alone")
	}
}

// The two frames the report reads. Opt-in like the other dumps. The suffix is
// the round's own knob so the same pin can write the "before" frame from the
// previous commit (`FLOTER_PNG_SUFFIX=-before`) without a second test.
func TestDumpP11BCResidentStripPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	suffix := os.Getenv("FLOTER_PNG_SUFFIX")
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := New(newStore(), Actions{}, nil)
		withSession(a, &terminal.Terminal{})
		a.ResidentAt(0, 3)
		tt := render(t, a, 860, 600)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		writePNG(t, "/tmp/floter-P11-bc-resident-"+appearance+suffix+".png", tt.Image())
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path the round names
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", path)
}
