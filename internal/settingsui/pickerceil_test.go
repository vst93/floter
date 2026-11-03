package settingsui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/launcher"
	"floter/internal/settings"
)

// P13-fr（`docs/liquid-glass.md` §6.137）：P13-fq 的「仍待」第 1 条问的是
// `chooseSelect` 少了 `paletteRow` 拿到的那条天花板（`Shrink(1)`，§6.135）——
// 「长值会不会把字体族 picker 顶出卡外」。
//
// 先量（720 宽真实 General 页，四台阶 × 深浅；92 字的族名对出厂的
// `JetBrains Mono`）：
//
//	台阶     出厂：标签列右沿  picker 盒    92 字：标签列右沿  picker 盒
//	tiny          464.8          215            194.8          497
//	small         460.4          218            200.4          491
//	default       456.0          216            206.0          485
//	large         451.6          220            211.6          480
//
// 两条答案：
//
// ① **`Shrink(1)` 在这里是空操作**：把 `chooseSelect` 的 `MinWidth(220)` 改成
// `MinWidth(220).Shrink(1)`，八个 arm（四台阶 × 深浅）的整帧 CRC32 **逐位相同**
// ——因为 picker 是控制槽（`rows.go:1617` `ui.Row(c).Shrink(0)`）的**唯一孩子**，
// 槽自己的盒子就是 picker 的盒子，槽不收缩，picker 收缩也没人压它。所以天花板
// 这件事不在 picker 身上，在槽身上（见 ③）。
//
// ② **画面不出卡**：出厂帧与 92 字帧逐列比，最后一列不同的位置是
// 693 / 690 / 687 / 684——正好是同一张卡「14 px」读数的右沿（行内容盒右沿，
// §6.136 量过 693.4 / 690.2 / 687.0 / 683.8），再往右（含卡片自己的右框 703.8）
// 两帧**逐像素相同**。也就是说长值最多画到行内容盒的右沿为止，卡框不被它碰到。
//
// ③ 剩下的是另一件事（**需拍板**，`.pending-bugs.md`）：值长到框装不下时，框按
// 自然宽排版（`ui/layout.go:200` 用元素自己的 width），行边把这段文字**切断**，
// 而不是像 paletteRow 那样在自己的 220 地板里收口成省略号。要改成后者，得让
// 控制槽在标签列让尽之后可以收缩到 220——那是 §6.132 钉住的 `Shrink(0)`
// （「控件不收缩，让标签列先让」）的语义改动，不是这一轮能单方面决定的。

// pickerCeilFrame draws the General page at one step with one font family and
// returns the frame, the label column's right edge (where the picker starts),
// the picker's painted box, and the card's control column (the font-size
// readout's right edge, i.e. the row content box's right edge).
func pickerCeilFrame(t *testing.T, step, family string, dark bool) (*ui.Tester, float32, int, float32) {
	t.Helper()
	en := i18n.For("en").Settings
	s := settings.Default()
	s.UIScale = step
	s.FontFamily = family
	a := New(settings.NewStore(s), Actions{
		SetIntegrationEnabled: func(string, bool, func(error)) {},
		SetCommandEnabled:     func(string, string, bool) {},
	})
	a.Page = PageGeneral
	tt, _ := renderIn(t, a, launcher.InputWindowWidth, 1400, dark)
	label := mustFind(t, tt, en.FontFamily)
	box := pillBox(tt.Image(), label)
	if box < 100 {
		t.Fatalf("step=%s/dark=%v family=%q: the picker's face is only %dpx wide — the box could not be "+
			"read off the frame", step, dark, family, box)
	}
	readout := mustFind(t, tt, "14 px")
	return tt, label.X + label.W, box, readout.X + readout.W
}

// ceilLongFamily is longer than any row can hold at 720: 92 characters.
const ceilLongFamily = "Iosevka Term Extended Nerd Font Mono " +
	"WWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWWW"

// TestThePickerNeverPaintsPastTheRowColumn pins answer ②: whatever the value
// does to its own box, the painted frame stops at the row content box's right
// edge. The shipped frame and the 92-character frame are compared column by
// column across the picker's own band; the last column where they differ must be
// the card's control column (not one pixel past it, and not far short of it —
// the long value does reach the column, it just does not pass it).
func TestThePickerNeverPaintsPastTheRowColumn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, step := range []string{"tiny", "small", "default", "large"} {
			shipped, _, _, column := pickerCeilFrame(t, step, "JetBrains Mono", dark)
			long, _, _, _ := pickerCeilFrame(t, step, ceilLongFamily, dark)
			label := mustFind(t, shipped, i18n.For("en").Settings.FontFamily)
			y0, y1 := int(label.Y)-6, int(label.Y+label.H)+6
			a, b := shipped.Image(), long.Image()
			last := -1
			for x := 0; x < a.Bounds().Dx(); x++ {
				for y := y0; y < y1; y++ {
					if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
						last = x
						break
					}
				}
			}
			if last < 0 {
				t.Fatalf("step=%s/dark=%v: a 92-character family paints exactly like the shipped one — "+
					"the value never reached the picker", step, dark)
			}
			if float32(last) > column+0.5 {
				t.Errorf("step=%s/dark=%v: the long value's frame reaches x=%d but the row's content box ends "+
					"at %.1f (%+.1fpx): the picker is painting outside the card's column",
					step, dark, last, column, float32(last)-column)
			}
			if float32(last) < column-3 {
				t.Errorf("step=%s/dark=%v: the long value's frame stops at x=%d, %.1fpx short of the row's "+
					"content edge (%.1f): the picker no longer uses the whole column",
					step, dark, last, column-float32(last), column)
			}
		}
	}
}

