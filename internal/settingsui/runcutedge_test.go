package settingsui

import (
	"image"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// P13-at · 运行输出块的两端：一块 180px 的凹槽不该在两条边上刀切。
//
// 这块 `MaxHeight(45u)` 的滚动视口（`settingsui.go` 的 `runOutput`）上下两条边原
// 都是平刀切：滚到一半时视口第一行的墨展开 193 对最内的 202（浅色 96%），最后一行
// 194（96%），框外一行立刻归 0——切由 `.Clip()` 造成。P13-as 给设置页滚动体补上了
// 下沿软边，这一块是同一族的**两端都硬**的那个。
//
// 四条 pin：① 中间任意位置两头都要退让；② 边按滚动位置开关——停在开头、滚到底时
// 正在读的那一行不许被淡；③ 边淡向凹槽自己的面（`runOutputFace`），不是窗口底色；
// ④ 盒子零改动（视口尺寸与 `MaxY` 逐像素不动），边只是画上去的两条。

// runOutputBox is the block's scroll viewport on screen: the drawn box of the
// captured text, which does not move as the content scrolls.
func runOutputBox(t *testing.T, tt *ui.Tester, text string) image.Rectangle {
	t.Helper()
	vp, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the captured text was not drawn: %q", tt.Texts())
	}
	return image.Rect(int(vp.X), int(vp.Y), int(vp.X+vp.W), int(vp.Y+vp.H))
}

// bandMax is the strongest ink in a horizontal band: the max of the per-row
// luma spread, so a blank gap row cannot stand in for the band.
func bandMax(img *image.RGBA, y0, y1, x0, x1 int) int {
	best := 0
	for y := y0; y < y1; y++ {
		if s := spreadAt(img, y, x0, x1); s > best {
			best = s
		}
	}
	return best
}

// P13-at ①：滚在中间时，视口两头要退让到最内那一带的一小部分。
//
// 两道软边都以 0.85 的底色起步、在一个边高内落到 0，而边高 3u(12px) 小于一个行盒
// （caption 11px × 1.45 ≈ 16px），所以最外侧 4 行的墨只该剩 FadeShare 以下。
func TestTheRunOutputBlockFadesAtBothEdges(t *testing.T) {
	// 两套外观各跑一遍：边的底色取凹槽自己的面（浅色 Sunken 229 / 深色 39），
	// 一套绿不代表另一套也绿。
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a, text := runOutputScrollApp(t)
			tt := integrationPage(t, a, 720, 2400)
			if dark {
				tt.SetDark(true)
				tt.Frame()
			}
			scroll := a.runOutputScroll("dev.floter.tool")
			if scroll.MaxY <= 0 {
				t.Fatalf("the output block does not overflow its frame (MaxY = %v), so there is nothing "+
					"to scroll and the pins below would pass on a block that never cuts anything", scroll.MaxY)
			}
			// 19 档滚动位置上的最坏值，而不是单看中间一档：边是自门控的，门在
			// 0 < Y < MaxY 开，最坏值应当来自这些位置中的某一个。
			worstTop, worstBottom, deepest := 0, 0, 0
			for k := 1; k <= 19; k++ {
				scroll.Y = scroll.MaxY * float32(k) / 20
				tt.Frame()
				tt.Frame()
				box := runOutputBox(t, tt, text)
				img := tt.Image()
				if s := bandMax(img, box.Min.Y, box.Min.Y+4, box.Min.X, box.Max.X); s > worstTop {
					worstTop = s
				}
				if s := bandMax(img, box.Max.Y-4, box.Max.Y, box.Min.X, box.Max.X); s > worstBottom {
					worstBottom = s
				}
				// 最内的一带：离两头都有半个视口，边够不到。
				if s := bandMax(img, box.Min.Y+70, box.Min.Y+100, box.Min.X, box.Max.X); s > deepest {
					deepest = s
				}
			}
			if deepest < 100 {
				t.Fatalf("no ink inside the block in any of the sampled frames (deepest spread %d): "+
					"the ratios below would be judged against a blank box", deepest)
			}
			t.Logf("worst spread: top 4 rows %d, bottom 4 rows %d, inside %d", worstTop, worstBottom, deepest)
			const fadeShare = 45
			if worstTop*100 > deepest*fadeShare {
				t.Errorf("the block's first 4 rows read at %d against the inside's %d (%.0f%% of it): the top "+
					"of the viewport is cutting the captured output flat — it needs the soft edge a scrolling "+
					"box wears (want ≤%d%%)",
					worstTop, deepest, 100*float64(worstTop)/float64(deepest), fadeShare)
			}
			if worstBottom*100 > deepest*fadeShare {
				t.Errorf("the block's last 4 rows read at %d against the inside's %d (%.0f%% of it): the bottom "+
					"of the viewport is cutting the captured output flat — it needs the same soft edge its top "+
					"has (want ≤%d%%)",
					worstBottom, deepest, 100*float64(worstBottom)/float64(deepest), fadeShare)
			}
		})
	}
}

