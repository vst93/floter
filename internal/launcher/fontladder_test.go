package launcher

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-de · 启动器的字是**阶梯上的档**，不是 body ± N。
//
// 这一族上一轮（P13-dd）在设置页收掉了 24 处，本轮收启动器的 9 处。判据同上一轮：
// 偏移量**不随界面步长缩放**，所以在出货档（0.9）上它与阶梯几乎重合（差 0.1 点），
// 但用户把「界面大小」调到最小一档时两者就分家（P13-dd 实测，参考字体）：
//
//	表达式             tiny 0.8   small 0.9   default 1.0   large 1.1
//	`t.FontSize-1` 行盒    13          15           17          19
//	`tokens.Caption`      14          15           17          19
//	`t.FontSize+2` 行盒    18          19           22          23
//	`tokens.Title`        17          19           22          23
//
// 这两条 pin 在 **tiny 档**上量，并把「尺子」也画进同一套主题里：期望值不是写死的
// 数字，而是同一个字符串在同一个主题、同一族字体下按两个寄存器各画一次的盒子
// （`ui.Tester` 的 `MeasureText` 走的是画笔默认字体，与页面画出来的字形不是同一族，
// 量出来的高度差 2px——所以这里量的是**画出来的**盒子，不是 `MeasureText` 的数）。
// 每条 pin 先断言两个寄存器在这个档位上真的画得不一样，再断言产品画的那个取的是阶梯。

// p13deProbe is a result row's title: the one text this package draws at the
// ladder's *title* step.
const p13deProbe = "Recent 00"

// p13deProbeBox draws one string at one register, in the app's own theme at one
// interface step, and returns the box the frame laid it out in.
func p13deProbeBox(t *testing.T, step string, text string, size func(theme.Tokens) float32) (h float32) {
	t.Helper()
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.UIScale = step }); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		tk := a.tokens(c)
		c.SetTheme(tk.Theme)
		ui.Text(c, text).FontSize(size(tk))
	}, 600, 200)
	tt.Frame()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the probe %q was not drawn: %q", text, tt.Texts())
	}
	return box.H
}

// p13deFrame draws one launcher frame at one interface step, on the app's own
// theme (the shell installs it on the real window; a bare tester would draw the
// framework's stock 12px). One App is one window: the tester is built once and
// re-framed, because an App bound to two windows panics in `FocusSearch`.
func p13deFrame(a *App) *ui.Tester {
	tt := ui.NewTester(func(c *ui.Context) {
		tk := a.tokens(c)
		c.SetTheme(tk.Theme)
		a.View(c)
	}, InputWindowWidth, 1400)
	tt.Frame()
	return tt
}

// TestTheArmedDeletesConfirmIsACaption pins de-7: the two-step delete's
// confirmation is a caption, and it is the one text on the row that is neither
// grown nor stretched (`NoWrap().Shrink(0)`), so its width is the register it is
// drawn at.
func TestTheArmedDeletesConfirmIsACaption(t *testing.T) {
	a := stepApp(t, "tiny")
	copy := a.copy()
	text := copy.DeleteConfirm
	rungH := p13deProbeBox(t, "tiny", text, func(tk theme.Tokens) float32 { return tk.Caption })
	oldH := p13deProbeBox(t, "tiny", text, func(tk theme.Tokens) float32 { return tk.Theme.FontSize - 1 })
	if rungH == oldH {
		t.Fatalf("the tiny step draws caption and body-1 in the same %v line box over %q: this pin "+
			"cannot see the register it pins", rungH, text)
	}
	tt := p13deFrame(a)
	results := a.Results()
	if len(results) == 0 {
		t.Fatal("the launcher drew no rows")
	}
	a.armedDelete, a.armedDeleteAt = results[a.Selected].ID, a.now()
	tt.Frame()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the armed confirmation is not drawn: %q", tt.Texts())
	}
	if box.H != rungH {
		t.Errorf("the confirmation's line box = %v, want the caption rung's %v (body-1 draws %v)",
			box.H, rungH, oldH)
	}
	if box.W <= 0 {
		t.Errorf("the confirmation was laid out %v wide", box.W)
	}
}

// TestTheFilterRowsHintIsACaption pins de-1: the chips row's hint is the ladder's
// caption, so at the tiny step its line box is the caption probe's and not
// `body-1`'s.
func TestTheFilterRowsHintIsACaption(t *testing.T) {
	a, _ := clipboardApp(t)
	if err := a.Store.Update(func(s *settings.Settings) { s.UIScale = "tiny" }); err != nil {
		t.Fatal(err)
	}
	a.clipboard = false
	a.EnterBrowser()
	copy := a.copy()
	text := copy.BrowserFilterHint
	rungH := p13deProbeBox(t, "tiny", text, func(tk theme.Tokens) float32 { return tk.Caption })
	oldH := p13deProbeBox(t, "tiny", text, func(tk theme.Tokens) float32 { return tk.Theme.FontSize - 1 })
	if rungH == oldH {
		t.Fatalf("the tiny step draws caption and body-1 in the same %v line box over %q: this pin "+
			"cannot see the register it pins", rungH, text)
	}
	tt := p13deFrame(a)
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the chips row's hint is not drawn: %q", tt.Texts())
	}
	if box.H != rungH {
		t.Errorf("the hint's line box = %v, want the caption rung's %v (body-1 draws %v)",
			box.H, rungH, oldH)
	}
}

// The opt-in dump: the launcher's rows at the tiny step before and after the
// round, both appearances. Run with FLOTER_P13DE_TAG=before while the launcher
// still says `t.FontSize ± N` to get the pair.
func TestDumpP13DEFontLadderPNGs(t *testing.T) {
	if os.Getenv("FLOTER_P13DE_DUMP") == "" {
		t.Skip("set FLOTER_P13DE_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13DE_TAG")
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
		// The drawn register this frame is evidence for: the armed delete's
		// confirmation (the one caption sentence a rendered list carries).
		if results := a.Results(); len(results) > 0 {
			a.armedDelete, a.armedDeleteAt = results[a.Selected].ID, a.now()
		}
		tt.Frame()
		name := fmt.Sprintf("/tmp/floter-P13-de-launcher-%s-%s.png", appearance, tag)
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", name)
	}
}
