package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-eu · 只读行不答指针（`docs/liquid-glass.md` §6.116）。
//
// 一张设置卡上的行分两种：一种有可按的东西，一种只是**报告一件事**（一个由
// 后端铸造的 id、一个程序自己选的值、一段块的标题）。第二种此前与第一种答同
// 一个指针——`cardRow` 无条件问 `Hovered()`/`Pressed()`，而这两个方法说的是
// 「指针在不在这儿」，不是「这行能不能点」。于是只读行的面在指针底下抬得和它
// 下面真的能编辑的字段**一模一样**，按下去也塌，而按下去什么都不发生。
//
// 这两条 pin 量的是同一句话的两种写法：真页面上那行只读的集成 id（工具表单，
// `settingsui.go:5197` 的 `fixedRow`），和一张合成卡上五个只读构造器
// （`factRow` / `keyValue` / `dataRow` / `labelRow` / `fixedRow`）。

// rowInsetFace reads a card row's own face: a three-row band in the row's left
// inset, just left of the label's own box, at the label's vertical centre.
// Reading the inset (not the label's box) keeps ink out of the sample, and
// reading a band (not one pixel) survives a row that resolves to a fractional
// coordinate.
func rowInsetFace(img *image.RGBA, label ui.Rect) ui.Color {
	y := int(label.Y + label.H/2)
	x1 := int(label.X) - 3
	return lensMean(img, x1-3, y-1, x1, y+1)
}

// faceLevels is the signed brightness distance between two faces in the unit
// the contract's tables use: the sum of the three channel deltas.
func faceLevels(a, b ui.Color) int {
	return int(b.R) - int(a.R) + int(b.G) - int(a.G) + int(b.B) - int(a.B)
}

// settlePointer puts the pointer at the centre of a box and lets the 120ms
// colour transition run out, so the next frame reads the settled face.
func settlePointer(tt *ui.Tester, r ui.Rect) {
	tt.Move(float32(r.X+r.W/2), float32(r.Y+r.H/2))
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
}

// TestAReadOnlyRowDoesNotAnswerThePointer is the real page: Integrations →
// Edit… draws the read-only integration id above the tool's own name field.
// The id row must stay on the card's plane under the pointer and under a press,
// while the field right under it still lifts — that second half is the control:
// without it a probe that never reached the pointer at all would pass.
func TestAReadOnlyRowDoesNotAnswerThePointer(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, copy := editFormFixture(t, dark)
		idLabel := mustFind(t, tt, copy.LocalToolId)
		fieldLabel := mustFind(t, tt, copy.LocalToolName)
		restID := rowInsetFace(tt.Image(), idLabel)
		restField := rowInsetFace(tt.Image(), fieldLabel)

		settlePointer(tt, idLabel)
		hotID := rowInsetFace(tt.Image(), idLabel)
		if hotID != restID {
			t.Errorf("%s: the read-only row's face moved %v → %v under the pointer (%+d levels) — "+
				"nothing on that row can be pressed, so the face must stay the card's plane",
				appearance, restID, hotID, faceLevels(restID, hotID))
		}
		tt.Press(float32(idLabel.X+idLabel.W/2), float32(idLabel.Y+idLabel.H/2))
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		pressID := rowInsetFace(tt.Image(), idLabel)
		if pressID != restID {
			t.Errorf("%s: the read-only row's face moved %v → %v under a press (%+d levels) — "+
				"pressing it does nothing, so it must not sink either",
				appearance, restID, pressID, faceLevels(restID, pressID))
		}
		tt.Release(float32(idLabel.X+idLabel.W/2), float32(idLabel.Y+idLabel.H/2))
		tt.Frame()

		// The control: the editable row under it still answers the pointer.
		settlePointer(tt, fieldLabel)
		hotField := rowInsetFace(tt.Image(), fieldLabel)
		if d := faceLevels(restField, hotField); d < 12 && d > -12 {
			t.Errorf("%s: the editable row under the id did not answer the pointer (%v → %v, %+d levels) — "+
				"the probe is not on the row, so the reading above proves nothing",
				appearance, restField, hotField, d)
		}
		t.Logf("%s: read-only %v → %v (hover) → %v (press); editable %v → %v (%+d levels)",
			appearance, restID, hotID, pressID, restField, hotField, faceLevels(restField, hotField))
	}
}

