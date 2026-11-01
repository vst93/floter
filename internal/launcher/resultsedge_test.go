package launcher

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
)

// P13-cf · 启动器结果列表的满态收口（`docs/rounds/P13-cf-spec.md`、
// `docs/liquid-glass.md` §6.49）。
//
// P13-ce 给捕获输出页补上的规则是「页头下面的呼吸属于这一页，不属于内容」。
// 上一轮留了一句「启动器结果列表读同一条规则吗？先量」。量完的答案是**读**，
// 而且比输出页早：P11-bj 就把那 19px 放在**流里**（`ui.Box(...).Height(listEdge)`，
// `launcher.go:1446`），列表自己的上 padding 是 0，于是内容永远吃不掉它。
//
// 这一族的三条账，全部钉在这里：
//
//  1. 字段墨底到裁剪上沿那一段，在任何滚动位置上零墨（呼吸不滚）；
//  2. 首行的盒与裁剪上沿重合——即「列表自己没有上 padding」，空气是页面的；
//  3. 折边在顶边与下沿读同一个公式：露出一半的行画一半的墨，两端成对。

// resultApps is the fixture: n applications, all matching the query "app".
func resultApps(n int) []apps.App {
	found := make([]apps.App, 0, n)
	for i := 0; i < n; i++ {
		found = append(found, apps.App{
			Name: fmt.Sprintf("App %02d", i),
			Path: fmt.Sprintf("/Applications/App %02d.app", i),
		})
	}
	return found
}

// resultFrame renders the launcher with a result list long enough to scroll.
func resultFrame(t *testing.T, dark bool) (*App, *ui.Tester) {
	t.Helper()
	a := testApp()
	a.SetApps(resultApps(30))
	a.Query = "app"
	tt := render(t, a)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return a, tt
}

// The field's air is the page's, not the content's. This card's own number is
// small and it is the height chain's: the field block's last pixel is its
// hairline (`launcher.go` draws it at `fieldRow-1`), and the list's clip starts
// one `breathUnits` below that. The pin reads three things: where the hairline
// is, where the clip is, and that no row's ink ever enters the band between
// them — at rest or scrolled.
func TestTheResultListNeverScrollsIntoTheFieldBand(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a, tt := resultFrame(t, dark)
		full := ui.Rect{X: 0, Y: 0, W: 720, H: float32(tt.Image().Bounds().Dy())}
		clip := a.listRect
		hairline := hairlineAbove(tt.Image(), int(clip.Y))
		if want := float64(fieldRowUnits)*4 - 1; hairline != want {
			t.Fatalf("dark=%v: the field block's hairline is at y=%v, want %v — the band under the "+
				"field is measured from that line", dark, hairline, want)
		}
		// The air between the hairline and the content belongs to the page: it is
		// the height chain's breath, outside the clip, so it cannot scroll away.
		if air := float64(clip.Y) - (hairline + 1); air != float64(breathUnits)*4 {
			t.Errorf("dark=%v: %vpx of air between the field's hairline (y=%v) and the clip (y=%v), "+
				"want the height chain's %vpx (breathUnits) — the air is not the page's own band",
				dark, air, hairline, clip.Y, float64(breathUnits)*4)
		}
		// The list's own top padding is zero: the first row's box starts on the
		// clip edge. That is *why* the air below the field cannot scroll away.
		if first := a.rowRects[0]; first.Y != clip.Y {
			t.Errorf("dark=%v: the first row starts at y=%v, the clip at y=%v — the list carries a top "+
				"padding of its own, so that air belongs to the content and scrolls away",
				dark, first.Y, clip.Y)
		}
		for _, step := range []float32{0, 10, 20} {
			if step > 0 {
				tt.Scroll(340, 200, 0, step)
				tt.Frame()
				tt.Frame()
			}
			img := tt.Image()
			band := ui.Rect{X: 0, Y: float32(hairline) + 1, W: 720, H: clip.Y - float32(hairline) - 1}
			if ink := bandPeak(img, band, full); ink != 0 {
				t.Errorf("dark=%v step=%v: the page's air under the field (y=%v..%v) carries ink %d — "+
					"a row's ink reached the field's hairline", dark, step, hairline+1, clip.Y, ink)
			}
			t.Logf("dark=%v step=%v: hairline y=%v, clip y=%v → %vpx of page air, band ink %d",
				dark, step, hairline, clip.Y, float64(clip.Y)-hairline-1, bandPeak(img, band, full))
		}
	}
}

