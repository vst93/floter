package settingsui

import (
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-ch · 参数编辑器的空槽说的是一句**结论**，却和它上面的**提示**一样淡。
//
// `ParamsEmpty`（`parameditor.go:81-93`，参考 `.extension-argument-editor__empty`
// `extensions.css:1860`）在参考里是 `--text-caption` + `--text-tertiary`，而本仓
// 的 P8-b（`docs/raycast-design.md` §4.7「陈述压过提示」）把**空态的句子**抬到了
// `TextStrong`：会话页、自定义快捷键、集成页、`custom.go:50` 四处都走
// `emptyState` 的陈述档，只有这一处留在参考旧值上。
//
// 量出来的事实不是「它比较淡」，是**它与提示逐像素同值**：浅 100/100、深 96/96
// 级差。一张卡里最淡的两行分别是「说明」和「结论」，结论没有比说明重一级。
//
// 三条 pin（深浅各跑一遍，全部量像素）：
//   - `TestTheEmptySlotStatesItsConclusion`：结论的墨级差 ≥ 提示 + 60，且结论
//     **不弱于**卡片的标题（标题是这一段的名字，结论是这一段的话）。
//   - `TestTheEmptySlotKeepsItsBox`：槽的几何不变——1px 虚线 + 8u padding，盒子
//     高 33（1 + 8 + 15 + 8 + 1）。抬的是墨，不是格子。
//   - `TestTheEmptySlotKeepsItsDashedEdge`：虚线边仍比面色深 ≥ 15 级（0.09 的发丝线
//     压在同一张面上；P13-ch 量到 22 级）。这条防的是「顺手把槽也调亮/调没」。

// inkStep is how far the furthest pixel in box is from face, in the same
// 0..255 brightness the rest of the suite reads. It is the *ink* reading: a text
// box's own antialiased edge is included, which is exactly what the eye sees.
func inkStep(img *image.RGBA, box image.Rectangle, face int) int {
	best := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			c := img.RGBAAt(x, y)
			v := (int(c.R) + int(c.G) + int(c.B)) / 3
			d := v - face
			if d < 0 {
				d = -d
			}
			if d > best {
				best = d
			}
		}
	}
	return best
}

// inkPixels counts the pixels of a text box that are at least floor levels off
// the face. It is the *weight* reading: two runs of one sentence at 400 and 580
// share their darkest pixel (the ink colour is the same token) but not the
// amount of ink the glyphs put down — measured 512 at 400 and 611/619 at 580.
func inkPixels(img *image.RGBA, box image.Rectangle, face, floor int) int {
	n := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			d := pixelLevel(img, x, y) - face
			if d < 0 {
				d = -d
			}
			if d >= floor {
				n++
			}
		}
	}
	return n
}

func pixelLevel(img *image.RGBA, x, y int) int {
	c := img.RGBAAt(x, y)
	return (int(c.R) + int(c.G) + int(c.B)) / 3
}

func boxOf(r ui.Rect) image.Rectangle {
	return image.Rect(int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
}

// paramEmptyFrame opens the declared-inputs editor on a tool that declares
// nothing: the state this round is about.
func paramEmptyFrame(t *testing.T, dark bool) (*ui.Tester, i18n.Settings) {
	t.Helper()
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{ConnectLocalTool: func(tool CustomTool) {}})
	tt := render(t, a, 720, 3200)
	tt.SetDark(dark)
	tt.Frame()
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	return tt, copy
}

// hairlineRow reports whether row y carries a hairline somewhere in x0..x1:
// 12..40 levels off the face, the band a 0.09 rule lands in (the slot's edge is
// 22). A glyph's antialiased rim also passes through that band, so every caller
// also checks the rows on either side are *pure face* — a 1px rule has no
// neighbours, a glyph has plenty.
func hairlineRow(img *image.RGBA, y, x0, x1, face int) bool {
	for x := x0; x <= x1; x++ {
		d := pixelLevel(img, x, y) - face
		if d < 0 {
			d = -d
		}
		if d >= 12 && d <= 40 {
			return true
		}
	}
	return false
}

// noHairlineRow is pureFaceRow with the antialiasing allowance a 1px rule needs:
// a rule that lands on a half pixel bleeds two levels into its neighbour row, and
// that is still "no rule here" — the band that counts starts at 12.
func noHairlineRow(img *image.RGBA, y, x0, x1, face int) bool {
	for x := x0; x <= x1; x++ {
		d := pixelLevel(img, x, y) - face
		if d < 0 {
			d = -d
		}
		if d >= 12 {
			return false
		}
	}
	return true
}