// P13-at ②：边按滚动位置开关，而不是常亮。
//
// 这块凹槽没有、也不能预留 padding（`MaxHeight(45u)` 是固定的，加 padding 等于从
// 视口里偷掉两行字），所以设置页那处的「自门控」（边落在预留的空白上）在这里不成立：
// 停在开头时视口第一行、滚到底时最后一行，都是用户正在读的那一行，常亮的边会把它
// 淡掉。门必须开在 `0 < Y < MaxY` 上。
func TestTheRunOutputEdgesAreGatedOnTheOffset(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a, text := runOutputScrollApp(t)
			// 这套装置的行文本尾部带一个换行，滚到底时最后 16px 恰是那个空行——
			// 拿它当「最后一行有没有被淡」的判据会量到空白上（实测 0）。这段 pin
			// 用一张**不以空行结尾**的同一段文本，滚到底时末行真有字。
			text = strings.TrimSuffix(text, "\n")
			a.SetRunResult("dev.floter.tool", RunResult{Stdout: text, Success: true})
			tt := integrationPage(t, a, 720, 2400)
			if dark {
				tt.SetDark(true)
				tt.Frame()
			}
			scroll := a.runOutputScroll("dev.floter.tool")
			if scroll.MaxY <= 0 {
				t.Fatalf("the output block does not overflow its frame (MaxY = %v)", scroll.MaxY)
			}
			// 停在开头：第一行是「line 00」，是一块满对比的字。
			scroll.Y = 0
			tt.Frame()
			tt.Frame()
			box := runOutputBox(t, tt, text)
			img := tt.Image()
			restTop := bandMax(img, box.Min.Y, box.Min.Y+8, box.Min.X, box.Max.X)
			inside := bandMax(img, box.Min.Y+70, box.Min.Y+100, box.Min.X, box.Max.X)
			t.Logf("at the start: first 8 rows %d, inside %d", restTop, inside)
			if inside < 100 {
				t.Fatalf("no ink inside the block (spread %d)", inside)
			}
			if restTop*100 < inside*90 {
				t.Errorf("at the start of the scroll the block's first 8 rows read at %d against the inside's "+
					"%d (%.0f%% of it): the block is fading the line the user is reading — the edge must open "+
					"only once the content has moved under it (want ≥90%%)",
					restTop, inside, 100*float64(restTop)/float64(inside))
			}
			// 滚到底：最后一行同理。
			scroll.Y = scroll.MaxY
			tt.Frame()
			tt.Frame()
			img = tt.Image()
			endBottom := bandMax(img, box.Max.Y-8, box.Max.Y, box.Min.X, box.Max.X)
			inside = bandMax(img, box.Min.Y+70, box.Min.Y+100, box.Min.X, box.Max.X)
			t.Logf("at the end: last 8 rows %d, inside %d", endBottom, inside)
			if inside < 100 {
				t.Fatalf("no ink inside the block at the end (spread %d)", inside)
			}
			if endBottom*100 < inside*90 {
				t.Errorf("at the end of the scroll the block's last 8 rows read at %d against the inside's "+
					"%d (%.0f%% of it): the bottom edge is still on over the last line (want ≥90%%)",
					endBottom, inside, 100*float64(endBottom)/float64(inside))
			}
		})
	}
}

