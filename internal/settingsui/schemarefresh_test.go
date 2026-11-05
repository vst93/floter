package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/plugincfg"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func schemaRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HI_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hi-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func schemaPluginFrame(t *testing.T, dark bool, sheet *PluginSheet) (*App, *ui.Tester) {
	t.Helper()
	s := settings.Default()
	s.Theme, s.Language = appearanceName(dark), "zh"
	a := New(settings.NewStore(s), Actions{})
	sheet.Change = func(key string, value any) { sheet.Values[key] = value }
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(c.Theme().Background)
		a.PluginSheet(c, *sheet)
	}, 720, 400)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return a, tt
}

func TestSchemaRefreshPluginTextRetiresOnTypeChange(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := pluginIdentitySheet()
			sheet.Schema.Fields = sheet.Schema.Fields[2:]
			a, tt := schemaPluginFrame(t, dark, &sheet)
			pluginIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/unsent")
			tt.Frame()
			sheet.Schema.Fields[0].Kind = plugincfg.Toggle
			tt.Frame()
			_, stale := a.pluginTextDrafts["local.search\x00directory"]
			sheet.Schema.Fields[0].Kind = plugincfg.Text
			sheet.Values["directory"] = "/workspace/current"
			tt.Frame()
			schemaRefreshDump(t, tt, "plugin-text-type", dark)
			if stale {
				t.Error("retired text kind kept its unsent draft for the next text field")
			}
			pluginIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/recovered")
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if sheet.Values["directory"] != "/workspace/recovered" {
				t.Error("text field did not recover after type change")
			}
		})
	}
}

func TestSchemaRefreshPluginNumberRetiresOnTypeChange(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := PluginSheet{Schema: &plugincfg.Schema{Plugin: "local.search", Title: "搜索配置", Fields: []plugincfg.Field{{Key: "limit", Label: "结果上限", Kind: plugincfg.Number, Min: 0, Max: 100, Unit: "条"}}}, Values: plugincfg.Values{"limit": float64(5)}}
			a, tt := schemaPluginFrame(t, dark, &sheet)
			unit := mustFind(t, tt, "条")
			tt.ClickAt(unit.X-25, unit.Y+unit.H/2)
			tt.Frame()
			if !tt.Focused("结果上限") {
				t.Fatal("number field did not take focus")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("1e")
			tt.Frame()
			sheet.Schema.Fields[0].Kind = plugincfg.Toggle
			tt.Frame()
			_, stale := a.pluginNumberDrafts["local.search\x00limit"]
			sheet.Schema.Fields[0].Kind = plugincfg.Number
			sheet.Values["limit"] = float64(8)
			tt.Frame()
			schemaRefreshDump(t, tt, "plugin-number-type", dark)
			if stale {
				t.Error("retired number kind kept its half-input for the next number field")
			}
			unit = mustFind(t, tt, "条")
			tt.ClickAt(unit.X-25, unit.Y+unit.H/2)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("9")
			tt.Frame()
			if sheet.Values["limit"] != float64(9) {
				t.Error("number field did not recover after type change")
			}
		})
	}
}

func TestSchemaRefreshConfigTypeRetiresHalfInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "value", Label: "配置值", Type: "number"}}, ConfigValues: map[string]any{"value": float64(5)}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, "配置值")
			tt.Type("1e")
			tt.Frame()
			integration.Config[0].Type = "text"
			tt.Frame()
			schemaRefreshDump(t, tt, "config-type-draft", dark)
			if a.draft(integration).liveText(integration.Config[0], "") != "" || tt.HasText(a.copy().ConfigNumber) {
				t.Errorf("text field inherited the number half-input or error: %q", tt.Texts())
			}
			configIdentityFocus(t, tt, "配置值")
			tt.Type("搜索")
			tt.Frame()
			if got := a.draft(integration).values["value"]; got != "搜索" {
				t.Errorf("new text editor could not recover: %v", got)
			}
		})
	}
}

