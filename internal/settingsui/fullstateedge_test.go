package settingsui

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// 满态（full state）：会话页 22 条会话，列表比窗口高 998u。这一页此前只量过
// 空态（P13-fj / P13-fs）与单行（P13-ea 的 56 地板），"很多行"这一态没有任何 pin。
//
// 四条判据（`docs/liquid-glass.md` §6.141）：
//
//	141-1 行距是常数：22 行、21 个间隔，逐值 69.00（深浅同值）；
//	141-2 滚到底时卡的下沿仍在带子之上（下沿不被软边吃掉）；
//	141-3 滚到底时软边**不可见**：最后 edge 行 == 脚下的面（±1）——这条是本轮修的
//	      缺陷：软边此前拿的是窗口地（浅 255），而它坐在 FaceOver(地, Sunken) = 246
//	      上，于是页尾凭空多出一条 246→252 的白雾带（不滚动时也有）；
//	141-4 软边仍然**会淡内容**：静止帧里穿过带子的那一行墨，比带子上方那一行淡。
func TestTheFullSessionsListKeepsItsRhythmAndItsEdge(t *testing.T) {
	const rows = 22
	const pitch = 67.2 // 实测（u=4 的 16.8u），深浅逐值相同
	base := time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		t.Run(appearance, func(t *testing.T) {
			store := settings.NewStore(settings.Default())
			if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{})
			a.Sessions = func() []Session {
				out := make([]Session, 0, rows)
				for i := 0; i < rows; i++ {
					out = append(out, Session{
						ID:      uint64(i + 1),
						Index:   i,
						Title:   fmt.Sprintf("zsh — floter %02d", i+1),
						Running: i%3 != 0,
						Active:  i == 0,
						Note:    "Running",
						Dir:     "/home/tar/workspace/floter",
						Size:    "80×24",
						Created: base.Add(-time.Duration(i) * time.Minute),
					})
				}
				return out
			}
			a.selectPage(PageSessions)
			tt, tokens := renderIn(t, a, 720, 760, dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			tt.Frame()
			if a.Body.MaxY <= 0 {
				t.Fatalf("22 sessions do not overflow this frame (MaxY = %v): there is no full state to "+
					"measure, so every judgement below would pass on a list that never scrolls", a.Body.MaxY)
			}
			img := tt.Image()
			W, H := img.Bounds().Dx(), img.Bounds().Dy()

			// 141-1 行距。
			var tops []float32
			for i := 0; i < rows; i++ {
				r, ok := tt.Find(fmt.Sprintf("zsh — floter %02d", i+1))
				if !ok {
					t.Fatalf("row %d is missing from the list: %q", i+1, tt.Texts())
				}
				tops = append(tops, r.Y)
			}
			for i := 1; i < len(tops); i++ {
				if d := tops[i] - tops[i-1]; d < pitch-0.01 || d > pitch+0.01 {
					t.Errorf("row %d sits %.2f below row %d, want %.1f: the row rhythm is the list's own number, "+
						"not a remainder of the card", i+1, d, i, float64(pitch))
				}
			}

			// 滚到底：下沿软边落在那 6u 空白上，不许吃到卡的下沿，也不许自己显形。
			a.Body.Y = a.Body.MaxY
			tt.Frame()
			tt.Frame()
			img = tt.Image()
			edge := int(tokens.Theme.Space(4))
			if edge < 4 {
				t.Fatalf("the scroll edge resolves to %dpx, too thin to measure", edge)
			}
			plane := lumaOf(img, 350, H-edge-8)
			if plane <= 0 {
				t.Fatalf("no plane under the band to judge against")
			}
			// 141-3：带子里每一行都必须是面本身。
			// 逐值相等，不给容差：带子的填充色**就是**面本身（`FaceOver(地, Sunken)`），
			// 所以它落在空面上时必须一个字节都不改。给 ±1 会让「淡向窗口地」（浅 247 对
			// 面 246）这种一等级的错误从网眼里漏过去。
			for y := H - edge; y < H; y++ {
				if l := lumaOf(img, 350, y); l != plane {
					t.Errorf("at the end of the scroll the band's row %d reads %d against a plane of %d: the "+
						"edge is painting its own colour over the page's bottom padding (want the plane, exactly)",
						y, l, plane)
					break
				}
			}
			// 141-2：卡的下沿仍在带子之上。列 x=660 落在卡的填充里、避开文字。
			bottom := -1
			for y := H - edge - 1; y > H-edge-64; y-- {
				if l := lumaOf(img, 660, y); l < plane-6 || l > plane+6 {
					bottom = y
					break
				}
			}
			if bottom < 0 {
				t.Errorf("the card's bottom edge is not visible above the band (no row off the plane %d by 6 "+
					"levels within 64px above it): a full list's last row runs into the window's edge", plane)
			} else if gap := H - edge - bottom; gap < 2 {
				t.Errorf("the card's bottom edge sits %dpx above the band, want it clear of it: the soft edge "+
					"is eating the card's own frame", gap)
			}

			// 141-4：软边仍在淡内容。同一段墨，落在带子里 vs 被滚出带子，前者的展开
			// 必须更小（不是「看不见」——是「淡了」）。静止帧里第 9 行的字形顶正好压在
			// 带子里，把整页再滚 60u 它就整段露出来了。
			a.Body.Y = 0
			tt.Frame()
			tt.Frame()
			img = tt.Image()
			inBand := 0
			for y := H - 4; y < H; y++ {
				if s := spreadAt(img, y, 250, W-20); s > inBand {
					inBand = s
				}
			}
			if inBand < 8 {
				t.Fatalf("no ink passes through the band's last rows (spread %d): the frame cannot judge the "+
					"fade — the list's 9th row does not reach the window's bottom edge", inBand)
			}
			a.Body.Y = 60
			tt.Frame()
			tt.Frame()
			img = tt.Image()
			free := 0
			for y := H - 64; y < H-60; y++ {
				if s := spreadAt(img, y, 250, W-20); s > free {
					free = s
				}
			}
			if free <= inBand {
				t.Errorf("the same glyphs read at %d inside the band and at %d once scrolled out of it: the "+
					"band is not fading content", inBand, free)
			}
		})
	}
}