// hairlineAbove finds the field block's last pixel: the one full-width line
// under y1 that differs from the line above it. `contrastIn` cannot see it (a
// uniform line is its own face), so this reads the card's left gutter, where
// nothing but the block's own fill and that line is ever drawn.
func hairlineAbove(img *image.RGBA, y1 int) float64 {
	lum := func(y int) int {
		c := img.RGBAAt(5, y)
		return (int(c.R) + int(c.G) + int(c.B)) / 3
	}
	// The card's fill is whatever the gutter shows for most of the band; the
	// hairline is the last line that is not it.
	count := map[int]int{}
	for y := 0; y < y1; y++ {
		count[lum(y)]++
	}
	face, best := 0, -1
	for v, n := range count {
		if n > best {
			face, best = v, n
		}
	}
	last := -1.0
	for y := 0; y < y1; y++ {
		if lum(y) != face {
			last = float64(y)
		}
	}
	return last
}

// A row the fold cuts recedes by the share it shows — and the top edge reads
// the same formula as the bottom one. The two are measured against their own
// whole-row ink, so the comparison is a ratio, not a renderer constant.
func TestTheResultRowRecedesByItsVisibleShareAtBothEnds(t *testing.T) {
	const tolerance = 0.125 // the same ±12.5% P13-ce used for the output page
	for _, dark := range []bool{false, true} {
		a, tt := resultFrame(t, dark)
		full := ui.Rect{X: 0, Y: 0, W: 720, H: float32(tt.Image().Bounds().Dy())}
		clip := a.listRect

		// top end: at rest row 0 is whole (60..100); scroll 20 puts it at
		// 40..80, so half of it shows through the clip edge at y=60.
		wholeTop := contrastIn(tt.Image(), a.rowRects[0], full)
		tt.Scroll(340, 200, 0, 20)
		tt.Frame()
		tt.Frame()
		cutTop := contrastIn(tt.Image(), a.rowRects[0], clip)
		shareTop := visibleShare(a.rowRects[0], clip)
		if want := float64(wholeTop) * shareTop; !within(cutTop, want, tolerance*float64(wholeTop)) {
			t.Errorf("dark=%v: the row at the top fold shows %.2f of its box but draws ink %d of its whole "+
				"%d (want %.0f) — it is cut, not receded", dark, shareTop, cutTop, wholeTop, want)
		} else {
			t.Logf("dark=%v top: share %.2f, ink %d / %d (want %.0f)", dark, shareTop, cutTop, wholeTop, want)
		}

		// bottom end: at rest the viewport's own bottom cuts row 9 (420..460
		// against a clip that ends at 443). The whole-row ink is the same row
		// after the scroll above put it fully inside.
		wholeBottom := contrastIn(tt.Image(), a.rowRects[9], full)
		a2, tt2 := resultFrame(t, dark)
		shareBottom := visibleShare(a2.rowRects[9], a2.listRect)
		cutBottom := contrastIn(tt2.Image(), a2.rowRects[9], a2.listRect)
		if want := float64(wholeBottom) * shareBottom; !within(cutBottom, want, tolerance*float64(wholeBottom)) {
			t.Errorf("dark=%v: the row at the bottom fold shows %.2f of its box but draws ink %d of its "+
				"whole %d (want %.0f) — the two ends do not read the same formula",
				dark, shareBottom, cutBottom, wholeBottom, want)
		} else {
			t.Logf("dark=%v bottom: share %.2f, ink %d / %d (want %.0f)", dark, shareBottom, cutBottom, wholeBottom, want)
		}
	}
}

// bandPeak is the strongest ink on any single line of the band. It reads one
// line at a time on purpose: `contrastIn` samples the face at the middle of the
// box it is given, and the panel's face drifts a few levels down its own
// height — a 19px box would then measure that drift as "ink".
func bandPeak(img *image.RGBA, band, full ui.Rect) int {
	peak := 0
	for y := int(band.Y); y < int(band.Y+band.H); y++ {
		if ink := contrastIn(img, ui.Rect{X: band.X, Y: float32(y), W: band.W, H: 1}, full); ink > peak {
			peak = ink
		}
	}
	return peak
}

// within is |got-want| <= slack, with slack in the same units as the ink.
func within(got int, want, slack float64) bool {
	d := float64(got) - want
	if d < 0 {
		d = -d
	}
	return d <= slack
}

// P13-cf visual evidence: the rest frame and one scrolled frame, both
// appearances, written to /tmp for the report. Opt-in (`FLOTER_PNG=1`) for the
// same reason as every other dump: a test run should not litter /tmp.
func TestDumpP13CFResultListPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		_, tt := resultFrame(t, dark)
		writePNG(t, filepath.Join("/tmp", "floter-P13-cf-results-"+appearance+"-rest.png"), tt)
		tt.Scroll(340, 200, 0, 20)
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P13-cf-results-"+appearance+"-scrolled.png"), tt)
	}
}
