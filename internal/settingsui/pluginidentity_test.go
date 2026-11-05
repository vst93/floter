package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/plugincfg"
	"floter/internal/settings"
)

func pluginIdentityFrame(t *testing.T, dark bool, sheet *PluginSheet) *ui.Tester {
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
	return tt
}

func pluginIdentityFocus(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r := mustFind(t, tt, label)
	tt.ClickAt(r.X+100, r.Y+r.H+19)
	tt.Frame()
	if !tt.Focused(label) {
		t.Fatalf("%s did not take focus", label)
	}
	tt.Key(ui.Cmd, ui.KeyA)
}

func pluginIdentitySheet() PluginSheet {
	return PluginSheet{Schema: &plugincfg.Schema{Plugin: "local.search", Title: "搜索配置", Fields: []plugincfg.Field{
		{Key: "enabled", Label: "启用", Kind: plugincfg.Toggle},
		{Key: "endpoint", Label: "服务地址", Kind: plugincfg.Text},
		{Key: "directory", Label: "工作目录", Kind: plugincfg.Text},
	}}, Values: plugincfg.Values{"enabled": true, "endpoint": "https://localhost", "directory": "/workspace/search"}}
}

func TestPluginIdentityReorderKeepsUnsubmittedInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := pluginIdentitySheet()
			tt := pluginIdentityFrame(t, dark, &sheet)
			pluginIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/edited")
			tt.Frame()
			if sheet.Values["directory"] != "/workspace/search" {
				t.Fatal("text committed before Enter")
			}
			sheet.Schema.Fields[1], sheet.Schema.Fields[2] = sheet.Schema.Fields[2], sheet.Schema.Fields[1]
			tt.Frame()
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			configIdentityDump(t, tt, "plugin-reorder", dark)
			if sheet.Values["directory"] != "/workspace/edited" || sheet.Values["endpoint"] != "https://localhost" {
				t.Errorf("plugin reorder lost or misdirected edit: %v", sheet.Values)
			}
		})
	}
}

func TestPluginIdentityHistoryDoesNotCrossPlugins(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := pluginIdentitySheet()
			tt := pluginIdentityFrame(t, dark, &sheet)
			pluginIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/changed")
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			sheet.Schema.Plugin, sheet.Schema.Title = "local.notes", "便条配置"
			sheet.Values = plugincfg.Values{"enabled": true, "endpoint": "https://localhost", "directory": "/workspace/notes"}
			tt.Frame()
			pluginIdentityFocus(t, tt, "工作目录")
			tt.Key(ui.Cmd, ui.KeyZ)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			configIdentityDump(t, tt, "plugin-history", dark)
			if sheet.Values["directory"] != "/workspace/notes" {
				t.Errorf("plugin inherited another undo history: %v", sheet.Values["directory"])
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("/workspace/own-edit")
			tt.Frame()
			tt.Key(ui.Cmd, ui.KeyZ)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if sheet.Values["directory"] != "/workspace/notes" {
				t.Errorf("plugin lost its own undo history: %v", sheet.Values["directory"])
			}
		})
	}
}

func TestPluginIdentityFirstRowKeepsItsInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := pluginIdentitySheet()
			tt := pluginIdentityFrame(t, dark, &sheet)
			pluginIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/edited")
			tt.Frame()
			sheet.Schema.Fields = sheet.Schema.Fields[2:]
			tt.Frame()
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			configIdentityDump(t, tt, "plugin-first-row", dark)
			if sheet.Values["directory"] != "/workspace/edited" || !tt.Focused("工作目录") {
				t.Errorf("first row lost its active edit: %v, focused=%v", sheet.Values["directory"], tt.Focused("工作目录"))
			}
		})
	}
}

func TestPluginIdentitySectionHeadingKeepsItsInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := pluginIdentitySheet()
			tt := pluginIdentityFrame(t, dark, &sheet)
			pluginIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/edited")
			tt.Frame()
			for i := range sheet.Schema.Fields {
				sheet.Schema.Fields[i].Section = "搜索数据"
			}
			tt.Frame()
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			configIdentityDump(t, tt, "plugin-heading", dark)
			if sheet.Values["directory"] != "/workspace/edited" || !tt.Focused("工作目录") {
				t.Errorf("section heading lost its active edit: %v, focused=%v", sheet.Values["directory"], tt.Focused("工作目录"))
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("/workspace/without-heading")
			for i := range sheet.Schema.Fields {
				sheet.Schema.Fields[i].Section = ""
			}
			tt.Frame()
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if sheet.Values["directory"] != "/workspace/without-heading" {
				t.Errorf("removing section heading lost its edit: %v", sheet.Values["directory"])
			}
		})
	}
}
