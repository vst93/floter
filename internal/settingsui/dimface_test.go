package settingsui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-s · 禁用面：面还在，光熄了。
//
// 这一族此前是「整棵子树 `Opacity(0.4)`」，但框架的画笔**已经**给 `Disabled` 的子树
// 乘了 0.5（`ui/paint.go:257-259`）——两道淡乘起来 = 有效 0.2。于是禁用胶囊：
//
//	浅色：地面 247，面 243（台阶只剩 **4** 级），顶内线 = 面，底内线 240（−3）
//	深色：地面 15，面 19（台阶 **+4**），顶内线 = 面，底内线 18（−1）
//
// 那不是「一块不能碰的面」，是一块几乎不存在的面——用户说的「糊」与「空间感不够」
// 在状态反馈这一侧的算术来源。P13-s 之后（`docs/liquid-glass.md` §6.2b-6）：
//
//	浅色：地面 247，面 238（**−9**），顶内线 = 面（**没有受光边**），底内线 231（−7）
//	深色：地面 15，面 27（**+12**），顶内线 = 面，底内线 24（−3）
//
// 三条判据，就是这一轮的三条 pin：①面还有台阶（≥6 级）；②顶内线**不亮**（受光边是
// 「这块面可以被碰」的记号，禁用面熄灯）；③墨只淡了一道（有效 0.2 时对比 43 级，
// 一道 0.5 时 105 级）。

// dimProbe renders one pill through the product's own draw function and returns
// the frame, the pill's box and the four readings the pins use.
type dimReadings struct {
	ground, face, crown, shade ui.Color
	ink, inkContrast           int
}

func dimProbe(t *testing.T, dark bool, build func(a *App, c *ui.Context) ui.Element) (ui.Rect, dimReadings) {
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
	x := int(box.X + box.W/2)
	r := dimReadings{ground: ground}
	// The face is read four rows in, down the middle column: clear of the 1px
	// frame, above the label's glyph band.
	r.face = imagePixel(img, x, int(box.Y)+4)
	// The lit edge and the counter-side straddle two device rows at a
	// fractional y, so each is read as the extreme of its own pair.
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
	// The ink is the pixel of the label band furthest from the face, in either
	// direction: the label is darker than the face on the light appearance and
	// lighter on the dark one.
	lo, hi := 1<<30, -1
	for y := int(box.Y) + 3; y < int(box.Y+box.H)-3; y++ {
		for xx := int(box.X) + 4; xx < int(box.X+box.W)-4; xx++ {
			q := litSum(imagePixel(img, xx, y))
			if q < lo {
				lo = q
			}
			if q > hi {
				hi = q
			}
		}
	}
	f := litSum(r.face)
	if hi-f > f-lo {
		r.ink, r.inkContrast = hi, hi-f
	} else {
		r.ink, r.inkContrast = lo, f-lo
	}
	return box, r
}

// assertDisabledFace is the shared verdict: the face still steps off the
// ground, the top inner row is not a lit edge, and the bottom one is still a
// shade.
func assertDisabledFace(t *testing.T, what string, dark bool, box ui.Rect, r dimReadings) {
	t.Helper()
	if box.W < 20 || box.H < 20 {
		t.Fatalf("%s: measured a %v×%v box, not a control", what, box.W, box.H)
	}
	// litSum is a three-channel sum: one brightness level is three of these.
	const level = 3
	f, g := litSum(r.face), litSum(r.ground)
	step := f - g
	if dark && step < 6*level {
		t.Errorf("%s (dark): the disabled face is only %d levels off the ground (%v vs %v); want a face, not a ghost",
			what, step/level, r.face, r.ground)
	}
	if !dark && -step < 6*level {
		t.Errorf("%s (light): the disabled face is only %d levels under the ground (%v vs %v); want a face, not a ghost",
			what, -step/level, r.face, r.ground)
	}
	if over := litSum(r.crown) - f; over > 2 {
		t.Errorf("%s (%s): the disabled pill kept a lit edge: top inner row %v is %d levels over the face %v",
			what, appearanceName(dark), r.crown, over/level, r.face)
	}
	// The disabled face is **half** a step, not a full one: if it ever steps as
	// far off the ground as the resting face (17 levels light / 25 dark), the
	// pill reads as enabled and the state feedback is gone.
	if step < 0 {
		step = -step
	}
	if step > 15*level {
		t.Errorf("%s (%s): the disabled face steps %d levels off the ground — that is a resting face, not a disabled one",
			what, appearanceName(dark), step/level)
	}
	drop := f - litSum(r.shade)
	want := 5 * level
	if dark {
		// The dark sheet's shade is 0.20 of black under the same 0.5, which is
		// a thinner line in absolute levels than the light one's 0.06 over 238.
		want = 2 * level
	}
	if drop < want {
		t.Errorf("%s (%s): the disabled pill lost its counter-side: bottom inner row %v is only %d levels under the face %v",
			what, appearanceName(dark), r.shade, drop/level, r.face)
	}
}

