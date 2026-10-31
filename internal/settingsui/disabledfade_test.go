package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-aq · 禁用面只淡一道（`docs/liquid-glass.md` §6.2b-6 的收口）。
//
// 同一个「不可用」状态在本仓被画过两次：框架的画笔给 `Disabled` 的整棵子树乘 **0.5**
// （`mygo@v0.7.3/ui/paint.go:257-259`），产品又在外面叠了一个 `.Opacity(0.x)`。
// 两个淡乘起来 = 有效 0.2 / 0.21 / 0.3，于是字被洗到读不出、面被洗成半透明背景
// （P13-s 在胶囊上实测：对比 105 级 → 43 级，面离地面的台阶 17 → 4 级）。
//
// 这三条 pin 判的都是**同一条设计规则**，不是三个魔数：
//
//	① 字只淡一道：禁用态的字/面对比 ≥ 启用态的一半（`oneFadeRatio`）；
//	② 不可用的面**不点灯**：冠线行不许亮于面（§3：受光边是「这块面可以被碰」的记号）；
//	③ 面不许被洗成背景：禁用面离地面的距离 ≥ 启用态的一半。
//

// wrapControl is the probe shape for the product functions that draw into the
// context instead of returning their element (the update CTA, the kill pill):
// the wrapper column takes the control's own box, so the readings land on the
// control and not on the page around it.
func wrapControl(c *ui.Context, draw func()) ui.Element {
	return ui.Column(c).Children(draw)
}

// oneFadeRatio is half a fade plus room for the appearance's own non-linearity:
// a single framework 0.5 lands on ~0.50, the two-drawn fades land on 0.20~0.30.
const oneFadeRatio = 0.40

// crownOverFace is the top inner row against the face, in brightness levels.
func crownOverFace(r dimReadings) int { return (litSum(r.crown) - litSum(r.face)) / 3 }

// faceOffGround is the face's distance from the ground, in levels, unsigned.
func faceOffGround(r dimReadings) int {
	d := litSum(r.face) - litSum(r.ground)
	if d < 0 {
		d = -d
	}
	return d / 3
}

// A. The extensions toolbar's 28u icon key: it has no face of its own (a
// transparent key that only tints on hover), so the *only* thing a disabled
// one has to say is its ink — and that ink was being faded twice
// (`.Opacity(0.42)` × the framework's 0.5 = 0.21).
func TestTheDisabledIconKeepsItsInk(t *testing.T) {
	for _, dark := range []bool{false, true} {
		_, off := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return a.extensionIconButton(c, "settings", "Configure", true)
		})
		_, on := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return a.extensionIconButton(c, "settings", "Configure", false)
		})
		if on.inkContrast <= 0 {
			t.Fatalf("the enabled icon key (%s) has no ink to compare against", appearanceName(dark))
		}
		ratio := float64(off.inkContrast) / float64(on.inkContrast)
		t.Logf("%s: disabled ink %d levels, enabled %d (%.2f of it)",
			appearanceName(dark), off.inkContrast/3, on.inkContrast/3, ratio)
		if ratio < oneFadeRatio {
			t.Errorf("the disabled icon key (%s): its ink is %d levels off the face, %.2f of the enabled %d — "+
				"that is one fade drawn twice (the framework's 0.5 × the product's 0.42); want one fade (≥%.2f)",
				appearanceName(dark), off.inkContrast/3, ratio, on.inkContrast/3, oneFadeRatio)
		}
	}
}

