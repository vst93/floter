package settingsui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-t · 锁定的表单：一道淡，不是三道。
//
// 这一族此前在同一张表单上画了三个深度：字段 / 开关 / 下拉是有效 **0.3**
// （框架给 `Disabled` 子树的 0.5 × 产品自己的 0.6），列表框是 **0.6**（只有产品
// 那一档，`Disabled` 忘了挂），框里的勾选框是 **0.25**（0.6 × 0.5）。三块表示的是
// 同一件事：这个控件现在不能碰。
//
// 实测（浅色地面 247 / 深色 14）：
//
//	字段面      静息 230 / 39   → 改前 242（−5）/ 22（+7）   → 改后 238（−9）/ 27（+12）
//	字段墨对比  203             → 改前 65                   → 改后 101
//	开关轨道    79 / 187        → 改前 195 / 67             → 改后 163 / 101
//	列表框面    230 / 39        → 改前 236（−11）/ 29       → 改后 238（−9）/ 27
//	选项文字    108（−121）      → 改前 160（−76）            → 改后 177（−61）
//
// 判据只有一条，就是「一道淡」的算术：**锁定的每一个读数都必须正好是
// 「静息与地面各一半」的混合**。改前是 0.3 / 0.6 / 0.25 三个不同的系数，
// 每一个都会让这条 pin 变红。

// lockReadings is the four pixels the pins read off one control: the face, the
// lit edge, the counter-side and the ink.
type lockReadings struct{ face, crown, shade, ink ui.Color }

// lockProbe renders one control twice — resting and locked — through the
// product's own builder, and returns the ground plus both readings.
func lockProbe(t *testing.T, dark bool, build func(a *App, c *ui.Context, locked bool) ui.Element) (ui.Color, lockReadings, lockReadings) {
	return lockProbeAt(t, dark, 0.5, build)
}

// lockProbeAt is lockProbe with the reading column chosen by the caller: a
// switch's knob and its shadow sit over the middle of the track, so the
// switch's planes are read off the clear third instead.
func lockProbeAt(t *testing.T, dark bool, frac float64, build func(a *App, c *ui.Context, locked bool) ui.Element) (ui.Color, lockReadings, lockReadings) {
	t.Helper()
	read := func(locked bool) lockReadings {
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
				build(a, c, locked).Bind(&ref)
				box = c.Resolve(ref).Bounds()
			})
		}, 400, 200)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		img := tt.Image()
		if box.W < 20 || box.H < 12 {
			t.Fatalf("measured a %v×%v box, not a control", box.W, box.H)
		}
		x := int(box.X + box.W*float32(frac))
		r := lockReadings{}
		r.face = imagePixel(img, x, int(box.Y)+5)
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
		// The ink is the pixel of the label band furthest from the face, in
		// either direction: the label is darker than the face on the light
		// appearance and lighter on the dark one.
		lo, hi := 1<<30, -1
		var loPx, hiPx ui.Color
		for y := int(box.Y) + 5; y < int(box.Y+box.H)-5; y++ {
			for xx := int(box.X) + 6; xx < int(box.X+box.W)-6; xx++ {
				q := imagePixel(img, xx, y)
				s := litSum(q)
				if s < lo {
					lo, loPx = s, q
				}
				if s > hi {
					hi, hiPx = s, q
				}
			}
		}
		if hi-litSum(r.face) > litSum(r.face)-lo {
			r.ink = hiPx
		} else {
			r.ink = loPx
		}
		return r
	}
	return realPageGround(t, dark), read(false), read(true)
}

// halfMix is the arithmetic of one fade: the resting colour and the ground, in
// equal parts, channel by channel (the painter composites the disabled subtree
// as a group, so this is linear in every channel).
func halfMix(rest, ground ui.Color) ui.Color {
	mix := func(a, b uint8) uint8 { return uint8((int(a) + int(b) + 1) / 2) }
	return ui.Color{R: mix(rest.R, ground.R), G: mix(rest.G, ground.G), B: mix(rest.B, ground.B), A: 255}
}

