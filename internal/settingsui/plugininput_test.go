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
