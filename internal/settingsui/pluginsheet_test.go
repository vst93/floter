package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/scriptlang"
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
		// The toggle rows keep their help line too, not just the selects and
		// sliders: the switch used to drop it.
		copy.BrowserPluginHint,
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

// The local-tool form builds two kinds of tool: a program on this machine, and
// a script the host writes. The kind decides which rows the form shows, and the
// script's language and text travel with the draft — the backend writes them
// into the package as `provider.<ext>`.
func TestLocalToolFormBuildsAScript(t *testing.T) {
	connected := []CustomTool{}
	a := New(newStore(t), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
		ScriptRuntime: func(language string) scriptlang.Check {
			if language == "python" {
				return scriptlang.Check{Available: true, Path: "/usr/bin/python3", Version: "3.12.1"}
			}
			return scriptlang.Check{Candidates: []string{scriptlang.Lookup(language).Toolchain}}
		},
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Add a local tool"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	copy := i18n.For("en").Settings
	// The kind is the form's own row, and a program is what it starts as.
	for _, want := range []string{copy.LocalToolMode, copy.LocalToolModeProgram, copy.LocalToolModeScript, copy.LocalToolProgram} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatalf("the kind: %v", err)
	}
	tt.Frame()
	// The script rows replace the program row's own field (the segmented
	// control keeps its two labels either way), and the language's toolchain is
	// reported as the muted line under the picker.
	if tt.HasText("/usr/bin/rg") {
		t.Errorf("the program row survived the switch: %q", tt.Texts())
	}
	for _, want := range []string{copy.LocalToolLanguage, copy.LocalToolScript, "JavaScript", copy.LocalToolRuntimeMissing("node")} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	// Switching to a language this machine has says so, with its version, and
	// the editor holds that language's template.
	if err := tt.Click("JavaScript"); err != nil {
		t.Fatalf("the language trigger: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Python"); err != nil {
		t.Fatalf("the language option: %v", err)
	}
	tt.Frame()
	if a.toolLanguage != "python" {
		t.Fatalf("language = %q", a.toolLanguage)
	}
	if a.toolScript != scriptlang.Template("python") {
		t.Errorf("the editor did not take the template: %q", a.toolScript)
	}
	if !tt.HasText(copy.LocalToolRuntimeReady("python3", "3.12.1")) {
		t.Errorf("the runtime line is missing: %q", tt.Texts())
	}

	// Connect hands the script on: the language, the text, and the kind.
	a.toolName = "Weather"
	a.toolCommand = "weather"
	a.toolScript = "print('hi')\n"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 1 {
		t.Fatalf("connected = %+v", connected)
	}
	tool := connected[0]
	if tool.Mode != "script" || tool.ScriptLanguage != "python" || tool.ScriptContent != "print('hi')\n" {
		t.Errorf("the draft lost the script: %+v", tool)
	}
}

// A script with nothing in it is not connected: the form says what is missing
// by staying put, as it does for a program with no path.
func TestLocalToolFormRefusesAnEmptyScript(t *testing.T) {
	connected := []CustomTool{}
	a := New(newStore(t), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
	})
	tt := render(t, a, 720, 1600)
	tt.Click("Integrations")
	tt.Frame()
	tt.Click("Add a local tool")
	tt.Frame()
	copy := i18n.For("en").Settings
	tt.Click(copy.LocalToolModeScript)
	tt.Frame()
	a.toolName = "Weather"
	a.toolScript = "   \n"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatalf("connect: %v", err)
	}
	tt.Frame()
	if len(connected) != 0 {
		t.Errorf("an empty script was connected: %+v", connected)
	}
}

// The toolchain probe is cached per language for the form's own session: the
// old panel's `runtimeChecksRef` ran it once per language per drawer rather
// than per keystroke, so a redraw must not shell out again and switching back
// to a language already checked must be instant.
func TestScriptRuntimeProbeIsCachedPerLanguage(t *testing.T) {
	probes := map[string]int{}
	a := New(newStore(t), Actions{
		ScriptRuntime: func(language string) scriptlang.Check {
			probes[language]++
			return scriptlang.Check{Candidates: []string{scriptlang.Lookup(language).Toolchain}}
		},
	})
	// Two reads of one language in one session: the probe runs once.
	a.scriptRuntimeLine("js")
	a.scriptRuntimeLine("js")
	if probes["js"] != 1 {
		t.Errorf("js was probed %d times, want 1", probes["js"])
	}
	// A different language has its own verdict.
	a.scriptRuntimeLine("python")
	if probes["python"] != 1 {
		t.Errorf("python was probed %d times, want 1", probes["python"])
	}
	// Switching back reuses the first verdict.
	a.scriptRuntimeLine("js")
	if probes["js"] != 1 {
		t.Errorf("js was re-probed on a switch back: %d", probes["js"])
	}
	// A fresh form drops the verdicts: the machine may have changed.
	a.clearScriptRuntimeCache()
	a.scriptRuntimeLine("js")
	if probes["js"] != 2 {
		t.Errorf("a fresh form did not re-probe: %d", probes["js"])
	}
}

