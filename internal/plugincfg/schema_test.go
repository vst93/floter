package plugincfg

import (
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
