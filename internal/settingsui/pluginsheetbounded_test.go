package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/plugincfg"
)

// P13-hk（`docs/liquid-glass.md` §6.186）：读数是「拖一下会存下的那个数」。一个
// 手改过的配置文件把 `max_items` 放到界外时，轨道按框架自己的区间停在头上，读数
// 此前照样印着界外的那个数。收进界之后两者同一条账。
func TestTheBoundedReadoutStaysInsideTheSchemaBounds(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := New(newStore(t), Actions{})
		schema := &plugincfg.Schema{Plugin: "probe", Title: "Probe", Fields: []plugincfg.Field{{
			Key: "n", Kind: plugincfg.Slider, Label: "Number",
			Min: 10, Max: 1000, Step: 10, Unit: "items",
		}}}
		sheet := PluginSheet{Schema: schema, Values: plugincfg.Values{"n": 9999.0}}
		tt := sheetTester(t, a, &sheet, 720, 400)
		tt.SetDark(dark)
		tt.Frame()
		if !tt.HasText("1000 items") {
			t.Fatalf("dark=%v: the readout does not show the stored value clamped to the bound: %q", dark, tt.Texts())
		}
		if tt.HasText("9999 items") {
			t.Fatalf("dark=%v: the readout still prints a value the schema does not allow", dark)
		}
	}
}

// P13-hk（§6.186）：量化网格锚在字段自己的 `Min` 上，不是锚在 0 上——与
// `plugincfg.coerce` 的 `clampStep` 同一条。`Min` 落在步长网格上时两者逐字节相
// 同；`Min: 1, Step: 10` 的字段此前会把 1.5 量化成 0，一个比下界还小、宿主立刻夹
// 回 1 的数。
func TestTheStepGridIsAnchoredAtTheMinimum(t *testing.T) {
	field := plugincfg.Field{Key: "n", Kind: plugincfg.Number, Min: 1, Max: 99, Step: 10, Unit: "u"}
	for _, v := range []float64{1.5, 5.5, 6, 0.4} {
		if got := quantize(field, v); got < field.Min {
			t.Errorf("quantize(%v) = %v, below the field's minimum %v", v, got, field.Min)
		}
	}
	if got := quantize(field, 11.5); got != 11 {
		t.Errorf("quantize(11.5) = %v, want 11 on the grid anchored at 1", got)
	}
	if got := boundedText(field, 1.5); got != "1 u" {
		t.Errorf("boundedText(1.5) = %q, want %q", got, "1 u")
	}
	if got := initialNumberText(plugincfg.Field{Key: "n", Kind: plugincfg.Number, Min: 1, Max: 65535, Step: 1}, 70000); got != "65535" {
		t.Errorf("initialNumberText(70000) = %q, want %q", got, "65535")
	}
	if got := initialNumberText(plugincfg.Field{Key: "n", Kind: plugincfg.Number}, 30); got != "30" {
		t.Errorf("initialNumberText(30) = %q, want %q (no declared bounds)", got, "30")
	}
	// 量化之后仍在声明过的界内：96 最近的网格点是 100，而 Max 是 99。
	bounded := plugincfg.Field{Key: "n", Kind: plugincfg.Slider, Min: 0, Max: 99, Step: 10}
	if got := boundedValue(bounded, 96); got != 99 {
		t.Errorf("boundedValue(96) = %v, want 99 (the declared maximum)", got)
	}
	if got := boundedText(bounded, 96); got != "99" {
		t.Errorf("boundedText(96) = %q, want %q", got, "99")
	}
	// 没有写界（`Min` 与 `Max` 都是零值）的字段不夹：`Field{}` 是「没界」，不是 0。
	if got := boundedText(plugincfg.Field{Step: 1, Unit: "days"}, 30); got != "30 days" {
		t.Errorf("boundedText(30) = %q, want %q", got, "30 days")
	}
	// A minimum that sits on the grid keeps its old spelling.
	plain := plugincfg.Field{Key: "n", Kind: plugincfg.Number, Min: 0, Max: 1000, Step: 10, Unit: "items"}
	if got := boundedText(plain, 300); got != "300 items" {
		t.Errorf("boundedText(300) = %q, want %q", got, "300 items")
	}
}

// P13-hk（§6.186）：数字框交给宿主的数与滑块交给宿主的是同一条账——落在该字段
// 自己的网格上、收进它声明过的界。此前只夹界，`Step: 1` 的字段把 3.7 交上去，
// 宿主再按网格收成 4：这一帧的派发值与插件实际存下的数不是同一个。
func TestTheNumberBoxHandsOverTheSnappedValue(t *testing.T) {
	a := New(newStore(t), Actions{})
	schema := &plugincfg.Schema{Plugin: "probe", Title: "probe", Fields: []plugincfg.Field{{
		Key: "n", Kind: plugincfg.Number, Label: "Number",
		Min: 0, Max: 100, Step: 1, Unit: "days",
	}}}
	var changes []change
	sheet := PluginSheet{Schema: schema, Values: plugincfg.Values{"n": 1.0}}
	sheet.Change = func(key string, value any) {
		changes = append(changes, change{key, value})
	}
	tt := sheetTester(t, a, &sheet, 720, 300)
	row := mustFind(t, tt, "Number")
	tt.ClickAt(720-70, row.Y+row.H/2)
	tt.Frame()
	if !tt.Focused("Number") {
		t.Fatalf("the number box did not take focus: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyBackspace)
	tt.Type("3.7")
	tt.Frame()
	if len(changes) == 0 {
		t.Fatal("typing into the number box dispatched nothing")
	}
	last, _ := changes[len(changes)-1].value.(float64)
	if last != 4 {
		t.Fatalf("the number box handed over %v; the field's own grid and bounds say 4 (%v)", last, changes)
	}
}
