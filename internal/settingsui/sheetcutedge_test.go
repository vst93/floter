package settingsui

import (
	"fmt"
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/plugincfg"
)

// P13-aw · 插件配置浮层的两端：带子不该在内容中间平刀切。
//
// 这块 `plugin.config` 的 scroller 是「滚动收口」这一族的最后一块面（§6.16 设置体 /
// §6.17 输出凹槽 / §6.18 多选箱之后）。改前它的内容在带子的上下两条边被
// `.Clip()` 平刀切：滚到一半时最外 4 行的墨展开读到最内那一带的 ~96%，框外一行
// 立刻归 0。
//
// 三条 pin：① 中间任意位置两头都要退让；② 边按滚动位置开关——停在开头、滚到底时
// 正在读的那一行不许被淡；③ 浮层里的一切几何零改动（`MaxY` 与标题盒逐像素不动），
// 边只是画上去的两条。

// sheetProbeSchema is a schema taller than the window it is drawn in, so the
// sheet overflows and its scroller has somewhere to scroll to. Two sections,
// so the frame also carries a section heading band on both sides of the cut.
func sheetProbeSchema(count int) PluginSheet {
	fields := make([]plugincfg.Field, 0, count)
	values := plugincfg.Values{}
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("f%d", i)
		section := "First section"
		if i >= count/2 {
			section = "Second section"
		}
		fields = append(fields, plugincfg.Field{
			Key: key, Kind: plugincfg.Toggle, Section: section,
			Label: fmt.Sprintf("Toggle number %d", i+1),
			Help:  fmt.Sprintf("The help line for toggle %d.", i+1),
		})
		values[key] = true
	}
	return PluginSheet{
		Schema: &plugincfg.Schema{Plugin: "dev.floter.probe", Title: "Probe plugin", Fields: fields},
		Values: values,
		Change: func(string, any) {}, Arm: func(string) {}, Run: func(string) {}, Unarm: func(string) {},
	}
}

// pluginSheetFrame draws the sheet the way the launcher does: over one panel
// of the panel's own face (`Sunken` on the window's ground), in a window too
// short for the schema.
func pluginSheetFrame(t *testing.T, dark bool) (*ui.Tester, *App) {
	t.Helper()
	a := New(newStore(t), Actions{})
	sheet := sheetProbeSchema(14)
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := a.tokens(c)
		c.Root().Background(tokens.Theme.Background)
		// The launcher's own chain: the panel (`Sunken`, filling the window),
		// then the band the sheet is drawn in (`launcher/config.go`
		// `configBody`, padded by the field row above it).
		ui.Box(c).Fill().Background(tokens.Sunken).Children(func() {
			ui.Box(c).Fill().Padding(56, 4, 0, 4).Children(func() {
				a.PluginSheet(c, sheet)
			})
		})
	}, 720, 600)
	tt.Frame()
	if dark {
		tt.SetDark(true)
		tt.Frame()
	}
	return tt, a
}

// sheetBand is the band the sheet was given on screen: the panel's box less the
// 56px field row and the 4px breath above it (`launcher/config.go` `configBody`
// draws the sheet with that top padding, and the launcher's own inset on both
// sides). The band's two ends are what the scroll edges must soften.
func sheetBand(tt *ui.Tester) image.Rectangle {
	return image.Rect(4, 56, tt.Image().Bounds().Dx()-4, tt.Image().Bounds().Dy())
}

// P13-aw ①：滚在中间时，视口两头要退让到最内那一带的一小部分。
func TestThePluginSheetFadesAtBothEdges(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			tt, a := pluginSheetFrame(t, dark)
			scroll := &a.PluginScroll
			if scroll.MaxY <= 0 {
				t.Fatalf("the sheet does not overflow its window (MaxY = %v), so there is nothing to "+
					"scroll and the pins below would pass on a sheet that never cuts anything", scroll.MaxY)
			}
			worstTop, worstBottom, deepest := 0, 0, 0
			band := sheetBand(tt)
			for k := 1; k <= 19; k++ {
				scroll.Y = scroll.MaxY * float32(k) / 20
				tt.Frame()
				tt.Frame()
				img := tt.Image()
				if s := bandMax(img, band.Min.Y, band.Min.Y+4, band.Min.X, band.Max.X); s > worstTop {
					worstTop = s
				}
				if s := bandMax(img, band.Max.Y-4, band.Max.Y, band.Min.X, band.Max.X); s > worstBottom {
					worstBottom = s
				}
				// 最内的一带：离两头都有半个视口，边够不到。
				if s := bandMax(img, band.Dy()/2-30, band.Dy()/2+10, band.Min.X, band.Max.X); s > deepest {
					deepest = s
				}
			}
			if deepest < 100 {
				t.Fatalf("no ink inside the sheet in any of the sampled frames (deepest spread %d): the "+
					"ratios below would be judged against a blank band", deepest)
			}
			t.Logf("worst spread: top 4 rows %d, bottom 4 rows %d, inside %d", worstTop, worstBottom, deepest)
			const fadeShare = 45
			if worstTop*100 > deepest*fadeShare {
				t.Errorf("the sheet's first 4 rows read at %d against the inside's %d (%.0f%% of it): the "+
					"top of the band is cutting the sheet flat — it needs the soft edge a scrolling "+
					"surface wears (want ≤%d%%)", worstTop, deepest, 100*float64(worstTop)/float64(deepest), fadeShare)
			}
			if worstBottom*100 > deepest*fadeShare {
				t.Errorf("the sheet's last 4 rows read at %d against the inside's %d (%.0f%% of it): the "+
					"bottom of the band is cutting the sheet flat — it needs the same soft edge its top has "+
					"(want ≤%d%%)", worstBottom, deepest, 100*float64(worstBottom)/float64(deepest), fadeShare)
			}
		})
	}
}

