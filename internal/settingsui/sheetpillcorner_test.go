package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// P13-eo（`docs/liquid-glass.md` §6.110）：浮层的动作胶囊的四角是**胶囊的半高**，
// 不是 8px 的圆角矩形。
//
// 改前三处证据都写着「胶囊」，而像素给的是圆角矩形：参考
// `main:src/styles/plugin-config.css:241` 是 `border-radius: 999px`；
// `pluginsheet.go` 那段注释写的是「a `999px` corner (half the 28u height, so the shape
// is a pill)」；守它的 pin 那句是「want a pill's wide curve」——阈值 5、读数 5，卡着边过。
//
// 量法：两臂（中性 / 危险）各自单独渲染在黑底上（危险的发丝是暖墨、中性的冠线是白墨，
// 两者在黑底上都读得到），取**第一行有墨的行**，读它离盒左沿的内缩。圆角矩形（半径 8）
// 在那行读 **5**，胶囊（半高 12.6）读 **10**。
//
// 判据三条：① 首行内缩 ≥ 8（圆角矩形过不去）；② 内缩必须逐行**收小**——那是曲线，
// 不是削角；③ 盒本身一个字节不动（高 28u、标签两端各 10u）：半径不进布局。
func TestThePluginSheetActionPillCutsAStadiumsCorner(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			store := newStore(t)
			a := New(store, Actions{})
			tokens := theme.For(store.Snapshot(), dark)
			type arm struct {
				name   string
				danger bool
				box    ui.Rect
			}
			var arms []arm
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tokens.Theme)
				c.Root().Background(ui.RGB(0, 0, 0))
				ui.Column(c).Padding(40).Gap(tokens.Theme.Space(5)).Children(func() {
					var neutral, danger ui.Handle
					ui.Row(c).Children(func() {
						a.sheetActionButton(c, "Clear history", false, func() {}).Bind(&neutral)
					})
					ui.Row(c).Children(func() {
						a.sheetActionButton(c, "Delete", true, func() {}).Bind(&danger)
					})
					arms = []arm{
						{"neutral", false, c.Resolve(neutral).Bounds()},
						{"danger", true, c.Resolve(danger).Bounds()},
					}
				})
			}, 400, 240)
			tt.SetDark(dark)
			tt.Frame()

			for _, ar := range arms {
				// The first row that carries ink. A neutral pill's plane is a
				// translucent dark, so its very first row is empty on a black
				// root; the danger arm's warm hairline starts on row 0. The pin
				// reads whichever row the paint starts on rather than a constant.
				first, inset := -1, float32(-1)
				for row := 0; row < 4 && first < 0; row++ {
					for x := int(ar.box.X) - 2; x < int(ar.box.X+ar.box.W); x++ {
						if sample(tt, float32(x), ar.box.Y+float32(row)+0.5).R > 30 {
							first, inset = row, float32(x)-ar.box.X
							break
						}
					}
				}
				if first < 0 {
					t.Fatalf("[%s] the pill's first four rows draw no ink at all", ar.name)
				}
				t.Logf("%s pill box %v, first inked row %d, corner inset %v", ar.name, ar.box, first, inset)
				// ① 胶囊的曲线：圆角矩形（本轮退掉的 RadiusSM(8)）在这一行读 5。
				if inset < 8 {
					t.Errorf("[%s] the pill's first inked row starts %v px in: that is the 8px rounded "+
						"rectangle this round retires (it read 5), not a stadium's half-height curve "+
						"(want ≥8)", ar.name, inset)
				}
				// ② 曲线而不是削角：内缩必须逐行收小。
				prev := inset
				for row := first + 1; row < first+4; row++ {
					next := float32(-1)
					for x := int(ar.box.X) - 2; x < int(ar.box.X+ar.box.W); x++ {
						if sample(tt, float32(x), ar.box.Y+float32(row)+0.5).R > 30 {
							next = float32(x) - ar.box.X
							break
						}
					}
					if next < 0 {
						break
					}
					if next > prev {
						t.Errorf("[%s] the corner inset grew from %v to %v on row %d: that is not a "+
							"curve", ar.name, prev, next, row)
					}
					prev = next
				}
				// ③ 半径不进布局：盒高与标签两端的内缩一个字节不动。
				if want := tokens.Theme.Space(7); absFloat(ar.box.H-want) > 0.5 {
					t.Errorf("[%s] the pill's box height = %v, want the reference's 28u (%v): the corner "+
						"is ink, it must not move the box", ar.name, ar.box.H, want)
				}
				label := "Clear history"
				if ar.danger {
					label = "Delete"
				}
				text, ok := tt.Find(label)
				if !ok {
					t.Fatalf("[%s] the pill's label %q is missing: %q", ar.name, label, tt.Texts())
				}
				pad := tokens.Theme.Space(2.5)
				if lead := text.X - ar.box.X; absFloat(lead-pad) > 2 {
					t.Errorf("[%s] the label's leading inset = %v, want the reference's 10u (%v)", ar.name, lead, pad)
				}
				if trail := ar.box.X + ar.box.W - (text.X + text.W); absFloat(trail-pad) > 2 {
					t.Errorf("[%s] the label's trailing inset = %v, want the reference's 10u (%v)", ar.name, trail, pad)
				}
			}
		})
	}
}

// P13-eo 出图：`FLOTER_PNG=1` 才跑。
func TestDumpP13EOSheetPillCorner(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: set FLOTER_PNG=1 to dump the sheet's action pills")
	}
	tag := os.Getenv("FLOTER_P13EO_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := newStore(t)
		a := New(store, Actions{})
		tokens := theme.For(store.Snapshot(), dark)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(ui.RGB(0, 0, 0))
			ui.Column(c).Padding(40).Gap(tokens.Theme.Space(5)).Children(func() {
				ui.Row(c).Children(func() { a.sheetActionButton(c, "Clear history", false, func() {}) })
				ui.Row(c).Children(func() { a.sheetActionButton(c, "Delete", true, func() {}) })
			})
		}, 400, 160)
		tt.SetDark(dark)
		tt.Frame()
		path := "/tmp/floter-P13-eo-pill-" + tag + "-" + appearance + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatalf("%v", err)
		}
		f.Close()
		t.Log(path)
	}
}
