package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func controlRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HR_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hr-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func controlRefreshSheet(kind plugincfg.Kind) PluginSheet {
	return PluginSheet{Schema: &plugincfg.Schema{Plugin: "local.controls", Title: "搜索配置", Fields: []plugincfg.Field{
		{Key: "enabled", Label: "启用搜索", Kind: plugincfg.Toggle, Section: "搜索"},
		{Key: "value", Label: "当前配置", Kind: kind, Section: "搜索", Options: []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "apps", Label: "应用"}, {Value: "notes", Label: "便条"}}, Min: 0, Max: 100, Step: 1, Confirm: "确认清理", Cancel: "取消清理"},
		{Key: "advanced", Label: "后台搜索", Kind: plugincfg.Toggle, Section: "高级"},
	}}, Values: plugincfg.Values{"enabled": true, "value": "files"}}
}

func controlRefreshMove(sheet *PluginSheet) {
	sheet.Schema.Fields[1], sheet.Schema.Fields[2] = sheet.Schema.Fields[2], sheet.Schema.Fields[1]
	sheet.Schema.Fields[2].Section = "高级"
}

func TestControlRefreshSelectKeepsOpenChoice(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := controlRefreshSheet(plugincfg.Select)
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyDown)
			tt.Frame()
			controlRefreshMove(&sheet)
			tt.Frame()
			tt.Frame()
			controlRefreshDump(t, tt, "select-move", dark)
			if !tt.HasText("应用") {
				t.Error("group move closed the active select menu")
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if sheet.Values["value"] != "apps" {
				t.Errorf("group move lost pending choice: %v, want apps", sheet.Values["value"])
			}
			if !tt.Focused("当前配置") {
				t.Error("group move lost select focus")
			}
		})
	}
}

func TestControlRefreshToggleKeepsKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := controlRefreshSheet(plugincfg.Toggle)
			sheet.Values["value"] = false
			tt := pluginIdentityFrame(t, dark, &sheet)
			r := mustFind(t, tt, "当前配置")
			tt.ClickAt(670, r.Y+r.H/2)
			tt.Frame()
			if !tt.Focused("当前配置") {
				t.Fatal("toggle did not take focus")
			}
			controlRefreshMove(&sheet)
			tt.Frame()
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			controlRefreshDump(t, tt, "toggle-move", dark)
			if sheet.Values["value"] != false || !tt.Focused("当前配置") {
				t.Errorf("group move lost toggle keyboard: value=%v focus=%v", sheet.Values["value"], tt.Focused("当前配置"))
			}
		})
	}
}

func TestControlRefreshRadioKeepsKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := controlRefreshSheet(plugincfg.Radio)
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.Focused("文件") {
				t.Fatal("radio did not take focus")
			}
			controlRefreshMove(&sheet)
			tt.Frame()
			tt.Key(0, ui.KeyRight)
			tt.Frame()
			controlRefreshDump(t, tt, "radio-move", dark)
			if sheet.Values["value"] != "apps" {
				t.Errorf("group move lost radio keyboard: %v, want apps", sheet.Values["value"])
			}
		})
	}
}

func TestControlRefreshCheckboxesKeepKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := controlRefreshSheet(plugincfg.Checkboxes)
			sheet.Values["value"] = []string{}
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.Focused("文件") {
				t.Fatal("checkbox did not take focus")
			}
			controlRefreshMove(&sheet)
			tt.Frame()
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			controlRefreshDump(t, tt, "checkboxes-move", dark)
			if got := sheet.Values["value"]; !configValuesEqual(got, []string{}) || !tt.Focused("文件") {
				t.Errorf("group move lost checkbox keyboard: value=%v focus=%v", got, tt.Focused("文件"))
			}
		})
	}
}

func TestControlRefreshSliderKeepsKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := controlRefreshSheet(plugincfg.Slider)
			sheet.Values["value"] = 50.0
			tt := pluginIdentityFrame(t, dark, &sheet)
			r := mustFind(t, tt, "当前配置")
			tt.ClickAt(340, r.Y+r.H+16)
			tt.Frame()
			tt.Key(0, ui.KeyHome)
			tt.Frame()
			if sheet.Values["value"] != 0.0 {
				t.Fatalf("slider did not take keyboard: %v", sheet.Values["value"])
			}
			controlRefreshMove(&sheet)
			tt.Frame()
			tt.Key(0, ui.KeyEnd)
			tt.Frame()
			controlRefreshDump(t, tt, "slider-move", dark)
			if sheet.Values["value"] != 100.0 {
				t.Errorf("group move lost slider keyboard: %v, want 100", sheet.Values["value"])
			}
		})
	}
}

func TestControlRefreshActionKeepsCancelKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := controlRefreshSheet(plugincfg.Action)
			sheet.Armed = "value"
			runs, cancels := 0, 0
			sheet.Run = func(string) { runs++ }
			sheet.Unarm = func(string) { cancels++; sheet.Armed = "" }
			tt := pluginIdentityFrame(t, dark, &sheet)
			// Tab to the two existing confirmation buttons without activating them.
			for i := 0; i < 6 && !tt.Focused("取消清理"); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused("取消清理") {
				t.Fatal("cancel did not take keyboard focus")
			}
			controlRefreshMove(&sheet)
			tt.Frame()
			controlRefreshDump(t, tt, "action-move", dark)
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if cancels != 1 || runs != 0 || sheet.Armed != "" {
				t.Errorf("group move lost cancellation: cancels=%d runs=%d armed=%q", cancels, runs, sheet.Armed)
			}
		})
	}
}
