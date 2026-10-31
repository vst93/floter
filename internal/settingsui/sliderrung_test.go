package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-bf · 设置页的禁用滑块：全行唯一一句「为什么」比它的提示还轻
// （`docs/rounds/P13-bf-spec.md`、`docs/liquid-glass.md` §6.25）。
//
// 先量的数（640×320、真设置页地面；地面浅 247 / 深 14，卡面浅 230 / 深 39）：
//
//	行名       FontSize+1 / 580 / TextStrong   对卡面 203 / 198
//	读数       FontSize   /     / TextStrong   对卡面 193 / 188  ← 与行名只差 10
//	功能提示   FontSize   /     / TextMuted    对卡面 102 /  94
//	轴两端     Caption    / 500 / TextMuted    对卡面 101 /  93
//	原因句     Caption    / 500 / TextTertiary 对卡面  88 /  71  ← 全行最淡
//
// 两件事：①「为什么滑不动」比「这个滑块是什么」还轻，与 `emptyState`
// （P8-b，§4.7「陈述压过提示」）正好相反；②一个不会动的读数与行名只差 10 级，
// 是这一行里唯一没跟着「槽」一起退的东西（轨道 209→219 / 58→49 已退）。

// sliderFace renders one slider row on the settings page's own ground and
// returns the frame, the card's face luma and the row's boxes.
type sliderFace struct {
	tt                      *ui.Tester
	face                    int
	label, value, reason    ui.Rect
	hint, low, high         ui.Rect
	valueShown, reasonShown string
}

func sliderFrame(t *testing.T, dark, disabled bool) sliderFace {
	t.Helper()
	en := i18n.For("en").Settings
	ground := realPageGround(t, dark)
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	why := ""
	if disabled {
		why = en.TransparencyUnavailable
	}
	var out sliderFace
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(24).Width(560).Children(func() {
			a.card(c, func() {
				a.slider(c, sliderRow{
					label: en.MainOpacity, description: en.TransparencyHint,
					low: en.TransparencyScaleLow, high: en.TransparencyScaleHigh,
					value: 47, lo: 0, hi: 100,
					format:   func(v float64) string { return en.Percent(uint8(v)) },
					apply:    func(float64) {},
					disabled: why,
				})
			})
		})
	}, 640, 340)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	out.tt = tt
	out.valueShown = en.Percent(47)
	if why != "" {
		out.reasonShown = why
	}
	out.label, _ = tt.Find(en.MainOpacity)
	// The card's own face: the strip of card between its left edge and the
	// row's own inset, on the label's line — never the ground behind it.
	out.face = pageLuma(tt, out.label.X-4, out.label.Y+out.label.H/2)
	out.value, _ = tt.Find(out.valueShown)
	out.hint, _ = tt.Find(en.TransparencyHint)
	out.low, _ = tt.Find(en.TransparencyScaleLow)
	out.high, _ = tt.Find(en.TransparencyScaleHigh)
	if why != "" {
		out.reason, _ = tt.Find(why)
	}
	return out
}

func pageLuma(tt *ui.Tester, x, y float32) int {
	c := sample(tt, x, y)
	return luma(c.R, c.G, c.B)
}

// rowWeight is how heavy a line reads on the card: the distance between the
// line's own ink and the face it sits on. Both appearances measure the same
// number — the sign flips with the appearance, the weight does not.
func rowWeight(tt *ui.Tester, face int, box ui.Rect) int {
	d := pageInkPeak(tt, box) - face
	if d < 0 {
		d = -d
	}
	return d
}

// pageInkPeak is the pixel inside a box furthest from the box's own corner:
// the glyph's own luma, whichever way round the appearance runs.
func pageInkPeak(tt *ui.Tester, box ui.Rect) int {
	img := tt.Image()
	lo, hi := 255, 0
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			i := y*img.Stride + x*4
			l := luma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			if l < lo {
				lo = l
			}
			if l > hi {
				hi = l
			}
		}
	}
	corner := pageLuma(tt, box.X+1, box.Y+1)
	if hi-corner > corner-lo {
		return hi
	}
	return lo
}

