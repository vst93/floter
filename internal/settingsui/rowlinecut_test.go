package settingsui

import (
	"image"
	"sort"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-bm · 设置行**第二行**被切的时候，切在哪里、有没有记号——**先量**。
//
// 起点是一条看起来像缺陷的代码：`rows.go` 的 `rowTitle` 画那行灰字时写的是
// `ui.Text(...).SingleLine()`，注释却说「single-line so a long value
// ellipsizes in the text column」，而同一段里名字那一行（`rowName.Nowrap`）明
// 写着 `.SingleLine().Ellipsis("…")`。第二行漏了 `Ellipsis`，看起来就是「一条
// 路径被切在半个字形上、两端不留记号」。
//
// 量下来**不是**：框架的 `SingleLine()` 自己就带省略号。
// `mygo/ui/element.go:1322` 只把 `maxLines` 钉成 1，但 `mygo/internal/text/
// layout.go:81` 的 `Params.Ellipsis` 文档是「Ellipsis ("…" when empty)」，
// `:1194-1206` 在 `MaxLines` 用尽时无条件补上那个记号——`Ellipsis(s)` 只是换一个
// **记号的样子**，不是「有没有记号」。所以 `rows.go:1136` 那个
// `Ellipsis("\u2026")` 是把默认值又说了一遍，而第二行什么都不写也照样有记号。
//
// 实测（`sessionsPageWithDir`，`M` 的重复串当第二行，深色）：
//
//	宽 860u：盒子 533u，墨 448 列，整行放得下 → 尾端是真正的字形（墨顶 191）
//	宽 560u：盒子 233u，被切 → 尾端是三个低处的点（墨 197..198，只有 2 行高）
//	宽 460u：盒子 133u，被切 → 同上
//
// 于是这条 pin 钉的不是「补一个 Ellipsis」（那会是零像素的假修复），而是
// **被切的那一行必须留下记号**：以后谁把它换成 `Clip()`、或者给它一个空记号，
// 这里会红。
//
// 夹具用 `M` 的重复串而不是路径：`M` 是每个等宽字体都有、且**通高**的字形，
// 所以「切在半个 M 上」和「以低处的省略号收尾」是 100% 与 ~20% 的墨高差别；
// 换成一条真路径，最后一个字形可能正好是个逗号，这条尺子就什么也量不出来。

// sessionsPageWithDir renders the sessions page at width with one session whose
// name is title and whose working directory is dir.
func sessionsPageWithDir(t *testing.T, title, dir string, width int, dark bool) *ui.Tester {
	t.Helper()
	store := settings.NewStore(settings.Default())
	a := New(store, Actions{SelectSession: func(int) {}, CloseSessionAt: func(int) {}})
	created := time.Date(2024, 1, 2, 15, 4, 0, 0, time.Local)
	a.Sessions = func() []Session {
		return []Session{{
			ID: 1, Title: title, Running: true, Index: 0,
			Note: "Running · attached", Size: "80×24", Created: created,
			Dir: dir,
		}}
	}
	a.selectPage(PageSessions)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, width, 600)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return tt
}

// inkColumn is one painted column of a text line: its topmost and bottommost
// ink row, in image coordinates.
type inkColumn struct {
	X      int
	Top    int
	Bottom int
}

// lineInk returns the painted columns inside box. A pixel counts as ink when it
// is more than 8 levels away from the line's own background, which is taken as
// the median level of the box: a text box is mostly face, so the median *is* the
// face, and no glyph is wide enough to move it.
func lineInk(img *image.RGBA, box ui.Rect) []inkColumn {
	x0, y0 := int(box.X), int(box.Y)
	x1, y1 := int(box.X+box.W), int(box.Y+box.H)
	if x1 > img.Bounds().Max.X {
		x1 = img.Bounds().Max.X
	}
	if y1 > img.Bounds().Max.Y {
		y1 = img.Bounds().Max.Y
	}
	var levels []int
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			levels = append(levels, lumaOf(img, x, y))
		}
	}
	if len(levels) == 0 {
		return nil
	}
	sort.Ints(levels)
	face := levels[len(levels)/2]
	var cols []inkColumn
	for x := x0; x < x1; x++ {
		top, bottom := -1, -1
		for y := y0; y < y1; y++ {
			if d := lumaOf(img, x, y) - face; d > 8 || d < -8 {
				if top < 0 {
					top = y
				}
				bottom = y
			}
		}
		if top >= 0 {
			cols = append(cols, inkColumn{X: x, Top: top, Bottom: bottom})
		}
	}
	return cols
}

// lumaOf is the pixel's own level.
func lumaOf(img *image.RGBA, x, y int) int {
	r, g, b, _ := img.At(x, y).RGBA()
	return (int(r>>8) + int(g>>8) + int(b>>8)) / 3
}