// Reopening the form re-probes the toolchain: "install it, then reopen this
// form to check again" is the whole point of clearing the cache on open.
func TestReopeningTheLocalToolFormReprobesTheToolchain(t *testing.T) {
	probes := 0
	a := New(newStore(t), Actions{
		ConnectLocalTool: func(tool CustomTool) {},
		ScriptRuntime: func(language string) scriptlang.Check {
			probes++
			return scriptlang.Check{Candidates: []string{scriptlang.Lookup(language).Toolchain}}
		},
	})
	tt := render(t, a, 720, 1600)
	if err := tt.Click("Integrations"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	copy := i18n.For("en").Settings
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if probes != 1 {
		t.Fatalf("probes after the first form = %d, want 1", probes)
	}
	// Redraws of the open form reuse the verdict.
	tt.Frame()
	tt.Frame()
	if probes != 1 {
		t.Errorf("a redraw re-probed: %d", probes)
	}
	// Closing and reopening the form is a fresh check.
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if probes != 2 {
		t.Errorf("reopening did not re-probe: %d", probes)
	}
}

// A script saved on a machine without its toolchain is still saved, and said
// so once: the old panel's `createCustomIntegration` waited for the probe and
// raised one warning, because a missing runtime must not stop the save but the
// integration must not be "silently dead on arrival" either.
func TestSavingAScriptWithoutItsToolchainWarnsAndStillSaves(t *testing.T) {
	connected := []CustomTool{}
	warnings := []string{}
	a := New(newStore(t), Actions{
		ConnectLocalTool: func(tool CustomTool) { connected = append(connected, tool) },
		NotifyWarning:    func(text string) { warnings = append(warnings, text) },
		ScriptRuntime: func(language string) scriptlang.Check {
			if language == "python" {
				return scriptlang.Check{Available: true, Path: "/usr/bin/python3"}
			}
			return scriptlang.Check{Candidates: []string{scriptlang.Lookup(language).Toolchain}}
		},
	})
	tt := render(t, a, 720, 1600)
	copy := i18n.For("en").Settings
	tt.Click("Integrations")
	tt.Frame()
	tt.Click(copy.LocalToolAdd)
	tt.Frame()
	tt.Click(copy.LocalToolModeScript)
	tt.Frame()
	// The form opens on JavaScript, whose toolchain this machine does not have.
	a.toolName, a.toolCommand, a.toolScript = "Weather", "weather", "console.log('hi')\n"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(connected) != 1 {
		t.Fatalf("a script without its toolchain was not saved: %+v", connected)
	}
	want := copy.LocalToolRuntimeSaveWarning("JavaScript")
	if len(warnings) != 1 || warnings[0] != want {
		t.Fatalf("warnings = %q, want one %q", warnings, want)
	}

	// A language this machine has says nothing: the warning is about a script
	// that cannot run yet, not about the save.
	if err := tt.Click(copy.LocalToolAdd); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(copy.LocalToolModeScript); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("JavaScript"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Python"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.toolLanguage != "python" {
		t.Fatalf("language = %q", a.toolLanguage)
	}
	a.toolName, a.toolCommand, a.toolScript = "Weather", "weather2", "print('hi')\n"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(connected) != 2 {
		t.Fatalf("the second script was not saved: %+v", connected)
	}
	if len(warnings) != 1 {
		t.Errorf("a present toolchain warned: %q", warnings)
	}
}

// A host that wired no toolchain probe gets no guess: the warning is raised
// from a verdict, never from the absence of one.
func TestAScriptSaveWarnsOnlyFromAVerdict(t *testing.T) {
	warnings := []string{}
	a := New(newStore(t), Actions{
		ConnectLocalTool: func(tool CustomTool) {},
		NotifyWarning:    func(text string) { warnings = append(warnings, text) },
	})
	tt := render(t, a, 720, 1600)
	copy := i18n.For("en").Settings
	tt.Click("Integrations")
	tt.Frame()
	tt.Click(copy.LocalToolAdd)
	tt.Frame()
	tt.Click(copy.LocalToolModeScript)
	tt.Frame()
	a.toolName, a.toolCommand, a.toolScript = "Weather", "weather", "console.log('hi')\n"
	tt.Frame()
	if err := tt.Click(copy.LocalToolConnect); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(warnings) != 0 {
		t.Errorf("an unanswered probe warned: %q", warnings)
	}
}

// A refused action prints the field's own failure line beside its button, not
// the sheet's write-failure line: the old build's `ActionControl` kept the two
// apart (`plugin-config-action__status` vs the overlay's caption).
func TestPluginSheetActionFailureHasItsOwnLine(t *testing.T) {
	copy := i18n.For("en").Settings
	schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
	a := New(newStore(t), Actions{})

	sheet := PluginSheet{Schema: schema, Values: values, ActionFailed: "clear_history"}
	tt := renderSheet(t, a, sheet, 720, 700)
	if !tt.HasText(copy.ClipboardClearFailed) {
		t.Errorf("the action's own failure line is missing: %q", tt.Texts())
	}
	if tt.HasText(copy.SaveFailed) {
		t.Errorf("an action failure borrowed the write line: %q", tt.Texts())
	}

	// Armed, the row is the confirm question and the line gives way to it.
	sheet.Armed = "clear_history"
	tt = renderSheet(t, a, sheet, 720, 700)
	if tt.HasText(copy.ClipboardClearFailed) {
		t.Errorf("the failure line survived into the armed row: %q", tt.Texts())
	}

	// Clearing the key clears the line.
	sheet = PluginSheet{Schema: schema, Values: values}
	tt = renderSheet(t, a, sheet, 720, 700)
	if tt.HasText(copy.ClipboardClearFailed) {
		t.Errorf("the failure line outlived the failure: %q", tt.Texts())
	}
}
