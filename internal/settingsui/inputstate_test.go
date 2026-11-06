package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/plugincfg"
	"floter/internal/settings"
)

func inputStateSheet(t *testing.T, dark bool, field plugincfg.Field, value any) (*App, *PluginSheet, *ui.Tester) {
	t.Helper()
	s := settings.Default()
	s.Theme, s.Language = appearanceName(dark), "zh"
	a := New(settings.NewStore(s), Actions{})
	sheet := &PluginSheet{Schema: &plugincfg.Schema{Plugin: "local.input", Title: "输入配置", Fields: []plugincfg.Field{field}}, Values: plugincfg.Values{field.Key: value}}
	sheet.Change = func(key string, value any) { sheet.Values[key] = value }
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		a.PluginSheet(c, *sheet)
	}, 720, 220)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return a, sheet, tt
}

func inputStateNumberFocus(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r := mustFind(t, tt, label)
	tt.ClickAt(650, r.Y+r.H/2)
	tt.Frame()
	if !tt.Focused(label) {
		t.Fatal("number input did not take focus")
	}
	tt.Key(ui.Cmd, ui.KeyA)
}

func inputStateDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HM_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hm-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInputStateUnboundedNumberKeepsTypedValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "limit", Label: "结果上限", Kind: plugincfg.Number, Unit: "项"}
			a, sheet, tt := inputStateSheet(t, dark, field, 5.0)
			inputStateNumberFocus(t, tt, field.Label)
			tt.Type("42")
			tt.Frame()
			inputStateDump(t, tt, "unbounded", dark)
			if got := sheet.Values[field.Key]; got != 42.0 {
				t.Errorf("unbounded input stored %v, want 42", got)
			}
			if got := a.pluginNumberDrafts["local.input\x00limit"]; got != "42" {
				t.Errorf("unbounded input displays %q, want 42", got)
			}
		})
	}
}

func TestInputStateNumberDisplaysSnappedValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "limit", Label: "结果上限", Kind: plugincfg.Number, Min: 0, Max: 100, Step: 1, Unit: "项"}
			a, sheet, tt := inputStateSheet(t, dark, field, 5.0)
			inputStateNumberFocus(t, tt, field.Label)
			tt.Type("3.7")
			tt.Frame()
			inputStateDump(t, tt, "snapped", dark)
			if got := sheet.Values[field.Key]; got != 4.0 {
				t.Errorf("snapped input stored %v, want 4", got)
			}
			if got := a.pluginNumberDrafts["local.input\x00limit"]; got != "4" {
				t.Errorf("snapped input displays %q, want 4", got)
			}
			inputStateNumberFocus(t, tt, field.Label)
			tt.Type("1e")
			tt.Frame()
			if a.pluginNumberDrafts["local.input\x00limit"] != "1e" || sheet.Values[field.Key] != 4.0 {
				t.Error("normalization destroyed half-typed number or wrote it")
			}
		})
	}
}
