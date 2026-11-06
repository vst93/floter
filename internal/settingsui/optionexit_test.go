package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func optionExitDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13ID_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-id-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func optionExitFocus(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for i := 0; i < 25; i++ {
		tt.Key(0, ui.KeyTab)
		tt.Frame()
		if tt.Focused(label) {
			return
		}
	}
	t.Fatalf("cannot Tab to %s", label)
}

func checkOptionExitSelect(t *testing.T, scenario string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			in := Integration{ID: "local.options", Config: []ConfigField{{Key: "mode", Label: "搜索方式", Type: "select", Required: true, Options: []string{"文件", "应用", "便条"}}}, ConfigValues: map[string]any{"mode": "文件"}}
			a, tt := configIdentityFrame(t, dark, &in)
			optionExitFocus(t, tt, "搜索方式")
			tt.Key(0, ui.KeyDown)
			tt.Frame()
			if !tt.HasText("应用") {
				t.Fatal("menu did not open")
			}
			switch scenario {
			case "external":
				in.ConfigValues["mode"] = "远程来源"
			case "removed":
				in.Config[0].Options = []string{"应用", "便条"}
			case "empty":
				in.Config[0].Options = nil
			case "unlock":
				a.repairBusy = map[string]bool{in.ID: true}
			}
			tt.Frame()
			if tt.HasText("应用") {
				t.Error("stale popup remained open")
			}
			if scenario == "empty" || scenario == "unlock" {
				in.Config[0].Options = []string{"文件", "应用", "便条"}
				delete(a.repairBusy, in.ID)
				tt.Frame()
			}
			optionExitDump(t, tt, "select-"+scenario, dark)
			if !tt.Focused("搜索方式") {
				t.Errorf("%s refresh lost select keyboard exit", scenario)
			}
			if a.configDirty(in, a.draft(in)) {
				t.Error("refresh changed draft")
			}
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyEnd)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			optionExitDump(t, tt, "select-"+scenario+"-recovered", dark)
			if got := a.draft(in).values["mode"]; got != "便条" {
				t.Errorf("%s cannot choose after refresh: %v", scenario, got)
			}
		})
	}
}

func TestOptionExitSelectExternal(t *testing.T) { checkOptionExitSelect(t, "external") }
func TestOptionExitSelectRemoved(t *testing.T)  { checkOptionExitSelect(t, "removed") }

func checkOptionExitMulti(t *testing.T, empty bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			in := choiceMultiIntegration()
			in.Config = append([]ConfigField{{Key: "before", Label: "前项", Type: "text"}}, in.Config...)
			in.Config = append(in.Config, ConfigField{Key: "after", Label: "后项", Type: "text"})
			a, tt := configIdentityFrame(t, dark, &in)
			optionExitFocus(t, tt, "文件")
			name := "multi-removed"
			in.Config[1].Options = []string{"应用", "书签"}
			if empty {
				name = "multi-empty"
				in.Config[1].Options = nil
			}
			tt.Frame()
			optionExitDump(t, tt, name, dark)
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if a.configDirty(in, a.draft(in)) {
				t.Error("removed option activated a neighbour")
			}
			if empty {
				in.Config[1].Options = []string{"应用", "书签"}
				tt.Frame()
			}
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			optionExitDump(t, tt, name+"-recovered", dark)
			if !tt.Focused("应用") {
				t.Errorf("%s Tab did not continue at remaining options", name)
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if got := a.draft(in).values["sources"]; !configValuesEqual(got, []any{"应用"}) {
				t.Errorf("%s cannot toggle surviving option: %v", name, got)
			}
		})
	}
}

func checkOptionExitPlugin(t *testing.T, kind plugincfg.Kind) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectDynamicSheet()
			sheet.Schema.Fields[0].Kind = kind
			sheet.Schema.Fields = append([]plugincfg.Field{{Key: "before", Label: "前项", Kind: plugincfg.Toggle}}, sheet.Schema.Fields...)
			name := "radio-removed"
			if kind == plugincfg.Checkboxes {
				name = "checkbox-removed"
				sheet.Values["mode"] = []string{}
			}
			tt := pluginIdentityFrame(t, dark, &sheet)
			if kind == plugincfg.Radio {
				optionExitFocus(t, tt, "文件")
				tt.Key(0, ui.KeyRight)
				tt.Frame()
				if !tt.Focused("应用") {
					t.Fatal("radio arrow did not focus 应用")
				}
				sheet.Values["mode"] = "files"
			} else {
				optionExitFocus(t, tt, "应用")
			}
			opts := sheet.Schema.Fields[1].Options
			sheet.Schema.Fields[1].Options = []plugincfg.Option{opts[0], opts[2]}
			tt.Frame()
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			optionExitDump(t, tt, name, dark)
			want := any("files")
			if kind == plugincfg.Checkboxes {
				want = []string{}
			}
			if !configValuesEqual(sheet.Values["mode"], want) {
				t.Error("refresh selected an unrequested option")
			}
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			optionExitDump(t, tt, name+"-recovered", dark)
			if !tt.Focused("文件") {
				t.Errorf("%s Tab did not return to the surviving group", name)
			}
		})
	}
}
