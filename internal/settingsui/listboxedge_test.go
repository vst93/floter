package settingsui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-av · 多选列表箱的 56px 视口：两端不该是平刀切。
//
// 配置表单的多选箱（`config.go` 的 `configMultiselectBox`）是「滚动收口」这一族
// 最后一块面：66px 的原生地板减去 4px padding 与两条 1px 发丝，滚动区只剩 56px，
// 而一行选项是 20px 盒子 + 2px 缝——六项以上的列表必然有一行被切在边上，两端什么
// 都没有（`.Clip()` 一裁到底）。
//
// 三条 pin：
//  ① 滚在中间时视口两头退让到最内那一带的一小部分；
//  ② 边按滚动位置开关（停在开头 / 滚到底时正在读的那行不许被淡）；
//  ③ 盒子零改动（66px 地板与滚动行程不动，边只是画上去的两条）。

// listBoxOptions is n option words, a/b/c… so a probe can read them back.
func listBoxOptions(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, "option-"+string(rune('a'+i)))
	}
	return out
}

// listBoxProbe draws one live multiselect box on the stack it really sits on — the
// window ground, the settings body's `Sunken` pane and the integration card's
// `Control` — with its tick rows inside, and returns the tester plus the box's
// own frame. The stack matters twice: the box's face is a composite, and the
// probe's ink readings have to be judged against the face the page really shows.
func listBoxProbe(t *testing.T, dark bool, options []string, scroll *ui.ScrollState) (*ui.Tester, ui.Rect) {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
	a := New(store, Actions{})
	var ref ui.Handle
	var box ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		t := a.cardTheme(c)
		ui.Column(c).FillWidth().FillHeight().Background(t.Background).Padding(20).
			AlignItems(ui.Start).Justify(ui.Start).Children(func() {
			ui.Column(c).FillWidth().Padding(t.Space(4)).Background(tokens.Sunken).Children(func() {
				ui.Column(c).FillWidth().Padding(t.Space(4)).Background(tokens.Control).Children(func() {
					a.extensionMetrics(func() {
						a.configMultiselectBox(c, scroll, false, func() {
							for _, option := range options {
								on, changed := false, false
								a.configOptionRow(c, option, &on, &changed, false)
							}
						}).Bind(&ref)
					})
				})
			})
		})
		box = c.Resolve(ref).Bounds()
	}, 420, 260)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if box.W < 300 || box.H < 50 {
		t.Fatalf("the resolved box %+v is not the multiselect field's 66u pane", box)
	}
	return tt, box
}

// listBoxViewport is the box's scrolling area on screen: its frame less the 1px
// keyline and the 4px padding the box keeps inside it.
func listBoxViewport(box ui.Rect) image.Rectangle {
	return image.Rect(int(box.X)+1+4, int(box.Y)+1+4, int(box.X+box.W)-1-4, int(box.Y+box.H)-1-4)
}

