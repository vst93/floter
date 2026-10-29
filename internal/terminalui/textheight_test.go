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
//	17 — 滑轴的读数（`tokens.Body` + mono，`drawer.go:302`；换成下一档
//	     `tokens.Emphasis` 是 18，既不在阶梯上、也让 17 消失）
//	20 — 迷你终端预览的一行（`t.FontSize - 1` + mono，`terminalui.go:1801`）
//	22 — 行标签 / select 的当前值（`t.FontSize`，`drawer.go:318`、`:343`）
//	28 — 抽屉右上角的关闭按钮（`barButton`，`terminalui.go:1364`）
//	30 — 分段控件的 30u 下限（`control.go:110`），也是行自己的 `MinHeight`
//	     （`drawer.go:315`，同值 `t.Space(7.5)`；P11-ca 的绿变异证明这一档是
//	     控件给的——把行那一个改成 `Space(8)`，画面一个字节都不动）
//
// 16 帧（en/zh × 深/浅 × 640×360 / 720×560 / 900×600 / 1100×700）**逐帧同一组**，
// 所以阶梯只有一份。640×360 是 P11-au 的最小窗口：那时条带被压到画布之下、自己滚，
// 五档仍然都在。
//
// 条带自己的 label 盒子（整条带）与预览的方框（`Palette preview`，88u）是**祖先**，
// `frameaudit.Heights` 的叶子规则本来就把它们排除——它们不是一行字，本轮不钉，
// 也别把它们加进阶梯。
func TestTheDrawerStaysOnTheHeightLadder(t *testing.T) {
	rungs := []float32{17, 20, 22, 28, 30}
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
				{fmt.Sprintf("%d px", s.FontSize), 17, "the font-size readout"},
				// The miniature terminal under the palette picker.
				{"floter --version", 20, "a line of the palette preview"},
				// A row's name (and a select's current value).
				{scopy.LineHeight, 22, "a row's label"},
				// The drawer's own dismiss, in a card's close corner.
				{term.SettingsClose, 28, "the drawer's close button"},
				// A segmented control: the row's box height.
				{optionLabel(scopy.Paddings, s.TerminalPadding), 30, "a segmented control"},
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
	tt := render(t, a, w, h)
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
