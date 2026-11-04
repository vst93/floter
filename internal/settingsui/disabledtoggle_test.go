package settingsui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-ge · 禁用的开关仍要说得出它现在是哪一档。
//
// 家族里已经定过的三件事：一道淡（P13-u）、槽不叠材质（P13-t）、禁用面熄灯
// （P13-s）。它们都在问「它被淡了几次」，没有一条问**淡完之后那句状态还在不在**。
// 一枚开着的开关禁用后如果与一枚关着的开关禁用后长得一样，那它就在说反话：
// 用户读到的是「这个功能关着，而且点不动」，而实际状态是「开着，只是现在改不了」。
//
// 这一族的算术风险是写死的：框架给 `Disabled` 的整棵子树乘 0.5（`ui/paint.go:257-259`），
// 而开关的「开/关」两句话各自都靠**面**说——开是 accent 轨、关是中性轨，再叠一个
// 位置相反的旋钮。两道淡之后，accent 轨离地面的台阶、旋钮对轨的对比，都只剩一半。
// 半多少、还剩没剩，此前没人量过。
//
// 四条判据，每条是一句人话：
//  1. 两帧（禁用-开 / 禁用-关）在**面积上**真的不同（不是一两个抗锯齿像素）；
//  2. 轨道填充仍分得开——这是不看旋钮位置也能读到的那一句；
//  3. 两种状态都仍**离得开地面**（槽还在 z 上，没被淡成背景）；
//  4. 旋钮仍读得出（对轨道的对比够），否则「开」只剩一块更亮的面，位置没了。

// toggleFrame renders the product's own disabled switch in one state and
// returns the frame and the switch's box. Both states go through
// `switchControl` + `disabledSwitch`, i.e. the two functions the settings page
// actually calls — not a re-built approximation of them.
func toggleFrame(t *testing.T, dark, on bool) (*image.RGBA, ui.Rect) {
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
			flag := on
			disabledSwitch(a.switchControl(c, &flag)).Bind(&ref)
			box = c.Resolve(ref).Bounds()
		})
	}, 400, 160)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if box.W < 20 || box.H < 10 {
		t.Fatalf("measured a %v×%v box, not a switch", box.W, box.H)
	}
	return tt.Image(), box
}

// toggleReadings is what one disabled switch says about itself.
//
// `track` is the fill of the **third away from the knob** (the knob sits over
// the middle, so the opposite third is clear in both states); `knob` is the
// extreme pixel inside the knob's own half and `knobContrast` its distance from
// the track. `trackOffGround` is the same fill's distance from the page ground.
type toggleReadings struct {
	track          ui.Color
	knob           ui.Color
	knobContrast   int
	trackOffGround int
}

func readToggle(img *image.RGBA, box ui.Rect, ground ui.Color, on bool) toggleReadings {
	y := int(box.Y + box.H/2)
	// The knob travels, so the clear third flips with the state: on reads the
	// left third and the knob's own half is the right one; off mirrors it.
	x := int(box.X + box.W*0.18)
	knobFrom, knobTo := int(box.X+box.W*0.55), int(box.X+box.W*0.95)
	if !on {
		x = int(box.X + box.W*0.82)
		knobFrom, knobTo = int(box.X+box.W*0.05), int(box.X+box.W*0.45)
	}
	r := toggleReadings{track: imagePixel(img, x, y)}
	r.trackOffGround = absDiff(litSum(r.track), litSum(ground)) / 3
	r.knob = r.track
	for yy := int(box.Y) + 2; yy < int(box.Y+box.H)-2; yy++ {
		for xx := knobFrom; xx < knobTo; xx++ {
			if q := imagePixel(img, xx, yy); absDiff(litSum(q), litSum(r.track)) > absDiff(litSum(r.knob), litSum(r.track)) {
				r.knob = q
			}
		}
	}
	r.knobContrast = absDiff(litSum(r.knob), litSum(r.track)) / 3
	return r
}

// toggleDifference is how much of the switch's interior the two states share:
// it returns the number of interior pixels that differ by at least 8 levels and
// the largest such difference. Antialiasing at the frame costs a few pixels;
// a real state change costs the knob and the track.
func toggleDifference(on, off *image.RGBA, box ui.Rect) (int, int) {
	count, max := 0, 0
	for y := int(box.Y) + 2; y < int(box.Y+box.H)-2; y++ {
		for x := int(box.X) + 2; x < int(box.X+box.W)-2; x++ {
			d := absDiff(litSum(imagePixel(on, x, y)), litSum(imagePixel(off, x, y))) / 3
			if d >= 8 {
				count++
			}
			if d > max {
				max = d
			}
		}
	}
	return count, max
}

