package settingsui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/frameaudit"
	"floter/internal/settings"
)

// P11-bz · **第二把尺子：字盒 / 行盒多高** —— 设置页。
//
// 同 `internal/launcher/textheight_test.go`：P11-aj 那把尺子量的是「两段字压在同一块
// 像素上」，它对**字号暴涨**是聋的（P11-bx M4：横幅字号 ×6 全绿，因为页是重排的列，
// 字变大只把下面的顶下去）。这里量的是**这一帧画出了哪几档高度**。
//
// 阶梯是五个页 × 两种语言 × 两种外观 × 两个窗口尺寸**实测出来的并集**（本轮实测，
// 数字见 `docs/rounds/P11-bz-report.md` §2）：
//
//	15 / 17 / 18 / 19 — 四档字盒：键帽图例、副标题、正文、强调行
//	27 / 30 / 30.6    — 三档行盒：带尾随控件的行、滑块轨、多行输入框
//	34               — About 页在 720×560 下的构建信息块
//
// 容差半格（0.5u）：框架按分数像素排版，同一档在相邻两页会报成 18.999998 与 19，
// 半格之内算同一档。**下限 10u**（`frameaudit.MinHeight`）把被滚动口裁剩的碎片
// （720 宽下实测有 2.7 / 5.2 / 6.5u 的几条）挡在外面——那是裁剪，不是行高。
func TestTheSettingsPagesStayOnTheHeightLadder(t *testing.T) {
	rungs := []float32{15, 17, 18, 19, 27, 30, 30.6, 34}
	keep := func(r frameaudit.Run) bool { return r.R.H >= frameaudit.MinHeight }
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			for _, size := range [][2]int{{860, 900}, {720, 560}} {
				for page := 0; page < pageCount; page++ {
					t.Run(fmt.Sprintf("%s %s %s %dx%d", lang, appearance, PageName(page), size[0], size[1]), func(t *testing.T) {
						store := settings.NewStore(settings.Default())
						if err := store.Update(func(s *settings.Settings) { s.Theme = appearance; s.Language = lang }); err != nil {
							t.Fatal(err)
						}
						a := New(store, Actions{})
						a.Integrations = func() []Integration {
							return []Integration{{
								ID: "dev.floter.plain", Name: "Plain Tool", Enabled: true, Running: true,
								RuntimeAvailable: true, Version: "1.0.0", Publisher: "vst",
								Description: "A tool that does one thing.",
							}}
						}
						a.selectPage(page)
						tt := ui.NewTester(func(c *ui.Context) {
							c.SetTheme(a.tokens(c).Theme)
							a.View(c)
						}, size[0], size[1])
						tt.SetDark(dark)
						tt.Frame()
						tt.Frame()
						frameaudit.AuditHeights(t, fmt.Sprintf("%s %s %s %dx%d", appearance, lang, PageName(page), size[0], size[1]),
							tt, keep, rungs, nil, 0.5)
					})
				}
			}
		}
	}
}

// The coverage arm: the General page at the shipped size shows one of each kind of
// line it is built from (a caption legend, a row's subtitle and title, the
// emphasis line, a row with a trailing control). If one of those stops being
// painted — collapsed, pushed out of the scrolling body, or dropped by a font
// change — this reds while the ladder above stays green.
func TestTheGeneralPageStillPaintsEveryKindOfLine(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			store := settings.NewStore(settings.Default())
			if err := store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{})
			a.selectPage(0)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				a.View(c)
			}, 860, 900)
			tt.Frame()
			tt.Frame()
			keep := func(r frameaudit.Run) bool { return r.R.H >= frameaudit.MinHeight }
			frameaudit.AuditHeights(t, lang, tt, keep,
				[]float32{15, 17, 18, 19, 27, 30, 30.6, 34},
				[]float32{17, 18, 19, 27, 30.6}, 0.5)
		})
	}
}
