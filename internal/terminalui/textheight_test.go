package terminalui

import (
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/frameaudit"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// P11-ca · **第二把尺子的第三个表面：终端的外观抽屉。**
//
// P11-bz 给启动器与设置页各钉了一份「字盒 / 行盒高度」的阶梯
// （`internal/launcher/textheight_test.go`、`internal/settingsui/textheight_test.go`）。
// 抽屉是同一把尺子的第三个表面，但它的行不是 `rows.go` 那种「标题 + 副标题」的行：
// 条带自己的 `drawerRow` / `drawerRowStacked` / `drawerPalette` 建的是终端面的控件
// ——滑轴、开关、分段控件、迷你终端预览——所以它的档位与另外两个表面**没有一档相同**，
// 本轮是**实测**出来的，不是抄的：
//
//	15 — 迷你终端预览的一行（`t.FontSize - 1` + mono，`terminalui.go:1801`）
//	17 — 滑轴的读数（`tokens.Body` + mono，`drawer.go:302`）与行标签 / select 的
//	     当前值（`t.FontSize`，`drawer.go:318`、`:343`）——**同一档**
//	25.2 — 抽屉右上角的关闭按钮（`barButton`，`t.Space(7)`）
//	27 — 分段控件的下限（`t.Space(7.5)`），也是行自己的 `MinHeight`
//	     （`drawer.go:315`，同值）
//
// P13-dp 取景收口：上面这五档是**库存尺**上的读数（17 / 20 / 22 / 28 / 30）。
// 换成设计尺（`Spacing 3.6`、`FontSize 10.80`，见 §6.76）之后实测是
// **15 / 17 / 25.2 / 27**：行盒吃 0.77（22 → 17）、空格吃 0.90（28 → 25.2、
// 30 → 27），而预览那行 `10.80 - 1 = 9.8` 落到 15。两处「档位」塌了一档：
// ① 预览（9.8pt）与读数（10.8pt）现在是两个高度（15 / 17），
// ② 读数与行标签现在是**同一个** 17 —— `tokens.Body` 与 `t.FontSize` 在设计尺上
// 都是 10.80，库存尺的 17 vs 22 是那把尺子（正文 14.00）造出来的差，不是设计。
// 所以这条阶梯在设计尺上只有四档，且读数/标签之间不再有几何判别力：
// 两者的区别是 **mono 字栈**，不是字号（`TestEachDrawerRungHasItsOwnProducer` 照实钉）。
//
// 16 帧（en/zh × 深/浅 × 640×360 / 720×560 / 900×600 / 1100×700）**逐帧同一组**。
// 所以阶梯只有一份。640×360 是 P11-au 的最小窗口：那时条带被压到画布之下、自己滚，
// 五档仍然都在。
//
// 条带自己的 label 盒子（整条带）与预览的方框（`Palette preview`，88u）是**祖先**，
// `frameaudit.Heights` 的叶子规则本来就把它们排除——它们不是一行字，本轮不钉，
// 也别把它们加进阶梯。
func TestTheDrawerStaysOnTheHeightLadder(t *testing.T) {
	rungs := []float32{15, 17, 7 * designSpace, 7.5 * designSpace}
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			for _, size := range [][2]int{
				{settings.MinTerminalWidth, 360},
				{720, 560},
				{900, 600},
				{1100, 700},
			} {
				t.Run(fmt.Sprintf("%s %s %dx%d", lang, appearance, size[0], size[1]), func(t *testing.T) {
					tt, stripTop := drawerHeightFrame(t, lang, dark, size[0], size[1])
					// The drawer's own band: everything below the strip's top edge.
					// Above it are the bar and the canvas, which have their own rungs
					// (the bar's 22/28, the PTY's 20) and are not this surface's.
					keep := func(r frameaudit.Run) bool {
						return r.R.H >= frameaudit.MinHeight && r.R.Y+r.R.H > stripTop
					}
					// Every rung, on every frame: the strip paints its rows at one
					// fixed size, so a missing rung means a kind of row stopped being
					// drawn (P11-ca M3) or got clipped into a fragment.
					frameaudit.AuditHeights(t, fmt.Sprintf("%s %s %dx%d", lang, appearance, size[0], size[1]),
						tt, keep, rungs, rungs, 0.5)
				})
			}
		}
	}
}

