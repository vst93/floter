// P11-bz · **第二把尺子：字盒 / 行盒多高。**
//
// `frameaudit.Audit`（同包 `frameaudit.go`）问的是「有没有两段字压在同一块
// 像素上、有没有一段字跑出窗口」。它对**字号暴涨 / 行距失控**这一类回归是聋的：
// 那些页都是**会重排的列**——字变大只是把下面的东西顶下去，谁也不压谁；被滚动口
// 裁掉的 run 又按裁剪后的盒子报出来，也出不了窗。P11-bx 的 M4（横幅字号 ×6）因此
// 全绿，P11-by §2 把这条写进了仍待。
//
// 这个文件补上第二把尺子：**量高度的集合**。一帧里每一个「叶子」run（盒子不包含
// 别的 run 的盒子——包含别人的那个是列表自己的 label、卡片的 label，它报出来的是
// 它画的框，不是一行字）的高度，必须落在该表面**宣告过的行高阶梯**上（`Ladder`），
// 而且该出现的那几档必须真的被画出来了（`Coverage`）。
//
// 两条断言各自抓不同的东西：
//   - `Ladder` 抓「有一档变了」：字号被乘大、行高从 24 掉到 20、控件高度换了数，
//     都会让某个高度离开阶梯；
//   - `Coverage` 抓「某一档不见了」：字被顶出视口 / 被裁成碎片 / 整块没画，
//     都会让该出现的档位缺失。
//
// 已知盲点（别拿它当 TODO 重领）：它只量**盒子**。字号在 ±5% 以内、四舍五入后仍落在
// 同一档上时它看不见（实测见 `docs/rounds/P11-bz-report.md` §4 M4）；被滚动口裁剩
// 的碎片盒子（设置页 720 宽时有 2.7 / 5.2 / 6.5 u 的几条）不在阶梯上，调用方要用
// 一个下限把它们挡在外面（`MinHeight`），那是「裁剪」不是「行高」。
package frameaudit

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"
)

// MinHeight is the floor under which a box is a clipped fragment rather than a
// line of text: the smallest rung this build declares is the caption's 15u
// (a keycap's legend), so anything under 10u cannot be a whole line. Callers
// that audit a scrolling surface pass it to `Heights` as `keep`.
const MinHeight = 10

// Heights returns the distinct box heights of the frame's **leaf** runs, in
// ascending order, rounded to a tenth of a DIP (the framework lays text out at
// fractional offsets, so a 19 is reported as 18.999998 on one page and 19 on
// the next; a tenth of a unit is the resolution a ladder needs).
func Heights(tt *ui.Tester, keep func(Run) bool) []float32 {
	_, _, runs := Read(tt)
	boxes := make([]Run, 0, len(runs))
	for _, run := range runs {
		if keep != nil && !keep(run) {
			continue
		}
		boxes = append(boxes, run)
	}
	out := make([]float32, 0, len(boxes))
	seen := make(map[float32]bool, len(boxes))
	for i, a := range boxes {
		leaf := true
		for j, b := range boxes {
			if i == j {
				continue
			}
			if Encloses(a.R, b.R) {
				leaf = false
				break
			}
		}
		if !leaf {
			continue
		}
		h := round1(a.R.H)
		if seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Ladder reports every height that is not one of the rungs, within `tol`. The
// rungs are the surface's own declared line heights (the launcher's 15/18/22/
// 24/25/28, the settings pages' 15/17/18/19/27/30.6/34) — they are the theme's
// type ladder plus the control heights the rows are built from, and a rung only
// moves when the design moves it (then this list is re-pinned on purpose).
func Ladder(rungs []float32, tol float32, heights []float32) []string {
	var out []string
	for _, h := range heights {
		on := false
		for _, rung := range rungs {
			if math.Abs(float64(h-rung)) <= float64(tol) {
				on = true
				break
			}
		}
		if !on {
			out = append(out, fmt.Sprintf("height %.1f is not on the ladder %v (±%.2f)", h, rungs, tol))
		}
	}
	return out
}

// Missing reports every rung the frame did not paint, within `tol`. A surface
// that should show six rungs and shows five has lost a kind of line — text
// pushed out of the viewport, a row that collapsed, a block that stopped being
// drawn.
func Missing(rungs []float32, tol float32, heights []float32) []string {
	var out []string
	for _, rung := range rungs {
		on := false
		for _, h := range heights {
			if math.Abs(float64(h-rung)) <= float64(tol) {
				on = true
				break
			}
		}
		if !on {
			out = append(out, fmt.Sprintf("no run of height %.1f was painted (heights %v)", rung, heights))
		}
	}
	return out
}

// AuditHeights is Heights + Ladder + (optionally) Missing for one painted frame.
// `must` is the subset of rungs the caller expects this frame to show; pass nil
// to only hold the ladder.
func AuditHeights(t *testing.T, tag string, tt *ui.Tester, keep func(Run) bool, rungs, must []float32, tol float32) {
	t.Helper()
	heights := Heights(tt, keep)
	if len(heights) == 0 {
		t.Fatalf("%s: no runs to measure", tag)
	}
	for _, v := range Ladder(rungs, tol, heights) {
		t.Errorf("%s: %s", tag, v)
	}
	for _, v := range Missing(must, tol, heights) {
		t.Errorf("%s: %s", tag, v)
	}
}

func round1(v float32) float32 {
	return float32(math.Round(float64(v)*10)) / 10
}
