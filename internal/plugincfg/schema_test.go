package plugincfg

import (
	"math"
	"reflect"
	"testing"

	"floter/internal/i18n"
	"floter/internal/settings"
)

func copyFor(language string) i18n.Settings {
	return i18n.For(language).Settings
}

func TestSectionsGroupConsecutiveFields(t *testing.T) {
	schema, _ := Browser(copyFor("en"), settings.DefaultBrowserPlugin(), nil)
	sections := Sections(schema)
	if len(sections) != 4 {
		t.Fatalf("sections = %d, want 4", len(sections))
	}
	for i, section := range sections {
		if section.Title == "" {
			t.Errorf("section %d is untitled", i)
		}
		if len(section.Fields) == 0 {
			t.Errorf("section %d has no fields", i)
		}
	}
	// Order is schema order: the fields of the first section come first.
	if got := sections[0].Fields[0].Key; got != "enabled" {
		t.Errorf("first field = %q, want enabled", got)
	}
	if got := sections[1].Fields[0].Key; got != "target" {
		t.Errorf("second section's first field = %q, want target", got)
	}
	// Every field lands in exactly one section, in order.
	var keys []string
	for _, section := range sections {
		for _, f := range section.Fields {
			keys = append(keys, f.Key)
		}
	}
	if len(keys) != len(schema.Fields) {
		t.Fatalf("grouped %d fields, want %d", len(keys), len(schema.Fields))
	}
	for i, f := range schema.Fields {
		if keys[i] != f.Key {
			t.Errorf("field %d = %q, want %q", i, keys[i], f.Key)
		}
	}
}

func TestClipboardSchemaIsOneUntitledCard(t *testing.T) {
	schema, values := Clipboard(copyFor("en"), settings.DefaultClipboardSettings())
	if len(schema.Fields) != 3 {
		t.Fatalf("fields = %d, want 3", len(schema.Fields))
	}
	if sections := Sections(schema); len(sections) != 1 || sections[0].Title != "" {
		t.Errorf("clipboard grouped as %d sections, want one untitled", len(sections))
	}
	if values["enabled"] != true {
		t.Errorf("enabled = %v, want true", values["enabled"])
	}
	if values["max_items"] != float64(settings.DefaultClipboardMaxItems) {
		t.Errorf("max_items = %v", values["max_items"])
	}
	// The action holds no value: it is a command, not a field to store.
	if _, ok := values["clear_history"]; ok {
		t.Error("an action field carries a value")
	}
}

func TestApplyCoercesPerKind(t *testing.T) {
	schema, values := Clipboard(copyFor("en"), settings.DefaultClipboardSettings())
	next := schema.Apply(values, "max_items", 400.0)
	if next["max_items"] != 400.0 {
		t.Errorf("max_items = %v, want 400", next["max_items"])
	}
	// The original is untouched: an edit copies.
	if values["max_items"] == 400.0 {
		t.Error("Apply wrote through the values it was given")
	}
	// A value past the bounds is clamped to them.
	if got := schema.Apply(values, "max_items", 1e9)["max_items"]; got != float64(settings.MaxClipboardMaxItems) {
		t.Errorf("clamped = %v, want %v", got, settings.MaxClipboardMaxItems)
	}
	// The wrong shape for the kind is refused, not coerced.
	if got := schema.Apply(values, "max_items", "400")["max_items"]; got != values["max_items"] {
		t.Errorf("a string reached a slider: %v", got)
	}
	if got := schema.Apply(values, "enabled", "yes")["enabled"]; got != true {
		t.Errorf("a string reached a toggle: %v", got)
	}
	// A key the schema does not declare changes nothing.
	if got := schema.Apply(values, "nonsense", true); len(got) != len(values) {
		t.Errorf("an undeclared key was added: %v", got)
	}
	// An action is not writable.
	if got := schema.Apply(values, "clear_history", true); len(got) != len(values) {
		t.Errorf("an action field was written: %v", got)
	}
}

