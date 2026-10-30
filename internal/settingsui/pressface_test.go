package settingsui

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// P13-q · 设置页动作面的第三档：按下。
//
// 这一族此前只有静息与悬停两档（`grep -n "Pressed()" internal/settingsui` 只
// 命中 6 处，而 `Hovered()` 命中 19 处），于是「点一下就执行」的面在指针按下去
// 的那一下与悬停一模一样 —— 读起来像点了没反应。契约 §5 把这一档写死为
// `--glass-control-press` 的面（沉到静息面之下）加一次几何收档；启动器的 20u/28u
// 小格早就是三档，设置页同尺寸的格只有两档。
//
// 两条像素 pin（深浅两套外观各跑一遍，全部量像素）：
//   - `TestPressedIconButtonSinksBelowItsHoverFace`：按下的面必须比悬停的面**暗**
//     （浅色 `ControlPress` 黑 0.032 vs 悬停白 0.14；深色白 0.05 vs 白 0.105）；
//   - `TestPressedIconButtonSinksItsBox`：几何收 0.92（`Scale` 是渲染变换，
//     布局盒不动 —— 这一条同时钉住「收的是画出来的盒，不是排版」）。
//
// 出图：`/tmp/floter-P13-q-iconbutton-<light|dark>-<hover|press>.png`。

// pressProbe hovers the 30u extension icon button and returns the frame plus the
// box the pointer is over. The pointer goes to the button's own centre.
func pressProbe(t *testing.T, dark bool, press bool) (*ui.Tester, image.Image, ui.Color) {
	t.Helper()
	return pressProbeWith(t, dark, press, func(c *ui.Context, a *App) {
		a.extensionIconButton(c, "copy", "Copy", false)
	})
}

// pressProbeWith is pressProbe over any control that fits in the 200x120 probe
// at (20, 20); the pointer lands on its centre.
func pressProbeWith(t *testing.T, dark, press bool, build func(c *ui.Context, a *App)) (*ui.Tester, image.Image, ui.Color) {
	t.Helper()
	tt, ground := lensProbe(t, dark, 200, 120, build)
	tt.Move(35, 35)
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	if press {
		tt.Press(35, 35)
		// The first frame dispatches the press, then the colour transition has to
		// run out (the face is `Transition{Colors: true, 120ms}` — reading the
		// frame the press landed on grades the *hover* face mid-fade), and the
		// last frame builds the view with the state it set.
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
	}
	return tt, tt.Image(), ground
}

// pressFace is the mean colour of the button's own middle row, a few units under
// its top frame so the reading grades the material and not the glyph on it.
func pressFace(img image.Image, box ui.Rect) ui.Color {
	x0, x1 := int(box.X+box.W*0.25), int(box.X+box.W*0.75)
	return lensMean(img.(*image.RGBA), x0, int(box.Y)+4, x1, int(box.Y)+4)
}

// TestPressedIconButtonSitsBetweenRestAndHover pins the third rung: with the
// pointer held down the chip must paint a face of its own, **between** the
// resting face and the hovered one — nearer the rest than the hover is, because
// the press rung is the ladder stepping back down (`ControlPress`). The
// assertion is an ordering, not a direction: on the light appearance the rung is
// a weaker black wash than the hover (0.032 against 0.105) and therefore
// *lighter*, on the dark one a weaker white wash (0.05 against 0.105) and
// therefore *darker*; both are "one step back toward rest".
//
// Mutation: drop the `case button.Pressed():` arm in `extensionIconButton` →
// the held chip reports the hovered face and the ordering collapses.
func TestPressedIconButtonSitsBetweenRestAndHover(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			read := func(press bool) ui.Color {
				_, img, ground := pressProbe(t, appearance.dark, press)
				box := paintedBox(img.(*image.RGBA), ground)
				if box.W == 0 {
					t.Fatal("the chip painted nothing")
				}
				return pressFace(img, box)
			}
			// The resting chip is transparent, so its face is the ground itself;
			// the probe's own ground is what the two washes are mixed over.
			rest := lensGround()
			hover, pressed := read(false), read(true)
			if channelDelta(hover, rest) < 4 {
				t.Fatalf("[%s] the hovered chip is %v against a %v ground: the hover arm painted no face",
					appearance.name, hover, rest)
			}
			between := func(a, mid, b ui.Color) bool {
				lo, hi := a, b
				if liteBrighter(lo, hi) {
					lo, hi = hi, lo
				}
				return liteBrighter(mid, lo) && liteBrighter(hi, mid)
			}
			if !between(rest, pressed, hover) {
				t.Errorf("[%s] the chip under the pointer is %v whether or not the button is down (rest %v, "+
					"hover %v): the press rung paints no face of its own — `--glass-control-press` is one step "+
					"back toward the resting face (liquid-glass.md §5)", appearance.name, pressed, rest, hover)
			}
		})
	}
}

