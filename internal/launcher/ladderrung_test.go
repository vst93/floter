package launcher

import (
	"fmt"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-fe · 启动器最后两处「正文 ± N」：结果行的标题、捕获输出页的状态条。
//
// 这一族（`docs/liquid-glass.md` §6.74 / §6.126）的判据不是「好看」而是**跟不跟界面
// 步长走**：偏移量（`t.FontSize ± N`）不缩放，阶梯档（`tokens.Title` / `tokens.Caption`）
// 缩放。出货档（0.9）上两者恰好重合，所以这两条 pin 在 **tiny 档**上量——那里 `+2`
// 的行盒是 18、`title` 是 17，`-1` 是 13、`caption` 是 14。
//
// 量法照 `fontladder_test.go`（P13-de）的约定：期望值不是写死的数字，而是同一个字符串
// 在同一个主题、同一族字体下按两个寄存器**各画一次**的盒子；每条 pin 先断言两个寄存器
// 在这一档上真的画得不一样（否则它看不见自己要钉的东西），再断言产品画的那个取的是阶梯。

// TestTheResultRowsTitleIsTheLaddersTitle pins de-6: the result row's title is the
// ladder's title rung, and the row box `View` measures for it is the same name.
func TestTheResultRowsTitleIsTheLaddersTitle(t *testing.T) {
	rungH := p13deProbeBox(t, "tiny", p13deProbe, func(tk theme.Tokens) float32 { return tk.Title })
	oldH := p13deProbeBox(t, "tiny", p13deProbe, func(tk theme.Tokens) float32 { return tk.Theme.FontSize + 2 })
	if rungH == oldH {
		t.Fatalf("the tiny step draws the title rung and body+2 in the same %v line box over %q: this pin "+
			"cannot see the register it pins", rungH, p13deProbe)
	}
	a := stepApp(t, "tiny")
	tt := p13deFrame(a)
	box, ok := tt.Find(p13deProbe)
	if !ok {
		t.Fatalf("the row title is not drawn: %q", tt.Texts())
	}
	if box.H != rungH {
		t.Errorf("the row title's line box = %v, want the title rung's %v (body+2 draws %v)",
			box.H, rungH, oldH)
	}
	// The box the window is charged for is the same register the row draws: at
	// the tiny step `View` measured 18 for body+2 and 17 for the rung, so a
	// frame that draws the rung while charging body+2 would be a pixel taller
	// than its own ink.
	// 两把尺子：`a.TitleLine` 走 `MeasureText`（画笔默认字体），`rungH` 是画出来的盒，
	// 同一档上两者差 ~2px（见 `fontladder_test.go` 的开头），所以这里只钉**不高于**
	// 画的盒两个像素——body+2 在这一档比阶梯高 1px，改回去会把它顶出这个窗口。
	if want := rungH; float32(a.TitleLine) > want+2 {
		t.Errorf("the row's measured title line = %v, want at most the title rung's %v +2: the window "+
			"is charged for a line the row does not draw", a.TitleLine, want)
	}
}

// TestTheOutputStatusStripIsACaption pins de-9: the captured-output page's status
// strip is the ladder's caption, not body-1.
func TestTheOutputStatusStripIsACaption(t *testing.T) {
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.UIScale = "tiny" }); err != nil {
		t.Fatal(err)
	}
	tt := listOutputShipped(t, a, outputRowsWithActions(6), false)
	if a.output == nil {
		t.Fatal("the captured-output page did not open")
	}
	status := a.output.Status
	if status == "" {
		t.Fatal("the fixture's run has no status word to draw")
	}
	// 条子是**一行三段**的 `RichText`（结局 / 中点 / 图例），`Tester` 认的是整行的字。
	line := status + "  \u00b7  " + a.outputHint(a.copy(), a.output)
	box, ok := tt.Find(line)
	if !ok {
		t.Fatalf("the status strip's line is not drawn: %q", tt.Texts())
	}
	rungH := p13deProbeBox(t, "tiny", line, func(tk theme.Tokens) float32 { return tk.Caption })
	oldH := p13deProbeBox(t, "tiny", line, func(tk theme.Tokens) float32 { return tk.Theme.FontSize - 1 })
	if rungH == oldH {
		t.Fatalf("the tiny step draws caption and body-1 in the same %v line box over %q: this pin "+
			"cannot see the register it pins", rungH, status)
	}
	if box.H != rungH {
		t.Errorf("the status strip's line box = %v, want the caption rung's %v (body-1 draws %v)",
			box.H, rungH, oldH)
	}
}

// TestDumpP13FELadderPNGs is this round's visual evidence: the launcher at the
// tiny step (where the two registers part company) and the captured-output
// page's status strip, both appearances. Run with FLOTER_P13FE_DIR=<dir> and
// FLOTER_P13FE_TAG=before|after.
func TestDumpP13FELadderPNGs(t *testing.T) {
	dir := os.Getenv("FLOTER_P13FE_DIR")
	if dir == "" {
		t.Skip("opt-in: set FLOTER_P13FE_DIR to write this round's frames")
	}
	tag := os.Getenv("FLOTER_P13FE_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := stepApp(t, "tiny")
		tt := ui.NewTester(func(c *ui.Context) {
			tk := a.tokens(c)
			c.SetTheme(tk.Theme)
			a.View(c)
		}, InputWindowWidth, 900)
		tt.SetDark(dark)
		tt.Frame()
		writePNG(t, fmt.Sprintf("%s/floter-P13-fe-rows-%s-%s.png", dir, tag, appearance), tt)

		b := testApp()
		if err := b.Store.Update(func(s *settings.Settings) { s.UIScale = "tiny" }); err != nil {
			t.Fatal(err)
		}
		strip := listOutputShipped(t, b, outputRowsWithActions(6), dark)
		writePNG(t, fmt.Sprintf("%s/floter-P13-fe-strip-%s-%s.png", dir, tag, appearance), strip)
	}
}
