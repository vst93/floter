package launcher

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

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
// 这里量的是另一件事：**这一帧画出了哪几档高度**。
//
// # 阶梯是出货尺的读数，不是框架库存尺的读数（P13-dy，§6.94）
//
// 这份文件从 P11-bz 起量的一直是 `render`——**框架库存主题**的帧（`Spacing 4.0`、
// `FontSize 14`）。那时记下的六档 `15 / 18 / 22 / 24 / 25 / 28` 是库存尺的刻度，
// 其中 `18 / 22 / 24` 是同一把尺子的三个字号，app 一个都不画。P13-dq 把两把尺子
// 分开命名之后（`render` 库存 / `renderShipped` 出货），这条 pin 是 `dg-l` 最后一格。
//
// 出货尺（`u = 3.6/4 = 0.9`）实测，en/zh × 深/浅 四个组合逐字节相同：
//
//	15.0 — 键帽的图例（`tokens.Caption` 9.9 的一行）
//	17.0 — `tokens.Body` 10.8 的一行：结果行副标题 / 动作栏徽标 / 页脚提示 / 空查询页那句
//	18.0 — 空态的陈述句（`quietStatement`，`tokens.Emphasis` 11.7 的一行）
//	19.0 — 结果行的标题（body + `rowTitleStep` 的一行）
//	25.0 — 搜索框里的一行（placeholder / 查询；两把尺子同值，它是控件高度不是字）
//	25.2 — 28u 的图标键（`7 × 3.6`，设置 / 清除）
//
// 所以阶梯是**六档，其中两档只差 0.2**（25 与 25.2）：一个是字段的行，一个是 28u 的
// 方框。0.2 在 1x 屏上落不进不同的行（`round` 之后同高），所以「眼里的阶梯」是五档；
// 但它们是两件不同的东西，账要分开记（见 `TestTheLauncherIconBoxIsSevenUnitsTall`）。
//
// 变红只有两种情况：某一档被改了（`Ladder`），或者该出现的那档没被画出来
// （`Missing`）——后者正是「字被顶出视口 / 某一块没画」的形状。
func TestTheLauncherHeightsStayOnTheLadder(t *testing.T) {
	// P13-dy 重定基线：库存尺的 `[15 18 22 24 25 28]` → 出货尺的这六档。
	const (
		rungKeycap    float32 = 15   // caption 9.9 的图例行
		rungBody      float32 = 17   // body 10.8 的一行
		rungStatement float32 = 18   // 空态陈述句（emphasis 11.7 的一行）
		rungRowTitle  float32 = 19   // 结果行标题（body + rowTitleStep）
		rungField     float32 = 25   // 搜索框里的一行
		rungIconBox   float32 = 25.2 // 28u 的图标键 = 7 × 3.6
	)
	rungs := []float32{rungKeycap, rungBody, rungStatement, rungRowTitle, rungField, rungIconBox}

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
				tt := renderShipped(t, a, dark)
				dumpLadderFrame(t, tt, appearance, "shipped")

				// The empty page: the field's line, the two icon boxes and the
				// placeholder — no result row (no 19), no keycap (no 15) and no
				// statement (no 18: `emptyMessage`'s default branch returns "").
				frameaudit.AuditHeights(t, appearance+" "+lang+" empty page", tt, nil,
					rungs, []float32{rungBody, rungField, rungIconBox}, 0)

				tt.Type("git")
				tt.Frame()
				// A match: the row's title and subtitle, the keycap legend on the
				// action bar and the footer's own keycaps — five of the six rungs;
				// only the empty-state statement is absent.
				frameaudit.AuditHeights(t, appearance+" "+lang+" match", tt, nil,
					rungs, []float32{rungKeycap, rungBody, rungRowTitle, rungField, rungIconBox}, 0)

				tt.Type("zzzz")
				tt.Frame()
				// No match: the statement (emphasis), the field and the boxes —
				// no result row and no keycaps.
				frameaudit.AuditHeights(t, appearance+" "+lang+" no results", tt, nil,
					rungs, []float32{rungStatement, rungField, rungIconBox}, 0)
			})
		}
	}

	// The stock frame — the one this pin used to measure — is kept as a
	// contrast image only (opt-in): the two frames differ in every rung, and
	// that is the point of the re-baseline.
	if os.Getenv("FLOTER_P13DY_DUMP") != "" {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			stock := testApp()
			stock.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
			tt := render(t, stock)
			tt.SetDark(dark)
			tt.Frame()
			tt.Type("git")
			tt.Frame()
			dumpLadderFrame(t, tt, appearance, "stock")
		}
	}
}

// TestTheLauncherIconBoxIsSevenUnitsTall pins the two rungs that are only a
// fifth of a pixel apart, because a ladder that hides a pair that close is a
// ladder that will lose one of them (§6.86's ±0.5 band swallowed exactly this).
//
// The icon box is `28u` in the reference — `7 × Space(1)` = `7 × Spacing` = 25.2 — and
// the field's line is 25. They are 0.2 apart: on a 1x screen both round to the same
// row, so they read as one rung to the eye, and they stay two entries in the ledger.
func TestTheLauncherIconBoxIsSevenUnitsTall(t *testing.T) {
	spacing, _, _, _ := designMetrics()
	box := 7 * spacing // 28u
	if got, want := tenths(box), 252; got != want {
		t.Errorf("the icon box is %d tenths, want %d (7 × %v)", got, want, spacing)
	}
	if gap := tenths(box) - tenths(25); gap != 2 {
		t.Errorf("the icon box and the field's line are %d tenths apart, want 2", gap)
	}
	// The same two numbers the ladder is pinned on, read off a real frame.
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	tt := renderShipped(t, a, false)
	seen := map[float32]bool{}
	for _, h := range frameaudit.Heights(tt, nil) {
		seen[h] = true
	}
	for _, rung := range []float32{25, 25.2} {
		if !seen[rung] {
			t.Errorf("the frame did not paint the %v rung (heights %v)", rung, frameaudit.Heights(tt, nil))
		}
	}
}

// dumpLadderFrame writes one frame to `/tmp/floter-P13-dy-ladder-<appearance>-<ruler>.png`
// for a human eye (opt-in: `FLOTER_P13DY_DUMP=1`).
func dumpLadderFrame(t *testing.T, tt *ui.Tester, appearance, ruler string) {
	t.Helper()
	if os.Getenv("FLOTER_P13DY_DUMP") == "" {
		return
	}
	path := fmt.Sprintf("/tmp/floter-P13-dy-ladder-%s-%s.png", appearance, ruler)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", path)
}
