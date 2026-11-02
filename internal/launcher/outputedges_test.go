package launcher

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-ce · 捕获输出页的满态收口（`docs/rounds/P13-ce-spec.md`、`docs/liquid-glass.md` §6.48）。
//
// 上一轮（P13-cd）留下的问题：这一页的满态收口没量过。量完的结论有两条：
//
//  1. **「顶边是硬切」这个前提不成立**：列表分支两端都走 `foldAlpha`，实测滚一行后
//     顶边那一行的墨是 **92**（整行 228 的 40%），下沿那一行是 **160**（70%）——两端
//     按同一个比例规则退后，成对。
//  2. **真正不成对的是「条子下面那 4u 呼吸」**：它此前是列表自己的 padding，属于
//     **内容**，于是滚一行就被内容吃掉——实测顶边那行被切成两半的字尾落在 y=102，
//     而条子的最后一行墨在 y=100，中间只隔 **2px**。正文分支更糟：它连折边都没有
//     （正文不是行，`foldAlpha` 没有可量的盒子），滚起来第一行字是**满墨**贴在条子上。
//
// 落地的规则一条：**页头下面的呼吸属于这一页，不属于内容**——16px 留在裁剪之外。
// 静止帧逐像素不变（首行仍在 118、视口下沿仍在 466，少掉的 16px 高度与多出来的
// 16px 空气是同一笔，四张图 md5 逐字节相同）。

// lastInkAbove is the last row with ink in [0, y1): the bottom of the status
// strip, which is in flow and therefore does not move when the body scrolls.
func lastInkAbove(img *image.RGBA, y1 int) int {
	full := ui.Rect{X: 0, Y: 0, W: float32(img.Bounds().Dx()), H: float32(img.Bounds().Dy())}
	last := -1
	for y := 0; y < y1; y++ {
		if contrastIn(img, ui.Rect{X: 0, Y: float32(y), W: full.W, H: 1}, full) > 8 {
			last = y
		}
	}
	return last
}

// bandInk is the strongest departure from the panel face inside box, on
// either theme (dark ink is lighter than the face, light ink darker).
func bandInk(img *image.RGBA, box ui.Rect) int {
	return contrastIn(img, box, ui.Rect{X: 0, Y: 0, W: float32(img.Bounds().Dx()), H: float32(img.Bounds().Dy())})
}

// outputAirBand is the band the page reserves under its status strip: `Space(4)`
// of the page's own air, never the content's.
//
// P13-dv: both edges come from the page's own boxes — the top of the clip the
// rows scroll in, minus `Space(4)` — instead of the stock ruler's literal 118.
// The band therefore reads the same rule at the shipped step (§6.86's
// premise-break row: 呼吸带按页面自己的 `Space(4)` 从裁剪上沿推出来).
func outputAirBand(clipTop int) ui.Rect {
	spacing, _, _, _ := designMetrics()
	h := int(math.Round(4 * spacing))
	return ui.Rect{X: 0, Y: float32(clipTop - h), W: 720, H: float32(h)}
}