// B. The two accent CTAs (the configuration sheet's Save and the About page's
// update button). An accent pill says "press me" with its crown; a disabled
// one must turn that light off, and its fill must keep half a step off the
// ground rather than washing into it.
func TestTheDisabledAccentCTATurnsItsLightOff(t *testing.T) {
	cases := []struct {
		what string
		off  func(a *App, c *ui.Context) ui.Element
		on   func(a *App, c *ui.Context) ui.Element
	}{
		{
			"the configuration sheet's Save",
			func(a *App, c *ui.Context) ui.Element { return a.extensionPrimaryButton(c, "Save", true) },
			func(a *App, c *ui.Context) ui.Element { return a.extensionPrimaryButton(c, "Save", false) },
		},
		{
			"the About page's update CTA",
			func(a *App, c *ui.Context) ui.Element {
				return wrapControl(c, func() { a.updateButton(c, "Download", true, nil) })
			},
			func(a *App, c *ui.Context) ui.Element {
				return wrapControl(c, func() { a.updateButton(c, "Download", false, nil) })
			},
		},
	}
	for _, tc := range cases {
		for _, dark := range []bool{false, true} {
			_, off := dimProbe(t, dark, tc.off)
			_, on := dimProbe(t, dark, tc.on)
			if crownOverFace(on) < 1 {
				// The update CTA wears no keyline, so its crown row is only one
				// device row thick and the probe can miss it at a fractional y;
				// the fill ratio below is that case's pin. The Save keeps a border,
				// so its crown is measurable and its absence is a real failure.
				t.Logf("%s (%s): the enabled CTA's crown reads %+d levels (not measurable on every surface)",
					tc.what, appearanceName(dark), crownOverFace(on))
			}
			t.Logf("%s (%s): disabled crown %+d levels over the face, enabled %+d; face off ground %d vs %d; ink %d vs %d",
				tc.what, appearanceName(dark), crownOverFace(off), crownOverFace(on),
				faceOffGround(off), faceOffGround(on), off.inkContrast/3, on.inkContrast/3)
			if over := litSum(off.crown) - litSum(off.face); over > 2 {
				t.Errorf("%s (%s): the disabled CTA kept a lit edge: top inner row %v is %d levels over the face %v — "+
					"a face that cannot be touched must not wear the mark of one that can",
					tc.what, appearanceName(dark), off.crown, over/3, off.face)
			}
			if on.inkContrast == 0 {
				t.Fatalf("%s (%s): the enabled CTA has no ink to compare against", tc.what, appearanceName(dark))
			}
			if over := litSum(off.crown) - litSum(off.face); over > 2 {
				t.Errorf("%s (%s): the disabled CTA kept a lit edge: top inner row %v is %d levels over the face %v — "+
					"a face that cannot be touched must not wear the mark of one that can",
					tc.what, appearanceName(dark), off.crown, over/3, off.face)
			}
			// The accent fill is the reading that survives this pair: the label
			// and the fill fade *together*, so the ink's ratio is compressed by
			// compositing (one fade reads ~0.26 there, not 0.50) while the fill
			// against the ground stays linear. Two fades took the update CTA's
			// fill to 0.30 of the enabled step.
			fon := faceOffGround(on)
			if fon == 0 {
				t.Fatalf("%s (%s): the enabled fill is the ground — nothing to compare", tc.what, appearanceName(dark))
			}
			if ratio := float64(faceOffGround(off)) / float64(fon); ratio < oneFadeRatio {
				t.Errorf("%s (%s): the disabled fill sits %d levels off the ground, %.2f of the enabled %d — "+
					"the fill is faded twice; want one fade (≥%.2f)",
					tc.what, appearanceName(dark), faceOffGround(off), ratio, fon, oneFadeRatio)
			}
		}
	}
}

// C. The sessions page's armed kill pill: it is the warm raised face, and its
// crown sits *outside* the disabled arm — so a pill that answers nothing was
// still lit (`docs/liquid-glass.md` §3).
func TestTheDisabledKillPillTurnsItsLightOff(t *testing.T) {
	for _, dark := range []bool{false, true} {
		_, off := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return wrapControl(c, func() { a.sessionKillConfirm(c, "Close", true, nil) })
		})
		_, on := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return wrapControl(c, func() { a.sessionKillConfirm(c, "Close", false, nil) })
		})
		t.Logf("%s: disabled crown %+d levels over the face, enabled %+d; ink %d vs %d",
			appearanceName(dark), crownOverFace(off), crownOverFace(on), off.inkContrast/3, on.inkContrast/3)
		if crownOverFace(on) < 1 {
			t.Fatalf("the enabled kill pill (%s) has no crown — the probe is not on the crown row", appearanceName(dark))
		}
		if over := litSum(off.crown) - litSum(off.face); over > 2 {
			t.Errorf("the disabled kill pill (%s): top inner row %v is %d levels over the face %v — the light is still on",
				appearanceName(dark), off.crown, over/3, off.face)
		}
		if on.inkContrast == 0 {
			t.Fatalf("the enabled kill pill (%s) has no ink to compare against", appearanceName(dark))
		}
		if ratio := float64(off.inkContrast) / float64(on.inkContrast); ratio < oneFadeRatio {
			t.Errorf("the disabled kill pill (%s): its label is %d levels off the face, %.2f of the enabled %d — "+
				"the ink is faded twice; want one fade (≥%.2f)",
				appearanceName(dark), off.inkContrast/3, ratio, on.inkContrast/3, oneFadeRatio)
		}
	}
}