// assertOneFade is the round's verdict: every reading of the locked control is
// exactly half-way to the ground. A second fade (the old 0.6, or a nested
// `Disabled` making 0.25) moves them, and so does a missing one.
func assertOneFade(t *testing.T, what string, dark bool, ground ui.Color, rest, locked lockReadings, withInk bool) {
	t.Helper()
	// The glyph band is antialiased, so its extreme pixel moves a few levels
	// between the two renders; the planes are flat fills and do not.
	check := func(field string, rest, locked ui.Color, tol int) {
		t.Helper()
		want := halfMix(rest, ground)
		if d := absDiff(litSum(want), litSum(locked)); d > tol {
			t.Errorf("%s (%s): the locked %s is %v, but one fade of %v over the ground %v is %v (off by %d levels); the control is faded twice — or not at all",
				what, appearanceName(dark), field, locked, rest, ground, want, d/3)
		}
	}
	check("face", rest.face, locked.face, 6)
	check("lit edge", rest.crown, locked.crown, 6)
	check("counter-side", rest.shade, locked.shade, 6)
	if withInk {
		check("ink", rest.ink, locked.ink, 27)
	}
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// A. A locked config field: the groove keeps half its step off the ground and
// its E1 pair, and its ink steps back by the same single fade.
func TestTheLockedFieldFadesOnce(t *testing.T) {
	for _, dark := range []bool{false, true} {
		ground, rest, locked := lockProbe(t, dark, func(a *App, c *ui.Context, isLocked bool) ui.Element {
			s := "hello"
			e := a.extensionField(c, &s, configFieldMetrics)
			if isLocked {
				e = lockedControl(e)
			}
			return e
		})
		assertOneFade(t, "the config field", dark, ground, rest, locked, true)
		// And the face is still a face: half of a 17-level step is 8~9 levels,
		// which is the same rung the disabled pill sits on (P13-s: 9).
		if g := absDiff(litSum(locked.face), litSum(ground)); g < 18 {
			t.Errorf("the locked config field (%s): the face is only %d levels off the ground; a locked groove still has to be a groove",
				appearanceName(dark), g/3)
		}
	}
}

// B. A locked switch: the track keeps half its fill step.
func TestTheLockedSwitchFadesOnce(t *testing.T) {
	for _, dark := range []bool{false, true} {
		ground, rest, locked := lockProbeAt(t, dark, 0.25, func(a *App, c *ui.Context, isLocked bool) ui.Element {
			on := true
			e := a.switchControl(c, &on)
			if isLocked {
				e = lockedControl(e)
			}
			return e
		})
		// The planes only: the framework paints a switch's knob through its own
		// Draw callback, outside the element's opacity, so the knob keeps more
		// of its white than a uniform fade would give it. That is the
		// framework's picture of a disabled widget, not this round's.
		assertOneFade(t, "the config switch", dark, ground, rest, locked, false)
	}
}

// C. The list box: one `Disabled` at the root of the control, so the box, the
// option words and the tick boxes all land on the same rung. The words used to
// stay at 0.6 while the tick boxes inside them went to 0.25.
func TestTheLockedListBoxFadesOnce(t *testing.T) {
	for _, dark := range []bool{false, true} {
		ground, rest, locked := lockProbe(t, dark, func(a *App, c *ui.Context, isLocked bool) ui.Element {
			var scroll ui.ScrollState
			box := a.configMultiselectBox(c, &scroll, isLocked, func() {
				// One ticked option, so the tick box's own ink is on screen and
				// its fade can be read: it was the third depth (0.25) inside the
				// box's 0.6.
				for index, option := range []string{"alpha", "beta"} {
					on, changed := index == 0, false
					a.configOptionRow(c, option, &on, &changed, isLocked)
				}
			})
			return box
		})
		assertOneFade(t, "the multiselect list box", dark, ground, rest, locked, true)
	}
}

// D. One depth per form: the field and the list box are the same material
// (`Control` on the card), so a locked one must land on the same pixel. Before
// this round they read 242 and 236 — two rungs for one word.
func TestTheLockedFormHasOneDepth(t *testing.T) {
	for _, dark := range []bool{false, true} {
		_, _, field := lockProbe(t, dark, func(a *App, c *ui.Context, isLocked bool) ui.Element {
			s := "hello"
			e := a.extensionField(c, &s, configFieldMetrics)
			if isLocked {
				e = lockedControl(e)
			}
			return e
		})
		_, _, box := lockProbe(t, dark, func(a *App, c *ui.Context, isLocked bool) ui.Element {
			var scroll ui.ScrollState
			b := a.configMultiselectBox(c, &scroll, isLocked, func() {
				on, changed := false, false
				a.configOptionRow(c, "alpha", &on, &changed, isLocked)
			})
			return b
		})
		if d := absDiff(litSum(field.face), litSum(box.face)); d > 3 {
			t.Errorf("the locked form (%s): the field's face is %v and the list box's is %v — %d levels apart, two depths for one state",
				appearanceName(dark), field.face, box.face, d/3)
		}
	}
}

// The opt-in dump: a resting and a locked field, switch and list box, both
// appearances. `FLOTER_PNG_TAG=before` re-applies the old second fade
// (`Opacity(0.6)`, and the list box's forgotten `Disabled`) so the pair can be
// compared without checking out the old tree.
func TestDumpP13TLockedForm(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	before := os.Getenv("FLOTER_PNG_TAG") == "before"
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
			ui.Column(c).Padding(20).Width(320).Gap(12).Children(func() {
				for _, locked := range []bool{false, true} {
					locked := locked
					s := "hello"
					f := a.extensionField(c, &s, configFieldMetrics)
					if locked {
						f = lockedControl(f)
						if before {
							f = f.Opacity(0.6)
						}
					}
				}
				for _, locked := range []bool{false, true} {
					locked := locked
					on := true
					sw := a.switchControl(c, &on)
					if locked {
						sw = lockedControl(sw)
						if before {
							sw = sw.Opacity(0.6)
						}
					}
				}
				for _, locked := range []bool{false, true} {
					locked := locked
					var scroll ui.ScrollState
					b := a.configMultiselectBox(c, &scroll, locked, func() {
						for _, option := range []string{"alpha", "beta"} {
							on, changed := false, false
							a.configOptionRow(c, option, &on, &changed, locked)
						}
					})
					if locked && before {
						b = b.Disabled(true).Opacity(0.6)
					}
				}
			})
		}, 360, 320)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tag := os.Getenv("FLOTER_PNG_TAG")
		if tag == "" {
			tag = "after"
		}
		name := filepath.Join("/tmp", "floter-P13-t-lockedform-"+map[bool]string{false: "light", true: "dark"}[dark]+"-"+tag+".png")
		file, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatal(err)
		}
		file.Close()
		t.Logf("wrote %s", name)
	}
}