// TestTheRowSecondLineMarksItsCut: a second line that runs past its column ends
// in the framework's own ellipsis — a low, dotted mark — not in the top half of
// a chopped glyph.
func TestTheRowSecondLineMarksItsCut(t *testing.T) {
	const blocks = "MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM"
	// The line's natural width: the same run drawn in a window wide enough to
	// hold it, so "is this one cut" is a measurement rather than a guess.
	natural := lineSpan(t, sessionsPageWithDir(t, "zsh", blocks, 2000, true), blocks)
	for _, dark := range []bool{true, false} {
		for _, width := range []int{860, 560, 460} {
			width := width
			tt := sessionsPageWithDir(t, "zsh", blocks, width, dark)
			box, ok := tt.Find(blocks)
			if !ok {
				t.Fatalf("dark=%v %du: the second line is not drawn at all: %q", dark, width, tt.Texts())
			}
			cols := lineInk(tt.Image(), box)
			if len(cols) < 8 {
				t.Fatalf("dark=%v %du: only %d painted columns in the second line (%+v)",
					dark, width, len(cols), box)
			}
			top, bottom := cols[0].Top, cols[0].Bottom
			for _, c := range cols {
				if c.Top < top {
					top = c.Top
				}
				if c.Bottom > bottom {
					bottom = c.Bottom
				}
			}
			height := bottom - top + 1
			right := float32(cols[len(cols)-1].X)
			cut := float32(right-box.X) < natural-1
			marked := cols[len(cols)-1].Top >= top+height/2
			t.Logf("dark=%v %du: box %.0f..%.0f (w=%.0f), %d painted columns, ink rows %d..%d (h=%d), last column x=%.0f (natural %.0f), cut=%v marked=%v",
				dark, width, box.X, box.X+box.W, box.W, len(cols), top, bottom, height, right, natural, cut, marked)
			switch {
			case cut && !marked:
				t.Errorf("dark=%v %du: the second line is cut (%.0f of its %.0f natural width) with no ellipsis — its last ink reaches y=%d of a line that runs %d..%d, so the last thing painted is the top half of a glyph",
					dark, width, right-box.X+1, natural, cols[len(cols)-1].Top, top, bottom)
			case !cut && marked:
				t.Errorf("dark=%v %du: the line fits (%.0f of its %.0f natural width) but ends in a low mark — a glyph with room left was replaced by an ellipsis",
					dark, width, right-box.X+1, natural)
			}
		}
	}
}

// lineSpan is the width the line's ink covers when it is given room to run.
func lineSpan(t *testing.T, tt *ui.Tester, text string) float32 {
	t.Helper()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the line is not drawn: %q", tt.Texts())
	}
	cols := lineInk(tt.Image(), box)
	if len(cols) < 8 {
		t.Fatalf("only %d painted columns (%+v)", len(cols), box)
	}
	return float32(cols[len(cols)-1].X) - float32(cols[0].X) + 1
}

// TestALongDirectoryLineStaysOutOfTheRowControls: the cut is a paint property,
// so the whole string is still on the element, and the painted line still stops
// at its own column rather than running under the row's trailing Play/Trash
// pair.
func TestALongDirectoryLineStaysOutOfTheRowControls(t *testing.T) {
	const dir = "/home/vst/workspace/floter/internal/settingsui/some/deeper/directory/than/the/column"
	for _, width := range []int{860, 560, 460} {
		width := width
		tt := sessionsPageWithDir(t, "zsh", dir, width, true)
		if !tt.HasText(dir) {
			t.Fatalf("%du: the directory was cut in the data, not in the paint: %q", width, tt.Texts())
		}
		box, ok := tt.Find(dir)
		if !ok {
			t.Fatalf("%du: the directory line is not drawn: %q", width, tt.Texts())
		}
		cols := lineInk(tt.Image(), box)
		if len(cols) == 0 {
			t.Fatalf("%du: the directory line painted nothing (%+v)", width, box)
		}
		right := float32(cols[len(cols)-1].X)
		if right > box.X+box.W {
			t.Errorf("%du: the directory's ink runs past its own column (x=%.0f, column ends at %.0f)",
				width, right, box.X+box.W)
		}
		play, ok := tt.Find("Resume session")
		if !ok {
			t.Fatalf("%du: the row's trailing control is not drawn: %q", width, tt.Texts())
		}
		if right >= play.X {
			t.Errorf("%du: the directory's ink runs to x=%.0f, into the trailing control at x=%.0f",
				width, right, play.X)
		}
		t.Logf("%du: dir ink ends at x=%.0f, its column ends at %.0f, the trailing control starts at %.0f",
			width, right, box.X+box.W, play.X)
	}
}
