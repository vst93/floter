package settingsui

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-ab · 会话页的「结束会话」确认药丸：补齐 E1 的对侧。
//
// 这枚 28u 高的暖色药丸（`.session-manager__kill-confirm`）此前只有**顶**受光边
// ——`pill.InsetShadow(0, 1, 0, 0, tokens.LensRim)`——没有对侧，与 P13-aa 修掉的电源
// execute 键是同一种「半对」：一块只能看见光、看不见影的面。
//
// 为什么这块**可以有影**：它的身份是参考的 `--glass-raised-shadow`（`ShadowContact`
// 那一笔接触影），所以这里不用 `lensAssertE1`（那条尺子写着「E1 外面不许有影」，
// §2 判据 2 管的是中性 E1）。这里只量这一对线本身，两套外观 × 两个状态：
//
//   - 冠（`box.Y+1..Y+2`，在 1px 警告边之内）必须**亮于**面；
//   - 对侧（`box.Y+H-3..H-2`）必须**暗于**面；
//   - 悬停臂只换面（`IconSurfaceHover`），所以四个状态里这一对线都成立。
//
// 出图：`FLOTER_PNG=1` 写 `/tmp/floter-P13-ab-killconfirm-<appearance>-<rest|hover>.png`。

const killConfirmLabel = "End session"

// killConfirmProbe renders the pill alone and holds the pointer over it (`hi`) or away
// from it. The probe's control sits at (20,20); the pointer goes to (35,35).
func killConfirmProbe(t *testing.T, dark, hover bool) (image.Image, ui.Rect, ui.Color) {
	t.Helper()
	var ref ui.Handle
	var box ui.Rect
	tt, ground := lensProbe(t, dark, 240, 120, func(c *ui.Context, a *App) {
		ui.Column(c).Bind(&ref).Children(func() {
			a.sessionKillConfirm(c, killConfirmLabel, false, func() {})
		})
		box = c.Resolve(ref).Bounds()
	})
	if hover {
		tt.Move(35, 35)
	}
	tt.Frame()
	tt.Advance(300_000_000) // the family's 120ms colour transition, settled
	tt.Frame()
	if box.W < 40 || box.H < 20 || box.H > 34 {
		t.Fatalf("the resolved box %+v is not the pill's 28u pane", box)
	}
	return tt.Image(), box, ground
}

// killConfirmBands reads the pill's four rows: the face (four units under its top
// frame, above the glyph and the word), the crown band, and the counter-side band.
func killConfirmBands(img *image.RGBA, box ui.Rect) (face, crown, shade ui.Color) {
	x0, x1 := int(box.X+box.W*0.25), int(box.X+box.W*0.75)
	face = lensMean(img, x0, int(box.Y)+5, x1, int(box.Y)+5)
	crown = lensMean(img, x0, int(box.Y)+1, x1, int(box.Y)+2)
	shade = lensMean(img, x0, int(box.Y+box.H)-3, x1, int(box.Y+box.H)-2)
	return face, crown, shade
}

func TestTheKillConfirmPillSitsOnAnE1Pair(t *testing.T) {
	for _, appearance := range appearanceNames {
		for _, hover := range []bool{false, true} {
			state := "rest"
			if hover {
				state = "hover"
			}
			t.Run(appearance.name+"/"+state, func(t *testing.T) {
				img, box, _ := killConfirmProbe(t, appearance.dark, hover)
				face, crown, shade := killConfirmBands(img.(*image.RGBA), box)
				t.Logf("[%s/%s] face %v crown %v shade %v (box %+v)", appearance.name, state, face, crown, shade, box)
				if !liteBrighter(crown, face) {
					t.Errorf("[%s/%s] the rows inside its top frame are %v, not brighter than its face %v — "+
						"the pill lost its lit edge (liquid-glass.md §3)", appearance.name, state, crown, face)
				}
				if !liteDarker(shade, face) {
					t.Errorf("[%s/%s] the rows inside its bottom frame are %v, not darker than its face %v — "+
						"E1 is a pair (顶亮一分、底暗一分); this pill used to carry only the lit half",
						appearance.name, state, shade, face)
				}
			})
		}
	}
}

// TestTheKillConfirmPillLiftsItsFaceUnderThePointer is the second half: the hover arm
// must still change the **face** (the reference's `--glass-raised-warm-hover`), so
// completing the pair did not turn the pointer feedback into a no-op. The direction is
// not pinned (the light appearance lifts, the dark one lifts too but by a different
// amount); only "one step" is.
func TestTheKillConfirmPillLiftsItsFaceUnderThePointer(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			restImg, restBox, _ := killConfirmProbe(t, appearance.dark, false)
			hoverImg, hoverBox, _ := killConfirmProbe(t, appearance.dark, true)
			rest, _, _ := killConfirmBands(restImg.(*image.RGBA), restBox)
			hover, _, _ := killConfirmBands(hoverImg.(*image.RGBA), hoverBox)
			if channelDelta(rest, hover) < 2 {
				t.Errorf("[%s] the pill's face is %v at rest and %v under the pointer: the hover arm paints "+
					"no face of its own", appearance.name, rest, hover)
			}
		})
	}
}

// TestDumpKillConfirmFrames writes the round's visual evidence. Opt-in: the suite must
// not write files unless asked.
func TestDumpKillConfirmFrames(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-ab-killconfirm-<appearance>-<rest|hover>.png")
	}
	for _, appearance := range appearanceNames {
		for _, hover := range []bool{false, true} {
			state := "rest"
			if hover {
				state = "hover"
			}
			img, box, _ := killConfirmProbe(t, appearance.dark, hover)
			path := "/tmp/floter-P13-ab-killconfirm-" + appearance.name + "-" + state + ".png"
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s (box %+v)", path, box)
		}
	}
}
