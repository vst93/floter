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

// P11-cb · **第二把尺子的第四个表面：终端页本身**（不是抽屉）。
//
// 上一轮（P11-ca）量的是外观抽屉的五行；P11-ca 的「仍待」第 2 条把终端页的
// 另外两段点了名：标题栏 / 画布上方浮条（22 / 28）与 PTY 自己画的行（20）。
// 本轮把能看见的那一半量完，并**实测**证明另一半这把尺子看不见：
//
//	15 — 空态键帽的图例（`tokens.Caption`，`terminalui.go:964`）
//	22 — 正文档位：浮条的身份行（`t.FontSize`，`:1485`）、空态两行的标签
//	     （`:966`）、常驻便条的两句（`:1262`/`:1264`）、失败便条（`:1336`）
//	23 — 空态的标题（`t.FontSize + 1`，`:861`）——**单独一档**，全页只此一处
//	28 — 浮条上的按钮（`Size(t.Space(7), Space(7))`，`barButton` `:1366`）
//
// 四档在 **64 帧**（empty / live / resident / feedback × en/zh × 深/浅 ×
// 640×360 / 720×360 / 720×560 / 1100×700）上**逐帧同一组**；live / resident /
// feedback 三臂只是 `{22,28}`，空态是全集。
//
// **PTY 自己画的行（20）不在这份阶梯上，而且不该被假装在这里**：live 臂
// 的阶梯只有 `{22,28}`，画布自己那条 run 是 `Terminal input`（`w×h` = 整个
// 画布，是**祖先**，`Heights` 的叶子规则把它排除），插件按单元格画的那些字
// 一个都没有进 `tt.Texts()`。所以「终端里的字有没有变高/被裁」这把尺子量
// 不到，要另想办法（见 `docs/rounds/P11-cb-report.md` §3）。
//
// 22 与 15 跟启动器 / 抽屉的 22、15 同数不同源（那两处是 `rows.go` 的行标签
// 与键帽图例），别把两边的常量合并——它们各自跟自己的产生者走。
func TestTheTerminalPageStaysOnTheHeightLadder(t *testing.T) {
	rungs := []float32{15, 22, 23, 28}
	for _, arm := range []struct {
		name string
		// must are the rungs this state is required to paint: the empty state
		// owns all four, the three in-session states own the bar's two (there
		// is no keycap and no empty title once a session is on screen).
		must []float32
	}{
		{"empty", []float32{15, 22, 23, 28}},
		{"live", []float32{22, 28}},
		{"resident", []float32{22, 28}},
		{"feedback", []float32{22, 28}},
	} {
		for _, lang := range []string{"en", "zh"} {
			for _, dark := range []bool{false, true} {
				appearance := "light"
				if dark {
					appearance = "dark"
				}
				for _, size := range [][2]int{
					{settings.MinTerminalWidth, settings.MinTerminalHeight},
					{720, 360},
					{720, 560},
					{1100, 700},
				} {
					tag := fmt.Sprintf("%s %s %s %dx%d", arm.name, lang, appearance, size[0], size[1])
					t.Run(tag, func(t *testing.T) {
						tt := pageHeightFrame(t, arm.name, lang, dark, size[0], size[1])
						keep := func(r frameaudit.Run) bool { return r.R.H >= frameaudit.MinHeight }
						frameaudit.AuditHeights(t, tag, tt, keep, rungs, arm.must, 0.5)
					})
				}
			}
		}
	}
}

// heightPair is one run on the page held to the rung its producer declares.
type heightPair struct {
	text string
	h    float32
	why  string
}

// TestEachTerminalPageRungHasItsOwnProducer pins **which piece of text paints
// which rung**, on the arm that actually draws it. The ladder alone cannot see
// a swap (make the empty title 22 and the keycap legend 23 and the set is still
// `{15,22,23,28}` while the design has moved), and it cannot see a rung that
// survived because a *different* surface's copy happens to be 15u tall.
func TestEachTerminalPageRungHasItsOwnProducer(t *testing.T) {
	for _, arm := range []struct {
		lang  string
		dark  bool
		state string
	}{{"en", false, "empty"}, {"zh", true, "empty"}, {"en", false, "resident"}} {
		appearance := "light"
		if arm.dark {
			appearance = "dark"
		}
		t.Run(arm.state+" "+arm.lang+" "+appearance, func(t *testing.T) {
			term := i18n.For(arm.lang).Terminal
			tt := pageHeightFrame(t, arm.state, arm.lang, arm.dark, 900, 600)
			// The bar's identity line and a bar button are on every state: they
			// are the page's chrome, not the body's copy. A held session's name
			// carries its exit code (`terminalui.go:1470-1474`).
			identity := term.Title
			if arm.state == "resident" {
				identity += " \u00b7 " + term.HeaderExited(3)
			}
			pairs := []heightPair{
				{identity, 22, "the bar's identity line"},
				{term.NewCommand, 28, "a bar button"},
			}
			if arm.state == "empty" {
				pairs = append(pairs,
					heightPair{term.EmptyTitle, 23, "the empty state's title"},
					heightPair{"\u21a9", 15, "a keycap's legend"},
					heightPair{term.EmptyNew, 22, "an empty state line's label"},
				)
			} else {
				pairs = append(pairs, heightPair{term.ProcessExited(3), 22, "the resident note's first line"})
			}
			for _, p := range pairs {
				box, ok := tt.Find(p.text)
				if !ok {
					t.Fatalf("%s: the page does not draw %q (%s): %v", appearance, p.text, p.why, tt.Texts())
				}
				if math.Abs(float64(box.H-p.h)) > 0.5 {
					t.Errorf("%s: %q (%s) is %.1f tall, not %.1f — the rung moved, or two runs swapped their heights",
						appearance, p.text, p.why, box.H, p.h)
				}
			}
		})
	}
}

// pageHeightFrame renders one state of the terminal page — no session, a live
// one, one held after its program exited, one with the failure note up — in one
// language and appearance. The bar is revealed, because at rest its content is
// drawn at zero opacity and a hidden run is not the rung under test.
func pageHeightFrame(t *testing.T, state, lang string, dark bool, w, h int) *ui.Tester {
	t.Helper()
	a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
		t.Fatal(err)
	}
	if state != "empty" {
		withSession(a, &terminal.Terminal{})
	}
	switch state {
	case "resident":
		a.ResidentAt(0, 3)
	case "feedback":
		a.ShowFeedback(i18n.For(lang).Terminal.Pin)
	}
	tt := render(t, a, w, h)
	if state != "empty" {
		revealBar(t, tt)
	}
	if dark {
		tt.SetDark(true)
		tt.Frame()
	}
	return tt
}