// P13-av ①：滚在中间时，视口上下两头都要退让。
//
// 两道软边都以 0.85 的底色起步、在 2u(8px) 的边高内落到 0，而边高小于一行选项
// （20px），所以最外侧 4 行的墨只该剩最内那一带的一半以下。
func TestTheMultiselectBoxFadesAtBothEdges(t *testing.T) {
	options := listBoxOptions(12)
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			var scroll ui.ScrollState
			tt, box := listBoxProbe(t, dark, options, &scroll)
			if scroll.MaxY <= 0 {
				t.Fatalf("the twelve-option list does not overflow its viewport (MaxY = %v), so nothing "+
					"is cut and the pins below would pass on a box that never cuts anything", scroll.MaxY)
			}
			vp := listBoxViewport(box)
			worstTop, worstBottom, deepest := 0, 0, 0
			for k := 1; k <= 19; k++ {
				scroll.Y = scroll.MaxY * float32(k) / 20
				tt.Frame()
				tt.Frame()
				img := tt.Image()
				if s := bandMax(img, vp.Min.Y, vp.Min.Y+4, vp.Min.X, vp.Max.X); s > worstTop {
					worstTop = s
				}
				if s := bandMax(img, vp.Max.Y-4, vp.Max.Y, vp.Min.X, vp.Max.X); s > worstBottom {
					worstBottom = s
				}
				if s := bandMax(img, vp.Min.Y+20, vp.Min.Y+36, vp.Min.X, vp.Max.X); s > deepest {
					deepest = s
				}
			}
			if deepest < 60 {
				t.Fatalf("no ink inside the box in any of the sampled frames (deepest spread %d): the "+
					"ratios below would be judged against a blank box", deepest)
			}
			t.Logf("worst spread: top 4 rows %d, bottom 4 rows %d, inside %d", worstTop, worstBottom, deepest)
			const fadeShare = 55
			if worstTop*100 > deepest*fadeShare {
				t.Errorf("the box's first 4 rows read at %d against the inside's %d (%.0f%% of it): the "+
					"viewport is cutting the option list flat at the top — it needs the soft edge its "+
					"siblings wear (want ≤%d%%)",
					worstTop, deepest, 100*float64(worstTop)/float64(deepest), fadeShare)
			}
			if worstBottom*100 > deepest*fadeShare {
				t.Errorf("the box's last 4 rows read at %d against the inside's %d (%.0f%% of it): the "+
					"viewport cuts the list flat at the bottom too (want ≤%d%%)",
					worstBottom, deepest, 100*float64(worstBottom)/float64(deepest), fadeShare)
			}
		})
	}
}

// P13-av ②：两条边按滚动位置开，不是常亮。
//
// 这块箱子的 66px 是原生地板、padding 不许长，所以 §6.16 那条「边落在预留的空白
// 上」的自门控在这里不成立：门必须开在 `0 < Y < MaxY` 上——停在开头时第一行、滚到
// 底时最后一行，都是用户正在读的那一行。
//
// 判据是**一格滚动的 A/B**：`Y=0`（门关）对 `Y=1`（门开），两次的内容只差一个
// 像素，所以读数差只能用边来解释。不去比「开头 vs 中间」——中间的排版和开头根本不是
// 同一批行，那种比法量的是行盒，不是边。
func TestTheMultiselectEdgesAreGatedOnTheOffset(t *testing.T) {
	options := listBoxOptions(12)
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			var scroll ui.ScrollState
			tt, box := listBoxProbe(t, dark, options, &scroll)
			if scroll.MaxY < 4 {
				t.Fatalf("the option list barely overflows (MaxY = %v): a one-step A/B needs room", scroll.MaxY)
			}
			vp := listBoxViewport(box)
			read := func() (int, int) {
				tt.Frame()
				tt.Frame()
				img := tt.Image()
				return bandMax(img, vp.Min.Y, vp.Min.Y+8, vp.Min.X, vp.Max.X),
					bandMax(img, vp.Max.Y-8, vp.Max.Y, vp.Min.X, vp.Max.X)
			}
			scroll.Y = 0
			topClosed, _ := read()
			scroll.Y = 1
			topOpen, _ := read()
			scroll.Y = scroll.MaxY
			_, bottomClosed := read()
			scroll.Y = scroll.MaxY - 1
			_, bottomOpen := read()
			t.Logf("top: closed %d / open %d · bottom: closed %d / open %d",
				topClosed, topOpen, bottomClosed, bottomOpen)
			const minimum = 8
			if topOpen > topClosed-minimum {
				t.Errorf("scrolling one pixel off the start does not soften the first line (%d → %d): the top "+
					"edge is gated on the wrong side, and the line the user is reading at offset 0 gets faded",
					topClosed, topOpen)
			}
			if bottomOpen > bottomClosed-minimum {
				t.Errorf("scrolling one pixel short of the end does not soften the last line (%d → %d): the "+
					"bottom edge is gated on the wrong side", bottomClosed, bottomOpen)
			}
		})
	}
}

