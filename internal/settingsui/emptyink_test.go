package settingsui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The empty region's vertical rhythm, measured on the ink.
//
// `docs/liquid-glass.md` §4 fixes the *scale* (u = 4n, gap 8u) and the region
// asked that scale for **one** number — `Gap(8u)` — between every pair of its
// parts. A gap is not owned by the region, it is owned by the pair it sits
// between: a mark and the sentence it introduces are one block, a sentence and
// its why are two. Nominal 8u between boxes of different optical weight does
// not read as 8u — a 22u glyph's ink does not fill its box — so what a viewer
// resolves is the *ink* gap, and the region's own measurement found the mark
// 12px above its sentence against 13px between the sentence and the why: three
// peers at one distance.
//
// P13-aj 重定基线：那一对收到 4u，墨上 12 → **8px**（句子→理由仍是 12px）。
// These pins hold the *resolved* distance, not the requested one, because the
// contract's spacing rule is a claim about what the eye reads.

// inkBands finds the vertical bands of ink in one column window of the last
// frame: contiguous rows holding at least three pixels that differ from the
// region's own plane by more than 12 luma levels. The plane is the median luma
// of the window, so a region painted on a card face and one painted on a
// sunken plane measure the same way. Bands two rows apart or closer are one
// band: a descender belongs to the line it hangs from.
func inkBands(tt *ui.Tester, x0, x1, y0, y1 int) [][2]int {
	img := tt.Image()
	lumas := make([]int, 0, (x1-x0)*(y1-y0))
	plane := make([][]int, 0, y1-y0)
	for y := y0; y < y1; y++ {
		row := make([]int, 0, x1-x0)
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			l := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			row = append(row, l)
			lumas = append(lumas, l)
		}
		plane = append(plane, row)
	}
	sort.Ints(lumas)
	base := lumas[len(lumas)/2]
	var bands [][2]int
	open := -1
	for i, row := range plane {
		ink := 0
		for _, l := range row {
			if d := l - base; d > 12 || d < -12 {
				ink++
			}
		}
		switch {
		case ink >= 3 && open < 0:
			open = y0 + i
		case ink < 3 && open >= 0:
			bands = append(bands, [2]int{open, y0 + i - 1})
			open = -1
		}
	}
	if open >= 0 {
		bands = append(bands, [2]int{open, y1 - 1})
	}
	merged := bands[:0]
	for _, band := range bands {
		if n := len(merged); n > 0 && band[0]-merged[n-1][1] <= 3 {
			merged[n-1][1] = band[1]
			continue
		}
		merged = append(merged, band)
	}
	return merged
}

// bandInk is one band's strongest ink as a viewer resolves it: how far its peak
// pixel sits from the plane in the row above the band, in luma levels — the
// number the contract means when it says a rung is "N 级" away from the plane.
func bandInk(tt *ui.Tester, band [2]int, x0, x1 int) int {
	img := tt.Image()
	plane := 0
	if y := band[0] - 3; y >= 0 {
		i := y*img.Stride + x0*4
		plane = luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
	}
	peak := 0
	for y := band[0]; y <= band[1]; y++ {
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			d := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2]) - plane
			if d < 0 {
				d = -d
			}
			if d > peak {
				peak = d
			}
		}
	}
	return peak
}

func luma(r, g, b uint8) int {
	return (299*int(r) + 587*int(g) + 114*int(b)) / 1000
}

// emptyRegion renders the sessions page's empty state inside the content
// column and returns its tester plus the ink bands a viewer resolves in it.
func emptyRegion(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	const width, height = 720, 620
	a := New(settings.NewStore(settings.Default()), Actions{})
	a.selectPage(PageSessions)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, width, height)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return tt
}

// emptyContent is the window the region is measured in: past the 148px sidebar
// and the page's own inset, above the footer.
const emptyLeft, emptyRight, emptyTop, emptyBottom = 148 + 40, 720 - 40, 120, 580

// TestTheEmptyRegionHangsItsMarkOffItsSentence pins the empty region's ink
// rhythm: the mark sits **closer** to the sentence it introduces than the
// sentence sits to the why under it, by at least one visible step (2px), and
// the three parts keep their ink ladder (sentence ≫ mark ≫ why).
func TestTheEmptyRegionHangsItsMarkOffItsSentence(t *testing.T) {
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := emptyRegion(t, dark)
		bands := inkBands(tt, emptyLeft, emptyRight, emptyTop, emptyBottom)
		if len(bands) < 3 {
			t.Fatalf("%s: the empty region resolved %d ink bands, want mark + sentence + why: %v",
				appearance, len(bands), bands)
		}
		mark, sentence, why := bands[0], bands[1], bands[2]
		markToSentence := sentence[0] - mark[1] - 1
		sentenceToWhy := why[0] - sentence[1] - 1
		if sentenceToWhy-markToSentence < 2 {
			t.Errorf("%s: the mark hangs %dpx off its sentence and the why %dpx further down — "+
				"the mark reads as a third peer instead of belonging to the sentence (§4.3)",
				appearance, markToSentence, sentenceToWhy)
		}
		if markToSentence < 6 {
			t.Errorf("%s: mark→sentence gap = %dpx, want at least 6px of air under the 22u mark",
				appearance, markToSentence)
		}
		title, markInk, whyInk := bandInk(tt, sentence, emptyLeft, emptyRight),
			bandInk(tt, mark, emptyLeft, emptyRight), bandInk(tt, why, emptyLeft, emptyRight)
		if title-markInk < 20 {
			t.Errorf("%s: sentence ink %d vs mark ink %d — the mark must stay one rung below "+
				"the statement it introduces (§4.7)", appearance, title, markInk)
		}
		if markInk-whyInk < 20 {
			t.Errorf("%s: mark ink %d vs why ink %d — the why must stay the quietest of the "+
				"three (§4.7)", appearance, markInk, whyInk)
		}
	}
}

// TestDumpEmptyRegionInk writes the frames this round measured, so the report
// can point at a file instead of describing one. Opt-in, like every dump here.
func TestDumpEmptyRegionInk(t *testing.T) {
	if os.Getenv("FLOTER_P13AJ_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13AJ_DUMP=1 to write /tmp/floter-P13-aj-*.png")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt := emptyRegion(t, dark)
		bands := inkBands(tt, emptyLeft, emptyRight, emptyTop, emptyBottom)
		report := make([]string, len(bands))
		for i, band := range bands {
			report[i] = fmt.Sprintf("%v ink %d", band, bandInk(tt, band, emptyLeft, emptyRight))
		}
		t.Logf("%s: %v", appearance, report)
		path := filepath.Join("/tmp", "floter-P13-aj-sessions-empty-"+appearance+".png")
		writePNG(t, path, tt)
	}
}
