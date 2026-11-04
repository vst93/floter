package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
)

func TestPluginTextInputSubmitsToItsExistingCallback(t *testing.T) {
	copy := i18n.For("en").Settings
	schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
	var changes []change
	sheet := PluginSheet{Schema: schema, Values: values, Change: func(key string, value any) {
		changes = append(changes, change{key, value})
	}}
	a := New(newStore(t), Actions{})
	tt := renderPluginSheet(t, a.Store, sheet, 720, 900, false)
	hint := mustFind(t, tt, copy.BrowserCustomDirHint)
	// The existing text slot is the stacked row directly below the help.
	tt.ClickAt(hint.X+90, hint.Y+hint.H+20)
	if !tt.Focused(copy.BrowserCustomDir) {
		t.Fatal("the text editor did not take focus")
	}
	tt.Type("/workspace/browser-profile")
	tt.Frame()
	if len(changes) != 0 {
		t.Fatalf("text committed before Enter: %v", changes)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(changes) != 1 || changes[0].key != "custom_base_dir" || changes[0].value != "/workspace/browser-profile" {
		t.Fatalf("Enter did not submit the text field: %v", changes)
	}
	tt.Frame()
	if len(changes) != 1 {
		t.Fatalf("redraw repeated the submission: %v", changes)
	}
}

func TestConfigNumberInvalidTextStaysLocalAndRecovers(t *testing.T) {
	for _, language := range []string{"en", "zh"} {
		copy := i18n.For(language).Settings
		a := New(newStore(t), Actions{})
		field := ConfigField{Key: "limit", Label: "Limit", Type: "number"}
		integration := Integration{ID: "local.search", Config: []ConfigField{field}, ConfigValues: map[string]any{"limit": float64(5)}}
		draft := a.draft(integration)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			ui.Column(c).FillWidth().Padding(20).Children(func() {
				a.extensionMetrics(func() { a.configField(c, copy, integration, draft, field) })
			})
		}, 520, 150)
		tt.ClickAt(210, 34)
		for _, text := range []string{"NaN", "Inf", "-Inf", "12.5"} {
			tt.Key(0, ui.KeyEnd)
			for range 8 {
				tt.Key(0, ui.KeyBackspace)
			}
			tt.Type(text)
			tt.Frame()
			if text == "12.5" {
				if draft.values[field.Key] != float64(12.5) || a.configFieldProblem(integration, field) != "" {
					t.Errorf("valid number did not recover: values=%v error=%q", draft.values, a.configFieldProblem(integration, field))
				}
			} else if draft.values[field.Key] != nil || !tt.HasText(copy.ConfigNumber) || draft.liveText(field, "") != text {
				t.Errorf("%s/%s: invalid edit escaped or lost its feedback: values=%v error=%q live=%q", language, text, draft.values, a.configFieldProblem(integration, field), draft.liveText(field, ""))
			}
		}
	}
}

func TestPluginTextRefusedWriteKeepsTheEditForRetry(t *testing.T) {
	copy := i18n.For("en").Settings
	a := New(newStore(t), Actions{})
	schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
	var changes []change
	sheet := PluginSheet{Schema: schema, Values: values}
	sheet.Change = func(key string, value any) {
		changes = append(changes, change{key, value})
		sheet.Failed = true // Refused write: confirmed values remain unchanged.
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.PluginSheet(c, sheet)
	}, 720, 900)
	hint := mustFind(t, tt, copy.BrowserCustomDirHint)
	tt.ClickAt(hint.X+90, hint.Y+hint.H+20)
	tt.Type("/workspace/browser-profile")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if !tt.HasText(copy.SaveFailed) {
		t.Fatal("refused write lost its failure feedback")
	}
	if !tt.Focused(copy.BrowserCustomDir) {
		t.Fatal("the failure sentence replaced the focused input")
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if len(changes) != 2 || changes[0] != (change{"custom_base_dir", "/workspace/browser-profile"}) || changes[1] != changes[0] {
		t.Fatalf("retry lost the rejected edit: %v", changes)
	}
}

func TestDumpP13GqPluginText(t *testing.T) {
	tag := os.Getenv("FLOTER_P13GQ_DUMP")
	if tag == "" {
		t.Skip("opt-in screenshot dump: FLOTER_P13GQ_DUMP=before or after")
	}
	for _, dark := range []bool{false, true} {
		copy := i18n.For("zh").Settings
		s := settings.Default()
		s.Theme, s.Language = appearanceName(dark), "zh"
		store := settings.NewStore(s)
		schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
		sheet := PluginSheet{Schema: schema, Values: values, Change: func(key string, value any) { values[key] = value }}
		render := func() *ui.Tester {
			a := New(store, Actions{})
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				a.PluginSheet(c, sheet)
			}, 720, 900)
			tt.SetDark(dark)
			tt.Frame()
			return tt
		}
		tt := render()
		hint := mustFind(t, tt, copy.BrowserCustomDirHint)
		tt.ClickAt(hint.X+90, hint.Y+hint.H+20)
		tt.Type("/workspace/browser-profile")
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		// Reopening reads the confirmed values, not the focused editor's buffer.
		tt = render()
		name := fmt.Sprintf("/tmp/floter-P13-gq-plugin-%s-%s.png", appearanceName(dark), tag)
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
		t.Logf("%s: reopened custom_base_dir=%v", name, values["custom_base_dir"])
	}
}