// P13-at ③：边淡向的是凹槽自己的面，不是窗口底色。
//
// 两个错要一起防：① P13-as 仍待 2 记下的那个取 `t.Background`（浅色 247 / 深色
// 14）的写法，而凹槽脚下是 `Sunken` 叠在扩展卡上（实测浅色 236 / 深色 42）；② 更
// 隐形的一个：把 `Sunken` 这层**薄罩**直接交给 `ScrollEdge`——`ui.Color.Alpha` 是
// 乘，0.85 会被它自己的 0.17 乘成 0.14，淡就白做了。这里读一条**没有字**的竖带
// （文本是左对齐的短行，右侧一大片是纯面）：在边所在的那一带，读数必须仍是凹槽
// 自己的面，而不是任何别的面。
func TestTheRunOutputEdgesPaintTheBlocksOwnFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a, text := runOutputScrollApp(t)
			tt := integrationPage(t, a, 720, 2400)
			if dark {
				tt.SetDark(true)
				tt.Frame()
			}
			scroll := a.runOutputScroll("dev.floter.tool")
			if scroll.MaxY <= 0 {
				t.Fatalf("the output block does not overflow its frame (MaxY = %v)", scroll.MaxY)
			}
			scroll.Y = scroll.MaxY / 2
			tt.Frame()
			tt.Frame()
			box := runOutputBox(t, tt, text)
			img := tt.Image()
			// 纯面的竖带：离右沿 30-100px（避开滚动条与行尾），离两头都有余量。
			x0, x1 := box.Max.X-100, box.Max.X-30
			face := meanLuma(img, box.Min.Y+70, box.Min.Y+100, x0, x1)
			topBand := meanLuma(img, box.Min.Y, box.Min.Y+12, x0, x1)
			bottomBand := meanLuma(img, box.Max.Y-12, box.Max.Y, x0, x1)
			// 窗口自己的地：错的底色长什么样，拿它做对照。
			ground := int(theme.For(a.Store.Snapshot(), dark).Theme.Background.R)
			t.Logf("the block's own face %d, inside the top edge %d, inside the bottom edge %d, the window ground %d",
				face, topBand, bottomBand, ground)
			// 容许的差：这条链压得准（实测两套外观都是 0 级）；留 4 级给字体
			// 与缩放的不同。把边换成窗口底色会差 11（浅）/ 28（深）。
			const slack = 4
			for _, band := range []struct {
				name string
				v    int
			}{{"top", topBand}, {"bottom", bottomBand}} {
				diff := band.v - face
				if diff > slack || diff < -slack {
					t.Errorf("inside the %s edge the empty column reads %d while the block's own face reads %d "+
						"(%+d levels): the edge is laying down a surface other than the block's own, which "+
						"leaves a band of its own over the block (want the block's own face, ±%d)",
						band.name, band.v, face, diff, slack)
					continue
				}
				// 方向也对：它得比窗口的地更贴近凹槽的面，否则只是碰巧落在容差里。
				if absDiff(band.v, ground) < absDiff(band.v, face) {
					t.Errorf("inside the %s edge the empty column reads %d: nearer the window ground %d than the "+
						"block's own face %d", band.name, band.v, ground, face)
				}
			}
		})
	}
}

// absDiff is |a-b| for the two readings above (the package's own helper,
// declared once in lockedform_test.go).

// P13-at ④：盒子零改动。
//
// 边是两条 `Absolute` + `PassThrough` 的覆盖层，包住视口的那层 `Column` 不带
// padding、不带 gap。所以视口的尺寸与位置、以及内容还能滚多远，都必须和改动前一样
// —— `MaxHeight(45u)` 的 180px 视口，四档屏幕上逐像素相同。
func TestTheRunOutputEdgesDoNotMoveTheBox(t *testing.T) {
	a, text := runOutputScrollApp(t)
	tt := integrationPage(t, a, 720, 2400)
	scroll := a.runOutputScroll("dev.floter.tool")
	if got, want := scroll.MaxY, float32(735); got != want {
		t.Errorf("the block's content now scrolls %v px, want the shipped %v: the edges changed the box",
			got, want)
	}
	box := runOutputBox(t, tt, text)
	if got, want := box.Dx(), 475; got != want {
		t.Errorf("the block's viewport is %d px wide, want the shipped %d", got, want)
	}
	if got, want := box.Dy(), 180; got != want {
		t.Errorf("the block's viewport is %d px tall, want the shipped %d (45u)", got, want)
	}
	// 每档滚动位置都还在原地：边不许把视口往下推。
	for k := 1; k <= 5; k++ {
		scroll.Y = scroll.MaxY * float32(k) / 6
		tt.Frame()
		tt.Frame()
		if moved := runOutputBox(t, tt, text); moved != box {
			t.Fatalf("at Y = %v the viewport moved from %v to %v", scroll.Y, box, moved)
		}
	}
}
