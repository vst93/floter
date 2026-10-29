package settingsui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
)

// P11-bu · **插件配置表单也没有两段字压在同一块像素上，也没有一段字跑出窗口。**
//
// The settings audit (`auditSettingsFrame`) reads the five settings pages; the
// plugin configuration sheet is not one of them. It is the one form in the app
// that draws every control kind at once — a toggle with its help line, a
// select, radio and checkbox rows, a slider with its unit-riding value, a
// number slot, a text field, and an action pill that swaps its own wording when
// armed — inside the *launcher's* 720u window rather than the settings
// window's 860u (`internal/shell/pluginconfig.go:180` draws it in the band the
// result list would have taken). A row that overruns that narrower column is
// the way this surface goes wrong, and it goes wrong silently: `Tester.Texts`
// reports a run the layout clipped exactly as happily as one it painted.
//
// The three schemas are the ones that ship (`internal/plugincfg/schema.go`):
// the clipboard's, the calculator's and the browser's — the browser's being the
// only one that carries a text field and two numbers.
func TestThePluginSheetPaintsNoTextOverText(t *testing.T) {
	schemas := []struct {
		name  string
		build func(t *testing.T, lang string) (*settings.Store, PluginSheet)
	}{
		{"clipboard", func(t *testing.T, lang string) (*settings.Store, PluginSheet) {
			store := pluginSheetStore(t, lang)
			copy := i18n.For(lang).Settings
			schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
			return store, PluginSheet{Schema: schema, Values: values}
		}},
		{"calculator", func(t *testing.T, lang string) (*settings.Store, PluginSheet) {
			store := pluginSheetStore(t, lang)
			copy := i18n.For(lang).Settings
			schema, values := plugincfg.Calculator(copy, settings.DefaultCalculatorPlugin())
			return store, PluginSheet{Schema: schema, Values: values}
		}},
		// The sheet's own transient states: an action armed (its wording swaps
		// for the question and the way back) and the two failure lines, the
		// write's under the fields and the action's in its row.
		{"clipboard-armed", func(t *testing.T, lang string) (*settings.Store, PluginSheet) {
			store := pluginSheetStore(t, lang)
			copy := i18n.For(lang).Settings
			schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
			return store, PluginSheet{Schema: schema, Values: values, Armed: "clear_history"}
		}},
		{"clipboard-failed", func(t *testing.T, lang string) (*settings.Store, PluginSheet) {
			store := pluginSheetStore(t, lang)
			copy := i18n.For(lang).Settings
			schema, values := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
			return store, PluginSheet{Schema: schema, Values: values, Failed: true, ActionFailed: "clear_history"}
		}},
		{"browser", func(t *testing.T, lang string) (*settings.Store, PluginSheet) {
			store := pluginSheetStore(t, lang)
			copy := i18n.For(lang).Settings
			schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(),
				[]i18n.Option{{ID: "brave", Label: "Brave"}, {ID: "safari", Label: "Safari"}})
			return store, PluginSheet{Schema: schema, Values: values}
		}},
	}
	sizes := [][2]int{
		{720, 900}, // the launcher's window at the ten-row budget
		{720, 560}, // the same window with the list's band a few rows shorter
	}
	for _, lang := range []string{"en", "zh"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			for _, size := range sizes {
				for _, s := range schemas {
					tag := fmt.Sprintf("%s %s %s %dx%d", lang, appearance, s.name, size[0], size[1])
					t.Run(tag, func(t *testing.T) {
						store, sheet := s.build(t, lang)
						tt := renderPluginSheet(t, store, sheet, size[0], size[1], dark)
						auditSettingsFrame(t, tag, tt)
					})
				}
			}
		}
	}
}

// The audit is only meaningful if the sheet actually draws the fields it is
// auditing: a form that silently rendered empty would pass the overlap check
// for the wrong reason. Every label the schema names is painted, in every
// shipped schema — that is the arm that notices a field kind dropping out of
// the overlay.
func TestThePluginSheetAuditDrawsEveryField(t *testing.T) {
	copy := i18n.For("en").Settings
	clipboardSchema, clipboardValues := plugincfg.Clipboard(copy, settings.DefaultClipboardSettings())
	calculatorSchema, calculatorValues := plugincfg.Calculator(copy, settings.DefaultCalculatorPlugin())
	browserSchema, browserValues := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(), nil)
	for _, tc := range []struct {
		name   string
		schema *plugincfg.Schema
		values plugincfg.Values
	}{
		{"clipboard", clipboardSchema, clipboardValues},
		{"calculator", calculatorSchema, calculatorValues},
		{"browser", browserSchema, browserValues},
	} {
		store := pluginSheetStore(t, "en")
		tt := renderPluginSheet(t, store, PluginSheet{Schema: tc.schema, Values: tc.values}, 720, 900, false)
		if !tt.HasText(tc.schema.Title) {
			t.Errorf("%s: the sheet does not draw its title %q; it drew %q", tc.name, tc.schema.Title, tt.Texts())
		}
		for _, section := range plugincfg.Sections(tc.schema) {
			if section.Title != "" && !tt.HasText(section.Title) {
				t.Errorf("%s: the sheet does not draw the section heading %q", tc.name, section.Title)
			}
			for _, field := range section.Fields {
				if field.Label != "" && !tt.HasText(field.Label) {
					t.Errorf("%s: the sheet does not draw the %v field %q; it drew %q",
						tc.name, field.Kind, field.Label, tt.Texts())
				}
			}
		}
	}
}

// pluginSheetStore is a store set to one language, so the sheet's own copy
// (`a.language()`) and the schema's words agree.
func pluginSheetStore(t *testing.T, lang string) *settings.Store {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Language = lang }); err != nil {
		t.Fatal(err)
	}
	return store
}

// renderPluginSheet draws the sheet in a frame of its own, at the size and
// appearance asked for, with the app's palette installed on the context (as
// the settings pages' own audit does).
func renderPluginSheet(t *testing.T, store *settings.Store, sheet PluginSheet, w, h int, dark bool) *ui.Tester {
	t.Helper()
	a := New(store, Actions{})
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ui.Transparent)
		a.PluginSheet(c, sheet)
	}, w, h)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return tt
}