func TestTheEmptySlotStatesItsConclusion(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, copy := paramEmptyFrame(t, dark)
		img := tt.Image()
		hint, ok := tt.Find(copy.ParamsHint)
		if !ok {
			t.Fatalf("dark=%v: the hint is missing: %q", dark, tt.Texts())
		}
		statement, ok := tt.Find(copy.ParamsEmpty)
		if !ok {
			t.Fatalf("dark=%v: the empty state is missing: %q", dark, tt.Texts())
		}
		heading, ok := tt.Find(copy.ParamsTitle)
		if !ok {
			t.Fatalf("dark=%v: the heading is missing: %q", dark, tt.Texts())
		}
		cardFace := pixelLevel(img, 400, int(hint.Y)-4)
		hintStep := inkStep(img, boxOf(hint), cardFace)
		// The statement's own face is the slot's: the dashed box is transparent,
		// so it is the card's face too — read it inside the box to be sure.
		statementStep := inkStep(img, boxOf(statement), cardFace)
		headingStep := inkStep(img, boxOf(heading), cardFace)
		// P13-ch 重定基线：旧值 → 新值 浅 100 → 219 / 深 96 → 221（提示不动）。
		// 60 级是「一眼看得出」的门槛：同一族的两档墨在这个面上相隔 100 级，
		// 抬过之后至少要与提示隔开六成。
		if statementStep < hintStep+60 {
			t.Errorf("dark=%v: the empty state is %d levels off the face, its hint above it is %d"+
				" — a conclusion is not a second hint (§4.7)", dark, statementStep, hintStep)
		}
		if statementStep < headingStep {
			t.Errorf("dark=%v: the conclusion (%d) is lighter than the heading that names the section (%d)",
				dark, statementStep, headingStep)
		}
		// The weight is the half of the statement rung the ink step cannot see:
		// the same token at 400 measured 512 ink pixels, at 580 it is 611 (light)
		// / 619 (dark). 560 is the floor, not the reading.
		if ink := inkPixels(img, boxOf(statement), cardFace, 60); ink < 560 {
			t.Errorf("dark=%v: the conclusion puts down %d ink pixels, want the 580 weight's 560+ (400 was 512)",
				dark, ink)
		}
		t.Logf("dark=%v: heading %d, hint %d, conclusion %d, ink %d",
			dark, headingStep, hintStep, statementStep, inkPixels(img, boxOf(statement), cardFace, 60))
	}
}

func TestTheEmptySlotKeepsItsBox(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, copy := paramEmptyFrame(t, dark)
		img := tt.Image()
		statement, ok := tt.Find(copy.ParamsEmpty)
		if !ok {
			t.Fatalf("dark=%v: the empty state is missing: %q", dark, tt.Texts())
		}
		// The statement keeps the caption's own line box: this round moved ink
		// and weight, not the type step (§6.51 判据 1).
		if statement.H != 15 {
			t.Errorf("dark=%v: the statement's line box is %.0f, want the caption's 15", dark, statement.H)
		}
		x0, x1 := int(statement.X)-9, int(statement.X)+int(statement.W)+2
		y, h := int(statement.Y), int(statement.H)
		face := pixelLevel(img, int(statement.X)+int(statement.W)-2, y+2)
		// 1px border + 8u padding on each side: the rule is at -9 and +8, and its
		// neighbours are face. Reading the neighbours is what makes this a pin on
		// the *box* rather than on "there is something dark nearby".
		if !hairlineRow(img, y-9, x0, x1, face) {
			t.Errorf("dark=%v: no rule 9px above the statement (the slot's top edge)", dark)
		}
		if !noHairlineRow(img, y-10, x0, x1, face) || !noHairlineRow(img, y-8, x0, x1, face) {
			t.Errorf("dark=%v: the slot's top edge is not a 1px rule (neighbour rows are not face)", dark)
		}
		if !hairlineRow(img, y+h+8, x0, x1, face) {
			t.Errorf("dark=%v: no rule 8px under the statement (the slot's bottom edge)", dark)
		}
		if !noHairlineRow(img, y+h+7, x0, x1, face) || !noHairlineRow(img, y+h+9, x0, x1, face) {
			t.Errorf("dark=%v: the slot's bottom edge is not a 1px rule (neighbour rows are not face)", dark)
		}
		// 1 + 8 + 15 + 8 + 1 = 33.
		t.Logf("dark=%v: slot %d..%d = 33, statement %v", dark, y-9, y+h+8, statement)
	}
}

func TestTheEmptySlotKeepsItsDashedEdge(t *testing.T) {
	for _, dark := range []bool{false, true} {
		tt, copy := paramEmptyFrame(t, dark)
		img := tt.Image()
		statement, ok := tt.Find(copy.ParamsEmpty)
		if !ok {
			t.Fatalf("dark=%v: the empty state is missing: %q", dark, tt.Texts())
		}
		y, h := int(statement.Y), int(statement.H)
		face := pixelLevel(img, int(statement.X)+int(statement.W)-2, y+2)
		x := int(statement.X) - 9
		step, runs := 0, 0
		on := false
		for row := y - 8; row < y+h+8; row++ {
			d := pixelLevel(img, x, row) - face
			if d < 0 {
				d = -d
			}
			if d > step {
				step = d
			}
			if d >= 12 {
				if !on {
					runs++
				}
				on = true
			} else {
				on = false
			}
		}
		// P13-ch 量到：面色 246、虚线 224（深 15 / 19），即 22 级；3 on / 3 off。
		if step < 15 {
			t.Errorf("dark=%v: the slot's dashed edge is %d levels off its face — the slot is not a slot any more",
				dark, step)
		}
		if runs < 3 {
			t.Errorf("dark=%v: the slot's left edge is drawn in %d runs, want a dash (3 on / 3 off)", dark, runs)
		}
	}
}

// The round's visual evidence, opt-in for the same reason as every other dump:
// a test run should not litter /tmp.
func TestDumpP13CHEmptySlotPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, copy := paramEmptyFrame(t, dark)
		hint, _ := tt.Find(copy.ParamsHint)
		img := tt.Image()
		// The card's own band around the slot: the heading, the hint and the slot.
		crop := image.Rect(0, int(hint.Y)-48, img.Bounds().Max.X, int(hint.Y)+140)
		out, err := os.Create("/tmp/floter-P13-ch-paramempty-" + appearance + ".png")
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(out, img.SubImage(crop)); err != nil {
			t.Fatal(err)
		}
		out.Close()
	}
}
