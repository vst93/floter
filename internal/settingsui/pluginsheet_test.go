package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
)

// renderSheet draws one plugin's configuration sheet in a frame of its own,
// with the callbacks a test can watch.
func renderSheet(t *testing.T, a *App, sheet PluginSheet, w, h int) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.Transparent)
		a.PluginSheet(c, sheet)
	}, w, h)
	tt.Frame()
	return tt
}

// A change reports through the sheet's callback: the key the schema named and
// the value the control holds.
type change struct {
	key   string
	value any
}

// The sheet renders every field kind from a schema, and a change reports the
// field's own key and the control's value — the overlay knows no plugin, only
// the schema it was handed.
func TestPluginSheetRendersAndReportsChanges(t *testing.T) {
	copy := i18n.For("en").Settings
	schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(),
		[]i18n.Option{{ID: "brave", Label: "Brave"}})
	var changes []change
	run := []string{}
	sheet := PluginSheet{
		Schema: schema,
		Values: values,
		Change: func(key string, value any) { changes = append(changes, change{key, value}) },
		Arm:    func(key string) { run = append(run, "arm:"+key) },
		Run:    func(key string) { run = append(run, "run:"+key) },
		Unarm:  func(key string) { run = append(run, "unarm:"+key) },
	}
	a := New(newStore(t), Actions{})
	tt := renderSheet(t, a, sheet, 720, 1400)

	// The title, every section heading and every field's own label is drawn:
	// the sheet names a field only with the schema's words.
	for _, want := range []string{
		copy.BrowserPlugin, copy.SectionGeneral, copy.BrowserSectionData,
		copy.BrowserSectionSearch, copy.BrowserSectionTabs,
		copy.BrowserEnabled, copy.BrowserTarget, copy.BrowserCustomDir,
		copy.BrowserHistoryDays, copy.BrowserSort, copy.BrowserSearchField,
		copy.BrowserCDPEnabled, copy.BrowserCDPPort, copy.UnitDays,
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}

	// A toggle writes its key.
	if err := tt.Click(copy.BrowserEnabled); err != nil {
		t.Fatalf("toggle: %v", err)
	}
	tt.Frame()
	if len(changes) != 1 || changes[0].key != "enabled" || changes[0].value != false {
		t.Errorf("changes = %+v", changes)
	}

	// A select writes the option's value, not its label.
	if err := tt.Click(copy.BrowserAuto); err != nil {
		t.Fatalf("target trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Brave"); err != nil {
		t.Fatalf("target option: %v", err)
	}
	tt.Frame()
	if last := changes[len(changes)-1]; last.key != "target" || last.value != "brave" {
		t.Errorf("target change = %+v", last)
	}

	// A radio writes on click.
	if err := tt.Click(optionLabel(copy.BrowserSortOrders, "visits")); err != nil {
		t.Fatalf("radio: %v", err)
	}
	tt.Frame()
	if last := changes[len(changes)-1]; last.key != "sort_order" || last.value != "visits" {
		t.Errorf("sort change = %+v", last)
	}
}

