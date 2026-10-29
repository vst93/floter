package terminalui

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
)

// P11-bg · **失败提示也是挂在标题条下沿的一条带，不是压在输出上的玻璃片。**
//
// 用户实测 F3 的原话是「不要 floating/overlay 观感，元素要 inline、要贴进布局」。
// P11-bc 把「进程已退出」那条常驻便条改成了画布底边的整宽带，本条是同一页上的
// 另一半：动作失败时那条 4.5 秒的提示，以前是 `Absolute().Top(3).Left(3)` 的圆角
// 浮卡（`Float` 面、`ElevThrow` 投影、`LensRim` 受光边、160ms 从上方 2u 落下的
// 入场）。它现在挂在标题条（`barHeight`）下沿、整宽贴边，面用带子那一档
// （`Control`），收口是**底边**一条 `HairlineSoft`——底边那条带是上边一条，
// 两条带各自朝画布内侧收。
//
// 两条 pin：
//
//  1. 像素：带子的第一行就是整宽的面（左右边缘与中间同一个颜色、方角），上面
//     一行与更上面的画布逐字节相同（浮卡的投影会在这里留一圈更暗的像素），
//     下面只有**一行**发丝，发丝之下又是画布。
//  2. 源码契约：它 `PassThrough()`，并且不再带投影、内高光、圆角与入场动画。
func TestTheFailureNoteIsADockedStrip(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, size := range [][2]int{{860, 600}, {720, 488}} {
			tag := appearance + " " + strconv.Itoa(size[0]) + "x" + strconv.Itoa(size[1])
			a := New(newStore(), Actions{}, nil)
			withSession(a, &terminal.Terminal{})
			a.ShowFeedback("no emulator")
			tt := render(t, a, size[0], size[1])
			if dark {
				tt.SetDark(true)
				tt.Frame()
			}
			words, ok := tt.Find("no emulator")
			if !ok {
				t.Fatalf("%s: the failure note is not on the frame: %q", tag, tt.Texts())
			}
			img := tt.Image()
			w, h := img.Rect.Dx(), img.Rect.Dy()
			// The canvas's own fill, read where nothing is docked: the bottom
			// row's middle. There is no resident note in this fixture.
			canvas := inkAt(img, w/2, h-1)

			// The band's first row: the first row above the words that is not
			// the canvas. The bar above it is transparent at rest, so the row
			// before the band is the canvas too.
			top := -1
			for y := int(words.Y) - 1; y > 0; y-- {
				if inkAt(img, 2, y) != canvas {
					top = y
					continue
				}
				break
			}
			if top < 0 {
				t.Fatalf("%s: the note has no band above its words %+v", tag, words)
			}
			face := inkAt(img, 2, top)

			// 1. Edge to edge and square: the band's very first row is the same
			//    colour at both panel edges and in the middle.
			for _, x := range []int{0, 2, w / 2, w - 3, w - 1} {
				if got := inkAt(img, x, top); got != face {
					t.Errorf("%s: the band's first row runs %v at x=%d, not its face %v — "+
						"it is inset or rounded like a floating card", tag, got, x, face)
				}
			}
			// 2. It reads as a band, not as more output.
			if face == canvas {
				t.Errorf("%s: the band's face %v is the canvas itself: nothing marks the band", tag, face)
			}
			// 3. Nothing lies on the output above it: the row over the band is
			//    the canvas, the same colour it is twenty rows higher. A
			//    floater's `ElevThrow` darkens exactly this band, and a rim
			//    would light the row under it.
			if got := inkAt(img, 2, top-1); got != canvas {
				t.Errorf("%s: the row above the band is %v but the canvas is %v — "+
					"a shadow or a rim is lying on the output", tag, got, canvas)
			}
			// 4. One hairline, and only one, under the words: the band closes
			//    towards the canvas below it, and the row under that hairline is
			//    the canvas again (no second edge, no second shadow).
			seam := -1
			for y := int(words.Y+words.H) + 1; y < h-2; y++ {
				if inkAt(img, 2, y) != face {
					seam = y
					break
				}
			}
			if seam < 0 {
				t.Fatalf("%s: the band under its words %+v never ends", tag, words)
			}
			if left, right := inkAt(img, 2, seam), inkAt(img, w-3, seam); left != right {
				t.Errorf("%s: the band's bottom edge runs %v at x=2 and %v at x=%d — "+
					"it is inset like a floating card", tag, left, right, w-3)
			}
			if got := inkAt(img, 2, seam+1); got != canvas {
				t.Errorf("%s: the row under the band's hairline is %v but the canvas is %v — "+
					"something still lies between the band and the output", tag, got, canvas)
			}
			// 5. The words sit inside the band, not above its top edge.
			if int(words.Y) < top || int(words.Y+words.H) > seam {
				t.Errorf("%s: the words %+v are not inside the band (top %d, bottom %d)",
					tag, words, top, seam)
			}
		}
	}
}

// The source contract: the note lets the pointer through (the reference's
// `pointer-events: none`), it hangs from the bar's own band, and none of the
// floater's four chrome pieces are back on it.
func TestTheFailureNoteLetsThePointerThrough(t *testing.T) {
	body := funcBody(t, "feedbackNote")
	for _, gone := range []string{"ElevThrow(", "InsetShadow(", "Radius(", "Transition("} {
		if strings.Contains(body, gone) {
			t.Errorf("feedbackNote still carries %q: the note is a floating card again", gone)
		}
	}
	if !strings.Contains(body, ".PassThrough()") {
		t.Error("feedbackNote is not PassThrough: the band eats the pointer over the first lines of output")
	}
	if !strings.Contains(body, "Top(barHeight(t))") {
		t.Error("feedbackNote does not hang from the title bar's lower edge")
	}
	if !strings.Contains(body, "Left(0).Right(0)") {
		t.Error("feedbackNote is not edge to edge: an inset band reads as a card")
	}
	if !strings.Contains(body, "Background(tokens.Control)") {
		t.Error("feedbackNote does not wear the drawer's own band face")
	}
	if !strings.Contains(body, "BorderWidth(0, 0, 1, 0)") {
		t.Error("feedbackNote's hairline is not on its bottom edge alone")
	}
}

// The two frames the report reads. Opt-in like the other dumps; the suffix is
// the round's own knob so the same pin can write the "before" frame from the
// previous commit (`FLOTER_PNG_SUFFIX=-before`).
func TestDumpP11BGFailureStripPNGs(t *testing.T) {
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
		a.ShowFeedback("no emulator")
		tt := render(t, a, 860, 600)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		writePNG(t, "/tmp/floter-P11-bg-feedback-"+appearance+suffix+".png", tt.Image())
	}
}