// The strip's breathing band is the page's, not the content's: scrolling the
// list may not push a row's ink into it. Before P13-ce the band was the
// list's own padding, so one scroll put a folded row's tail at y=102 — two
// pixels under the strip's last ink.
func TestTheOutputListNeverScrollsIntoTheHeaderBand(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := testApp()
		tt := listOutputShipped(t, a, outputRowsWithActions(25), dark)
		tt.Frame()
		// P13-dv: the band's edges are the page's, not the stock ruler's: the
		// clip top the rows scroll in, and the strip's last ink above it.
		clipTop := int(math.Round(float64(a.outputListRect.Y)))
		strip := lastInkAbove(tt.Image(), clipTop)
		if strip < 0 {
			t.Fatalf("dark=%v: the status strip drew no ink above the clip (clip top y=%d)", dark, clipTop)
		}
		band := outputAirBand(clipTop)
		if band.H <= 0 {
			t.Fatalf("dark=%v: the page reserves no air under its strip: strip %d, clip top %d", dark, strip, clipTop)
		}
		if strip >= int(band.Y) {
			t.Fatalf("dark=%v: the band overlaps the strip's own ink: strip %d, band y=%d..%d",
				dark, strip, int(band.Y), int(band.Y+band.H)-1)
		}
		a.OutputList.ScrollTo(4, ui.Start)
		tt.Frame()
		tt.Frame()
		img := tt.Image()
		if ink := bandInk(img, band); ink != 0 {
			t.Errorf("dark=%v: the band under the strip (y=%d..%d) carries ink %d — the "+
				"page's own air scrolled away with the content (strip's last ink y=%d)",
				dark, int(band.Y), int(band.Y+band.H)-1, ink, strip)
		}
		t.Logf("dark=%v: strip's last ink y=%d, clip top y=%d, band y=%d..%d ink %d",
			dark, strip, clipTop, int(band.Y), int(band.Y+band.H)-1, bandInk(img, band))
		if _, _, ok := clippedRow(a); !ok {
			t.Fatalf("dark=%v: the premise failed — no row is cut after scrolling: %v", dark, a.outputRowRects)
		}
	}
}

// The same rule on the text branch: a body of text is not rows, so it has no
// fold at all — but the band under the strip is still the page's.
func TestTheOutputTextNeverScrollsIntoTheHeaderBand(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 60; i++ {
		b.WriteString("line ")
		b.WriteString(strings.Repeat("x", 20))
		b.WriteString("\n")
	}
	for _, dark := range []bool{false, true} {
		a := testApp()
		tt := listOutputShipped(t, a, b.String(), dark)
		tt.Frame()
		clipTop := int(math.Round(float64(a.outputListRect.Y)))
		strip := lastInkAbove(tt.Image(), clipTop)
		if strip < 0 {
			t.Fatalf("dark=%v: the status strip drew no ink above the clip (clip top y=%d)", dark, clipTop)
		}
		band := outputAirBand(clipTop)
		if band.H <= 0 {
			t.Fatalf("dark=%v: the page reserves no air under its strip: strip %d, clip top %d", dark, strip, clipTop)
		}
		a.Output.Y = 300
		tt.Frame()
		tt.Frame()
		if ink := bandInk(tt.Image(), band); ink != 0 {
			t.Errorf("dark=%v: the text scrolled into the strip's breathing band (y=%d..%d) "+
				"with ink %d; the strip's last ink is y=%d", dark, int(band.Y), int(band.Y+band.H)-1, ink, strip)
		}
	}
}

// And the structural statement behind it: the viewport the rows are clipped
// to starts on the first row's own top edge — the 4u sits outside the clip,
// which is why the resting frame is unchanged while the scrolled one is not.
func TestTheOutputViewportStartsOnItsFirstRow(t *testing.T) {
	a := testApp()
	tt := listOutputShipped(t, a, outputRowsWithActions(25), false)
	tt.Frame()
	tt.Frame()
	first, ok := a.outputRowRects[0]
	if !ok {
		t.Fatalf("the first row has no box: %v", a.outputRowRects)
	}
	if a.outputListRect.Y != first.Y {
		t.Errorf("the list's clip starts at y=%v but its first row at y=%v — the header's "+
			"breathing band is inside the clip, so it scrolls away (P13-ce)",
			a.outputListRect.Y, first.Y)
	}
	// The bottom edge is where the page's own geometry puts it at the shipped
	// step: P13-dv 重定基线 466（库存尺）→ 419.2（出货尺，帧高 488 → 439）。
	if got, want := tenths(float64(a.outputListRect.Y+a.outputListRect.H)), tenths(419.2); got != want {
		t.Errorf("the viewport's bottom edge moved to y=%v, want %v",
			float64(got)/10, float64(want)/10)
	}
}

