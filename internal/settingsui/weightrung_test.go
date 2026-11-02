package settingsui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// The weight ladder's real resolution, measured on the ink.
//
// `docs/liquid-glass.md` §6.51 判据 2 记下的那条教训是「只钉最暗像素的 pin
// 抓不到字重」——同一个 token 下 400 与 580 的最暗像素同值。所以字重只能量
// **墨量**。这一条探针把本仓实际在用的十二个字重（460/480/500/510/520/540/
// 560/580/600/620/650/700）各画一遍，同一串字、同一个字号，逐档量两件事：
//
//   - `mass`：墨量 = 逐像素 |luma − 面| 的和，也就是「这一行的字有多黑」；
//   - `px`：墨像素数 = 与面差 ≥ 8 级的像素个数，也就是「笔画有多粗」。
//
// 量三档字号（Caption / Body / Emphasis），因为「650 与 620 差 30」这件事
// 在小字号上可能根本不存在，而在大字号上存在。
//
// **本机读数（如实记录）**：这台机器上没有出货的 UI 字体族（`fc-match Inter`
// 退到 Noto Sans CJK），所以下面这张表量的是**回退族**的分辨率：460..540 一个
// 面、560..620 一个面、650..700 一个面 —— 十二个拼法只有三张字面。它说明的是
// 「10 单位的步进在少于十二张字面的族上不产生像素差」，**不是**出货字体族的
// 分辨率（那要一台装了参考字体的机器才能量）。这条测试只打印，不断言，正是
// 因为它量的是运行环境的字体解析，不是这一页的设计。

const weightProbeText = "Settings file"

// weightInk draws one line at one size and one weight on a flat card face and
// returns its ink mass and its ink pixel count.
func weightInk(t *testing.T, tk theme.Tokens, size float32, weight int) (mass, px int, box ui.Rect) {
	t.Helper()
	var probe ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tk.Theme)
		ui.Box(c).Fill().Background(tk.RaisedQuiet).Children(func() {
			ui.Text(c, weightProbeText).FontSize(size).FontWeight(weight).
				TextColor(tk.TextStrong).SingleLine().Bind(&probe)
		})
		box = c.Resolve(probe).Bounds()
	}, 260, 44)
	tt.Frame()
	img := tt.Image()
	// The plane is the top-left corner: the probe box fills the frame with
	// `RaisedQuiet` and the line is the only ink on it.
	i := 0
	plane := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			j := y*img.Stride + x*4
			l := luma(img.Pix[j], img.Pix[j+1], img.Pix[j+2])
			d := l - plane
			if d < 0 {
				d = -d
			}
			mass += d
			if d >= 8 {
				px++
			}
		}
	}
	return mass, px, box
}

// TestDumpP13EHWeightRungs prints the ladder as the pixels see it.
func TestDumpP13EHWeightRungs(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	tk := theme.For(settings.Default(), false)
	weights := []int{460, 480, 500, 510, 520, 540, 560, 580, 600, 620, 650, 700}
	for _, size := range []struct {
		name string
		pt   float32
	}{
		{"Caption", tk.Caption},
		{"Body", tk.Body},
		{"Emphasis", tk.Emphasis},
	} {
		prevMass, prevPx := -1, -1
		for _, w := range weights {
			mass, px, box := weightInk(t, tk, size.pt, w)
			dm, dp := mass-prevMass, px-prevPx
			t.Log(fmt.Sprintf("%-8s %6.1fpt w=%3d  mass=%5d (%+4d)  px=%3d (%+3d)  box=%.1fx%.1f",
				size.name, size.pt, w, mass, dm, px, dp, box.W, box.H))
			prevMass, prevPx = mass, px
		}
	}
}
