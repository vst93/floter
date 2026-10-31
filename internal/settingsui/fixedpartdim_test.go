package settingsui

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-ar · 卸载对话框里那一行「程序本体」：它是唯一一张**整卡**不可用的卡片
// (`uninstall.go` 的 `part.Fixed` 臂)。同一句「不可用」在这里又被画了两遍：
// 卡自己 `.Opacity(0.6)` 洗了整棵子树，里面那个勾选框还 `.Disabled(true)`
// 让框架再乘一个 0.5（`mygo@v0.7.3/ui/paint.go:257-259`）——勾选框有效 0.30。
// 而卡自己那条 `LitFace` 冠线写在 `Fixed` 臂**里**：一块按不动的面仍然点着灯
// （§3：受光边是「这块面可以被碰」的记号）。
//
// 判据与 P13-aq 的三条同源，只是对象换成一张卡：
//
//	① 面还在、光熄了：冠线行不许亮于面；
//	② 墨只淡一道：字/面对比 ≥ 启用态的一半（`oneFadeRatio`）；
//	③ 面不许被洗成背景：面离地面的台阶 ≥ 6 级（`assertDisabledFace`）。
//
// 第四条是本轮新加的（这一族唯一在**卡里**还套着禁用件的例子）：卡里的勾选框
// 也只许淡一道——它的墨/面距离不许比卡外那张可用卡的同件低过一半。

// fixedCardReadings are the four readings this round's pins use: the material
// (ground / face / crown / shade) plus the ink of the label band and of the
// checkbox inside the card.
type fixedCardReadings struct {
	ground, face, crown, shade ui.Color
	label, labelContrast       int
	box, boxContrast           int
}

// fixedCardProbe renders the uninstall dialog's component card with the fixed
// row's own arms: `fixed` picks the program row (the one that cannot be
// touched), otherwise the host-configuration row (the enabled twin, same box,
// same label band, same checkbox column).
func fixedCardProbe(t *testing.T, dark, fixed bool) (ui.Rect, fixedCardReadings) {
	t.Helper()
	ground := realPageGround(t, dark)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	on := true
	part := uninstallPart{Label: "Host configuration", Hint: "Remove the host configuration.", Value: &on}
	if fixed {
		part = uninstallPart{Label: "Program files", Hint: "The app itself, always removed.", Fixed: true}
	}
	var box ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(24).Width(360).Children(func() {
			a.uninstallPartRow(c, part).Bind(&ref)
			box = c.Resolve(ref).Bounds()
		})
	}, 440, 200)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()
	x := int(box.X + box.W/2)
	r := fixedCardReadings{ground: ground}
	r.face = imagePixel(img, x, int(box.Y)+4)
	// The 1px lit edge can straddle two device rows at a fractional y.
	r.crown = imagePixel(img, x, int(box.Y)+1)
	for dy := 1; dy <= 2; dy++ {
		if q := imagePixel(img, x, int(box.Y)+dy); litSum(q) > litSum(r.crown) {
			r.crown = q
		}
	}
	r.shade = imagePixel(img, x, int(box.Y+box.H)-2)
	for dy := -3; dy <= -2; dy++ {
		if q := imagePixel(img, x, int(box.Y+box.H)+dy); litSum(q) < litSum(r.shade) {
			r.shade = q
		}
	}
	f := litSum(r.face)
	// The label band: everything right of the checkbox's 20u column, clear of
	// the card's 1px frame and of its rounded corners.
	r.label, r.labelContrast = furthestFrom(img, f,
		int(box.X+box.W*0.35), int(box.X+box.W)-3,
		int(box.Y)+4, int(box.Y+box.H)-4)
	// The checkbox's own 20u column (`uninstall.go`: `Width(t.Space(5))`) after
	// the card's 12u padding. The label column starts at 44, so this window
	// stops at 40 and reads the box alone.
	r.box, r.boxContrast = furthestFrom(img, f,
		int(box.X)+13, int(box.X)+40,
		int(box.Y)+4, int(box.Y+box.H)-4)
	return box, r
}