// A. The extensions page's "Open configuration" pill, disabled because the
// provider declared no `openCommand` — the resting disabled state a user sees
// most often on that page.
func TestTheDisabledActionPillKeepsAFaceAndLosesItsCrown(t *testing.T) {
	for _, dark := range []bool{false, true} {
		box, r := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			th := a.cardTheme(c)
			b := a.extensionGlyphActionButton(c, "link", "Open configuration",
				th.Space(7.5), th.Space(3.75), a.tokens(c).Body, true)
			return b.Disabled(true).Cursor(ui.CursorDefault)
		})
		assertDisabledFace(t, "the disabled action pill", dark, box, r)
	}
}

// B. The in-flight pill (`busyActionButton`): same face, a spinner instead of
// the glyph, and the same one fade. This is the arm a write in progress puts on
// screen, so it is pinned separately from the resting one.
func TestTheBusyActionPillWearsTheDimFace(t *testing.T) {
	for _, dark := range []bool{false, true} {
		box, r := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return a.busyActionButton(c, "Refresh", "busy-pill")
		})
		assertDisabledFace(t, "the in-flight action pill", dark, box, r)
	}
}

// C. The ink is faded **once**, not twice. The reference's `.4` and the
// framework's `.5` used to multiply into an effective 0.2, which put the label
// at 43 levels of contrast on the light sheet — under the readability floor
// (`references/user-preferences.md` P-5). One fade puts it at 105.
//
// The window is two-sided on purpose: under 60 the label is washed out, over
// 160 the pill reads as enabled again (the enabled pill measures 202).
func TestTheDisabledActionPillInkIsOneFadeNotTwo(t *testing.T) {
	for _, dark := range []bool{false, true} {
		_, r := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			th := a.cardTheme(c)
			b := a.extensionGlyphActionButton(c, "link", "Open configuration",
				th.Space(7.5), th.Space(3.75), a.tokens(c).Body, true)
			return b.Disabled(true).Cursor(ui.CursorDefault)
		})
		// litSum is a three-channel sum, so a "level" is one third of it: the
		// window below is 60~160 levels of contrast, i.e. 180~480 in sums.
		if r.inkContrast < 180 {
			t.Errorf("the disabled pill (%s): the label is only %d levels off its face (%d vs %d); the ink is faded twice",
				appearanceName(dark), r.inkContrast, r.ink, litSum(r.face))
		}
		if r.inkContrast > 480 {
			t.Errorf("the disabled pill (%s): the label is %d levels off its face — that reads as enabled (the enabled pill measures 202)",
				appearanceName(dark), r.inkContrast)
		}
	}
}

// The opt-in dump: the disabled pill next to the enabled one, both appearances,
// before and after (set FLOTER_PNG_TAG=before while the fix is reverted).
func TestDumpP13SDimFace(t *testing.T) {
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
			ui.Column(c).Padding(24).Width(320).Gap(12).Children(func() {
				th := a.cardTheme(c)
				enabled := a.extensionGlyphActionButton(c, "link", "Open configuration",
					th.Space(7.5), th.Space(3.75), a.tokens(c).Body, false)
				_ = enabled
				dim := a.extensionGlyphActionButton(c, "link", "Open configuration",
					th.Space(7.5), th.Space(3.75), a.tokens(c).Body, true)
				dim.Disabled(true).Cursor(ui.CursorDefault)
				a.busyActionButton(c, "Refresh", "dump")
			})
		}, 360, 180)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		name := filepath.Join("/tmp", "floter-P13-s-dimface-"+appearanceName(dark)+"-"+tag+".png")
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
