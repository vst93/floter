package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-ct · 降级面的灯：一条关着的命令，冠线既不是亮的，也不是 62% 亮的。
//
// `commandPlate` 此前对**每一条**命令都戴 `LitFace`，再对关着的命令洗一道
// `.Opacity(0.62)`（参考的 `.extension-command-list__row--off`）。于是那块面既没有
// 「点亮」也没有「熄灯」——它亮了 62%：实测冠线行 − 面 浅色 **+5 → +2**（洗在一条
// 只有 5 级的冠线上，等于看不见）、深色 **+17 → +11**（还明明白白亮着）。同一句话在
// 两套外观上是两个说法，而 §3 说受光边是「这块面可以被碰」的记号。
//
// 这一轮把材质按状态分开：开着 = `LitFace`，关着 = `DimFace`（灯熄、暗线留），
// 参考的那一道 0.62 保留——它不是框架的 `Disabled`（开关与别名框仍然是活的）。

// plateReadings is one command plate's four vertical readings: the page ground it
// lies on, its own face, the row it wears on top, and the row it wears under.
type plateReadings struct {
	ground, face, crown, shade int
}

// plateStep is the crown's and the counter-side's distance from the face.
func (r plateReadings) plateStep() (crown, shade int) {
	return r.crown - r.face, r.face - r.shade
}

// plateProbe draws the command plate itself on a clean ground and returns its
// own box, so no pin below spells a pixel box: the plate's rect is resolved from
// the element tree, and its four rows are read the way the disabled-face pins
// read them (the lit edge straddles two device rows at a fractional y).
func plateProbe(t *testing.T, dark, enabled bool) (plateReadings, ui.Rect) {
	t.Helper()
	var r plateReadings
	ground := realPageGround(t, dark)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	text := i18n.For("en").Settings
	var box ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(24).Width(360).Children(func() {
			ui.Column(c).Children(func() {
				a.commandPlate(c, text, "ext", Command{ID: "build", Name: "Build", Enabled: enabled, Declared: true})
			}).Bind(&ref)
			box = c.Resolve(ref).Bounds()
		})
	}, 420, 220)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()
	x := int(box.X + box.W/2)
	r.ground = litSum(imagePixel(img, x, 2))
	r.face = litSum(imagePixel(img, x, int(box.Y)+6))
	r.crown = litSum(imagePixel(img, x, int(box.Y)+1))
	for dy := 2; dy <= 3; dy++ {
		if q := litSum(imagePixel(img, x, int(box.Y)+dy)); q > r.crown {
			r.crown = q
		}
	}
	r.shade = litSum(imagePixel(img, x, int(box.Y+box.H)-2))
	for dy := -3; dy <= -1; dy++ {
		if q := litSum(imagePixel(img, x, int(box.Y+box.H)+dy)); q < r.shade {
			r.shade = q
		}
	}
	return r, box
}

// revealCommandListFor renders the Integrations page on one appearance.
func revealCommandListFor(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.Integrations = func() []Integration { return []Integration{commandListIntegration()} }
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, 720, 900)
	tt.SetDark(dark)
	tt.Frame()
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt
}

func TestTheOffCommandPlateTurnsItsCrownOffAndKeepsItsShade(t *testing.T) {
	for _, dark := range []bool{false, true} {
		on, _ := plateProbe(t, dark, true)
		off, _ := plateProbe(t, dark, false)

		// litSum is a three-channel sum: one brightness level is three of these.
		const level = 3
		crownOn, shadeOn := on.plateStep()
		crownOff, shadeOff := off.plateStep()
		t.Logf("dark=%v  on: ground %d face %d crown %+d shade %d  |  off: ground %d face %d crown %+d shade %d",
			dark, on.ground, on.face, crownOn, shadeOn, off.ground, off.face, crownOff, shadeOff)

		// The enabled plate is lit: its crown is a step above its own face.
		if crownOn < 3*level {
			t.Errorf("dark=%v: the enabled plate's crown lifts only %d levels above its face (%d over %d) — the lit edge is what makes it a face at all",
				dark, crownOn/level, on.crown, on.face)
		}
		// The off plate is not. It used to be 62% lit — light +2 levels, dark +11 —
		// and that is the one thing this pin forbids: a face neither lit nor
		// unlit, the same statement made two ways on two appearances.
		if crownOff > 2 {
			t.Errorf("dark=%v: the off command's plate still wears a lit edge: its crown row is %d over a %d face (+%d levels). A command whose switch is off is drawn with the light out (`DimFace`), not with a softer light",
				dark, off.crown, off.face, crownOff/level)
		}
		// The counter-side is what is left of the pair, and it is what keeps the
		// block standing on the ground.
		wantDrop := 5 * level
		if dark {
			// The dark sheet's shade is 0.20 of black, a thinner line in absolute
			// levels than the light one's 0.06 over a near-white face.
			wantDrop = 2 * level
		}
		if shadeOff < wantDrop {
			t.Errorf("dark=%v: the off command's plate lost its counter-side too: %d under a %d face (−%d levels). `DimFace` keeps the shade and drops the light, not both",
				dark, off.shade, off.face, shadeOff/level)
		}
		// The face still steps off the page: de-emphasised, not erased.
		step := off.face - off.ground
		if dark && step < 6*level {
			t.Errorf("dark=%v: the off command's plate no longer steps off the page: face %d over ground %d (+%d levels)",
				dark, off.face, off.ground, step/level)
		}
		if !dark && -step < 6*level {
			t.Errorf("dark=%v: the off command's plate no longer steps off the page: face %d under ground %d (−%d levels)",
				dark, off.face, off.ground, -step/level)
		}
	}
}

// TestDumpP13CTCommandPlate writes the round's visual evidence: the whole
// Integrations page, both appearances, tagged by the caller so a before/after
// pair can be produced from the same build.
func TestDumpP13CTCommandPlate(t *testing.T) {
	if os.Getenv("FLOTER_P13CT_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13CT_DUMP=1 to write /tmp/floter-P13-ct-*.png")
	}
	tag := os.Getenv("FLOTER_P13CT_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		tt := revealCommandListFor(t, dark)
		name := fmt.Sprintf("/tmp/floter-P13-ct-commandplate-%s-%s.png", appearanceName(dark), tag)
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
