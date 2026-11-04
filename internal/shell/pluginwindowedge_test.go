package shell

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/launcher"
)

// P13-fz（`docs/liquid-glass.md` §6.145）· 插件窗口的下沿。
//
// 这个应用里每一块**自由滚动**的面都有下沿：设置页内容区（§6.16 的 P13-as，
// `glass.ScrollEdge` 上下各一道）、启动器满态（§6.1 表 1-10 的折边）、会话抽屉
// （§6.141）、捕获输出页的浮条（§6.20）。剩下两个没有：`detachedView` 的正文和
// `pinnedView` 的便条——它们是同一个 `.plugin-window` 表面（两个窗口的根节点都刷
// `tokens.Opaque`），也是同一族里最后两个刀口：滚到中间时正文最后一行在窗口
// **最后一行像素**上被水平切断，墨一点没退。
//
// 这一轮的判据是两条，和 P13-as 那两条同形：
//
//	① 满态下最后一行必须是**退后**的，不是满对比（边在干活）；
//	② 边是自门控的：滚到底时下面只剩 14 的正文内边距，没有内容可淡，
//	   10.0 的边整个落在空白上，不许在空白上留下一道灰带（§6.19 的门）。
//
// 边高 `Space(2.5)` = 10.0 < 内边距 `Space(3.5)` = 14.0，是 ① ② 同时成立的**唯一**
// 理由：边再高一点，滚到底时它就会盖住最后一行字（`MaxY` 时最后一行离下沿 14）。

// pluginEdgeLines is a body far longer than the 460px window: the point of the
// pins is the state a reader actually reaches, not the first frame.
const pluginEdgeLines = 90

func pluginEdgeText() string {
	var b strings.Builder
	for i := 0; i < pluginEdgeLines; i++ {
		fmt.Fprintf(&b, "line %02d  the quick brown fox jumps over the lazy dog\n", i)
	}
	return b.String()
}

// pluginEdgeDetached renders the captured-output window with a body that
// overflows it.
func pluginEdgeDetached(t *testing.T, dark bool) (*ui.Tester, *detachedOutput, int, int) {
	t.Helper()
	a := barApp(t, dark)
	detached := &detachedOutput{title: "jv --floter", ran: true}
	detached.view = launcher.OutputView{Title: "jv --floter", Text: pluginEdgeText()}
	band, inset := 0, 0
	tt := ui.NewTester(func(c *ui.Context) {
		band = int(c.Theme().Space(detachedEdgeInset))
		inset = int(c.Theme().Space(detachedBodyInset))
		a.detachedView(c, detached)
	}, 720, 460)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	if detached.scroll.MaxY <= 0 {
		t.Fatalf("the body does not overflow this window (MaxY = %v): there is no full state to "+
			"measure and both pins below would pass on a window that never scrolls", detached.scroll.MaxY)
	}
	return tt, detached, band, inset
}

// pluginEdgePinned renders the pinned-note window with a body that overflows
// it. Its scroller keeps no exported state, so the pin scrolls it the way a
// hand does — with the wheel.
func pluginEdgePinned(t *testing.T, dark bool) (*ui.Tester, int) {
	t.Helper()
	a := barApp(t, dark)
	state := &pinned{title: "note", text: pluginEdgeText()}
	band := 0
	tt := ui.NewTester(func(c *ui.Context) {
		band = int(c.Theme().Space(detachedEdgeInset))
		a.pinnedView(c, state)
	}, 720, 460)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return tt, band
}

// pluginEdgeLuma is a colour's brightness on a 0..255 scale (the package's own
// `luma` keeps the 299/587/114 weights un-divided, which is fine for "which is
// darker" but not for comparing a glyph row against a pane).
func pluginEdgeLuma(c ui.Color) int {
	return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000
}