// The fold is paired at both ends: a row recedes by the share of itself the
// viewport shows, whether it is leaving through the top or through the
// bottom. This is the measurement that retired the pending question — the
// top edge was never a hard cut.
func TestTheOutputFoldRecedesTheSameAtBothEnds(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, outputRowsWithActions(25))
	tt.Frame()
	tt.Frame()
	// A half-row offset: `Start` would land the clip on a row boundary, where
	// neither end cuts a row and there is nothing to compare.
	a.OutputList.ScrollTo(4, ui.Center)
	tt.Frame()
	tt.Frame()
	img := tt.Image()
	full := 0
	for i := 0; i < len(a.outputRowRects); i++ {
		box, ok := a.outputRowRects[i]
		if !ok || visibleShare(box, a.outputListRect) != 1 {
			continue
		}
		full = contrastIn(img, box, a.outputListRect)
		break
	}
	if full < 150 {
		t.Fatalf("no whole row on screen to read the resting ink from: %d", full)
	}
	var top, bottom ui.Rect
	var topShare, bottomShare float64
	for i := 0; i < len(a.outputRowRects); i++ {
		box, ok := a.outputRowRects[i]
		if !ok || box.H <= 0 {
			continue
		}
		share := visibleShare(box, a.outputListRect)
		if share <= 0 || share >= 1 {
			continue
		}
		if box.Y < a.outputListRect.Y {
			top, topShare = box, share
		} else {
			bottom, bottomShare = box, share
		}
	}
	if top.H == 0 || bottom.H == 0 {
		t.Fatalf("the premise failed: no row is cut at one of the ends (%v)", a.outputRowRects)
	}
	for _, c := range []struct {
		name  string
		box   ui.Rect
		share float64
	}{{"top", top, topShare}, {"bottom", bottom, bottomShare}} {
		ink := contrastIn(img, c.box, a.outputListRect)
		want := int(c.share * float64(full))
		t.Logf("%s: share %.3f → ink %d (whole row %d, want %d)", c.name, c.share, ink, full, want)
		if diff := ink - want; diff > full/8 || diff < -full/8 {
			t.Errorf("the row leaving through the %s is %.3f visible but carries ink %d "+
				"(a whole row is %d, %.0f%% of it is %d) — one end folds, the other cuts",
				c.name, c.share, ink, full, c.share*100, want)
		}
	}
}

// Opt-in: the round's two pictures, for the report.
func TestDumpP13CEOutputEdgesPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	var b strings.Builder
	for i := 0; i < 60; i++ {
		b.WriteString("line ")
		b.WriteString(strings.Repeat("x", 20))
		b.WriteString("\n")
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		// The list branch, scrolled one row: the top edge is a fold and the
		// strip keeps its band.
		a := testApp()
		tt := runListOutput(t, a, outputRowsWithActions(25))
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		a.OutputList.ScrollTo(4, ui.Start)
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P13-ce-list-"+name+"-"+tag+".png"), tt)
		// The text branch, scrolled: no fold, but the same band.
		a2 := testApp()
		tt2 := runListOutput(t, a2, b.String())
		tt2.SetDark(dark)
		tt2.Frame()
		tt2.Frame()
		a2.Output.Y = 300
		tt2.Frame()
		tt2.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P13-ce-text-"+name+"-"+tag+".png"), tt2)
	}
}

// Opt-in: P13-dv 的四张图——出货尺下捕获输出页的两条分支、深浅两套。
// 这一轮重新推导的正是条子下面那条呼吸带；图是眼睛的复核：它还是一条空气。
func TestDumpP13DVOutputBandPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	var text strings.Builder
	for i := 0; i < 60; i++ {
		text.WriteString("line ")
		text.WriteString(strings.Repeat("x", 20))
		text.WriteString("\n")
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		a := testApp()
		tt := listOutputShipped(t, a, outputRowsWithActions(25), dark)
		tt.Frame()
		a.OutputList.ScrollTo(4, ui.Start)
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P13-dv-list-"+name+".png"), tt)

		b := testApp()
		tb := listOutputShipped(t, b, text.String(), dark)
		tb.Frame()
		b.Output.Y = 300
		tb.Frame()
		tb.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P13-dv-text-"+name+".png"), tb)
	}
}