// TestEachDrawerRungHasItsOwnProducer pins **which piece of text paints which
// rung**. The ladder alone cannot see a swap: make the row label 20 and the
// preview line 22 and the set is still `{17,20,22,28,30}` while the design has
// moved. So each rung is held to its own producer, and each producer is held to
// the drawer's band (a string that also exists elsewhere in the frame must not
// stand in for the drawer's copy).
func TestEachDrawerRungHasItsOwnProducer(t *testing.T) {
	for _, arm := range []struct {
		lang string
		dark bool
	}{{"en", false}, {"zh", true}} {
		appearance := "light"
		if arm.dark {
			appearance = "dark"
		}
		t.Run(arm.lang+" "+appearance, func(t *testing.T) {
			s := settings.Default()
			scopy := i18n.For(arm.lang).Settings
			term := i18n.For(arm.lang).Terminal
			tt, stripTop := drawerHeightFrame(t, arm.lang, arm.dark, 900, 600)
			pairs := []struct {
				text string
				h    float32
				why  string
			}{
				// The slider's live value: the body step in the mono stack.
				// P13-dp 取景收口：旧值 17（库存尺）→ 17（设计尺，`tokens.Body`
				// = 10.80 画出来也是 17）——两条尺子上同数不同源，照实留着。
				{fmt.Sprintf("%d px", s.FontSize), 17, "the font-size readout"},
				// The miniature terminal under the palette picker.
				// P13-dp 取景收口：旧值 20 → 15（`t.FontSize - 1` = 9.8pt）。
				{"floter --version", 15, "a line of the palette preview"},
				// A row's name (and a select's current value).
				// P13-dp 取景收口：旧值 22 → 17（`t.FontSize` = 10.80）——与上面的
				// 读数同档，因为 `tokens.Body` 也是 10.80。
				{scopy.LineHeight, 17, "a row's label"},
				// The drawer's own dismiss, in a card's close corner.
				// P13-dp 取景收口：旧值 28 → 25.2（`t.Space(7)` = 7 × 3.6）。
				{term.SettingsClose, 7 * designSpace, "the drawer's close button"},
				// A segmented control: the row's box height.
				// P13-dp 取景收口：旧值 30 → 27（`t.Space(7.5)` = 7.5 × 3.6）。
				{optionLabel(scopy.Paddings, s.TerminalPadding), 7.5 * designSpace, "a segmented control"},
			}
			for _, p := range pairs {
				box, ok := tt.Find(p.text)
				if !ok {
					t.Fatalf("%s: the drawer does not draw %q (%s): %v", appearance, p.text, p.why, tt.Texts())
				}
				if box.Y+box.H <= stripTop {
					t.Errorf("%s: %q (%s) is above the strip (y=%.1f h=%.1f, strip top %.1f) — that box is another surface's copy",
						appearance, p.text, p.why, box.Y, box.H, stripTop)
					continue
				}
				if math.Abs(float64(box.H-p.h)) > 0.5 {
					t.Errorf("%s: %q (%s) is %.1f tall, not %.1f — the rung moved, or two rows swapped their heights",
						appearance, p.text, p.why, box.H, p.h)
				}
			}
		})
	}
}

// drawerHeightFrame renders the terminal with its appearance drawer open, in one
// language and appearance, and returns the frame plus the strip's top edge (the
// drawer's own band starts there).
func drawerHeightFrame(t *testing.T, lang string, dark bool, w, h int) (*ui.Tester, float32) {
	t.Helper()
	a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
		t.Fatal(err)
	}
	withSession(a, &terminal.Terminal{})
	a.Drawer = true
	tt := renderThemed(t, a, w, h)
	if dark {
		tt.SetDark(true)
		tt.Frame()
	}
	stripTop, ok := tt.Find(i18n.For(lang).Settings.GroupTerminal)
	if !ok {
		t.Fatalf("%s: the drawer's strip is not on the frame: %v", lang, tt.Texts())
	}
	return tt, stripTop.Y
}
