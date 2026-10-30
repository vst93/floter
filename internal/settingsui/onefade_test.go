package settingsui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-u · 设置页剩下的四处「两道淡」：一道淡收尾。
//
// P13-s 定过胶囊（`extensionActionFace` / `busyActionButton`），P13-t 定过槽与
// 开关（配置表单）。同一页上还有四处把**产品自己的 `Opacity`** 叠在框架给
// `Disabled` 子树的 0.5 之上：
//
//	disabledSwitch（开关）          0.6 × 0.5 = 0.30
//	sessionIconButton（会话小格）    0.4 × 0.5 = 0.20
//	uninstallActionButton（在飞）    0.4 × 0.5 = 0.20
//	barButton（确认带）              0.4 × 0.5 = 0.20
//
// 实测（浅色地面 247 / 深色 14）：
//
//	开关轨道   静息 (6,74,166) / (143,183,254)  → 改前 (174,194,223) / (53,65,88)  → 改后 (126,160,207) / (79,99,136)
//	会话小格   墨对比 静息 351 / 363             → 改前 71 / 73                  → 改后 177 / 183
//	在飞胶囊   面 243（−4）/ 19（+5）            → 改后 238（−9）/ 26（+12），底对侧 231 / 23
//	确认带胶囊 面 243（−4）/ 19（+5），底 243    → 改后 238（−9）/ 26（+12），底 231 / 23
//
// 判据沿用 P13-t 的算术：禁用的每一个读数都必须正好是「静息与地面各一半」的
// 混合；面类控件再叠一条「顶内线不亮、底内线还是对侧」（P13-s 的判据）。

// oneFadeReadings is the four pixels the pins read off one control. `shade` is
// the darkest of the control's **last four rows**, because a 21.6u pill puts
// its 1px counter-side on the bottom row while a 27u one puts it two rows up.
type oneFadeReadings struct {
	face, crown, shade, ink ui.Color
	inkContrast             int
}

// oneFadeProbe renders one control through the product's own builder and reads
// the four pixels. The builder is handed the appearance so a caller can build
// the resting and the disabled face from the same code path.
func oneFadeProbe(t *testing.T, dark bool, build func(a *App, c *ui.Context) ui.Element) (ui.Rect, oneFadeReadings) {
	t.Helper()
	ground := realPageGround(t, dark)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	var box ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(24).Width(320).Children(func() {
			build(a, c).Bind(&ref)
			box = c.Resolve(ref).Bounds()
		})
	}, 400, 160)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()
	if box.W < 20 || box.H < 12 {
		t.Fatalf("measured a %v×%v box, not a control", box.W, box.H)
	}
	x := int(box.X + box.W/2)
	r := oneFadeReadings{face: imagePixel(img, x, int(box.Y)+4)}
	// The crown is the **inner** row: row 0 is the control's own `Border(1, …)`,
	// which the state does not own (on the dark sheet it is a light hairline).
	r.crown = imagePixel(img, x, int(box.Y)+1)
	for dy := 1; dy <= 2; dy++ {
		if q := imagePixel(img, x, int(box.Y)+dy); litSum(q) > litSum(r.crown) {
			r.crown = q
		}
	}
	r.shade = r.face
	for dy := -1; dy >= -4; dy-- {
		if q := imagePixel(img, x, int(box.Y+box.H)+dy); litSum(q) < litSum(r.shade) {
			r.shade = q
		}
	}
	lo, hi := 1<<30, -1
	var loPx, hiPx ui.Color
	for y := int(box.Y) + 3; y < int(box.Y+box.H)-3; y++ {
		for xx := int(box.X) + 4; xx < int(box.X+box.W)-4; xx++ {
			q := imagePixel(img, xx, y)
			if s := litSum(q); s < lo {
				lo, loPx = s, q
			}
			if s := litSum(q); s > hi {
				hi, hiPx = s, q
			}
		}
	}
	if hi-litSum(r.face) > litSum(r.face)-lo {
		r.ink, r.inkContrast = hiPx, hi-litSum(r.face)
	} else {
		r.ink, r.inkContrast = loPx, litSum(r.face)-lo
	}
	return box, r
}

// A. The settings page's disabled switch: the groove keeps half its fill step.
// A switch's planes are read off the clear third — the knob and its shadow sit
// over the middle of the track.
func TestTheDisabledSwitchFadesOnce(t *testing.T) {
	for _, dark := range []bool{false, true} {
		ground, rest, off := lockProbeAt(t, dark, 0.25, func(a *App, c *ui.Context, isOff bool) ui.Element {
			on := true
			sw := a.switchControl(c, &on)
			if isOff {
				return disabledSwitch(sw)
			}
			return sw
		})
		assertOneFade(t, "the settings switch", dark, ground, rest, off, false)
	}
}

