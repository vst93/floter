package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// 满态（full state）的另一半：集成页的命令列表 24 条。
//
// 会话页那一半在 §6.141（P13-fv）：行距常数、卡下沿、软边淡向谁。这一页此前的
// pin 全是**三条命令**的（`commandlist_test.go`），"很多块板"这一态没有量过。
//
// 四条判据（`docs/liquid-glass.md` §6.142）：
//
//	142-1 板的节奏：24 块、23 个间隔逐值 129.00，板与板之间是**卡的 8px 面**
//	      （不是 0、也不是板自己的边贴边）；
//	142-2 卡的缩进：卡的两条竖边与板的两条竖边之间各有一段**卡的 8px 面**——
//	      这条是本轮修的缺陷：命令这一块此前是卡的直接子节点，一个缩进都不戴，
//	      板的边框紧贴卡的边框（浅 216/194、深 51/78 两行 1px 挨着），24 块板
//	      时它是沿整张卡左右两侧从上贯到下的一条双线，也就是 §1 要拆掉的那圈画框；
//	142-3 板自己的墨列：名字的 x 比板的左边框右 12.8（板自己的 12px 内距 + 1px 边），
//	      与卡里其他行的 8u 缩进是两件事，不许混成一条；
//	142-4 满态的下沿：滚到底时卡的下沿仍在（不被软边吃掉），且最后 edge 行**逐值
//	      等于页面的面**（软边不显形，§6.141 的同一条规矩在这一页上复核一次）。
func TestTheFullCommandListWearsTheCardsInset(t *testing.T) {
	const commands = 24
	const pitch = 129.0 // 实测（u=4：板高 121 + 卡的 Space(2) 8），深浅逐值相同
	const gutter = 8    // 卡自己的 8u 缩进（`rowInset`），与卡里其他每一行同一条竖线
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			a := commandFullApp(t, appearance, commands)
			tt, tokens := renderIn(t, a, 720, 900, dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			t.Logf("MaxY=%.2f", a.Body.MaxY)
			if a.Body.MaxY <= 0 {
				t.Fatalf("24 commands do not overflow this frame (MaxY = %v): there is no full state to "+
					"measure, so every judgement below would pass on a list that never scrolls", a.Body.MaxY)
			}

			// 142-1 板的节奏：名字的 y 就是板的顶（同一列，同一行高）。
			var tops []float32
			for i := 0; i < commands; i++ {
				r, ok := tt.Find(fmt.Sprintf("cmd-%02d", i+1))
				if !ok {
					t.Fatalf("command %d is missing from the list: %q", i+1, tt.Texts())
				}
				tops = append(tops, r.Y)
			}
			for i := 1; i < len(tops); i++ {
				if d := tops[i] - tops[i-1]; d < pitch-0.01 || d > pitch+0.01 {
					t.Errorf("plate %d sits %.2f below plate %d, want %.1f: the list's rhythm is the plate's "+
						"own height plus the card's Space(2), not a remainder", i+1, d, i, pitch)
				}
			}

			first, _ := tt.Find("cmd-01")
			mid := int(first.Y + 20)
			face := pixelAt(tt, 180, mid) // 卡外面的页面地/面
			// 142-2 卡的左边框 → 卡的面 → 板的边框。
			cardL := firstColumnOtherThan(tt, mid, 174, 300, face)
			// 卡的面的颜色从**这张卡自己的标题行**取（那一行没有板），不在命令块里取：
			// 缺陷态下 cardL+1 就是板的边框，取它会把「沟」量成 1。
			titleRow, ok := tt.Find("V Tools")
			if !ok {
				t.Fatalf("the integration card's title is missing: %q", tt.Texts())
			}
			cardFace := pixelAt(tt, cardL+3, int(titleRow.Y+titleRow.H/2))
			if cardFace == face {
				t.Fatalf("no card plane at x=%d, y=%d: the card did not paint, so the gutter below would be "+
					"measured against the page itself", cardL+1, mid)
			}
			gapL := 0
			for x := cardL + 1; x < 320; x++ {
				if pixelAt(tt, x, mid) != cardFace {
					break
				}
				gapL++
			}
			if gapL != gutter {
				t.Errorf("the card's plane runs %dpx between its own border and the command plate's border, "+
					"want %d: the plate must wear the card's 8u inset like every other row in this card "+
					"(measured left edge x=%d → plate starts at x=%d)", gapL, gutter, cardL, cardL+1+gapL)
			}
			// 同一行，从右侧对称地走一遍。
			cardR := lastColumnOtherThan(tt, mid, 718, 500, face)
			gapR := 0
			for x := cardR - 1; x > 320; x-- {
				if pixelAt(tt, x, mid) != cardFace {
					break
				}
				gapR++
			}
			if gapR != gapL {
				t.Errorf("the plate's gutters are %dpx on the left and %dpx on the right: the block is not "+
					"centred in the card's own inset", gapL, gapR)
			}
			// 缺陷的签名：板与卡的边框不许**挨着**（0 缩进 = 两行 1px 线合成一条双线）。
			if pixelAt(tt, cardL+1, mid) != cardFace {
				t.Errorf("the plate's border sits directly on the card's border (x=%d and x=%d): in a long "+
					"list that is one doubled frame running the whole height of the card", cardL, cardL+1)
			}

			// 142-3 板自己的墨列。
			plateL := cardL + 1 + gapL
			if got, want := first.X-float32(plateL), float32(12.8); got < want-0.2 || got > want+0.2 {
				t.Errorf("the command's name starts %.2f right of the plate's border, want %.1f (the plate's "+
					"own 12px padding + its 1px edge)", got, want)
			}

			// 142-1（下半）板与板之间是卡的面，不是板自己的边贴边。
			top := int(tops[0])
			next := int(tops[1])
			runs := 0
			for y := top + 60; y < next; y++ {
				if pixelAt(tt, int(first.X), y) == cardFace {
					runs++
				}
			}
			if runs != gutter {
				t.Errorf("%d rows of the card's plane between two plates, want %d: the plates are separated "+
					"by the card's own Space(2), not by their borders touching", runs, gutter)
			}

			// 142-4 满态的下沿。
			a.Body.Y = a.Body.MaxY
			tt.Frame()
			tt.Frame()
			col := int(first.X)
			edge := int(tokens.Theme.Space(4))
			if edge < 4 {
				t.Fatalf("the scroll edge is %dpx tall: too short to read", edge)
			}
			pageFace := pixelAt(tt, col, 899-edge)
			for y := 900 - edge; y < 900; y++ {
				if got := pixelAt(tt, col, y); got != pageFace {
					t.Errorf("row y=%d of the bottom soft edge reads %v against the page's own plane %v: the "+
						"edge is visible on a frame that has nothing left to fade", y, got, pageFace)
					break
				}
			}
			// 卡的下沿必须还在带子之上：从底往上找最后一行离面 ≥6 级的像素。
			found := false
			for y := 899 - edge; y > 700; y-- {
				px := pixelAt(tt, col, y)
				if lumaGap(px, pageFace) >= 6 {
					found = true
					if 900-y < edge+2 {
						t.Errorf("the card's bottom edge lands at y=%d, only %d rows above the viewport's last "+
							"row: the soft edge eats the card's own frame", y, 900-y)
					}
					break
				}
			}
			if !found {
				t.Errorf("no card edge in the last 200 rows at x=%d: the bottom of the full list lost its frame",
					col)
			}
		})
	}
}

