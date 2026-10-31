package settingsui

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-ai · 框架的灰盘按钮：设置页自己的动作胶囊戴上 E1 的一对面。
//
// `docs/liquid-glass.md` §6.8：这一族此前是四枚 `ui.Button`，框架的
// `styleButton`（`mygo/ui/widgets.go:152-186`）给的是「`Surface` 平涂 + 四边一圈
// 同值的 `stroke`」——四条边一个值（实测浅色面 119 / 四边全 108，深色面 141 /
// 四边全 151），读起来是一个**画框**而不是一块面。改后 `a.pageButton` 保持框架的
// 几何（30u 盒、6u/14u padding、9u 圆角），只换墨与面：面 `Control` 三档 +
// 边 `ControlEdge` + `LitFace` 的一对面。
//
// 两条账，深浅两套外观各跑一遍，全部量像素：
//   - `TestThePageActionWearsOneLitFace`：顶行**亮于**面、底行**暗于**面、左右列
//     **就是**面（不是四条边同值的画框）；
//   - `TestThePageActionKeepsItsThreeRungs`：静息 / 悬停 / 按下三档各自等于它自己
//     那个 token 合成出来的面（不是「大致变了」），且两档的受光边都在。
//
// 出图：`/tmp/floter-P13-ai-sessions-<light|dark>.png`（opt-in）。

// pageButtonTokens is the appearance's token set, built the same way `lensProbe`
// builds the probe's theme, so the expectations below are the real ones.
func pageButtonTokens(t *testing.T, dark bool) theme.Tokens {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	return theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
}

// pageButtonProbe renders one page button on the mid-grey probe ground, hovers it
// when asked, and holds the pointer down when asked.
func pageButtonProbe(t *testing.T, dark bool, hover, press bool) (*ui.Tester, image.RGBA, ui.Color) {
	t.Helper()
	tt, ground := lensProbe(t, dark, 220, 120, func(c *ui.Context, a *App) {
		a.pageButton(c, "New session")
	})
	if !hover {
		return tt, *tt.Image(), ground
	}
	tt.Move(45, 35)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if press {
		tt.Press(45, 35)
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
	}
	return tt, *tt.Image(), ground
}

// pageButtonRows finds the button's own box in the probe image by walking the
// column through its middle until the ground stops and starts again: the first
// row that is not ground is the frame's top, the last one is its bottom.
func pageButtonRows(img *image.RGBA, ground ui.Color, x int) (top, bottom int) {
	top, bottom = -1, -1
	for y := 0; y < img.Bounds().Dy(); y++ {
		if imagePixel(img, x, y) != ground {
			if top < 0 {
				top = y
			}
			bottom = y
		}
	}
	return top, bottom
}

// over is a source-over composite of ink at its own alpha on top of ground: the
// face a token should paint when it lands on the probe's grey.
func over(ground, ink ui.Color) ui.Color {
	a := float32(ink.A) / 255
	mix := func(g, i uint8) uint8 {
		return uint8(float32(g)*(1-a) + float32(i)*a + 0.5)
	}
	return ui.RGBA(mix(ground.R, ink.R), mix(ground.G, ink.G), mix(ground.B, ink.B), 1)
}

// levels is the largest single-channel distance of two colours, in 1x screen
// levels — the unit the contract's tables quote.
func levels(a, b ui.Color) int {
	worst := 0
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		d := int(p[0]) - int(p[1])
		if d < 0 {
			d = -d
		}
		if d > worst {
			worst = d
		}
	}
	return worst
}

