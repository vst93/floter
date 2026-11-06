package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func groupRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HQ_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hq-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkGroupRefreshEditor(t *testing.T, kind plugincfg.Kind, move bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := plugincfg.Field{Key: "value", Label: "配置值", Kind: kind, Section: "搜索"}
			initial, edit := any("/old"), "/local"
			focus, name := pluginIdentityFocus, "text"
			if kind == plugincfg.Number {
				initial, edit, focus, name = 5.0, "1e", inputStateNumberFocus, "number"
			}
			sheet := PluginSheet{Schema: &plugincfg.Schema{Plugin: "local.group", Title: "搜索配置", Fields: []plugincfg.Field{field}}, Values: plugincfg.Values{"value": initial}}
			if move {
				sheet.Schema.Fields = []plugincfg.Field{
					{Key: "enabled", Label: "启用", Kind: plugincfg.Toggle, Section: "搜索"},
					field,
					{Key: "advanced", Label: "后台搜索", Kind: plugincfg.Toggle, Section: "高级"},
				}
			}
			tt := pluginIdentityFrame(t, dark, &sheet)
			calls := 0
			sheet.Change = func(key string, value any) { calls++; sheet.Values[key] = value }
			focus(t, tt, field.Label)
			tt.Type(edit)
			tt.Frame()
			if move {
				sheet.Schema.Fields[1], sheet.Schema.Fields[2] = sheet.Schema.Fields[2], sheet.Schema.Fields[1]
				sheet.Schema.Fields[2].Section = "高级"
				name += "-move"
			} else {
				sheet.Schema.Fields = append([]plugincfg.Field{{Key: "enabled", Label: "启用", Kind: plugincfg.Toggle, Section: "常规"}}, field)
				name += "-prepend"
			}
			tt.Frame()
			tt.Frame()
			groupRefreshDump(t, tt, name, dark)
			if !tt.Focused(field.Label) {
				t.Errorf("%s lost focus on group refresh", name)
			}
			if got := refreshRead(tt); got != edit {
				t.Errorf("%s lost draft: %q, want %q", name, got, edit)
			}
			if calls != 0 {
				t.Errorf("group refresh saved %d times", calls)
			}
			if !move {
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if sheet.Values[field.Key] != edit || calls != 1 {
					t.Errorf("preserved edit did not commit to its own field: %v, %d", sheet.Values, calls)
				}
			}
		})
	}
}

func TestGroupRefreshTextPrependKeepsEdit(t *testing.T) {
	checkGroupRefreshEditor(t, plugincfg.Text, false)
}
func TestGroupRefreshNumberMoveKeepsEdit(t *testing.T) {
	checkGroupRefreshEditor(t, plugincfg.Number, true)
}

func checkGroupRefreshValidation(t *testing.T, number, schema bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			limit, length := 10.0, 3
			field := ConfigField{Key: "value", Label: "配置值", Type: "text", MaxLength: &length}
			bad, good, name := any("too long"), any("ok"), "text"
			if number {
				field.Type, field.Maximum, bad, good, name = "number", &limit, 25.0, 5.0, "number"
			}
			in := Integration{ID: "local.validate", Config: []ConfigField{field}, ConfigValues: map[string]any{"value": bad}}
			a, tt := configIdentityFrame(t, dark, &in)
			configIdentityFocus(t, tt, field.Label)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if a.configFieldProblem(in, field) == "" {
				t.Fatal("invalid stored value did not show validation")
			}
			if schema {
				limit, length = 100, 30
				name += "-relax"
			} else {
				in.ConfigValues["value"] = good
				name += "-correct"
			}
			tt.Frame()
			groupRefreshDump(t, tt, name, dark)
			if got := a.configFieldProblem(in, field); got != "" {
				t.Errorf("%s retains obsolete validation: %q", name, got)
			}
			if a.configDirty(in, a.draft(in)) {
				t.Error("refresh dirtied an untouched field")
			}
		})
	}
}

func TestGroupRefreshTextCorrectionClearsValidation(t *testing.T) {
	checkGroupRefreshValidation(t, false, false)
}

func TestGroupRefreshNumberCorrectionClearsValidation(t *testing.T) {
	checkGroupRefreshValidation(t, true, false)
}