func TestSchemaRefreshConfigTypeRetiresUndo(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "value", Label: "配置值", Type: "text"}}, ConfigValues: map[string]any{"value": "12"}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, "配置值")
			tt.Type("34")
			tt.Frame()
			integration.Config[0].Type = "number"
			tt.Frame()
			configIdentityFocus(t, tt, "配置值")
			tt.Key(ui.Cmd, ui.KeyZ)
			tt.Frame()
			schemaRefreshDump(t, tt, "config-type-undo", dark)
			if got := a.draft(integration).values["value"]; got != "34" {
				t.Errorf("number field inherited text undo and wrote an unrequested value: %v", got)
			}
			tt.Type("56")
			tt.Frame()
			if got := a.draft(integration).values["value"]; got != float64(56) {
				t.Errorf("new number editor could not recover: %v", got)
			}
		})
	}
}

func TestSchemaRefreshNumberBoundsUpdateError(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			limit := float64(10)
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "value", Label: "结果上限", Type: "number", Maximum: &limit}}, ConfigValues: map[string]any{"value": float64(5)}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, "结果上限")
			tt.Type("12")
			tt.Frame()
			if !tt.HasText(a.copy().ConfigOutOfRange) {
				t.Fatal("invalid number did not show its error")
			}
			limit = 20
			tt.Frame()
			schemaRefreshDump(t, tt, "number-bounds", dark)
			if tt.HasText(a.copy().ConfigOutOfRange) {
				t.Error("relaxed number bound left a stale error line")
			}
			limit = 8
			tt.Frame()
			if !tt.HasText(a.copy().ConfigOutOfRange) {
				t.Error("tightened number bound did not mark the existing edit")
			}
			if !tt.Focused("结果上限") || a.draft(integration).values["value"] != float64(12) {
				t.Error("constraint refresh changed focus or edited value")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("6")
			tt.Frame()
			if tt.HasText(a.copy().ConfigOutOfRange) {
				t.Error("valid number did not recover")
			}
		})
	}
}

func TestSchemaRefreshTextBoundsUpdateError(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			limit := 2
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "value", Label: "搜索名称", Type: "text", MaxLength: &limit}}, ConfigValues: map[string]any{"value": "索引"}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, "搜索名称")
			tt.Type("工作索引")
			tt.Frame()
			if !tt.HasText(a.copy().ConfigOutOfLength) {
				t.Fatal("long text did not show its error")
			}
			limit = 6
			tt.Frame()
			schemaRefreshDump(t, tt, "text-bounds", dark)
			if tt.HasText(a.copy().ConfigOutOfLength) {
				t.Error("relaxed text length left a stale error line")
			}
			limit = 3
			tt.Frame()
			if !tt.HasText(a.copy().ConfigOutOfLength) {
				t.Error("tightened text length did not mark the existing edit")
			}
			if !tt.Focused("搜索名称") || a.draft(integration).values["value"] != "工作索引" {
				t.Error("constraint refresh changed focus or edited text")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("索引")
			tt.Frame()
			if tt.HasText(a.copy().ConfigOutOfLength) {
				t.Error("valid text did not recover")
			}
		})
	}
}

func TestSchemaRefreshRemovedMultiRetiresAnchor(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			a, tt := configIdentityFrame(t, dark, &integration)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			field := integration.Config[0]
			integration.Config = nil
			tt.Frame()
			integration.Config = []ConfigField{field}
			tt.Frame()
			if err := tt.ClickWith(ui.Shift, "书签"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			schemaRefreshDump(t, tt, "removed-multi", dark)
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"文件", "书签"}) {
				t.Errorf("removed field resurrected a stale range: %v", got)
			}
			if err := tt.ClickWith(ui.Shift, "应用"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"文件", "应用", "书签"}) {
				t.Errorf("restored multi could not start a fresh range: %v", got)
			}
		})
	}
}

func TestSchemaRefreshRemovedPluginTextRetiresDraft(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := pluginIdentitySheet()
			a, tt := schemaPluginFrame(t, dark, &sheet)
			pluginIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/removed")
			tt.Frame()
			field := sheet.Schema.Fields[2]
			sheet.Schema.Fields = sheet.Schema.Fields[:2]
			tt.Frame()
			_, stale := a.pluginTextDrafts["local.search\x00directory"]
			sheet.Schema.Fields = append(sheet.Schema.Fields, field)
			sheet.Values["directory"] = "/workspace/restored"
			tt.Frame()
			schemaRefreshDump(t, tt, "removed-plugin-text", dark)
			if stale {
				t.Error("removed plugin field kept a draft and repainted stale text on return")
			}
			pluginIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/recovered")
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if sheet.Values["directory"] != "/workspace/recovered" {
				t.Error("restored text field could not commit")
			}
		})
	}
}