// TestThePageActionWearsOneLitFace is the shape of the face: one lit line on
// top, one counter-edge at the bottom, and nothing but the face on the sides.
// The stock button failed all three at once — its four edges were one value.
func TestThePageActionWearsOneLitFace(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			_, img, ground := pageButtonProbe(t, appearance.dark, false, false)
			const cx = 60
			top, bottom := pageButtonRows(&img, ground, cx)
			if top < 0 || bottom-top < 8 {
				t.Fatalf("no button box found in the probe (top=%d bottom=%d)", top, bottom)
			}
			mid := top + (bottom-top)/2
			face := imagePixel(&img, cx, top+3)
			// `LitFace` stacks two inset shadows inside the hairline, so the lit
			// line is the row *under* the border and the counter-edge the row
			// above the bottom border — the same shape every E1 face in this
			// build wears (`extensionActionFace`).
			lit := imagePixel(&img, cx, top+1)
			counter := imagePixel(&img, cx, bottom-1)
			left := imagePixel(&img, 20, mid)
			width := 0
			for x := 20; x < img.Bounds().Dx() && imagePixel(&img, x, mid) != ground; x++ {
				width++
			}
			right := imagePixel(&img, 20+width-1, mid)
			t.Logf("%s: box y=%d..%d w=%d face=%v lit=%v counter=%v left=%v right=%v",
				appearance.name, top, bottom, width, face, lit, counter, left, right)

			// The lit line is the one thing that says "this face is above the
			// ground" (§2). Without it the box is a picture frame.
			if l := levels(lit, face); l < 3 {
				t.Errorf("the lit row is only %d levels off the face (%v vs %v): "+
					"the face's lit line is gone", l, lit, face)
			}
			if lit.R <= face.R || lit.G <= face.G {
				t.Errorf("the lit row %v is not lit against the face %v", lit, face)
			}
			// Its opposite side: one step darker, so the control stands up.
			if l := levels(counter, face); l < 2 {
				t.Errorf("the counter row is only %d levels off the face (%v vs %v): "+
					"the counter-edge is gone, the face reads flat", l, counter, face)
			}
			if counter.R >= face.R || counter.G >= face.G {
				t.Errorf("the counter row %v is not darker than the face %v", counter, face)
			}
			// The verticals are the family's own hairline — one ink on both sides —
			// and the lit row is the brightest line on the face. That pair is what
			// separates a face (§2) from the stock button's four-sided frame, where
			// all four edges were one value: here the light has a direction.
			if left != right {
				t.Errorf("the two side hairlines disagree (%v vs %v): a face has one edge ink", left, right)
			}
			if lit.R < left.R || lit.G < left.G {
				t.Errorf("the lit row %v is not the brightest edge on the face (side hairline %v): "+
					"the light has no direction", lit, left)
			}
		})
	}
}

// TestThePageActionKeepsItsThreeRungs pins the ladder the framework already had
// (rest / hover / press) to the tokens that own it, so the face cannot drift to
// a hand-picked colour, and the lit line survives the hover.
func TestThePageActionKeepsItsThreeRungs(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			tokens := pageButtonTokens(t, appearance.dark)
			read := func(hover, press bool) (face, lit, ground ui.Color) {
				_, img, g := pageButtonProbe(t, appearance.dark, hover, press)
				top, _ := pageButtonRows(&img, g, 60)
				return imagePixel(&img, 60, top+3), imagePixel(&img, 60, top+1), g
			}
			rest, restLit, ground := read(false, false)
			hover, hoverLit, _ := read(true, false)
			press, _, _ := read(true, true)
			for _, step := range []struct {
				name string
				got  ui.Color
				want ui.Color
			}{
				{"rest", rest, over(ground, tokens.Control)},
				{"hover", hover, over(ground, tokens.ControlHover)},
				{"press", press, over(ground, tokens.ControlPress)},
			} {
				if l := levels(step.got, step.want); l > 3 {
					t.Errorf("%s face = %v, want %v (%d levels off): the rung is not its token",
						step.name, step.got, step.want, l)
				}
			}
			if l := levels(restLit, rest); l < 3 {
				t.Errorf("resting: the lit line %v is %d levels off the face %v", restLit, l, rest)
			}
			if l := levels(hoverLit, hover); l < 3 {
				t.Errorf("hovered: the lit line %v is %d levels off the face %v", hoverLit, l, hover)
			}
		})
	}
}

// TestDumpP13AISessionsPNGs is the opt-in dump for the round: the sessions page
// in its empty state, where the last stock button did the most work — it is the
// only control in a 160u void.
func TestDumpP13AISessionsPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to dump the sessions empty-state screenshots")
	}
	for _, appearance := range appearanceNames {
		a := New(newStore(t), Actions{NewSession: func() {}})
		a.Page = PageSessions
		tt, _ := renderIn(t, a, 720, 580, appearance.dark)
		img := tt.Image()
		path := "/tmp/floter-P13-ai-sessions-" + appearance.name + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s: %s", appearance.name, path)
	}
}
