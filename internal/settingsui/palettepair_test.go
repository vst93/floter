package settingsui

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
)

// P13-fq（`docs/liquid-glass.md` §6.136）：P13-fp 的「仍待」第 1 条问的是
// `paletteRow` 的**行内间距**——`Justify(ui.SpaceBetween)` 把 picker 与预览块
// 推到行的两端，出厂值下两者之间是一大片空。它同时问了两件事：这个空在四个台阶
// 上到底是多少，以及它和卡片其它「两端对齐」的行是不是同一条规则。
//
// 先量（720 宽的真实 General 页，四台阶；行内容盒 = 卡内宽 − 2×inset，随台阶
// 收窄 498.6 → 472.2；picker 盒恒为 220 字面；预览块 = `Space(37)` 随台阶长
// 118.4 → 162.8）：
//
//	台阶     行内容盒   picker 盒   预览块    中间的空   空/块
//	tiny     498.6     220        118.4    160.2     1.35
//	small    489.8     220        133.2    136.6     1.03
//	default  481.0     220        148.0    113.0     0.76
//	large    472.2     220        162.8     89.4     0.55
//
// 结论一：这个空**不是一条间距**，是余数——两个定值（picker 的 220 字面地板、
// 行内容盒）夹一个随台阶缩放的块，于是空跟着台阶从 160 摆到 89（70.8px）。
// 结论二：它和卡片其它两端对齐的行**是同一条规则**：预览块的右沿与同一张卡上
// 「14 px」读数（`slider` 标题行的第二件）的右沿逐像素相同，都是行内容盒的右沿；
// picker 的左沿与行名同一条左沿。也就是说这一行的两端都是真的「两端」，中间的
// 空是余数，不是漏改。
//
// 为什么不动（三条 pin 钉住的就是这三句话）：picker 的 220 是与同卡字体族 picker
// 共享的盒（§6.133/§6.134，P13-fo 刚钉死「两枚 picker 同一个盒」）；预览块必须
// 落在卡片控制列上（否则这一行的右沿会从卡片的右列里凹进去一块）；于是余数只能
// 待在两者之间。唯一的另一种排法——让预览块吃掉余数（`Grow(1)`，空恒定、块
// 变宽到 250…280）——会把这块缩微终端画成一大片空底色，是**可见的产品改动**，
// 已写进 `.pending-bugs.md` 的「需用户拍板」。

// palettePageFrame draws the General page at one step and returns the frame, the
// palette row's preview block, and the card's control-column right edge (the
// font-size readout's own right edge, which is the row content box's right).
func palettePageFrame(t *testing.T, step string, dark bool) (*ui.Tester, ui.Rect, float32) {
	t.Helper()
	en := i18n.For("en").Settings
	s := settings.Default()
	s.UIScale = step
	a := New(settings.NewStore(s), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	a.Page = PageGeneral
	tt, _ := renderIn(t, a, launcher.InputWindowWidth, 1800, dark)
	block, ok := tt.Find(en.PalettePreview)
	if !ok {
		t.Fatalf("step=%s/dark=%v: the preview block is missing", step, dark)
	}
	readout, ok := tt.Find("14 px")
	if !ok {
		t.Fatalf("step=%s/dark=%v: the card's control column could not be read (no font-size readout)", step, dark)
	}
	return tt, block, readout.X + readout.W
}

// funcBody is one function's source text from its declaration to the next
// top-level declaration: enough to pin what a builder hands its control without
// reading pixels off a pill's rounded end.
func funcBody(t *testing.T, src, name string) string {
	t.Helper()
	i := strings.Index(src, "func (a *App) "+name+"(")
	if i < 0 {
		t.Fatalf("%s is gone — the row's builder changed shape", name)
	}
	rest := src[i:]
	if j := strings.Index(rest[1:], "\nfunc "); j >= 0 {
		return rest[:j+1]
	}
	return rest
}

// TestThePreviewBlockEndsOnTheCardColumn is the answer to the second half of
// P13-fp's question: this row is the same rule as the card's other
// space-between rows, not a special case. The preview block's right edge is the
// row content box's right edge — the same edge the font-size readout lands on —
// at every step and in both appearances.
func TestThePreviewBlockEndsOnTheCardColumn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, step := range []string{"tiny", "small", "default", "large"} {
			_, block, column := palettePageFrame(t, step, dark)
			if got := block.X + block.W; absFloat(got-column) > 0.5 {
				t.Errorf("step=%s/dark=%v: the preview block ends at %.1f but the card's control column is "+
					"at %.1f (%+.1fpx): the palette row's right end is not the card's right end",
					step, dark, got, column, got-column)
			}
		}
	}
}