func TestADisabledToggleStillSaysWhichWayItIs(t *testing.T) {
	for _, dark := range []bool{false, true} {
		onImg, onBox := toggleFrame(t, dark, true)
		offImg, offBox := toggleFrame(t, dark, false)
		if onBox != offBox {
			t.Fatalf("%s: the two states are laid out differently (%v vs %v)", appearanceName(dark), onBox, offBox)
		}
		ground := realPageGround(t, dark)
		rOn := readToggle(onImg, onBox, ground, true)
		rOff := readToggle(offImg, offBox, ground, false)
		count, max := toggleDifference(onImg, offImg, onBox)
		interior := int(onBox.W-4) * int(onBox.H-4)
		t.Logf("%s: on track %v (off ground %d) / off track %v (off ground %d); on knob %v (%d) / off knob %v (%d); frames differ on %d/%d px, max %d levels",
			appearanceName(dark), rOn.track, rOn.trackOffGround, rOff.track, rOff.trackOffGround,
			rOn.knob, rOn.knobContrast, rOff.knob, rOff.knobContrast, count, interior, max)

		trackStep := absDiff(litSum(rOn.track), litSum(rOff.track)) / 3
		if trackStep < 24 {
			t.Errorf("%s: the two disabled tracks are %d levels apart (on %v, off %v); a disabled switch has to keep the state its track carries",
				appearanceName(dark), trackStep, rOn.track, rOff.track)
		}
		if count*4 < interior {
			t.Errorf("%s: only %d of %d interior pixels differ between the two disabled states (max %d levels); the knob's travel is gone",
				appearanceName(dark), count, interior, max)
		}
		if max < 24 {
			t.Errorf("%s: the largest interior difference between the two disabled states is %d levels; that is antialiasing, not a state",
				appearanceName(dark), max)
		}
		for what, r := range map[string]toggleReadings{"on": rOn, "off": rOff} {
			if r.trackOffGround < 16 {
				t.Errorf("%s: the disabled %s track is %d levels off the ground; the groove has washed into the page", appearanceName(dark), what, r.trackOffGround)
			}
			if r.knobContrast < 16 {
				t.Errorf("%s: the disabled %s knob is %d levels from its own track; the knob is on the track, not in it", appearanceName(dark), what, r.knobContrast)
			}
		}
	}
}

// busyToggleFrame renders the **in-flight** switch (`busySwitchControl`, the
// knob swapped for the turning loader) in one state. Its comment claims the
// track keeps the state's own fill so "which way the switch was moving is still
// readable" — that sentence had no pin; it has one now.
func busyToggleFrame(t *testing.T, dark, on bool) (*image.RGBA, ui.Rect) {
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
			flag := on
			a.busySwitchControl(c, &flag, "p13ge").Bind(&ref)
			box = c.Resolve(ref).Bounds()
		})
	}, 400, 160)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if box.W < 20 || box.H < 10 {
		t.Fatalf("measured a %v×%v box, not a switch", box.W, box.H)
	}
	return tt.Image(), box
}

// TestTheBusySwitchKeepsTheSameStateFill pins the sentence above: a write in
// flight draws the state with the **same** two fills a merely-disabled switch
// wears (accent on / neutral off), because both go through the framework's one
// fade. A second fade, or a busy face that forgets the state, lands here.
func TestTheBusySwitchKeepsTheSameStateFill(t *testing.T) {
	for _, dark := range []bool{false, true} {
		ground := realPageGround(t, dark)
		for _, on := range []bool{true, false} {
			wantImg, wantBox := toggleFrame(t, dark, on)
			gotImg, gotBox := busyToggleFrame(t, dark, on)
			want := readToggle(wantImg, wantBox, ground, on).track
			got := readToggle(gotImg, gotBox, ground, on).track
			if step := absDiff(litSum(got), litSum(want)) / 3; step > 2 {
				t.Errorf("%s (on %v): the in-flight track is %v but the disabled one is %v (%d levels apart); the busy face is fading the state twice",
					appearanceName(dark), on, got, want, step)
			}
		}
	}
}

// TestDumpP13GEDisabledToggle is the round's visual evidence: the two disabled
// states above their enabled twins, both appearances. Opt-in, like every other
// dump in this package.
func TestDumpP13GEDisabledToggle(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
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
			ui.Column(c).Padding(24).Width(360).Gap(12).Children(func() {
				on, off := true, false
				a.switchControl(c, &on)
				a.switchControl(c, &off)
				disabledSwitch(a.switchControl(c, &on))
				disabledSwitch(a.switchControl(c, &off))
			})
		}, 400, 260)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		name := filepath.Join("/tmp", "floter-P13-ge-disabledtoggle-"+appearanceName(dark)+".png")
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
