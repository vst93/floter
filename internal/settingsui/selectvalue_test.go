package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func selectValueDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HF_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hf-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func selectValueSheet(current string, options []plugincfg.Option) PluginSheet {
	sheet := pluginIdentitySheet()
	sheet.Schema.Fields = []plugincfg.Field{{Key: "mode", Label: "搜索方式", Kind: plugincfg.Select, Options: options}}
	sheet.Values["mode"] = current
	return sheet
}

func TestSelectValueDuplicateNames(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectValueSheet("apps", []plugincfg.Option{{Value: "files", Label: "搜索"}, {Value: "apps", Label: "搜索"}})
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("搜索"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyHome)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			selectValueDump(t, tt, "duplicate-name", dark)
			if got := sheet.Values["mode"]; got != "files" {
				t.Errorf("same-name option was not selected by ID: got %v, want files", got)
			}
		})
	}
}

func TestSelectValueUnknownIDMatchingALabel(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectValueSheet("文件", []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "apps", Label: "应用"}})
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyHome)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			selectValueDump(t, tt, "unknown-label", dark)
			if got := sheet.Values["mode"]; got != "files" {
				t.Errorf("unknown ID matching a label could not be replaced: got %v, want files", got)
			}
		})
	}
}