// The action field is the old build's two-step button: the first press arms
// it (the button becomes the question, with the way back beside it), the
// second runs the command. The sheet never runs one on the first press.
func TestPluginSheetActionArmsThenRuns(t *testing.T) {
	copy := i18n.For("en").Settings
	schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
	armed := ""
	events := []string{}
	change := func(key string, value any) { events = append(events, "change:"+key) }
	arm := func(key string) { armed = key; events = append(events, "arm:"+key) }
	unarm := func(key string) { armed = ""; events = append(events, "unarm:"+key) }
	run := func(key string) { events = append(events, "run:"+key) }
	sheet := PluginSheet{Schema: schema, Values: values, Change: change, Arm: arm, Unarm: unarm, Run: run}
	a := New(newStore(t), Actions{})
	tt := renderSheet(t, a, sheet, 720, 700)

	if !tt.HasText("Clear history") {
		t.Fatalf("the action is missing: %q", tt.Texts())
	}
	if err := tt.Click("Clear history"); err != nil {
		t.Fatalf("the action's first press: %v", err)
	}
	tt.Frame()
	if armed != "clear_history" {
		t.Fatalf("armed = %q", armed)
	}
	for _, event := range events {
		if event == "run:clear_history" {
			t.Fatal("the first press ran the command")
		}
	}

	// Armed, the button is the question and the way back is beside it. The
	// way back disarms without running.
	sheet.Armed = armed
	tt = renderSheet(t, a, sheet, 720, 700)
	if !tt.HasText(copy.ClipboardClearButton) || !tt.HasText(copy.ClipboardClearCancel) {
		t.Fatalf("the armed row is not the confirm: %q", tt.Texts())
	}
	if err := tt.Click(copy.ClipboardClearCancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	tt.Frame()
	if armed != "" {
		t.Errorf("cancel left it armed: %q", armed)
	}

	// Armed again, the second press runs it.
	sheet.Armed = "clear_history"
	tt = renderSheet(t, a, sheet, 720, 700)
	if err := tt.Click(copy.ClipboardClearButton); err != nil {
		t.Fatalf("the action's second press: %v", err)
	}
	tt.Frame()
	if len(events) == 0 || events[len(events)-1] != "run:clear_history" {
		t.Errorf("events = %v", events)
	}
}

// A refused write is one line under the fields: the sheet says so rather than
// pretending a change that was never stored.
func TestPluginSheetReportsARefusedWrite(t *testing.T) {
	copy := i18n.For("en").Settings
	schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
	a := New(newStore(t), Actions{})
	sheet := PluginSheet{Schema: schema, Values: values, Failed: true}
	tt := renderSheet(t, a, sheet, 720, 700)
	if !tt.HasText(copy.SaveFailed) {
		t.Errorf("the failure line is missing: %q", tt.Texts())
	}
	sheet.Failed = false
	tt = renderSheet(t, a, sheet, 720, 700)
	if tt.HasText(copy.SaveFailed) {
		t.Errorf("the failure line outlived the failure: %q", tt.Texts())
	}
}

// A bounded row shows the value the schema holds, trailed by the schema's own
// unit word: "300 items", not "300".
func TestPluginSheetSliderShowsItsUnit(t *testing.T) {
	copy := i18n.For("en").Settings
	schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
	sheet := PluginSheet{Schema: schema, Values: values}
	a := New(newStore(t), Actions{})
	tt := renderSheet(t, a, sheet, 720, 700)
	want := "300 " + copy.UnitItems
	if !tt.HasText(want) {
		t.Errorf("the bounded value is not %q: %q", want, tt.Texts())
	}
	// The calculator's age window is a choice: its trigger shows the value
	// the schema holds, and the choices behind it are the schema's own.
	calcSchema, calcValues := plugincfg.Calculator(copy, settings.DefaultCalculatorPlugin())
	sheet = PluginSheet{Schema: calcSchema, Values: calcValues}
	tt = renderSheet(t, a, sheet, 720, 700)
	if !tt.HasText(copy.CalculatorRetentionDays(30)) {
		t.Errorf("the chosen window is missing: %q", tt.Texts())
	}
	if err := tt.Click(copy.CalculatorRetentionDays(30)); err != nil {
		t.Fatalf("the window trigger: %v", err)
	}
	tt.Frame()
	for _, want := range []string{copy.CalculatorRetentionNever, copy.CalculatorRetentionDays(7)} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

// optionLabel is the label a choice's value is shown with.
func optionLabel(options []i18n.Option, value string) string {
	for _, option := range options {
		if option.ID == value {
			return option.Label
		}
	}
	return value
}

// A plugin with no declarative configuration draws nothing: there is no sheet
// to show and no settings button to show it from.
func TestPluginSheetWithoutASchemaDrawsNothing(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := renderSheet(t, a, PluginSheet{}, 720, 300)
	if len(tt.Texts()) != 0 {
		t.Errorf("an empty sheet drew %q", tt.Texts())
	}
}