// pluginEdgeSpread is the ink of one pixel row: max luma − min luma. A row
// holding a glyph reads in the hundreds; a bare pane reads 0-2.
func pluginEdgeSpread(img *image.RGBA, y, x0, x1 int) int {
	lo, hi := 999, -1
	for x := x0; x < x1; x++ {
		v := pluginEdgeLuma(shellPixelAt(img, x, y))
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if hi < 0 {
		return 0
	}
	return hi - lo
}

// pluginEdgeMean is the average luma of a block: what a veil of the wrong
// colour moves, where pluginEdgeSpread cannot see it (a lateral-uniform band
// has no spread).
func pluginEdgeMean(img *image.RGBA, y0, y1, x0, x1 int) int {
	sum, n := 0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			sum += pluginEdgeLuma(shellPixelAt(img, x, y))
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

// TestTheOutputWindowFadesAtItsBottomEdge is ① and ② on the captured-output
// window: at every sampled scroll position the window's last row reads well
// below the rows just above the edge, and at the end of the scroll the band is
// the window's own ground.
func TestTheOutputWindowFadesAtItsBottomEdge(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tt, detached, band, inset := pluginEdgeDetached(t, dark)
			img := tt.Image()
			H, W := img.Bounds().Dy(), img.Bounds().Dx()
			x0, x1 := 16, W-16
			if band < 4 {
				t.Fatalf("the scroll edge resolves to %d px, too thin to measure", band)
			}

			worstLast, worstAbove, deepest := 0, 0, 0
			for k := 1; k <= 19; k++ {
				detached.scroll.Y = detached.scroll.MaxY * float32(k) / 20
				tt.Frame()
				tt.Frame()
				img = tt.Image()
				if s := pluginEdgeSpread(img, H-1, x0, x1); s > worstLast {
					worstLast = s
				}
				for y := H - band - 4; y < H-band; y++ {
					if s := pluginEdgeSpread(img, y, x0, x1); s > worstAbove {
						worstAbove = s
					}
				}
				// The fade's depth, in device rows: how far up from the window's
				// bottom the ink still reads as faded. It is the drawn band, not
				// the constant that asked for it.
				depth := 0
				for y := H - 1; y >= 0; y-- {
					if pluginEdgeSpread(img, y, x0, x1)*100 >= worstAbove*45 {
						break
					}
					depth++
				}
				if depth > deepest {
					deepest = depth
				}
			}
			t.Logf("deepest fade %d rows, band %d, body inset %d", deepest, band, inset)
			if deepest == 0 {
				t.Errorf("no row of the window reads as faded in any of the sampled frames: the band is " +
					"not being drawn at all")
			}
			if deepest > inset {
				t.Errorf("the fade reaches %d rows up from the window's bottom while the body's own "+
					"padding is %d: a band taller than the empty space under the last line lands on a "+
					"glyph at the end of the scroll (want ≤ inset)", deepest, inset)
			}
			if worstAbove < 100 {
				t.Fatalf("no ink above the scroll edge in any of the sampled frames (worst spread %d): "+
					"the ratio below would be judged against a blank window", worstAbove)
			}
			t.Logf("worst spread: last row %d, above the edge %d", worstLast, worstAbove)
			if worstLast*100 > worstAbove*45 {
				t.Errorf("the last row of the window reads at %d while the rows above the edge read at %d "+
					"(%.0f%% of it): the body is being cut flat at the window's edge — the bottom of a "+
					"scrolling plugin window needs the same soft edge its siblings have (want ≤45%%)",
					worstLast, worstAbove, 100*float64(worstLast)/float64(worstAbove))
			}

			// ② 自门控：滚到底时最后一行停在离下沿 12.6 处，9.0 的边整块落在空白上。
			detached.scroll.Y = detached.scroll.MaxY
			tt.Frame()
			tt.Frame()
			img = tt.Image()
			ref := pluginEdgeMean(img, H-band-6, H-band-3, x0, x1)
			bandMean := pluginEdgeMean(img, H-band, H, x0, x1)
			t.Logf("at the end of the scroll: above %d, inside the band %d", ref, bandMean)
			if diff := bandMean - ref; diff > 6 || diff < -6 {
				t.Errorf("at the end of the scroll the bottom %dpx read %d while the rows above them read %d "+
					"(%+d levels): the edge is not self-gating — it lays a band of its own colour over the "+
					"body's bottom padding (want the window's own ground, ±6)", band, bandMean, ref, diff)
			}
		})
	}
}

// TestThePinnedNoteFadesAtItsBottomEdge is the same pair on the other
// `.plugin-window`: the pinned note's body is the same scroller with the same
// inset, so the same edge has to be there.
func TestThePinnedNoteFadesAtItsBottomEdge(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tt, band := pluginEdgePinned(t, dark)
			img := tt.Image()
			H, W := img.Bounds().Dy(), img.Bounds().Dx()
			x0, x1 := 16, W-16

			worstLast, worstAbove := 0, 0
			for k := 0; k < 19; k++ {
				tt.Wheel(float32(W)/2, float32(H)/2, 0, -60)
				tt.Frame()
				tt.Frame()
				img = tt.Image()
				if s := pluginEdgeSpread(img, H-1, x0, x1); s > worstLast {
					worstLast = s
				}
				for y := H - band - 4; y < H-band; y++ {
					if s := pluginEdgeSpread(img, y, x0, x1); s > worstAbove {
						worstAbove = s
					}
				}
			}
			if worstAbove < 100 {
				t.Fatalf("no ink above the scroll edge in any of the wheel frames (worst spread %d): "+
					"the wheel did not scroll the note's body, so nothing was measured", worstAbove)
			}
			t.Logf("worst spread: last row %d, above the edge %d", worstLast, worstAbove)
			if worstLast*100 > worstAbove*45 {
				t.Errorf("the note's last row reads at %d while the rows above the edge read at %d "+
					"(%.0f%% of it): the pinned note is cut flat at its window's edge (want ≤45%%)",
					worstLast, worstAbove, 100*float64(worstLast)/float64(worstAbove))
			}
		})
	}
}

