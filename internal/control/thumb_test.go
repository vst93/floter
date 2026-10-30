package control

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-r · 凹槽里的那一格：浅色档的分段控件曾经「槽里挖深一层」。
//
// 选中格的填充原来是 `tokens.ControlHover`。那个 token 在两种外观上指向**相反的
// z 方向**：浅色档的「抬起」是黑洗（`0,0,0,0.105`），压在凹槽面 232 上得到 207 ——
// 比它自己的槽还暗 24 级；深色档的「抬起」是白洗（`255,255,255,0.14`），压在 58 上
// 得到 86。于是同一枚控件在深色档说「这是抬起来的」，在浅色档说「这是挖下去的」，
// 而代码注释与 `docs/liquid-glass.md` §2 的 E1 都写着 raised pane。
//
// 「凹槽里更深的一格」正是用户点名的 macOS 10.x 读法（「有点偏 macOS 旧版本」）；
// 现代 macOS/iOS 的分段滑块是**纸色抬起**。所以这一轮给这一格一个自己的 token
// （`theme.Tokens.Thumb`）：浅色档 = 纸自己的颜色（0.85 白压在卡面的凹槽 216 上
// 复合到 249，页面是 247），深色档 = 原来那档白洗（**外观不变**）。
//
// 量在**卡面上**（不是裸页面）：分段控件在设置页里坐在分组卡的 `Control` 面上，
// 槽面因此是 216 而不是裸页面上的 232 —— 抬高量按它真实站的那张纸算。

// thumbReading is one render's five pixels: the page ground, the card face the
// control sits on, the groove's fill, the chosen slot's fill, and the accent
// ring on the chosen slot's top and bottom rows (the ring is `AccentEdge`, a
// blue line, so it is found as the bluest pixel in each boundary band).
type thumbReading struct {
	ground, card, track, thumb ui.Color
	ringTop, ringBottom        ui.Color
	box                        ui.Rect
}

// thumbProbe renders the **real** `SegmentedIndex` — not a hand-built copy of
// its stack — on a card face over the page ground, in one appearance.
func thumbProbe(t *testing.T, dark bool) thumbReading {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
	ground := ui.RGB(247, 247, 248)
	if dark {
		ground = ui.RGB(39, 40, 44)
	}
	sel := 1
	var cardBox, box ui.Rect
	var cardRef, segRef ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		ui.Column(c).FillWidth().FillHeight().Background(ground).Padding(16).Children(func() {
			ui.Column(c).FillWidth().Padding(c.Theme().Space(3)).Background(tokens.Control).
				Radius(tokens.RadiusMD).Bind(&cardRef).Children(func() {
				SegmentedIndex(c, tokens, &sel, []string{"Aaa", "Bbb", "Ccc"}, true, false,
					c.Theme().Space(0.5), c.Theme().Space(0.5)).Bind(&segRef)
			})
			cardBox = c.Resolve(cardRef).Bounds()
			box = c.Resolve(segRef).Bounds()
		})
	}, 420, 140)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()

	segW := box.W / 3
	// The chosen segment is the middle one; the unchosen column is the first,
	// taken at its centre so neither stadium corner is in the reading.
	thumbX := int(box.X + segW + segW/2)
	trackX := int(box.X + segW/2)
	mid := int(box.Y + box.H/2)
	r := thumbReading{
		ground: px(img, 2, 2),
		card:   px(img, int(cardBox.X+cardBox.W/2), int(cardBox.Y)+2),
		track:  px(img, trackX, mid),
		thumb:  px(img, thumbX, mid),
		box:    box,
	}
	// The accent ring: the bluest pixel of the chosen column's first and last
	// three rows. The ring is 1px on a fractional box, so it can straddle two
	// rows — a band read is what makes it findable in both appearances.
	bluest := func(from, to int) ui.Color {
		var best ui.Color
		seen := false
		for y := from; y <= to; y++ {
			q := px(img, thumbX, y)
			if !seen || int(q.B)-int(q.R) > int(best.B)-int(best.R) {
				best, seen = q, true
			}
		}
		return best
	}
	r.ringTop = bluest(int(box.Y)-1, int(box.Y)+3)
	r.ringBottom = bluest(int(box.Y+box.H)-3, int(box.Y+box.H)+1)
	t.Logf("[%s] ground %v card %v track %v thumb %v ring(top) %v ring(bottom) %v box %+v",
		name, r.ground, r.card, r.track, r.thumb, r.ringTop, r.ringBottom, r.box)
	return r
}

func px(img *image.RGBA, x, y int) ui.Color {
	if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		return ui.Color{}
	}
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

func levelSum(c ui.Color) int { return int(c.R) + int(c.G) + int(c.B) }