func TestApplyRejectsAValueOutsideTheOptions(t *testing.T) {
	schema, values := Calculator(copyFor("en"), settings.DefaultCalculatorPlugin())
	if got := schema.Apply(values, "copy_mode", "result")["copy_mode"]; got != "result" {
		t.Errorf("copy_mode = %v, want result", got)
	}
	if got := schema.Apply(values, "copy_mode", "everything")["copy_mode"]; got == "everything" {
		t.Error("a radio took a value its options do not name")
	}
	if got := schema.Apply(values, "retention_days", "7")["retention_days"]; got != "7" {
		t.Errorf("retention_days = %v, want 7", got)
	}
	if got := schema.Apply(values, "retention_days", "3")["retention_days"]; got == "3" {
		t.Error("a select took a value its options do not name")
	}
}

// A value of the right shape is *shaped*, not just stored: the old build's
// `normalizeConfigValue` (`main:src/plugins/config-schema.ts:448-500`) trimmed
// text, snapped bounded numbers to their step grid, and put a check box set
// back into schema order. The one deliberate difference is that a value of the
// wrong shape (or an unknown select/radio option) is still refused rather than
// silently replaced — see TestApplyRejectsAValueOutsideTheOptions.
func TestCoerceShapesValuesLikeTheReferenceNormalizer(t *testing.T) {
	schema := &Schema{Fields: []Field{
		{Key: "label", Kind: Text},
		{Key: "count", Kind: Number, Min: 10, Max: 50, Step: 10},
		{Key: "ratio", Kind: Slider, Min: 0, Max: 1, Step: 0.1},
		{Key: "tags", Kind: Checkboxes, Options: []Option{
			{Value: "a"}, {Value: "b"}, {Value: "c"},
		}},
	}}
	base := Values{"label": "x", "count": 10.0, "ratio": 0.0, "tags": []string{}}
	cases := []struct {
		name string
		key  string
		raw  any
		want any
	}{
		{"a text value is trimmed", "label", "  hi  ", "hi"},
		{"a whitespace-only text is no value", "label", "   \t ", nil},
		{"a number snaps down to its step grid", "count", 33.0, 30.0},
		{"a number snaps up to its step grid", "count", 36.0, 40.0},
		{"a snapped number still clamps to the bounds", "count", 99.0, 50.0},
		{"a non-finite number falls back to the minimum", "count", math.NaN(), 10.0},
		{"a floating step is rounded to six decimals", "ratio", 0.3, 0.3},
		{"checkboxes de-duplicate and follow schema order", "tags", []string{"c", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"an unknown check box is dropped", "tags", []string{"z", "b"}, []string{"b"}},
		{"a check box set with nothing valid is empty", "tags", []string{"z"}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := schema.Apply(base, tc.key, tc.raw)[tc.key]
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s(%#v) = %#v, want %#v", tc.key, tc.raw, got, tc.want)
			}
		})
	}
}

// The load path hands the schema the values the typed settings hold, not raw
// JSON; this pins that those values are already the shape `coerce` would
// accept, so a value a control reads and a value it writes agree. A shipped
// bounded number is already on its step grid, so loading one canonicalizes
// nothing.
func TestTheShippedValuesAlreadyFitTheSchema(t *testing.T) {
	en := copyFor("en")
	clipboardSchema, clipboardValues := Clipboard(en, settings.DefaultClipboardSettings())
	calculatorSchema, calculatorValues := Calculator(en, settings.DefaultCalculatorPlugin())
	browserSchema, browserValues := Browser(en, settings.DefaultBrowserPlugin(), nil)
	cases := []struct {
		name   string
		schema *Schema
		values Values
	}{
		{"clipboard", clipboardSchema, clipboardValues},
		{"calculator", calculatorSchema, calculatorValues},
		{"browser", browserSchema, browserValues},
	}
	for _, tc := range cases {
		for _, f := range tc.schema.Fields {
			raw, ok := tc.values[f.Key]
			if !ok {
				continue // an action holds no value
			}
			got, ok := coerce(f, raw)
			if !ok {
				t.Errorf("%s.%s: the shipped value %#v is not a %v", tc.name, f.Key, raw, f.Kind)
				continue
			}
			if f.Kind == Slider || f.Kind == Number {
				if got != raw {
					t.Errorf("%s.%s: shipped %v is off its step grid (would become %v)", tc.name, f.Key, raw, got)
				}
			}
		}
	}
}