// furthestFrom returns the pixel of the region farthest from the face in
// brightness, and its distance in levels: ink is darker than the face on the
// light appearance and lighter on the dark one.
func furthestFrom(img *image.RGBA, face int, x0, x1, y0, y1 int) (int, int) {
	best, bestD := 0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			q := litSum(imagePixel(img, x, y))
			d := q - face
			if d < 0 {
				d = -d
			}
			if d > bestD {
				best, bestD = q, d
			}
		}
	}
	return best, bestD
}

// A. The card that cannot be touched: its light is off, its face still steps
// off the ground, and its label is faded once rather than three times.
func TestTheFixedUninstallPartGoesUnlitNotWashed(t *testing.T) {
	for _, dark := range []bool{false, true} {
		box, off := fixedCardProbe(t, dark, true)
		_, on := fixedCardProbe(t, dark, false)
		t.Logf("%s: fixed crown %+d over the face, enabled %+d; face off ground %d vs %d; label %d vs %d",
			appearanceName(dark), crownOverFaceR(off), crownOverFaceR(on),
			faceOffGroundR(off), faceOffGroundR(on), off.labelContrast/3, on.labelContrast/3)
		if over := litSum(off.crown) - litSum(off.face); over > 2 {
			t.Errorf("the fixed component card (%s): top inner row %v is %d levels over the face %v — "+
				"a row that cannot be touched must not wear the mark of one that can",
				appearanceName(dark), off.crown, over/3, off.face)
		}
		assertDisabledFace(t, "the fixed component card", dark, box, dimReadings{
			ground: off.ground, face: off.face, crown: off.crown, shade: off.shade,
		})
		if on.labelContrast <= 0 {
			t.Fatalf("the enabled component card (%s) has no ink to compare against", appearanceName(dark))
		}
		if ratio := float64(off.labelContrast) / float64(on.labelContrast); ratio < oneFadeRatio {
			t.Errorf("the fixed component card (%s): its label is %d levels off the face, %.2f of the enabled %d — "+
				"that is one fade drawn twice; want one fade (≥%.2f)",
				appearanceName(dark), off.labelContrast/3, ratio, on.labelContrast/3, oneFadeRatio)
		}
	}
}

// B. The checkbox inside the fixed card is the one control in this family that
// sits under *both* a product fade and its own `Disabled` flag. Inheritance
// (`node.IsDisabled` walks up) is what keeps it to a single fade: the flag
// belongs to the card, and the box inside carries none.
func TestTheFixedUninstallPartsCheckboxIsFadedOnce(t *testing.T) {
	for _, dark := range []bool{false, true} {
		_, off := fixedCardProbe(t, dark, true)
		_, on := fixedCardProbe(t, dark, false)
		t.Logf("%s: box inside the fixed card reads %d levels off the face, the enabled twin's %d",
			appearanceName(dark), off.boxContrast/3, on.boxContrast/3)
		if on.boxContrast <= 0 {
			t.Fatalf("the enabled component card (%s) has no checkbox to compare against", appearanceName(dark))
		}
		if ratio := float64(off.boxContrast) / float64(on.boxContrast); ratio < oneFadeRatio {
			t.Errorf("the fixed card's checkbox (%s): %d levels off the face, %.2f of the enabled %d — "+
				"the box is faded on top of the card's own fade; want one fade (≥%.2f)",
				appearanceName(dark), off.boxContrast/3, ratio, on.boxContrast/3, oneFadeRatio)
		}
	}
}

// crownOverFaceR / faceOffGroundR repeat the two arithmetic readings on this
// file's own struct (the shared helpers in `disabledfade_test.go` take
// `dimReadings`).
func crownOverFaceR(r fixedCardReadings) int { return (litSum(r.crown) - litSum(r.face)) / 3 }

func faceOffGroundR(r fixedCardReadings) int {
	d := litSum(r.face) - litSum(r.ground)
	if d < 0 {
		d = -d
	}
	return d / 3
}