func absDiff(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TestTheChosenSegmentRisesOutOfItsGroove pins the one invariant this round
// turns on: on **both** appearances the chosen slot is brighter than the groove
// it sits in, by at least 10 levels. Before P13-r the light appearance read
// 207 against a 232 groove (−25): a hole inside a hole.
//
// Mutation: `Background(tokens.Thumb)` back to `tokens.ControlHover` → the
// light subtest reports −25.
func TestTheChosenSegmentRisesOutOfItsGroove(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) {
			r := thumbProbe(t, appearance.dark)
			lift := int(r.thumb.R) - int(r.track.R)
			if lift < 10 {
				t.Errorf("[%s] the chosen slot is %d levels above its groove (%v over %v): a groove is a recess, so the "+
					"pane inside it has to be the one raised thing in the stack — `ControlHover` points the other way on "+
					"this appearance (liquid-glass.md §2 的 E1、§6.2b-5)", appearance.name, lift, r.thumb, r.track)
			}
		})
	}
}

// TestTheLightThumbIsTheSheetColour: the light appearance has no brighter pane
// to borrow, so the raised slot is the **sheet's own colour** punched through
// the groove — the iOS/macOS segmented slider. It must land on the page ground
// (±3 levels), not above it: a thumb brighter than the paper reads as a glow,
// which is the `EdgeHighlight` failure §1 禁令 3 点名的「类玻璃边」.
//
// Mutation: `thumb` 0.85 → 0.99 → 254 against a 247 page, red.
func TestTheLightThumbIsTheSheetColour(t *testing.T) {
	r := thumbProbe(t, false)
	if d := absDiff(int(r.thumb.R) - int(r.ground.R)); d > 3 {
		t.Errorf("the chosen slot is %d levels off the page's own colour (%v against %v): on the light appearance the "+
			"raised slot is the sheet itself, so it must composite to the ground, not above it (liquid-glass.md §6.2b-5)",
			d, r.thumb, r.ground)
	}
	// And it has to be the *sheet*, not the card it was cut into: the card face
	// is the darker one (Control over the ground), so the thumb is above both.
	if levelSum(r.thumb) <= levelSum(r.card) {
		t.Errorf("the chosen slot (%v) is not above the card face it sits on (%v): the thumb is the sheet showing "+
			"through the groove", r.thumb, r.card)
	}
}

// TestTheChosenThumbKeepsItsAccentRing: the accent ring is the chosen slot's
// identity line (§6.2b 9-11), and it is the reason that slot keeps the
// four-layer `Lens` instead of demoting to `LitFace`. A near-white thumb must
// not wash it out, so the ring has to stay a *blue* line: its blue-minus-red
// spread must beat the face's own by ≥25 in both appearances (measured: light
// 58 against 0, dark 31 against 4). Red-channel distance is the wrong ruler
// here — on the dark appearance the ring is lighter on R by only 6 levels at
// the bottom row while it is still plainly blue (B 145 against 107).
//
// Mutation: drop the `InsetShadow(0,0,0,1, AccentEdge)` line → red.
func TestTheChosenThumbKeepsItsAccentRing(t *testing.T) {
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) {
			r := thumbProbe(t, appearance.dark)
			faceSpread := int(r.thumb.B) - int(r.thumb.R)
			for _, ring := range []struct {
				where string
				c     ui.Color
			}{{"top", r.ringTop}, {"bottom", r.ringBottom}} {
				if spread := int(ring.c.B) - int(ring.c.R) - faceSpread; spread < 25 {
					t.Errorf("[%s] the %s accent ring is only %d levels bluer than the chosen slot's face (%v against %v): the "+
						"ring is that slot's identity line, and a face that swallows it says nothing (liquid-glass.md §6.2b 9-11)",
						appearance.name, ring.where, spread, ring.c, r.thumb)
				}
			}
		})
	}
}

// TestDumpP13RSegmentPNGs is this round's visual evidence: the same control, in
// both appearances, on the settings page's own card face. Opt-in (`FLOTER_PNG=1`)
// so the suite does not write files; `FLOTER_PNG_TAG` names the pair
// (`before` is taken from the injected state).
func TestDumpP13RSegmentPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-r-segment-<tag>-<appearance>.png")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		thumbProbe(t, appearance.dark) // the readings are logged by the probe
		path := filepath.Join("/tmp", "floter-P13-r-segment-"+tag+"-"+appearance.name+".png")
		tt := thumbFrame(t, appearance.dark)
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", path)
	}
}

// thumbFrame is thumbProbe's render without the readings, so the dump and the
// pins cannot drift into two different stacks.
func thumbFrame(t *testing.T, dark bool) *ui.Tester {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
	ground := ui.RGB(247, 247, 248)
	if dark {
		ground = ui.RGB(39, 40, 44)
	}
	sel := 1
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		ui.Column(c).FillWidth().FillHeight().Background(ground).Padding(16).Children(func() {
			ui.Column(c).FillWidth().Padding(c.Theme().Space(3)).Background(tokens.Control).
				Radius(tokens.RadiusMD).Children(func() {
				SegmentedIndex(c, tokens, &sel, []string{"Aaa", "Bbb", "Ccc"}, true, false,
					c.Theme().Space(0.5), c.Theme().Space(0.5))
			})
		})
	}, 420, 140)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}
