package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-as · 满态的下沿：是和上沿同一道软边，还是一刀刃？
//
// 设置页的滚动区只有上沿有一道 `glass.ScrollEdge`（`settingsui.go` 的
// `body`）：4u 的高度被预留成 padding，滚起来时内容淡进背景。下沿什么都没有，
// 于是滚到某些位置时，一页比窗口长的内容就在**窗口最后一行像素**上被水平切断，
// 墨一点没退。实测（720×580，General 页，`Body.Y` 在 `[0, MaxY]` 上取 19 档）：
//
//	档位       窗口最后一行（y=579）的墨展开度      浅色      深色
//	30%        lo=27 hi=246                       219       222
//	70%        lo=229 hi=255                      167       165
//
// 上沿那一档的对照值（同一批帧里、软边上方几行）也是 219/222——两头正好是
// 「一软一刀」。启动器的满态早有折边（§6.1 表 1-10），这是同一族最后一个刀口。
//
// 两条 pin：① 最后一行必须是**退后**的，不是满对比；② 边是自门控的——滚到底
// （下面没有内容可淡）时它不许在空白上留下一道灰带。

// spreadAt is the ink of one pixel row in the content column: max luma − min
// luma. A row holding a glyph reads in the hundreds; a bare pane reads 0-2.
func spreadAt(img *image.RGBA, y, x0, x1 int) int {
	lo, hi := 999, -1
	for x := x0; x < x1; x++ {
		i := y*img.Stride + x*4
		v := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
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

// meanLuma is the average luma of a block: what a veil of a wrong colour
// moves, where spreadAt cannot see it (a lateral-uniform band has no spread).
func meanLuma(img *image.RGBA, y0, y1, x0, x1 int) int {
	sum, n := 0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			sum += luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

func TestTheFullSettingsBodyFadesAtItsBottomEdge(t *testing.T) {
	// 两套外观各跑一遍：软边的背景是主题的 Background，两套的差值不一样，一套绿
	// 不代表另一套也绿。
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a := New(newStore(t), Actions{})
			var bandPx float32
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				bandPx = c.Theme().Space(4)
				a.View(c)
			}, 720, 580)
			tt.SetDark(dark)
			tt.Frame()
			tt.Frame()
			if a.Body.MaxY <= 0 {
				t.Fatalf("the general page does not overflow this frame (MaxY = %v), so there is no "+
					"full state to measure: the pins below would pass on a page that never scrolls", a.Body.MaxY)
			}
			img := tt.Image()
			H, W := img.Bounds().Dy(), img.Bounds().Dx()
			x0, x1 := 240, W-24
			band := int(bandPx)
			if band < 4 {
				t.Fatalf("the scroll edge resolves to %v px, too thin to measure", bandPx)
			}

			// ① 19 档滚动位置上的最坏值：最后一行 vs 软边上方那一带。
			worstLast, worstAbove := 0, 0
			for k := 1; k <= 19; k++ {
				a.Body.Y = a.Body.MaxY * float32(k) / 20
				tt.Frame()
				tt.Frame()
				img = tt.Image()
				if s := spreadAt(img, H-1, x0, x1); s > worstLast {
					worstLast = s
				}
				for y := H - band - 4; y < H-band; y++ {
					if s := spreadAt(img, y, x0, x1); s > worstAbove {
						worstAbove = s
					}
				}
			}
			if worstAbove < 100 {
				t.Fatalf("no ink above the scroll edge in any of the sampled frames (worst spread %d): "+
					"the ratio below would be judged against a blank page", worstAbove)
			}
			t.Logf("worst spread: last row %d, above the edge %d", worstLast, worstAbove)
			if worstLast*100 > worstAbove*45 {
				t.Errorf("the last row of the window reads at %d while the rows above the edge read at %d "+
					"(%.0f%% of it): content is being cut flat at the window's edge — the bottom of a full "+
					"page needs the same soft edge its top has (want ≤45%%)",
					worstLast, worstAbove, 100*float64(worstLast)/float64(worstAbove))
			}

			if tag := os.Getenv("FLOTER_P13AS_TAG"); tag != "" {
				a.Body.Y = a.Body.MaxY * 6 / 20
				tt.Frame()
				tt.Frame()
				name := filepath.Join("/tmp", fmt.Sprintf("floter-P13-as-body-%s-%s.png", tag, name))
				f, err := os.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				t.Logf("wrote %s", name)
			}

			// ② 自门控：滚到底时下面只剩滚动区的 6u padding，没有内容可淡，这道边
			// 不许把空白染出一道带子。
			a.Body.Y = a.Body.MaxY
			tt.Frame()
			tt.Frame()
			img = tt.Image()
			ref := meanLuma(img, H-band-6, H-band-3, x0, x1)
			bandMean := meanLuma(img, H-band, H, x0, x1)
			t.Logf("at the end of the scroll: above %d, inside the band %d", ref, bandMean)
			if diff := bandMean - ref; diff > 6 || diff < -6 {
				t.Errorf("at the end of the scroll the bottom %dpx read %d while the rows above them read %d "+
					"(%+d levels): the edge is not self-gating — it lays a band of its own colour over the "+
					"page's bottom padding (want the surface's own colour, ±6)", band, bandMean, ref, diff)
			}

		})
	}
}
