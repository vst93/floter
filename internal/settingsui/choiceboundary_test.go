package settingsui

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func choiceBoundaryDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HG_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hg-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestChoiceBoundaryExternalValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectDynamicSheet()
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			sheet.Values["mode"] = "notes"
			tt.Frame()
			choiceBoundaryDump(t, tt, "external-value-menu", dark)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := sheet.Values["mode"]; got != "notes" {
				t.Errorf("Enter overwrote the externally refreshed value: got %v, want notes", got)
			}
		})
	}
}

func TestChoiceBoundaryLongMenu(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			long := strings.Repeat("工作目录", 45)
			sheet := selectValueSheet("files", []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "long", Label: long}})
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			choiceBoundaryDump(t, tt, "long-menu", dark)
			r := mustFind(t, tt, long)
			if r.X < 0 || r.X+r.W > 720 {
				t.Errorf("long menu label escaped the window: %v", r)
			}
			tt.Key(0, ui.KeyEnd)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := sheet.Values["mode"]; got != "long" {
				t.Errorf("long choice did not commit: %v", got)
			}
		})
	}
}

func TestChoiceBoundaryMultiLabelFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			a, tt := configIdentityFrame(t, dark, &integration)
			choiceMultiFocus(t, tt, "文件")
			if err := tt.Click("应用"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			choiceBoundaryDump(t, tt, "multi-label-focus", dark)
			if !tt.Focused("应用") {
				t.Error("label click did not focus its checkbox")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"文件"}) {
				t.Errorf("Space after label click changed the wrong option: %v", got)
			}
		})
	}
}

func TestChoiceBoundaryMultiLongLabel(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			long := strings.Repeat("工作目录", 40)
			integration.Config[0].Options = []string{long, "应用"}
			a, tt := configIdentityFrame(t, dark, &integration)
			choiceBoundaryDump(t, tt, "multi-long", dark)
			r, next := mustFind(t, tt, long), mustFind(t, tt, "应用")
			if r.H > 20 || next.Y-r.Y > 24 {
				t.Errorf("long multi option pushed the next choice out: label=%v next=%v", r, next)
			}
			if err := tt.Click("应用"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"应用"}) {
				t.Errorf("next choice unreachable under long option: %v", got)
			}
		})
	}
}

func TestChoiceBoundaryMultiDuplicate(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			integration.Config[0].Options = []string{"文件", "文件", "应用"}
			a, tt := configIdentityFrame(t, dark, &integration)
			choiceBoundaryDump(t, tt, "multi-duplicate", dark)
			count := 0
			for _, word := range tt.Texts() {
				if word == "文件" {
					count++
				}
			}
			// Texts includes the visible word and the checkbox's accessible label.
			if count != 2 {
				t.Errorf("same multi value has %d text/accessible labels, want one pair", count)
			}
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"文件"}) {
				t.Errorf("duplicate multi value submitted: %v", got)
			}
		})
	}
}

func TestChoiceBoundaryPluginCheckboxDuplicate(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := pluginIdentitySheet()
			sheet.Schema.Fields = []plugincfg.Field{{Key: "sources", Label: "搜索来源", Kind: plugincfg.Checkboxes, Options: []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "files", Label: "文件副本"}, {Value: "apps", Label: "应用"}}}}
			tt := pluginIdentityFrame(t, dark, &sheet)
			choiceBoundaryDump(t, tt, "checkbox-duplicate", dark)
			if tt.HasText("文件副本") {
				t.Error("duplicate checkbox ID still draws a second control")
			}
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := sheet.Values["sources"]; !configValuesEqual(got, []string{"files"}) {
				t.Errorf("checkbox submitted repeated ID: %v", got)
			}
		})
	}
}

func TestChoiceBoundaryPluginRadioDuplicate(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectValueSheet("apps", []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "files", Label: "文件副本"}, {Value: "apps", Label: "应用"}})
			sheet.Schema.Fields[0].Kind = plugincfg.Radio
			tt := pluginIdentityFrame(t, dark, &sheet)
			choiceBoundaryDump(t, tt, "radio-duplicate", dark)
			if tt.HasText("文件副本") {
				t.Error("duplicate radio ID still draws an unselectable segment")
			}
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			choiceBoundaryDump(t, tt, "radio-duplicate-chosen", dark)
			if sheet.Values["mode"] != "files" {
				t.Error("radio did not select files")
			}
		})
	}
}