// P13-aw ②：这道边靠**几何**自门控，不靠条件判断。
//
// 与 §6.16 同一条机制、与 §6.17 / §6.18 的 `ScrollState.Y` 门不同：这块浮层的两
// 端本来就有留白——顶边压在 scroller 的 1u 顶留白与标题行的行盒上身，底边的
// 3u(12px) 底留白与边高逐像素相等。所以停在开头时最外 12 行、滚到底时最后 12 行
// **没有墨可淡**，正在读的那一行（标题 / 最后一个字段）一格都不会被洗。
//
// 这条 pin 量的就是那个前提：两端 12 行是空白。若有人把底留白从 3u 改小（或把
// 标题行挪到第一行李），它会红——那时边就真的会吃掉正在读的那一行。
func TestThePluginSheetEdgesSitOnBlankAtBothEnds(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			tt, a := pluginSheetFrame(t, dark)
			band := sheetBand(tt)
			scroll := &a.PluginScroll
			if scroll.MaxY <= 0 {
				t.Fatalf("the sheet does not overflow its window (MaxY = %v)", scroll.MaxY)
			}
			measure := func() (top, bottom int) {
				tt.Frame()
				tt.Frame()
				img := tt.Image()
				return bandMax(img, band.Min.Y, band.Min.Y+12, band.Min.X, band.Max.X),
					bandMax(img, band.Max.Y-12, band.Max.Y, band.Min.X, band.Max.X)
			}
			scroll.Y = 0
			restTop, _ := measure()
			scroll.Y = scroll.MaxY
			_, endBottom := measure()
			t.Logf("blank ends: top 12 rows at rest %d, bottom 12 rows at the end %d", restTop, endBottom)
			const blank = 4
			if restTop > blank {
				t.Errorf("the sheet parked at its start has ink in its first 12 rows (spread %d): the top "+
					"edge would wash the row the user is reading — the top edge must land on the header "+
					"row's own leading, not on its ink (want ≤%d)", restTop, blank)
			}
			if endBottom > blank {
				t.Errorf("the sheet parked at its end has ink in its last 12 rows (spread %d): the 3u "+
					"bottom padding no longer equals the edge's height, so the bottom edge would wash the "+
					"last field (want ≤%d)", endBottom, blank)
			}
		})
	}
}

// P13-aw ③：浮层里的一切几何零改动。
//
// 边是画上去的两条，不该动盒子：`MaxY`、标题盒、每一行的文字盒逐像素不变。
func TestThePluginSheetEdgesAddNoGeometry(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			tt, a := pluginSheetFrame(t, dark)
			title, ok := tt.Find("Probe plugin")
			if !ok {
				t.Fatalf("the sheet's title was not drawn: %q", tt.Texts())
			}
			// 实测基线（P13-aw）：`MaxY` 500、标题盒 (20,63) 90×22。改前 `MaxY` 是
			// **0**——见 `internal/launcher/config.go` `configBody` 的注释：内容高度的
			// 父盒不给 scroller 可用高度，schema 比窗口高时末尾几个字段被窗口下沿切掉
			// 且滚不到。所以这一轮的 `MaxY` 是**修好的那个值**，不是被边推动的值；被
			// 推动的只有标题盒，它一格没动。
			if got, want := a.PluginScroll.MaxY, float32(500); got != want {
				t.Errorf("the sheet's scroll range moved: MaxY %v, want %v (the edges are paint, not "+
					"geometry)", got, want)
			}
			if title.Y != 63 || title.H != 22 {
				t.Errorf("the sheet's title band moved: %+v, want y=63 h=22 (the edges must not push the "+
					"content)", title)
			}
		})
	}
}
