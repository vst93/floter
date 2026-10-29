package launcher

import (
	"testing"

	"floter/internal/apps"
	"floter/internal/frameaudit"
	"floter/internal/settings"
)

// P11-bz · **第二把尺子：字盒 / 行盒多高。**
//
// P11-aj 那把尺子（`textframe_test.go`）问的是「有没有两段字压在同一块像素上」。
// 它对**字号暴涨 / 行距失控**这一类回归是聋的，P11-bx 的 M4（横幅字号 ×6）在它那里
// 全绿：启动器的每一页都是会重排的列，字变大只把下面的东西顶下去，谁也不压谁。
//
// 这里量的是另一件事：**这一帧画出了哪几档高度**。启动器的行高阶梯是六个值，各有
// 出处（`docs/rounds/P11-bz-report.md` §1 有逐条对照）：
//
//	15 — 键帽的图例（caption 9.9pt 的一行）
//	18 — 底部提示行 / 空态句（body 的一行）
//	22 — 结果行的副标题、最近启动的行文字
//	24 — 结果行的标题（emphasis 的一行 + 行的上下留白）
//	25 — 搜索框里的一行（placeholder / 查询）
//	28 — 图标按钮（设置、清除）的方框
//
// 六个值在 en/zh × 深/浅 × 三页上**逐字节相同**（本轮实测），所以阶梯只有一份；
// 变红只有两种情况：某一档被改了（`Ladder`），或者该出现的那档没被画出来
// （`Missing`）——后者正是「字被顶出视口 / 某一块没画」的形状。
func TestTheLauncherHeightsStayOnTheLadder(t *testing.T) {
	rungs := []float32{15, 18, 22, 24, 25, 28}
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			t.Run(lang+" "+appearance, func(t *testing.T) {
				a := testApp()
				if err := a.Store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
					t.Fatal(err)
				}
				a.SetTools([]apps.App{
					{Name: "git", Path: "/usr/bin/git"},
					{Name: "Supercalifragilisticexpialidocious-Application-Launcher-With-A-Very-Long-Name", Path: "/usr/bin/long"},
				}, true)
				tt := render(t, a)
				tt.SetDark(dark)
				tt.Frame()
				// The empty page: recents, the field, the two icon buttons, the
				// footer. No result row, so no 15 (keycap) and no 24 (row title).
				frameaudit.AuditHeights(t, appearance+" "+lang+" empty", tt, nil,
					rungs, []float32{18, 22, 25, 28}, 0.5)

				tt.Type("git")
				tt.Frame()
				// A match: the row's title and subtitle, the keycap legend on the
				// action bar and the footer's own keycaps — all six rungs.
				frameaudit.AuditHeights(t, appearance+" "+lang+" match", tt, nil,
					rungs, rungs, 0.5)

				tt.Type("zzzz")
				tt.Frame()
				// No match: the empty-sentence row and the field, no keycaps.
				frameaudit.AuditHeights(t, appearance+" "+lang+" no results", tt, nil,
					rungs, []float32{18, 25, 28}, 0.5)
			})
		}
	}
}