// TestThePalettePickerSharesTheCardsPickerBox is P13-fo's rule applied to the
// other picker on the card: both are built with the same floor (`MinWidth(220)`),
// so the card has one picker box, not two. The pixel half reads the palette
// picker's painted box off its own value and checks it is still the 220u box the
// step does not move.
func TestThePalettePickerSharesTheCardsPickerBox(t *testing.T) {
	b, err := os.ReadFile("settingsui.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	choose := funcBody(t, src, "chooseSelect")
	palette := funcBody(t, src, "paletteRow")
	for _, c := range []struct{ name, body string }{{"chooseSelect", choose}, {"paletteRow", palette}} {
		if !strings.Contains(c.body, ".MinWidth(220)") {
			t.Errorf("%s no longer floors its picker at 220: the card's two pickers are back to two rules", c.name)
		}
	}
	// `chooseSelect`'s other branch is the extension panel, whose fields are
	// literal px (`extensions.css:2402-2417`) — that literal is deliberate. The
	// settings page's own row must not have one.
	if strings.Contains(palette, ".Width(220)") {
		t.Errorf("paletteRow pins its picker to a literal 220 again: 220 is a floor, not a ceiling (P13-fn, §6.133)")
	}

	en := i18n.For("en").Settings
	for _, dark := range []bool{false, true} {
		var first int
		for i, step := range []string{"tiny", "small", "default", "large"} {
			tt, block, _ := palettePageFrame(t, step, dark)
			img := tt.Image()
			label, ok := tt.Find(en.Palette)
			if !ok {
				t.Fatalf("step=%s/dark=%v: the row label is missing", step, dark)
			}
			value, ok := tt.Find("Follow app")
			if !ok {
				t.Fatalf("step=%s/dark=%v: the picker's value is missing", step, dark)
			}
			lo, hi := faceBoxSpanAt(img, value, int(label.X), int(block.X)-1)
			width := hi - lo + 1
			if width < 100 {
				t.Fatalf("step=%s/dark=%v: the picker's face could not be read (%dpx wide)", step, dark, width)
			}
			if absFloat(float32(lo)-label.X) > 2.5 {
				t.Errorf("step=%s/dark=%v: the picker starts at %d but the row starts at %.1f (%+.1fpx): "+
					"the row's left end is not the picker's left end", step, dark, lo, label.X, float32(lo)-label.X)
			}
			// The painted face is the box minus its 1px edge and the antialiased
			// rounded ends: measured 216…220 across the four steps.
			if width < 214 || width > 222 {
				t.Errorf("step=%s/dark=%v: the picker paints %dpx, not its 220u floor: the box follows the "+
					"interface step, so it no longer holds its value", step, dark, width)
			}
			if i == 0 {
				first = width
			} else if absFloat(float32(width-first)) > 2 {
				t.Errorf("step=%s/dark=%v: the picker is %dpx wide where tiny's is %dpx: the box follows the "+
					"interface step, so it no longer holds its value", step, dark, width, first)
			}
		}
	}
}

// faceBoxSpanAt is a field's painted span: the face colour is sampled in the
// pill's own inner padding (left of the value), then the band's columns are
// unioned — a 999 radius means no single row spans the whole width.
func faceBoxSpanAt(img *image.RGBA, value ui.Rect, x0, x1 int) (int, int) {
	face := img.RGBAAt(int(value.X)-6, int(value.Y+value.H/2))
	y0, y1 := int(value.Y), int(value.Y+value.H)
	lo, hi := x1, x0
	for x := x0; x < x1; x++ {
		for y := y0; y < y1; y++ {
			if img.RGBAAt(x, y) == face {
				if x < lo {
					lo = x
				}
				if x > hi {
					hi = x
				}
				break
			}
		}
	}
	return lo, hi
}

// TestThePaletteHoleIsARemainderNotAToken is the honest reading of the gap: it
// is what is left over, and the interface step moves it by 70.8px (the block
// grows by 37u while the picker does not). The numbers are pinned so that a
// future change which turns the leftover into a spacing constant — or which
// lets the block absorb it — has to say so out loud.
func TestThePaletteHoleIsARemainderNotAToken(t *testing.T) {
	en := i18n.For("en").Settings
	want := map[string]float32{"tiny": 160.2, "small": 136.6, "default": 113.0, "large": 89.4}
	var last float32
	for i, step := range []string{"tiny", "small", "default", "large"} {
		tt, block, _ := palettePageFrame(t, step, false)
		label, ok := tt.Find(en.Palette)
		if !ok {
			t.Fatalf("step=%s: the row label is missing", step)
		}
		// The picker's box right edge is its own 220u floor off the row's left
		// edge; the hole is what the row has left over between that box and the
		// block. Both ends are layout rects, so no antialiased pill end enters it.
		hole := block.X - (label.X + 220)
		t.Logf("%s: row %.1f…%.1f (%.1f) | picker %.1f…%.1f (220) | block %.1f (%.1f) | hole %.1f",
			step, label.X, block.X+block.W, block.X+block.W-label.X, label.X, label.X+220,
			block.X, block.W, hole)
		if absFloat(hole-want[step]) > 0.6 {
			t.Errorf("step=%s: the leftover is %.1fpx, the pinned baseline is %.1fpx: the row's spacing "+
				"changed — if the leftover became a token, say so in the contract and re-pin it",
				step, hole, want[step])
		}
		if i > 0 && hole >= last {
			t.Errorf("step=%s: the leftover is %.1fpx, not less than the previous step's %.1fpx: the block "+
				"no longer grows with the step, so the gap is no longer a remainder", step, hole, last)
		}
		last = hole
	}
}

// TestDumpP13FQPaletteGap is the round's visual evidence. The round changes no
// pixels, so the picture is not a before/after: it is the row as it stands, at
// the four steps and both appearances, with the leftover between the picker and
// the block readable as a blank run of the card's plane. Opt-in.
func TestDumpP13FQPaletteGap(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		for _, step := range []string{"tiny", "small", "default", "large"} {
			tt, _ := paletteRowFrameIn(t, step, "High contrast", dark)
			name := "light"
			if dark {
				name = "dark"
			}
			writePNG(t, filepath.Join("/tmp", "floter-P13-fq-palettegap-"+step+"-"+name+".png"), tt)
		}
	}
}