// The overlay's budget is the old build's: `result-budget.ts` charged a
// header, a padding pair, a heading per titled section, a gap between
// sections, and one row per field at the worst case for its kind.
func TestContentUnitsMatchTheOldBudget(t *testing.T) {
	clipboard, _ := Clipboard(copyFor("en"), settings.DefaultClipboardSettings())
	// 28 header + 16 padding + 49 toggle + 73 slider + 49 action.
	if got := clipboard.ContentUnits(); got != 215 {
		t.Errorf("clipboard content = %v units, want 215", got)
	}
	browser, _ := Browser(copyFor("en"), settings.DefaultBrowserPlugin(), nil)
	// 28 + 16 + 4 headings × 25 + 3 gaps × 12
	//   + 49 + 49 + 85 + 49 + 91 + 91 + 49 + 49.
	want := 28.0 + 16 + 4*25 + 3*12 + 49 + 49 + 85 + 49 + 91 + 91 + 49 + 49
	if got := browser.ContentUnits(); got != want {
		t.Errorf("browser content = %v units, want %v", got, want)
	}
	// The height is the units at the step, plus each card's two edges (four
	// sections).
	height := browser.Height(1, 0)
	if height != want+8 {
		t.Errorf("browser height = %v, want %v", height, want+8)
	}
	// The display's cap binds.
	if capped := browser.Height(1, 400); capped != 400 {
		t.Errorf("capped height = %v, want 400", capped)
	}
	// A smaller step is a shorter sheet.
	if small := browser.Height(0.9, 0); small >= height {
		t.Errorf("the small step is %v, not shorter than %v", small, height)
	}
}

func TestCalculatorSchemaBoundsTheCapacity(t *testing.T) {
	schema, values := Calculator(copyFor("en"), settings.DefaultCalculatorPlugin())
	field, ok := schema.Field("max_items")
	if !ok {
		t.Fatal("no max_items field")
	}
	if field.Kind != Slider || field.Step != 10 {
		t.Errorf("max_items = kind %v step %v, want a slider stepping by 10", field.Kind, field.Step)
	}
	if field.Min != float64(settings.MinCalculatorMaxItems) || field.Max != float64(settings.MaxCalculatorMaxItems) {
		t.Errorf("max_items bounds = %v..%v", field.Min, field.Max)
	}
	if field.Unit != copyFor("en").UnitItems {
		t.Errorf("max_items unit = %q", field.Unit)
	}
	if values["copy_mode"] != settings.DefaultCalculatorPlugin().CopyMode {
		t.Errorf("copy_mode = %v", values["copy_mode"])
	}
}

// A plugin's schema is named in the user's language: the same field carries
// the same key and a different label.
func TestSchemasAreTranslated(t *testing.T) {
	en, _ := Clipboard(copyFor("en"), settings.DefaultClipboardSettings())
	zh, _ := Clipboard(copyFor("zh"), settings.DefaultClipboardSettings())
	if len(en.Fields) != len(zh.Fields) {
		t.Fatalf("the two dictionaries declare %d and %d fields", len(en.Fields), len(zh.Fields))
	}
	for i := range en.Fields {
		if en.Fields[i].Key != zh.Fields[i].Key {
			t.Errorf("field %d: %q vs %q", i, en.Fields[i].Key, zh.Fields[i].Key)
		}
		if en.Fields[i].Label == zh.Fields[i].Label {
			t.Errorf("field %q is not translated: %q", en.Fields[i].Key, en.Fields[i].Label)
		}
		if en.Fields[i].Label == "" || zh.Fields[i].Label == "" {
			t.Errorf("field %q has an empty label", en.Fields[i].Key)
		}
	}
}
