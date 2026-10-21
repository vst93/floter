package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"floter/internal/theme"
)

// R59's number control (`plugin-config.css:120-164`) is not the framework's
// `ui.NumberInput`: the reference's number lives in the same 28u slot as a text
// field, with its unit riding inside the box and the input right-aligned
// against it. The live browser sheet is the schema with a number field, so the
// slot is read there — the field's own plane and its height by pixel, the unit's
// register by measurement.
//
// P8-b 重定基线（V）：槽的面从 `--glass-field`（比卡亮得多的凹槽）收敛到
// `Control`（§6.1 第 5 条）。几何（28u 高、`RadiusSM`、8u 侧 padding、单位字
// 骑在槽内）全部不变，所以这里的尺寸断言原样保留；只有「槽是一个明显比卡亮
// 的面」这一条改成「槽是同一档 `Control` 洗，边界是 `ControlEdge` 发丝」。
func TestThePluginNumberControlIsTheReferenceGlassSlot(t *testing.T) {
	skipWithoutTheReferenceFont(t)
	copy := i18n.For("en").Settings
	store := newStore(t)
	a := New(store, Actions{})
	schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
	sheet := PluginSheet{Schema: schema, Values: values}

	var tok theme.Tokens
	var unitCapW, unitBodyW float32
	tt := ui.NewTester(func(c *ui.Context) {
		tok = a.tokens(c)
		unitCapW, _ = c.MeasureText(0, ui.Span{Text: copy.UnitDays, Size: tok.Caption})
		unitBodyW, _ = c.MeasureText(0, ui.Span{Text: copy.UnitDays, Size: tok.Theme.FontSize})
		c.Root().Background(ui.Transparent)
		a.PluginSheet(c, sheet)
	}, 720, 1400)
	tt.Frame()

	label, ok := tt.Find(copy.BrowserHistoryDays)
	if !ok {
		t.Fatalf("the number row's label is missing: %q", tt.Texts())
	}
	unit, ok := tt.Find(copy.UnitDays)
	if !ok {
		t.Fatalf("the number's unit is missing: %q", tt.Texts())
	}
	// The unit rides inside the slot, so a hair left of it is the slot's own
	// plane. The sheet is drawn on a transparent root, so the reading is the
	// fill's *alpha* against the card row's: the slot is one `Control` wash
	// denser than the card it sits on, not the old `--glass-field` pane.
	center := unit.Y + unit.H/2
	card := sample(tt, label.X-6, center)
	xs := unit.X - 1
	top, bottom := float32(-1), float32(-1)
	for y := int(center) - 24; y <= int(center)+24; y++ {
		if sample(tt, xs, float32(y)).A > card.A+4 {
			if top < 0 {
				top = float32(y)
			}
			bottom = float32(y)
		}
	}
	if top < 0 {
		t.Fatalf("no field slot around the number's unit %q", copy.UnitDays)
	}
	if h := bottom - top + 1; h < 27 || h > 29 {
		t.Errorf("the number slot's height = %v, want the reference's 28u", h)
	}
	// The unit is the reference's caption register, not the control's body
	// step the old port read it at.
	if absFloat(unit.W-unitCapW) > absFloat(unit.W-unitBodyW) {
		t.Errorf("the unit is not the caption register (unit %v, caption %v, body %v)",
			unit.W, unitCapW, unitBodyW)
	}
}

// R59's slider value (`plugin-config.css:174-192`) is the caption step in the
// muted register with tabular figures, right-aligned in a 48u column — not the
// body step the old port drew it at. The live clipboard sheet's capacity slider
// is read there.
func TestThePluginSliderValueIsTheReferenceCaption(t *testing.T) {
	copy := i18n.For("en").Settings
	store := newStore(t)
	a := New(store, Actions{})
	schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
	sheet := PluginSheet{Schema: schema, Values: values}
	value := boundedText(plugincfg.Field{Unit: copy.UnitItems}, settings.DefaultClipboardMaxItems)

	var capW, bodyW float32
	tt := ui.NewTester(func(c *ui.Context) {
		tok := a.tokens(c)
		capW, _ = c.MeasureText(0, ui.Span{Text: value, Size: tok.Caption})
		bodyW, _ = c.MeasureText(0, ui.Span{Text: value, Size: tok.Theme.FontSize})
		c.Root().Background(ui.Transparent)
		a.PluginSheet(c, sheet)
	}, 720, 700)
	tt.Frame()

	rect, ok := tt.Find(value)
	if !ok {
		t.Fatalf("the slider value %q is missing: %q", value, tt.Texts())
	}
	if absFloat(rect.W-capW) > absFloat(rect.W-bodyW) {
		t.Errorf("the slider value is not the caption register (value %v, caption %v, body %v)",
			rect.W, capW, bodyW)
	}
}

// The number control applies a parseable keystroke at once and clamps it to the
// schema's bounds, and an in-progress (or emptied) field keeps its own text
// while the box has the keyboard instead of being overwritten by the stored
// value mid-edit — the reference's `onChange` normalization plus the draft the
// framework's `ui.NumberInput` kept (`plugins/controls.tsx:130-155`).
func TestThePluginNumberControlAppliesAndClampsTyping(t *testing.T) {
	copy := i18n.For("en").Settings
	store := newStore(t)
	a := New(store, Actions{})
	schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
	sheet := PluginSheet{Schema: schema, Values: values}
	var changes []change
	sheet.Change = func(key string, value any) {
		changes = append(changes, change{key, value})
		sheet.Values[key] = value
	}
	tt := renderSheet(t, a, sheet, 720, 1400)

	unit, ok := tt.Find(copy.UnitDays)
	if !ok {
		t.Fatalf("the number's unit is missing: %q", tt.Texts())
	}
	// Put the caret in the input, at the end of its value, and clear it.
	tt.ClickAt(unit.X-30, unit.Y+unit.H/2)
	tt.Frame()
	if !tt.Focused(copy.BrowserHistoryDays) {
		t.Fatal("the number's input did not take the keyboard")
	}
	clear := func() {
		tt.Key(0, ui.KeyEnd)
		for i := 0; i < 8; i++ {
			tt.Key(0, ui.KeyBackspace)
		}
		tt.Frame()
	}

	// An emptied field keeps the draft: the next digits are what it holds, not
	// appended to a value the field snapped back to.
	clear()
	tt.Type("45")
	tt.Frame()
	if last := changes[len(changes)-1]; last.key != "history_days" || last.value != float64(45) {
		t.Fatalf("typed change = %+v, want history_days=45", last)
	}

	// A value past the schema's bound is clamped on the way out.
	clear()
	tt.Type("99999")
	tt.Frame()
	if last := changes[len(changes)-1]; last.value != float64(settings.MaxBrowserHistoryDays) {
		t.Errorf("out-of-range change = %+v, want the schema's max", last)
	}
}
