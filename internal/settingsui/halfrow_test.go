package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"
)

// P13-gh · 半行：卡里那些**画出来的规则线**到底占几行。
//
// 这一族最早只有一条口耳相传的经验（`docs/liquid-glass.md` §6.140 的 140-2：
// 「线被摊成两行时每行只会有**一半**强度」），着落在设置页的表格线（hairline
// grid）上。整页还没人量过：一个 720 宽的设置页，行与行之间那些**整幅宽**的
// 分隔线，是「一行整值」还是「两行各半」？
//
// 这一轮把整帧按行差分类（见 `classifyRow`）：一行整值的线是**极值**（前后两行
// 都在它同一侧），两行各半的线是一段**同向斜坡**（`y` 与 `y+1` 各走一半，中间
// 没有极值）。分辨这两件事不需要认识颜色，只需要行差的方向与幅度。
//
// 量下来的事实（720×1400、General 页、x∈[30,690)、深浅两帧**逐 y 相同**）：
//
//	卡里 9 条整幅宽的分隔线：255 / 346 / 544 / 597 / 651 / 811 / 1003 / 1098 / 1174
//	  浅 `688 → 625 → 688`（台阶 **63**）｜深 `119 → 176 → 119`（台阶 **57**）
//	  两侧**逐值相等**（688 = 688、119 = 119），`crisp` ≈ 490 列，`smeared` **= 0**
//
// 也就是说：半行这件事**没有**落到这些线上——它们是 `ui.Box().Height(1)` 画在
// 整数 y 上的一行（`rows.go` 的三处 `Background(tokens.Hairline)`）。会摊成两行的
// 是 **28u 控件自己的内线**（帧在 x.5 上，`litpair_census_test.go` 早就说明过），
// 以及页尾那条**软边**（§6.141/§6.145，本来就是多行渐变，不是线）。
//
// 三条判据（`internal/settingsui/halfrow_test.go` 前半的量尺）：
//
//	gh-1 卡里至少 8 条整幅宽分隔线（实测 9），深浅两帧落在**同一批 y** 上。
//	gh-2 每一条的两侧**逐值相等**、台阶落在 [40,100]：一行整值，不是两行各半。
//	gh-3 任何一条「整幅宽的极值行」在它自己和它下一行都不许是斜坡：整幅宽的线
//	     不许被摊成两行（违反时每行只剩一半强度，就是「糊」）。

// rowLum is one pixel row's luminance at one column, 0..765.
func rowLum(img *image.RGBA, x, y int) int {
	c := imagePixel(img, x, y)
	return litSum(c)
}

// lineShape is what one interior row looks like against its two neighbours.
type lineShape struct {
	y       int
	before  int // lum(y-1) - lum(y)
	after   int // lum(y)   - lum(y+1)
	crisp   bool
	smeared bool
}

// classifyRow says whether row y of column x is a one-row line (an extremum:
// both neighbours are on the same side of it) or the first half of a two-row
// ramp (both steps go the same way).
func classifyRow(img *image.RGBA, x, y int) (lineShape, bool) {
	if y < 1 || y+1 >= img.Rect.Dy() {
		return lineShape{}, false
	}
	up := rowLum(img, x, y-1)
	here := rowLum(img, x, y)
	down := rowLum(img, x, y+1)
	l := lineShape{y: y, before: up - here, after: here - down}
	// A one-row line: the row is an extremum, and both steps are at least the
	// contract's 4 levels (§3: 冠 > 面 by 3+, a hairline by 6+).
	if l.before*l.after < 0 {
		if absRow(l.before) >= 4 && absRow(l.after) >= 4 {
			l.crisp = true
		}
		return l, true
	}
	// A two-row ramp: both steps go the same way, each at least 2 levels, and
	// together they move at least 8 — half a hairline each is 3+3.
	if l.before*l.after > 0 && absRow(l.before) >= 2 && absRow(l.after) >= 2 &&
		absRow(l.before)+absRow(l.after) >= 8 {
		l.smeared = true
	}
	return l, true
}