// commandFullApp is the Integrations page holding one integration with n
// declared commands — the full state this round measures.
func commandFullApp(t *testing.T, appearance string, n int) *App {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.Integrations = func() []Integration {
		list := make([]Command, 0, n)
		for i := 0; i < n; i++ {
			list = append(list, Command{
				ID:          fmt.Sprintf("c%02d", i+1),
				Name:        fmt.Sprintf("cmd-%02d", i+1),
				Description: "does a thing",
				Aliases:     []string{fmt.Sprintf("c%02d", i+1)},
				Enabled:     true,
				Available:   true,
				Declared:    true,
			})
		}
		return []Integration{{
			ID: "ext", Name: "V Tools", Enabled: true, Running: true,
			DistributionSource: "local", RuntimeOwnership: "system",
			Commands: list,
		}}
	}
	a.selectPage(PageIntegrations)
	return a
}

// lumaGap is how many levels apart two planes read on the light ladder.
func lumaGap(a, b ui.Color) int {
	d := luma(a.R, a.G, a.B) - luma(b.R, b.G, b.B)
	if d < 0 {
		return -d
	}
	return d
}

// firstColumnOtherThan is the first column in [from, to) whose pixel differs
// from want — used to find a box's own edge without hardcoding its x.
func firstColumnOtherThan(tt *ui.Tester, y, from, to int, want ui.Color) int {
	for x := from; x < to; x++ {
		if pixelAt(tt, x, y) != want {
			return x
		}
	}
	return -1
}

// lastColumnOtherThan is firstColumnOtherThan walking left.
func lastColumnOtherThan(tt *ui.Tester, y, from, to int, want ui.Color) int {
	for x := from; x > to; x-- {
		if pixelAt(tt, x, y) != want {
			return x
		}
	}
	return -1
}

// TestDumpP13FWCommandFull is the opt-in frame dump: light and dark, resting and
// scrolled to the bottom of a 24-command list.
//
//	FLOTER_DUMP_P13FW=1 go test -run TestDumpP13FWCommandFull ./internal/settingsui/
func TestDumpP13FWCommandFull(t *testing.T) {
	if os.Getenv("FLOTER_DUMP_P13FW") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_DUMP_P13FW=1")
	}
	// FLOTER_DUMP_P13FW_TAG names the pair: run it once with the fix reverted and
	// once with it in place to get before/after.
	tag := os.Getenv("FLOTER_DUMP_P13FW_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := commandFullApp(t, appearance, 24)
		tt, _ := renderIn(t, a, 720, 900, dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		dir := "/tmp/p13fw"
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, frac := range []float32{0, 1} {
			a.Body.Y = a.Body.MaxY * frac
			tt.Frame()
			tt.Frame()
			name := filepath.Join(dir, fmt.Sprintf("floter-P13-fw-commands-%s-%s-y%02d.png",
				appearance, tag, int(frac*100)))
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
	}
}
