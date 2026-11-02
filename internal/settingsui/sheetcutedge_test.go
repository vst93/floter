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
//
// P13-dc：取景必须**装设计主题**。`PluginSheet` 里的间距与字号读的是 `c.Theme()`
// （`pluginsheet.go:62-63`），而这条夹具此前只取了 `tokens.Theme.Background` 画底、
// 从没 `c.SetTheme` ——于是 `t.FontSize` 是框架库存的 14、`t.Space(1)` 是库存的 4，
// 生产却是 10.8 / 3.6。三条 pin 因此在量一个**不存在的界面**（`MaxY` 500 而不是
// 生产的读数）。口径见 `.pending-bugs.md` 顶部 P13-cm 那条教训。
func pluginSheetFrame(t *testing.T, dark bool) (*ui.Tester, *App) {
	t.Helper()
	a := New(newStore(t), Actions{})
	sheet := sheetProbeSchema(14)
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := a.tokens(c)
		c.SetTheme(tokens.Theme)
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

// P13-dc ②：边靠**几何**自门控的前提——两端各有「一个边高」的空白。
//
// 与 §6.16 同一条机制、与 §6.17 / §6.18 的 `ScrollState.Y` 门不同：这块浮层的两
// 端本来就有留白。P13-aw 写的是「最外 12 行是空白」，那 12 是**库存字取景**里的读数；
// 装回设计主题（字 10.8 / 间距 3.6）后，空白恰好是**边高**那么多：最上 / 最下 11 行
// 为 0，第 12 行才是墨。所以窗口按边高取（`ceil(3u)` = 11），而不是一个凭手感写的
// 12：那条窗口比边本身宽，多量一行就量到标题的墨（实测浅 62 / 深 62），于是红线变成
// 了一条做不到的断言。
//
// 余量只有 0.2px（10.8 对 11）：标题行一旦变高、顶留白一旦变窄，这条 pin 会红——
// 那时该换成 §6.17 那条 `Y` 门，而不是把窗口调小。
func TestThePluginSheetEdgesSitOnBlankAtBothEnds(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			tt, a := pluginSheetFrame(t, dark)
			band := sheetBand(tt)
			scroll := &a.PluginScroll
			if scroll.MaxY <= 0 {
				t.Fatalf("the sheet does not overflow its window (MaxY = %v)", scroll.MaxY)
			}
			// 边高 3u = 10.8px，向上取整为 11 行：这 11 行必须没有墨。
			edgeRows := 11
			measure := func() (top, bottom, beyondTop, beyondBottom int) {
				tt.Frame()
				tt.Frame()
				img := tt.Image()
				return bandMax(img, band.Min.Y, band.Min.Y+edgeRows, band.Min.X, band.Max.X),
					bandMax(img, band.Max.Y-edgeRows, band.Max.Y, band.Min.X, band.Max.X),
					bandMax(img, band.Min.Y+edgeRows, band.Min.Y+edgeRows+1, band.Min.X, band.Max.X),
					bandMax(img, band.Max.Y-edgeRows-1, band.Max.Y-edgeRows, band.Min.X, band.Max.X)
			}
			scroll.Y = 0
			restTop, _, pastTop, _ := measure()
			scroll.Y = scroll.MaxY
			_, endBottom, _, pastBottom := measure()
			t.Logf("blank ends: top %d rows at rest %d (the row past them %d), bottom %d rows at the end "+
				"%d (the row past them %d)", edgeRows, restTop, pastTop, edgeRows, endBottom, pastBottom)
			if pastTop == 0 && pastBottom == 0 {
				t.Fatalf("there is no ink anywhere below the edge on both ends (past them: %d / %d), so this "+
					"pin is measuring blank band rather than the sheet's own leading", pastTop, pastBottom)
			}
			const blank = 4
			if restTop > blank {
				t.Errorf("the sheet parked at its start has ink inside its first %d rows (spread %d): the top "+
					"edge would wash the row the user is reading — it must land on the header row's own "+
					"leading, not on its ink (want ≤%d)", edgeRows, restTop, blank)
			}
			if endBottom > blank {
				t.Errorf("the sheet parked at its end has ink inside its last %d rows (spread %d): the bottom "+
					"edge would wash the last field (want ≤%d)", edgeRows, endBottom, blank)
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
			// 实测基线（P13-dc，装回设计主题之后）：`MaxY` 316.4、标题盒
			// `(18.4, 63.7) 70×17`。P13-aw 记的 `MaxY` 500 / `(20,63) 90×22` 是**库存字的
			// 那条夹具**的读数（取景没装设计主题，字 14 / 间距 4），不是生产的界面；
			// 本轮把取景修好，旧的三个数一起作废。
			//
			// 改前 `MaxY` 是 **0**——见 `internal/launcher/config.go` `configBody` 的注释：
			// 内容高度的父盒不给 scroller 可用高度，schema 比窗口高时末尾几个字段被窗口
			// 下沿切掉且滚不到。所以这里的 `MaxY` 是**修好的那个值**，不是被边推动的值。
			// P13-ej 重定基线：旧值 **316.4** → 新值 **330.8**。§6.105 把分组标题与
			// 它那张卡之间的空气从 `Space(2)` 抬到 `Space(4)`（+7.2px），两张分组卡
			// 各抬一次 → +14.4。这一条判的是「边是画出来的、不是几何」，所以基线跟着
			// 内容高度走，判据本身不动。
			if got, want := a.PluginScroll.MaxY, float32(330.8); got < want-0.05 || got > want+0.05 {
				t.Errorf("the sheet's scroll range moved: MaxY %v, want %v (the edges are paint, not "+
					"geometry)", got, want)
			}
			if title.Y < 63 || title.Y > 64 || title.H != 17 {
				t.Errorf("the sheet's title band moved: %+v, want y≈63.7 h=17 (the edges must not push "+
					"the content)", title)
			}
		})
	}
}