// The first pin: the sentence that says why the row cannot move is a
// **statement**, so it must outrank the hint above it — never the other way
// round. It also must not sink below the axis' own words: the reason is about
// this row, the axis is about the scale.
func TestTheDisabledSlidersReasonOutranksItsHint(t *testing.T) {
	for _, dark := range []bool{false, true} {
		dark := dark
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			f := sliderFrame(t, dark, true)
			if f.reason.W == 0 {
				t.Fatalf("the reason is missing: %v", f.tt.Texts())
			}
			reason := rowWeight(f.tt, f.face, f.reason)
			hint := rowWeight(f.tt, f.face, f.hint)
			axis := rowWeight(f.tt, f.face, f.low)
			if reason <= hint {
				t.Errorf("the reason reads %d against its hint's %d: a statement that "+
					"explains a state is lighter than the sentence describing the feature",
					reason, hint)
			}
			if reason <= axis {
				t.Errorf("the reason reads %d against the axis word's %d: the sentence "+
					"about this row is lighter than the scale it is about", reason, axis)
			}
			if gap := reason - hint; gap < 15 {
				t.Errorf("the reason is only %d above its hint (%d vs %d): one rung, not a "+
					"register", gap, reason, hint)
			}
			// The name is still the heaviest thing on the row: this row is not a
			// disabled subtree, the framework's 0.5 does not apply to it.
			name := rowWeight(f.tt, f.face, f.label)
			if name <= reason {
				t.Errorf("the row's name reads %d against its reason's %d", name, reason)
			}
		})
	}
}

// The second pin: a readout that cannot move steps back one rung, and the
// readout that can move does not. Before this round the number and the name
// were ten levels apart — the one thing on the row that is *alive* read as
// heavy as the name that identifies it, while the groove under it had already
// stepped back (§1's "a groove steps back as a whole").
func TestADisabledSlidersReadoutStepsBackAndALiveOneDoesNot(t *testing.T) {
	for _, dark := range []bool{false, true} {
		dark := dark
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			live := sliderFrame(t, dark, false)
			off := sliderFrame(t, dark, true)
			if live.value.W == 0 || off.value.W == 0 {
				t.Fatalf("a readout is missing: live %v off %v", live.tt.Texts(), off.tt.Texts())
			}
			liveValue := rowWeight(live.tt, live.face, live.value)
			offValue := rowWeight(off.tt, off.face, off.value)
			// The live readout keeps its strong ink: it is the row's answer, and it
			// is the only thing on the row that changes.
			if gap := rowWeight(live.tt, live.face, live.label) - liveValue; gap > 15 || gap < -15 {
				t.Errorf("the live readout reads %d against its name's %d: it used to be "+
					"one step under the name", liveValue, rowWeight(live.tt, live.face, live.label))
			}
			// The disabled one steps back to the reason's own rung — one fact, one
			// register — and stops being the second-heaviest thing on the row.
			if offValue >= liveValue-30 {
				t.Errorf("the disabled readout still reads %d against the live one's %d: "+
					"it has not stepped back", offValue, liveValue)
			}
			reason := rowWeight(off.tt, off.face, off.reason)
			if d := offValue - reason; d > 20 || d < -20 {
				t.Errorf("the disabled readout reads %d and the reason %d: the two facts of "+
					"one disabled row are on different rungs", offValue, reason)
			}
		})
	}
}

// The third pin is structural: the row's ink choices are one expression each,
// not a table of hex values — and the disabled readout is the *only* branch
// that changes the value's ink.
func TestTheSliderReadoutAndReasonAreOneBranchEach(t *testing.T) {
	src, err := os.ReadFile("settingsui.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	slider := func() string {
		i := strings.Index(body, "func (a *App) slider(c *ui.Context, row sliderRow) {")
		if i < 0 {
			t.Fatalf("the slider function is missing")
		}
		rest := body[i:]
		if j := strings.Index(rest[1:], "\nfunc "); j >= 0 {
			return rest[:j+1]
		}
		return rest
	}()
	for _, want := range []string{
		"ink := tokens.TextStrong",
		"if row.disabled != \"\" {\n\t\t\t\t\tink = tokens.TextSecondary",
		"FontWeight(580).TextColor(tokens.TextSecondary)",
	} {
		if !strings.Contains(slider, want) {
			t.Errorf("the slider is missing %q", want)
		}
	}
	// The reason's step does not move: changing it would change the row box,
	// which is the height chain.
	if strings.Contains(slider, "TextColor(tokens.TextTertiary)") {
		t.Errorf("the slider still paints something in the tertiary ink")
	}
}

// The fourth pin is the picture: the two frames, before and after, on the
// settings page's own ground (opt-in: `FLOTER_P13BF_DUMP=1`).
func TestDumpP13bfSlider(t *testing.T) {
	if os.Getenv("FLOTER_P13BF_DUMP") == "" {
		t.Skip("opt-in: FLOTER_P13BF_DUMP=1 writes the before/after frames")
	}
	tag := os.Getenv("FLOTER_P13BF_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, disabled := range []bool{false, true} {
			state := "on"
			if disabled {
				state = "off"
			}
			f := sliderFrame(t, dark, disabled)
			path := filepath.Join("/tmp", fmt.Sprintf("floter-P13-bf-slider-%s-%s-%s.png",
				appearance, state, tag))
			out, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(out, f.tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := out.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", path)
		}
	}
}