// TestEveryReadOnlyRowFamilyIsInert walks the five constructors that draw a
// row reporting something with nothing to press, on one synthetic card, next to
// an editable row that must still answer.
func TestEveryReadOnlyRowFamilyIsInert(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens, _ := liteTokens(t, dark)
		a := New(settings.NewStore(settings.Default()), Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(realPageGround(t, dark))
			ui.Column(c).Padding(20).Children(func() {
				a.card(c, func() {
					a.factRow(c, "Fact", "fact-value")
					a.keyValue(c, "Key", "key-value")
					a.dataRow(c, "data-value")
					a.labelRow(c, "Label", "label-sub")
					a.fixedRow(c, "Fixed", "fixed-value")
					a.row(c, "Editable", "", func() {})
				})
			})
		}, 480, 480)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Advance(500 * time.Millisecond)
		tt.Frame()

		families := []struct{ name, text string }{
			{"factRow", "Fact"},
			{"keyValue", "Key"},
			{"dataRow", "data-value"},
			{"labelRow", "Label"},
			{"fixedRow", "Fixed"},
		}
		rest := map[string]ui.Color{}
		restInk := map[string]int{}
		boxes := map[string]ui.Rect{}
		for _, family := range families {
			r := mustFind(t, tt, family.text)
			boxes[family.name] = r
			rest[family.name] = rowInsetFace(tt.Image(), r)
			restInk[family.name] = inkSpan(tt, r)
		}
		editable := mustFind(t, tt, "Editable")
		restEditable := rowInsetFace(tt.Image(), editable)

		for _, family := range families {
			settlePointer(tt, boxes[family.name])
			hot := rowInsetFace(tt.Image(), boxes[family.name])
			if hot != rest[family.name] {
				t.Errorf("%s: %s's face moved %v → %v under the pointer (%+d levels) — "+
					"that row reports a value, it offers nothing to press",
					appearance, family.name, rest[family.name], hot, faceLevels(rest[family.name], hot))
			}
			// 只熄面，不动墨（§6.116 第 116-5 行）：另一种「修法」是把只读行整行洗淡，
			// 那会与 §6.10 / §6.84 的「只读是一种结构，不是一种墨」直接冲突。
			if got := inkSpan(tt, boxes[family.name]); got != restInk[family.name] {
				t.Errorf("%s: %s's ink span moved %d → %d under the pointer — a read-only row is a structure, "+
					"not an ink: only the face may change (and here it must not change at all)",
					appearance, family.name, restInk[family.name], got)
			}
		}
		settlePointer(tt, editable)
		hotEditable := rowInsetFace(tt.Image(), editable)
		if d := faceLevels(restEditable, hotEditable); d < 12 && d > -12 {
			t.Errorf("%s: the editable row did not answer the pointer (%v → %v, %+d levels) — "+
				"the probe is not on the card, so the five readings above prove nothing",
				appearance, restEditable, hotEditable, d)
		}
		t.Logf("%s: five read-only families flat; editable %v → %v (%+d levels)",
			appearance, restEditable, hotEditable, faceLevels(restEditable, hotEditable))
	}
}

// TestDumpP13EUReadOnlyHoverPNGs is the round's visual evidence (opt-in:
// `FLOTER_P13EU_DIR`). It writes the tool form twice per appearance: with the
// pointer on the read-only id row, and with it on the editable field under it.
// Run it once on the round's code (`FLOTER_P13EU_TAG=after`) and once with the
// `case face.Inert:` branch reverted to `case false:` (`FLOTER_P13EU_TAG=before`)
// to get the comparison pair.
func TestDumpP13EUReadOnlyHoverPNGs(t *testing.T) {
	dir := os.Getenv("FLOTER_P13EU_DIR")
	if dir == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13EU_DIR=<dir>")
	}
	tag := os.Getenv("FLOTER_P13EU_TAG")
	if tag == "" {
		tag = "after"
	}
	out := filepath.Join(dir, tag)
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, copy := editFormFixture(t, dark)
		idLabel := mustFind(t, tt, copy.LocalToolId)
		fieldLabel := mustFind(t, tt, copy.LocalToolName)
		for _, stage := range []struct {
			name string
			at   ui.Rect
		}{
			{"readonly", idLabel},
			{"editable", fieldLabel},
		} {
			settlePointer(tt, stage.at)
			name := filepath.Join(out, fmt.Sprintf("floter-P13-eu-readonly-%s-%s.png", appearance, stage.name))
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
}
