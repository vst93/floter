package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-er（`docs/liquid-glass.md` §6.113）：浮层滚到底时，最后一张卡的边与底边
// 之间只隔着**软边本身**，没有呼吸。
//
// 病（改前实测，`pluginSheetFrame` 720×600，深浅同值）：滚到底时带子最下 **11 行**
// 是空白，第 12 行才是最后一张卡的下边——而底部那条软边的高度正是 `Space(3)`
// = **10.8px**。也就是说卡的边离软边上沿只剩 **1.2px**：浮层给自己留的那一格
// 下内边距（`pluginsheet.go` 的 `Padding(…, t.Space(3), …)`）**整好被软边吃掉**，
// 内容与视口下沿之间没有任何干净的空气，最后一张卡读起来是被切开的。
//
// 同一套滚动收口在设置体上一直是对的：那边 `Padding(edge, 0, t.Space(6), 0)`，
// 软边 `edge` 是 `Space(4)` = 14.4，下内边距 21.6 = **软边 + 一格 `Space(2)`**。
// 公式是「软边 + 一格呼吸」，每张面代入自己的软边高；浮层代入的是 10.8，答案 18.0
// （`Space(5)`），它给自己写的却是 10.8——少的那一格就是这条 pin 量的东西。
//
// 判据两条（都从这张面自己的主题里推，不写死像素）：
//  1. 滚到底时，最后一张卡的边离带子下沿 **≥ 软边高 + 一格 `Space(2)`**（改前 11 行）；
//  2. 停在开头时页头仍在原来的位置（顶部空白仍是 11 行）——这一轮只补下沿的呼吸，
//     不许把整块面往上或往下推。
func TestThePluginSheetKeepsAirAboveItsBottomFade(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			tt, a := pluginSheetFrame(t, dark)
			band := sheetBand(tt)
			scroll := &a.PluginScroll
			if scroll.MaxY <= 0 {
				t.Fatalf("the sheet does not overflow its window (MaxY = %v), so there is no full "+
					"state to measure: the pins below would pass on a sheet that never scrolls", scroll.MaxY)
			}
			steps := sheetSpace(t, a, 2, 3)
			breath, fade := float64(steps[0]), float64(steps[1])
			if fade < 10 || fade > 12 {
				t.Fatalf("the sheet's own fade is %v tall, not the Space(3) this pin was written "+
					"against (10.8): read the edge height off `pluginsheet.go` again", fade)
			}
			// Blank rows above / below the outermost thing the sheet paints. The
			// threshold is the one `sheetInkBands` uses to skip the faces
			// themselves (a card's edge reads 17 levels off its ground); here it
			// is counted the other way round, because the thing this round is
			// about is exactly that edge rather than a letter.
			blank := func() (top, bottom int) {
				t.Helper()
				tt.Frame()
				tt.Frame()
				img := tt.Image()
				for y := band.Min.Y; y < band.Max.Y; y++ {
					if spreadAt(img, y, band.Min.X, band.Max.X) > 4 {
						break
					}
					top++
				}
				for y := band.Max.Y - 1; y >= band.Min.Y; y-- {
					if spreadAt(img, y, band.Min.X, band.Max.X) > 4 {
						break
					}
					bottom++
				}
				return top, bottom
			}
			scroll.Y = 0
			restTop, _ := blank()
			scroll.Y = scroll.MaxY
			_, endBottom := blank()
			t.Logf("blank rows: %d above the head at rest, %d below the last card at the end "+
				"(fade %v, breath %v, MaxY %v)", restTop, endBottom, fade, breath, scroll.MaxY)

			if restTop < 10 || restTop > 12 {
				t.Errorf("the sheet's head moved: %d blank rows above its first paint, want 11 "+
					"(this round only gives the bottom its breath)", restTop)
			}
			want := fade + breath
			if float64(endBottom) < want-0.5 {
				t.Errorf("the last card ends %d rows above the band's bottom, but the bottom fade is %v "+
					"tall and the sheet owes one %v breath under it (%v rows): the fade is eating the "+
					"sheet's own padding, so the last card reads as cut rather than as ended",
					endBottom, fade, breath, want)
			}
		})
	}
}

// sheetSpace reads spacing steps from the same theme the sheet is drawn in
// (`pluginSheetFrame` installs `a.tokens(c).Theme`). The stock tester's own
// theme rounds the step differently — `contextSpace` there reads Space(2) as 8
// against this surface's 7.2 — so the numbers this pin compares against have to
// come from the frame's theme, not from a theme built beside it.
func sheetSpace(t *testing.T, a *App, steps ...float32) []float32 {
	t.Helper()
	out := make([]float32, len(steps))
	ui.NewTester(func(c *ui.Context) {
		tokens := a.tokens(c)
		c.SetTheme(tokens.Theme)
		tokens = a.tokens(c)
		for i, step := range steps {
			out[i] = tokens.Theme.Space(step)
		}
	}, 100, 100).Frame()
	return out
}

// TestDumpP13ERSheetBottom is the opt-in pair of frames this round is judged on:
// the sheet parked at its start and at its end, in both appearances.
//
//	FLOTER_DUMP_P13ER=1 go test ./internal/settingsui/ -run TestDumpP13ERSheetBottom -v
func TestDumpP13ERSheetBottom(t *testing.T) {
	if os.Getenv("FLOTER_DUMP_P13ER") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_DUMP_P13ER=1")
	}
	for _, dark := range []bool{false, true} {
		tt, a := pluginSheetFrame(t, dark)
		name := "light"
		if dark {
			name = "dark"
		}
		for _, spot := range []string{"rest", "end"} {
			if spot == "end" {
				a.PluginScroll.Y = a.PluginScroll.MaxY
			}
			tt.Frame()
			tt.Frame()
			f, err := os.Create("/tmp/floter-P13-er-sheet-" + spot + "-" + name + ".png")
			if err != nil {
				t.Fatalf("cannot write the frame: %v", err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatalf("cannot encode the frame: %v", err)
			}
			f.Close()
		}
	}
}