// B. The sessions row's disabled icon button: a **transparent** face, so there
// is no material to dim (P13-p's reverse pin keeps a lit edge off it) — the
// only thing the state may do is one fade. The bound is the enabled arm: a
// disabled glyph must land at half of it, which catches a second fade (0.2
// measured 71 against 351) and a missing one alike.
func TestTheDisabledSessionIconFadesOnce(t *testing.T) {
	for _, dark := range []bool{false, true} {
		ground := realPageGround(t, dark)
		read := func(disabled bool) (ui.Rect, oneFadeReadings) {
			return oneFadeProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
				th := c.Theme()
				w := ui.Box(c).Width(th.Space(7)).Height(th.Space(7))
				w.Children(func() {
					a.sessionIconButton(c, "play", "Resume", false, false, disabled, func() {})
				})
				return w
			})
		}
		box, on := read(false)
		_, off := read(true)
		if box.W < 20 {
			t.Fatalf("measured a %v box, not the 28u icon button", box.W)
		}
		// The face is the ground: a transparent control must not grow a plane.
		if litSum(off.face) != litSum(ground) {
			t.Errorf("the disabled session icon (%s): the transparent button grew a face %v over the ground %v",
				appearanceName(dark), off.face, ground)
		}
		// No lit edge either (P13-p: a transparent face has no 1px line).
		if over := litSum(off.crown) - litSum(off.face); over > 2 {
			t.Errorf("the disabled session icon (%s): it wears a lit edge %v over its transparent face (%d levels)",
				appearanceName(dark), off.crown, over/3)
		}
		// One fade: half of the enabled ink, ±20 of a three-channel sum.
		want := on.inkContrast / 2
		if d := absDiff(want, off.inkContrast); d > 20 {
			t.Errorf("the disabled session icon (%s): the glyph is %d levels off the ground, but one fade of the enabled %d is %d (off by %d); the control is faded twice — or not at all",
				appearanceName(dark), off.inkContrast/3, on.inkContrast/3, want/3, d/3)
		}
	}
}

// C. The uninstall dialog's in-flight pill: a raised pane, so it wears the same
// material as every other disabled pill on the page — `DimFace` (counter-side
// in, crown out) — and the same one fade.
func TestTheBusyUninstallPillWearsTheDimFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		box, r := oneFadeProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return a.uninstallActionButton(c, "Uninstall", true, true, nil)
		})
		assertDimPane(t, "the in-flight uninstall pill", dark, box, r)
	}
}

// D. The discard band's disabled pill: 21.6u tall, same material, same fade.
// Its counter-side sits on the bottom row (the taller pills put it two rows
// up), which is why `oneFadeProbe` reads the last four.
func TestTheDisabledBandPillWearsTheDimFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		box, r := oneFadeProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return a.barButton(c, "Discard", false, true, nil, nil)
		})
		assertDimPane(t, "the discard band's disabled pill", dark, box, r)
	}
}

// assertDimPane is P13-s's verdict, re-read with this file's probe: the face
// still steps off the ground (half a step, so it reads as disabled rather than
// resting), the top inner row is not a lit edge, and the bottom one still
// shades.
func assertDimPane(t *testing.T, what string, dark bool, box ui.Rect, r oneFadeReadings) {
	t.Helper()
	ground := realPageGround(t, dark)
	const level = 3
	step := litSum(r.face) - litSum(ground)
	if dark && step < 6*level {
		t.Errorf("%s (dark): the disabled face is only %d levels off the ground (%v vs %v); want a face, not a ghost",
			what, step/level, r.face, ground)
	}
	if !dark && -step < 6*level {
		t.Errorf("%s (light): the disabled face is only %d levels under the ground (%v vs %v); want a face, not a ghost",
			what, -step/level, r.face, ground)
	}
	if step < 0 {
		step = -step
	}
	if step > 15*level {
		t.Errorf("%s (%s): the disabled face steps %d levels off the ground — that is a resting face, not a disabled one",
			what, appearanceName(dark), step/level)
	}
	if over := litSum(r.crown) - litSum(r.face); over > 2 {
		t.Errorf("%s (%s): the disabled pill kept a lit edge: top inner row %v is %d levels over the face %v",
			what, appearanceName(dark), r.crown, over/level, r.face)
	}
	want := 5 * level
	if dark {
		want = 2 * level
	}
	if drop := litSum(r.face) - litSum(r.shade); drop < want {
		t.Errorf("%s (%s): the disabled pill lost its counter-side: bottom inner row %v is only %d levels under the face %v",
			what, appearanceName(dark), r.shade, drop/level, r.face)
	}
}

// The opt-in dump: the four controls, resting over disabled, both appearances.
// Set FLOTER_PNG_TAG=before while the fix is reverted to shoot the other half.
func TestDumpP13UOneFade(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		ground := realPageGround(t, dark)
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(24).Width(360).Gap(10).Children(func() {
				on := true
				a.switchControl(c, &on)
				disabledSwitch(a.switchControl(c, &on))
				w := ui.Box(c).Width(c.Theme().Space(7)).Height(c.Theme().Space(7))
				w.Children(func() {
					a.sessionIconButton(c, "play", "Resume", false, false, false, func() {})
				})
				w2 := ui.Box(c).Width(c.Theme().Space(7)).Height(c.Theme().Space(7))
				w2.Children(func() {
					a.sessionIconButton(c, "play", "Resume", false, false, true, func() {})
				})
				a.uninstallActionButton(c, "Uninstall", true, false, nil)
				a.uninstallActionButton(c, "Uninstall", true, true, nil)
				a.barButton(c, "Discard", false, false, nil, nil)
				a.barButton(c, "Discard", false, true, nil, nil)
			})
		}, 400, 320)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		name := filepath.Join("/tmp", "floter-P13-u-onefade-"+appearanceName(dark)+"-"+tag+".png")
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