// P13-av ③：盒子零改动——边是画上去的两条，不是靠长高或让出 padding 换来的。
func TestTheMultiselectBoxGeometryIsUntouched(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			var short ui.ScrollState
			_, shortBox := listBoxProbe(t, dark, listBoxOptions(2), &short)
			if short.MaxY != 0 {
				t.Errorf("a two-option box scrolls (MaxY = %v): the 66px floor is meant to hold it", short.MaxY)
			}
			if delta := shortBox.H - 66; delta > 1 || delta < -1 {
				t.Errorf("a two-option box is %vpx high, want the reference's 66px floor", shortBox.H)
			}
			var long ui.ScrollState
			_, longBox := listBoxProbe(t, dark, listBoxOptions(12), &long)
			if delta := longBox.H - 66; delta > 1 || delta < -1 {
				t.Errorf("a twelve-option box is %vpx high, want the same 66px viewport (the options "+
					"scroll inside it)", longBox.H)
			}
			if long.MaxY <= 0 {
				t.Errorf("the long list has nowhere to scroll (max %v): its viewport is 56px", long.MaxY)
			}
		})
	}
}

// P13-av ④：边画的是**箱子自己的面**，不是叠在上面的一层薄罩。
//
// `Control` 是一层 0.07 的罩子，`ui.Color.Alpha` 是**乘**，所以把 `Control` 直接
// 交给 `ScrollEdge` 会得到一枚比箱子暗十几级的带子（§6.17 的同一条反直觉）。判据：
// 视口最外两行（边上 85% 的起点）与箱子里一条**空白行**的平均亮度差不超过额 3 级——
// 边只该让内容淡出，不该自己变成第三块面。
func TestTheListBoxEdgePaintsItsOwnFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			var scroll ui.ScrollState
			tt, box := listBoxProbe(t, dark, listBoxOptions(12), &scroll)
			if scroll.MaxY <= 0 {
				t.Fatalf("the option list does not overflow (MaxY = %v)", scroll.MaxY)
			}
			scroll.Y = scroll.MaxY / 2
			tt.Frame()
			tt.Frame()
			img := tt.Image()
			vp := listBoxViewport(box)
			blank := -1
			for y := vp.Min.Y + 14; y < vp.Max.Y-14; y++ {
				if spreadAt(img, y, vp.Min.X, vp.Max.X) == 0 {
					blank = y
					break
				}
			}
			if blank < 0 {
				t.Fatalf("no blank row inside the box to compare the edge against")
			}
			face := meanLuma(img, blank, blank+1, vp.Min.X, vp.Max.X)
			edge := meanLuma(img, vp.Min.Y, vp.Min.Y+2, vp.Min.X, vp.Max.X)
			t.Logf("edge %d vs the box's own face %d", edge, face)
			if d := edge - face; d > 3 || d < -3 {
				t.Errorf("the top edge reads %d against the box's own face %d (Δ=%d): the edge is laying "+
					"down its own veil instead of the face the box is painted in", edge, face, d)
			}
		})
	}
}

// TestDumpP13AVListBoxPNGs is this round's visual evidence: one image per
// appearance, the box scrolled to its middle so both edges are open.
// Opt-in (`FLOTER_PNG=1`) so the suite does not write files; `FLOTER_PNG_TAG`
// names the pair (`before` / `after`) so the round's report can show both from
// the same probe.
func TestDumpP13AVListBoxPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-av-listbox-<tag>-<appearance>.png")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		var scroll ui.ScrollState
		tt, box := listBoxProbe(t, dark, listBoxOptions(12), &scroll)
		t.Logf("box %+v viewport %+v maxY %v", box, listBoxViewport(box), scroll.MaxY)
		scroll.Y = scroll.MaxY / 2
		tt.Frame()
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P13-av-listbox-"+tag+"-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", path)
	}
}