func TestChoiceBoundarySelectDuplicate(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectValueSheet("apps", []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "files", Label: "文件副本"}, {Value: "apps", Label: "应用"}})
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("应用"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			choiceBoundaryDump(t, tt, "select-duplicate", dark)
			if tt.HasText("文件副本") {
				t.Error("dropdown repeats the same stored ID")
			}
			tt.Key(0, ui.KeyHome)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := sheet.Values["mode"]; got != "apps" {
				t.Errorf("duplicate ID consumed a keyboard step: %v", got)
			}
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "mode", Label: "搜索方式", Type: "select", Required: true, Options: []string{"files", "files", "apps"}}}, ConfigValues: map[string]any{"mode": "apps"}}
			a, ct := configIdentityFrame(t, dark, &integration)
			if err := ct.Click("apps"); err != nil {
				t.Fatal(err)
			}
			ct.Frame()
			ct.Key(0, ui.KeyHome)
			ct.Key(0, ui.KeyDown)
			ct.Key(0, ui.KeyEnter)
			ct.Frame()
			if got := a.draft(integration).values["mode"]; got != "apps" {
				t.Errorf("config duplicate consumed a keyboard step: %v", got)
			}
		})
	}
}

func TestChoiceBoundaryEmptyChoiceName(t *testing.T) {
	for _, sample := range []struct {
		kind plugincfg.Kind
		name string
	}{{plugincfg.Select, "select"}, {plugincfg.Radio, "radio"}, {plugincfg.Checkboxes, "checkboxes"}} {
		kind, name := sample.kind, sample.name
		for _, dark := range []bool{false, true} {
			t.Run(name+"/"+appearanceName(dark), func(t *testing.T) {
				sheet := selectValueSheet("apps", []plugincfg.Option{{Value: "", Label: ""}, {Value: "apps", Label: "应用"}})
				sheet.Schema.Fields[0].Kind = kind
				if kind == plugincfg.Checkboxes {
					sheet.Values["mode"] = []string{}
				}
				tt := pluginIdentityFrame(t, dark, &sheet)
				if kind == plugincfg.Select {
					if err := tt.Click("应用"); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
				}
				choiceBoundaryDump(t, tt, "empty-name-"+name, dark)
				none := i18n.For("zh").Settings.ConfigNone
				if !tt.HasText(none) {
					t.Errorf("empty %s choice has no visible name", name)
					return
				}
				if err := tt.Click(none); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				got := sheet.Values["mode"]
				if kind == plugincfg.Checkboxes {
					if !configValuesEqual(got, []string{""}) {
						t.Errorf("empty checkbox ID changed: %#v", got)
					}
				} else if got != "" {
					t.Errorf("empty choice ID changed: %#v", got)
				}
			})
		}
	}
	for _, kind := range []string{"select", "multiselect"} {
		for _, dark := range []bool{false, true} {
			t.Run("config-"+kind+"/"+appearanceName(dark), func(t *testing.T) {
				integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "mode", Label: "搜索方式", Type: kind, Required: true, Options: []string{"", "apps"}}}, ConfigValues: map[string]any{"mode": "apps"}}
				a, tt := configIdentityFrame(t, dark, &integration)
				if kind == "select" {
					if err := tt.Click("apps"); err != nil {
						t.Fatal(err)
					}
					tt.Frame()
				}
				choiceBoundaryDump(t, tt, "empty-name-config-"+kind, dark)
				none := a.copy().ConfigNone
				if !tt.HasText(none) {
					t.Errorf("empty config %s choice has no visible name", kind)
					return
				}
				if err := tt.Click(none); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				got := a.draft(integration).values["mode"]
				if kind == "multiselect" {
					if !configValuesEqual(got, []any{""}) {
						t.Errorf("empty config multi ID changed: %#v", got)
					}
				} else if got != "" {
					t.Errorf("empty config ID changed: %#v", got)
				}
			})
		}
	}
}