func TestConfigNumbersRejectNonFiniteInput(t *testing.T) {
	for _, language := range []string{"en", "zh"} {
		copy := i18n.For(language).Settings
		for _, text := range []string{"NaN", "nan", "Inf", "+Inf", "-Inf", "Infinity", "1e999"} {
			value, problem := ConfigValueProblem(ConfigField{Type: "number"}, text, copy)
			if value != nil || problem != copy.ConfigNumber {
				t.Errorf("%s accepted non-finite input %q: value=%v problem=%q", language, text, value, problem)
			}
		}
	}
}

func TestPluginNumberInputDoesNotDispatchNonFiniteValues(t *testing.T) {
	copy := i18n.For("en").Settings
	for _, text := range []string{"NaN", "Inf", "-Inf"} {
		t.Run(text, func(t *testing.T) {
			a := New(newStore(t), Actions{})
			schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
			var changes []change
			sheet := PluginSheet{Schema: schema, Values: values, Change: func(key string, value any) {
				changes = append(changes, change{key, value})
			}}
			tt := renderSheet(t, a, sheet, 720, 1400)
			unit := mustFind(t, tt, copy.UnitDays)
			tt.ClickAt(unit.X-30, unit.Y+unit.H/2)
			tt.Key(0, ui.KeyEnd)
			for range 8 {
				tt.Key(0, ui.KeyBackspace)
			}
			changes = nil
			tt.Type(text)
			tt.Frame()
			if len(changes) != 0 {
				t.Errorf("invalid number reached the plugin callback: %v", changes)
			}
			tt.Key(0, ui.KeyEnd)
			for range 8 {
				tt.Key(0, ui.KeyBackspace)
			}
			tt.Type("45")
			tt.Frame()
			if len(changes) == 0 || changes[len(changes)-1] != (change{"history_days", float64(45)}) {
				t.Errorf("a valid edit did not recover after %q: %v", text, changes)
			}
		})
	}
}

// A number field's in-progress text is keyed by the plugin id **plus** the
// field key, exactly as the text field's `pluginTextDrafts` is. One settings
// App draws every plugin's sheet in turn (`shell/pluginconfig.go`), and a field
// key such as `size` is not unique across plugins: keyed by the field key alone,
// plugin A's half-typed `1e` survived the switch and painted inside plugin B's
// box in place of B's own stored value.
func TestPluginNumberDraftDoesNotBleedAcrossPlugins(t *testing.T) {
	schema := func(plugin string) *plugincfg.Schema {
		return &plugincfg.Schema{Plugin: plugin, Title: plugin, Fields: []plugincfg.Field{
			{Key: "size", Label: "size", Kind: plugincfg.Number, Min: 1, Max: 99, Step: 1, Unit: "items"},
		}}
	}
	a := New(newStore(t), Actions{})
	var changes []change
	// One sheet the test redraws with a swapped schema, as the shell does when
	// the user moves from one plugin's config to the next.
	sheet := PluginSheet{Schema: schema("plug.a"), Values: plugincfg.Values{"size": 2.0},
		Change: func(key string, value any) { changes = append(changes, change{key, value}) }}
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ui.Transparent)
		a.PluginSheet(c, sheet)
	}, 720, 300)
	tt.Frame()
	tt.Frame()

	// Focus plugin A's number box (the unit rides beside it) and type a
	// half-parseable `1e`: it lands in A's draft and reaches no callback.
	unit := mustFind(t, tt, "items")
	tt.ClickAt(unit.X-30, unit.Y+unit.H/2)
	tt.Frame()
	if !tt.Focused("size") {
		t.Fatalf("plugin A's number input did not take focus: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnd)
	for range 8 {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Type("1e")
	tt.Frame()
	if len(changes) != 0 {
		t.Fatalf("a half-typed number reached the plugin callback: %v", changes)
	}
	if got := a.pluginNumberDrafts["plug.a\x00size"]; got != "1e" {
		t.Fatalf("plugin A's draft was not stored under its own scoped key: %v", a.pluginNumberDrafts)
	}
	if _, ok := a.pluginNumberDrafts["size"]; ok {
		t.Fatalf("plugin A's draft leaked into the unscoped key: %v", a.pluginNumberDrafts)
	}

	// Swap to plugin B: same field key, its own stored value 7.
	sheet.Schema = schema("plug.b")
	sheet.Values = plugincfg.Values{"size": 7.0}
	tt.Frame()
	tt.Frame()

	// B's draft map holds nothing of its own yet, so B's box must read its
	// stored 7 — not A's stranded `1e`. A bleed would leave `1e` under the
	// shared key `size` and B's render would pick it up.
	if got, ok := a.pluginNumberDrafts["plug.b\x00size"]; ok {
		t.Errorf("plugin B inherited a draft it never typed: %q", got)
	}
	if _, ok := a.pluginNumberDrafts["size"]; ok {
		t.Errorf("plugin B still resolves the unscoped key A wrote: %v", a.pluginNumberDrafts)
	}

	// Typing in B now lands under B's own scoped key and reports B's value.
	unitB := mustFind(t, tt, "items")
	tt.ClickAt(unitB.X-30, unitB.Y+unitB.H/2)
	tt.Frame()
	tt.Key(0, ui.KeyEnd)
	for range 8 {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Type("9")
	tt.Frame()
	if len(changes) == 0 || changes[len(changes)-1] != (change{"size", float64(9)}) {
		t.Errorf("plugin B's own edit did not report: %v", changes)
	}
	if got := a.pluginNumberDrafts["plug.a\x00size"]; got != "1e" {
		t.Errorf("editing B disturbed A's draft: %v", a.pluginNumberDrafts)
	}
}