// TestTheLabelColumnYieldsFirstAndThePickerHasNoCeilingOfItsOwn pins answer ①:
// the label column is the elastic half (§6.132) — a value too long for the row
// eats the label column down to its own ink, and the picker's box grows by the
// same amount. The shipped/long pairs are the numbers measured this round, so a
// change in either half shows up here with the amount it moved.
func TestTheLabelColumnYieldsFirstAndThePickerHasNoCeilingOfItsOwn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for i, step := range []string{"tiny", "small", "default", "large"} {
			_, shippedRight, shippedBox, _ := pickerCeilFrame(t, step, "JetBrains Mono", dark)
			_, longRight, longBox, _ := pickerCeilFrame(t, step, ceilLongFamily, dark)
			// The label column's right edge in each arm (where the picker starts).
			wantShip := []float32{464.8, 460.4, 456.0, 451.6}[i]
			wantLong := []float32{194.8, 200.4, 206.0, 211.6}[i]
			if absFloat(shippedRight-wantShip) > 1.5 {
				t.Errorf("step=%s/dark=%v: the label column ends at %.1f, pinned at %.1f (shipped family)",
					step, dark, shippedRight, wantShip)
			}
			if absFloat(longRight-wantLong) > 1.5 {
				t.Errorf("step=%s/dark=%v: with a 92-character family the label column ends at %.1f, pinned "+
					"at %.1f — the label column's floor is its own ink and it may not move",
					step, dark, longRight, wantLong)
			}
			// The picker's box: a floor of 220 for shipped values, and whatever the
			// value needs (up to the row) for the long one. Nothing caps it.
			if shippedBox < 210 || shippedBox > 230 {
				t.Errorf("step=%s/dark=%v: the shipped family draws a %dpx picker, pinned at 215…220 "+
					"(the 220 floor): the picker's box stopped coming from its floor", step, dark, shippedBox)
			}
			wantLongBox := []int{497, 491, 485, 480}[i]
			if diff := longBox - wantLongBox; diff < -6 || diff > 6 {
				t.Errorf("step=%s/dark=%v: a 92-character family draws a %dpx picker, pinned at %d (±6): "+
					"the picker gained a ceiling of its own — if that is the intent, say so in the contract "+
					"(§6.137 ③) and re-pin it", step, dark, longBox, wantLongBox)
			}
			// Both halves moved, and by the same order: the label column gave up
			// ~240…270px and the picker took ~250…280px (the 10…20px difference is
			// the pill's painted face against its box — `pillBox` reads the face, and
			// the shipped pill sits just under its 220 floor). A row where only one
			// half moves is a different row.
			gave, took := shippedRight-longRight, longBox-shippedBox
			if gave < 200 || took < 200 {
				t.Errorf("step=%s/dark=%v: the label column gave up %.1fpx and the picker took %dpx — the "+
					"row is no longer one elastic column plus one control", step, dark, gave, took)
			}
		}
	}
}

// TestChooseSelectHasNoShrinkOfItsOwn records answer ① as a source pin: adding
// `Shrink(1)` to this picker is a measured no-op (eight arms, byte-identical
// frames), because the picker is the only child of a `Shrink(0)` control slot.
// If someone later moves the ceiling onto the slot, this pin is the one to
// re-read — the comment above it says why the picker is the wrong place.
func TestChooseSelectHasNoShrinkOfItsOwn(t *testing.T) {
	b, err := os.ReadFile("settingsui.go")
	if err != nil {
		t.Fatal(err)
	}
	body := funcBody(t, string(b), "chooseSelect")
	if strings.Contains(body, ".Shrink(") {
		t.Errorf("chooseSelect grew a Shrink of its own: it is the only child of the control slot " +
			"(`rows.go` `ui.Row(c).Shrink(0)`), so a shrink here is a no-op — the ceiling, if the row " +
			"needs one, belongs on the slot (§6.137)")
	}
	if !strings.Contains(body, ".MinWidth(220)") {
		t.Errorf("chooseSelect lost its 220 floor: the picker's box stopped coming from the reference's " +
			"`max-width` (settings.css:919-940, §6.133)")
	}
}

// The round's visual evidence: the font-family picker holding the shipped value
// and a value no row can hold, at the shipped step and the largest one, both
// appearances. Opt-in so a normal run does not litter /tmp.
func TestDumpP13FRPickerCeiling(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, arm := range []struct {
		step, name string
		dark       bool
	}{
		{"default", "light", false},
		{"default", "dark", true},
		{"large", "light", false},
		{"large", "dark", true},
	} {
		for _, f := range []struct{ tag, family string }{
			{"ship", "JetBrains Mono"},
			{"long", ceilLongFamily},
		} {
			tt, _, _, _ := pickerCeilFrame(t, arm.step, f.family, arm.dark)
			writePNG(t, filepath.Join("/tmp", "floter-P13-fr-pickerceiling-"+arm.step+"-"+arm.name+"-"+f.tag+".png"), tt)
		}
	}
}
