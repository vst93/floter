// Package frameaudit is the one ruler for「没有两段字压在同一块像素上，也没有一段字
// 跑出窗口」—— the text-over-text audit every painted surface of this build is held
// to (the launcher's card, the settings pages, the terminal's drawer band).
//
// P11-by：这条规则此前在三个测试文件里各有一份拷贝（`internal/launcher`、
// `internal/settingsui`、`internal/terminalui`），三份都用同一句
// `if r.W >= w-1 { continue }` 把**整宽文本框**整类跳过。跳过当初是为优化写的
// ——「整宽箱子是祖先，它按构造包含每一个 run，所以每一对都会被 `Encloses`
// 豁免」——但祖先关系本来就被 `Encloses` 覆盖，那句跳过顺手把**整宽叶子**
// （一条横跨面板的横幅、一段整宽正文）也一起扔掉，于是它们从来没被量过：
// P11-bx 的 M4（横幅字号 ×6）实测**仍然全绿**。本包把那一档放回审计，并把实现收成
// 一份（三处转发，`internal/ci` 有源码扫描守着）。
//
// 两处已知盲点**不在**本包能管的范围内，别拿它当 TODO 重领：
//   - `Tester.Find` 按字符串索引，所以「同一句话画两处」只报第一处的盒子；
//   - 它只比**字盒**：图标 / 面 / 描边与字的互压看不见。
package frameaudit

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Run is one text run and the box the frame registered for it.
type Run struct {
	S string
	R ui.Rect
}

// Read collects every text run `tt` painted, one box per distinct string, in the
// order the tester reports them. A run whose box is empty is dropped: it was
// never laid out, so it cannot be painted over anything.
//
// The window size comes back as 0×0 when the tester has no frame; `Audit` treats
// that as a failed arm rather than a clean one.
func Read(tt *ui.Tester) (w, h float32, runs []Run) {
	img := tt.Image()
	if img == nil {
		return 0, 0, nil
	}
	w, h = float32(img.Rect.Dx()), float32(img.Rect.Dy())
	seen := make(map[string]bool, len(tt.Texts()))
	for _, s := range tt.Texts() {
		if seen[s] {
			continue
		}
		seen[s] = true
		r, ok := tt.Find(s)
		if !ok || r.W <= 0 || r.H <= 0 {
			continue
		}
		runs = append(runs, Run{S: s, R: r})
	}
	return w, h, runs
}

// Audit holds one painted frame to the rule: no two runs on the same pixels, and
// no run outside the window it was painted into. `keep` narrows the frame to the
// band a surface owns (the drawer audits the strip and its rows, not the bar and
// its toast above it); pass nil for「整个窗口」.
//
// **整宽箱子两条断言都参加** —— this is the point of the round (see the package
// comment). A full-window ancestor is exempted by `Encloses`, which is the only
// reason it never needed a skip of its own.
func Audit(t *testing.T, tag string, tt *ui.Tester, keep func(Run) bool) {
	t.Helper()
	w, h, runs := Read(tt)
	if w <= 0 || h <= 0 {
		t.Fatalf("%s: no frame", tag)
	}
	boxes := make([]Run, 0, len(runs))
	for _, run := range runs {
		if keep != nil && !keep(run) {
			continue
		}
		boxes = append(boxes, run)
	}
	for _, v := range Violations(w, h, boxes) {
		t.Errorf("%s: %s", tag, v)
	}
}

// Violations is the rule itself, over a window and a set of runs — no tester, no
// frame, so the rule can be held to hand-made boxes
// (`frameaudit_test.go` does exactly that: a full-width run painted over another
// is a violation, which is the case the old `r.W >= w-1` skip in three test
// files used to throw away).
func Violations(w, h float32, runs []Run) []string {
	var out []string
	for _, run := range runs {
		r := run.R
		if r.X < -0.5 || r.Y < -0.5 || r.X+r.W > w+0.5 || r.Y+r.H > h+0.5 {
			out = append(out, fmt.Sprintf("%q leaves the window: %+v (window %.0fx%.0f)", run.S, r, w, h))
		}
	}
	for i := 0; i < len(runs); i++ {
		for j := i + 1; j < len(runs); j++ {
			a, b := runs[i], runs[j]
			if Encloses(a.R, b.R) || Encloses(b.R, a.R) {
				continue
			}
			if ox, oy := Overlap(a.R, b.R); ox > 1 && oy > 1 {
				out = append(out, fmt.Sprintf("%q %+v is painted over %q %+v (overlap %.1fx%.1f)",
					a.S, a.R, b.S, b.R, ox, oy))
			}
		}
	}
	return out
}

// Encloses reports whether outer contains inner (half-DIP slack, the same slack
// `Audit` uses for the window edges).
func Encloses(outer, inner ui.Rect) bool {
	return outer.X <= inner.X+0.5 && outer.Y <= inner.Y+0.5 &&
		outer.X+outer.W >= inner.X+inner.W-0.5 && outer.Y+outer.H >= inner.Y+inner.H-0.5
}

// Overlap is how far two boxes share pixels on each axis (0 or less when they
// do not overlap on that axis).
func Overlap(a, b ui.Rect) (ox, oy float32) {
	return minf(a.X+a.W, b.X+b.W) - maxf(a.X, b.X), minf(a.Y+a.H, b.Y+b.H) - maxf(a.Y, b.Y)
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