// TestPressedIconButtonSinksItsBox pins the geometry half: the press shrinks the
// painted box by the family's own 0.92 (the launcher's same-size chips), while
// the layout box — the height chain — does not move. Mutation: `Scale(0.92)` →
// `Scale(1, 1)` (the face still sinks, the box does not) → this pin goes red.
func TestPressedIconButtonSinksItsBox(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			_, hoverImg, ground := pressProbe(t, appearance.dark, false)
			hoverBox := paintedBox(hoverImg.(*image.RGBA), ground)
			_, pressImg, _ := pressProbe(t, appearance.dark, true)
			pressBox := paintedBox(pressImg.(*image.RGBA), ground)
			if hoverBox.W == 0 || pressBox.W == 0 {
				t.Fatalf("a box was not painted (hover %v, press %v)", hoverBox, pressBox)
			}
			ratio := pressBox.W / hoverBox.W
			if ratio > 0.95 || ratio < 0.85 {
				t.Errorf("[%s] the pressed chip's painted box is %.2f of the hovered one (%v → %v); the family's "+
					"press rung is 0.92 (liquid-glass.md §5, the launcher's own chips)",
					appearance.name, ratio, hoverBox, pressBox)
			}
		})
	}
}

// TestDumpPressedIconButton writes the two rungs to /tmp so the round's report
// can point at a picture of the change rather than at a number.
func TestDumpPressedIconButton(t *testing.T) {
	for _, appearance := range appearanceNames {
		for _, rung := range []struct {
			name  string
			press bool
		}{{"hover", false}, {"press", true}} {
			tt, _, _ := pressProbe(t, appearance.dark, rung.press)
			path := "/tmp/floter-P13-q-iconbutton-" + appearance.name + "-" + rung.name + ".png"
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// TestPressedPillSitsBetweenRestAndHover is the same ordering for the shared
// `.extensions-action-button` pill (`extensionActionFace`), whose press rung the
// reference states as `:active:not(:disabled) { transform: scale(0.97) }`
// (`extensions.css:138-140`) and whose face is `--glass-control-press`.
//
// Mutation: drop the pressed arm in `extensionActionFace` → the held pill
// reports the hovered face and the ordering collapses.
func TestPressedPillSitsBetweenRestAndHover(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			read := func(press bool) (ui.Color, ui.Rect) {
				tt, img, ground := pressProbeWith(t, appearance.dark, press, func(c *ui.Context, a *App) {
					a.extensionActionFace(c, 28, 8, false)
				})
				_ = tt
				box := paintedBox(img.(*image.RGBA), ground)
				if box.W == 0 {
					t.Fatal("the pill painted nothing")
				}
				return pressFace(img, box), box
			}
			rest := lensGround()
			hover, hoverBox := read(false)
			pressed, pressBox := read(true)
			if channelDelta(hover, rest) < 4 {
				t.Fatalf("[%s] the hovered pill is %v against a %v ground: the hover arm painted no face",
					appearance.name, hover, rest)
			}
			lo, hi := rest, hover
			if liteBrighter(lo, hi) {
				lo, hi = hi, lo
			}
			if !(liteBrighter(pressed, lo) && liteBrighter(hi, pressed)) {
				t.Errorf("[%s] the pill under the pointer is %v whether or not the button is down (rest %v, "+
					"hover %v): the press rung paints no face of its own (liquid-glass.md §5)",
					appearance.name, pressed, rest, hover)
			}
			// The pill's own `:active` nudge is 0.97, which on a 20u-wide probe is
			// 0.6px — inside the antialiasing of its rounded ends. The face is the
			// measurable half here; the chip's pin above carries the geometry.
			if pressBox.W > hoverBox.W {
				t.Errorf("[%s] the pressed pill grew (%v → %v)", appearance.name, hoverBox, pressBox)
			}
		})
	}
}