// TestTheEdgeCostsTheBodyNoLayout is the other half of ②, on the window that
// does overflow: the edge is an overlay, so it must not move the body's own
// geometry — `MaxY` stays the content's height, and at the end of the scroll the
// last line of text still stops 14 above the window's bottom, which is why a
// 10.0 band never lands on a glyph. (边 10.0 < 内边距 14.0, §6.145.)
func TestTheEdgeCostsTheBodyNoLayout(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tt, detached, band, inset := pluginEdgeDetached(t, dark)
			// 边高 < 内边距：滚到底时边整个落在空白上（§6.19 的自门控）。两个数
			// 都从同一个主题台阶读，改 `detachedEdgeInset` 这条会红。
			if band >= inset {
				t.Errorf("the edge band is %d px while the body's own padding is %d: the edge is taller "+
					"than the empty space under the last line, so at the end of the scroll it lands on a "+
					"glyph instead of on the padding (want band < inset)", band, inset)
			}
			// 叠层不吃布局：`MaxY` 是内容自己的高度（90 行 + 两侧 14 内边距），
			// 多一行就说明这道边在正文里占了位置。
			if got := detached.scroll.MaxY; got < 1151.7 || got > 1151.9 {
				t.Errorf("the body's MaxY is %v, want 1151.8 (the content's own height): the edge is "+
					"taking layout space instead of floating over the body", got)
			}
			t.Logf("MaxY = %v (content height; the fixture's %d lines), band %d, inset %d",
				detached.scroll.MaxY, pluginEdgeLines, band, inset)
			detached.scroll.Y = detached.scroll.MaxY
			tt.Frame()
			tt.Frame()
			img := tt.Image()
			H, W := img.Bounds().Dy(), img.Bounds().Dx()
			x0, x1 := 16, W-16
			// The last row holding ink, walking up from the window's bottom.
			last := -1
			for y := H - 1; y > H-40 && y >= 0; y-- {
				if pluginEdgeSpread(img, y, x0, x1) > 20 {
					last = y
					break
				}
			}
			if last < 0 {
				t.Fatalf("no ink in the bottom 40 rows at the end of the scroll: nothing was measured")
			}
			t.Logf("at the end of the scroll the last inked row is y=%d, %d px above the bottom (band %d)",
				last, H-1-last, band)
			if gap := H - 1 - last; gap <= band {
				t.Errorf("the last line of the body ends %d px above the window's bottom, inside the %d px "+
					"edge band: the edge covers the text instead of the empty padding below it "+
					"(want the body's 12.6 padding > the band)", gap, band)
			}
		})
	}
}

// TestDumpP13FZPluginWindowEdge writes the before/after evidence: the window at
// rest and the window scrolled to the middle, both appearances.
func TestDumpP13FZPluginWindowEdge(t *testing.T) {
	if os.Getenv("FLOTER_P13FZ_DUMP") == "" {
		t.Skip("opt-in: set FLOTER_P13FZ_DUMP=1 to write the PNGs")
	}
	dir := "/tmp/p13fz"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		tt, detached, _, _ := pluginEdgeDetached(t, dark)
		detached.scroll.Y = 0
		tt.Frame()
		tt.Frame()
		writeEdgePNG(t, filepath.Join(dir, fmt.Sprintf("floter-P13-fz-output-rest-%s.png", name)), tt.Image())
		detached.scroll.Y = detached.scroll.MaxY / 2
		tt.Frame()
		tt.Frame()
		writeEdgePNG(t, filepath.Join(dir, fmt.Sprintf("floter-P13-fz-output-mid-%s.png", name)), tt.Image())
		pin, _ := pluginEdgePinned(t, dark)
		for k := 0; k < 6; k++ {
			pin.Wheel(360, 230, 0, -60)
			pin.Frame()
		}
		writeEdgePNG(t, filepath.Join(dir, fmt.Sprintf("floter-P13-fz-pinned-mid-%s.png", name)), pin.Image())
	}
}

func writeEdgePNG(t *testing.T, name string, img *image.RGBA) {
	t.Helper()
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", name)
}