func absRow(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// halfRowScan is the whole-frame census: for every interior row, how many
// columns call it a one-row line, and how many a two-row ramp.
type halfRowScan struct {
	crispCols   map[int]int
	smearedCols map[int]int
	width       int
}

// scanHalfRows classifies every interior row over the columns x0..x1.
func scanHalfRows(img *image.RGBA, x0, x1 int) halfRowScan {
	s := halfRowScan{crispCols: map[int]int{}, smearedCols: map[int]int{}, width: x1 - x0}
	for y := 1; y+1 < img.Rect.Dy(); y++ {
		for x := x0; x < x1; x++ {
			line, ok := classifyRow(img, x, y)
			if !ok {
				continue
			}
			if line.crisp {
				s.crispCols[y]++
			}
			if line.smeared {
				s.smearedCols[y]++
			}
		}
	}
	return s
}

// fullWidthLine reports whether row y reads as a line over most of the frame.
func (s halfRowScan) fullWidthLine(y int) bool { return s.crispCols[y] >= s.width*3/5 }

// ruledLines are the page's ruled separators: rows that read as a one-row line
// across the frame, leave the same face on both sides (symmetric about
// themselves), and move the contract's step. A card's top border is excluded on
// purpose: its two neighbours are the ground and the crown, not one face.
func (s halfRowScan) ruledLines(img *image.RGBA) []int {
	var out []int
	x := img.Rect.Dx() / 2
	for y := 1; y+1 < img.Rect.Dy(); y++ {
		if !s.fullWidthLine(y) || s.smearedCols[y] >= s.width/10 {
			continue
		}
		up, here, down := rowLum(img, x, y-1), rowLum(img, x, y), rowLum(img, x, y+1)
		if up != down {
			continue
		}
		if step := absRow(up - here); step < 30 || step > 120 {
			continue
		}
		out = append(out, y)
	}
	return out
}

const scanX0, scanX1 = 30, 690

// TestTheRuledLinesOfACardAreOneRow is gh-1 and gh-2.
func TestTheRuledLinesOfACardAreOneRow(t *testing.T) {
	type frame struct {
		img  *image.RGBA
		scan halfRowScan
	}
	frames := map[string]frame{}
	for _, dark := range []bool{false, true} {
		img := renderInAt(t, PageGeneral, 720, dark).Image()
		frames[appearanceName(dark)] = frame{img: img, scan: scanHalfRows(img, scanX0, scanX1)}
	}
	var lightLines []int
	for _, name := range []string{"light", "dark"} {
		img, s := frames[name].img, frames[name].scan
		lines := s.ruledLines(img)
		// gh-1: the page rules its rows with nine lines, and the appearance may
		// not move one of them: a line that slid half a row stops being
		// symmetric about itself and drops out of the set.
		if len(lines) != 9 {
			t.Fatalf("%s: found %d ruled lines (%v); the card's rows are ruled by 9", name, len(lines), lines)
		}
		x := img.Rect.Dx() / 2
		for _, y := range lines {
			up, here, down := rowLum(img, x, y-1), rowLum(img, x, y), rowLum(img, x, y+1)
			// gh-2: both neighbours are the same face, value for value, and the
			// step is the ruled line's own: 63 light / 57 dark.
			if up != down {
				t.Errorf("%s: ruled line y=%d has neighbours %d and %d; a one-row line leaves the same face on both sides",
					name, y, up, down)
			}
			if step := absRow(up - here); step < 40 || step > 100 {
				t.Errorf("%s: ruled line y=%d steps %d levels (%d → %d); the ruled line is 63 light / 57 dark",
					name, y, step, up, here)
			}
			// gh-3: the line itself and its two neighbours are not ramps — a line
			// split across two rows would show up as one.
			for _, probe := range []int{y - 1, y, y + 1} {
				if s.smearedCols[probe] >= s.width/10 {
					t.Errorf("%s: ruled line y=%d sits in a ramp on y=%d over %d columns; the line is drawn on one row",
						name, y, probe, s.smearedCols[probe])
				}
			}
		}
		if name == "light" {
			lightLines = lines
		} else if fmt.Sprint(lightLines) != fmt.Sprint(lines) {
			t.Errorf("the ruled lines sit on %v light and %v dark; the appearance may not move them", lightLines, lines)
		}
		t.Logf("%s: %d ruled lines %v", name, len(lines), lines)
	}
}

// TestDumpP13GhHalfRows is the opt-in dump: every row that reads as a line over
// most of the frame, with its two steps, plus the two frames.
func TestDumpP13GhHalfRows(t *testing.T) {
	if os.Getenv("FLOTER_DUMP_HALFROW") == "" {
		t.Skip("opt-in: set FLOTER_DUMP_HALFROW=1")
	}
	dir := "/tmp/p13gh"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		img := renderInAt(t, PageGeneral, 720, dark).Image()
		f, err := os.Create(fmt.Sprintf("%s/floter-P13-gh-general-%s.png", dir, appearanceName(dark)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		s := scanHalfRows(img, scanX0, scanX1)
		t.Logf("== %s: %dx%d, %d columns, ruled %v", appearanceName(dark), img.Rect.Dx(), img.Rect.Dy(),
			s.width, s.ruledLines(img))
		for y := 1; y+1 < img.Rect.Dy(); y++ {
			c, m := s.crispCols[y], s.smearedCols[y]
			if c < s.width/2 && m < s.width/2 {
				continue
			}
			t.Logf("y=%4d crisp=%3d smeared=%3d  lum %d -> %d -> %d", y, c, m,
				rowLum(img, 360, y-1), rowLum(img, 360, y), rowLum(img, 360, y+1))
		}
	}
}
