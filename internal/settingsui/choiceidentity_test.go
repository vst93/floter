package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/plugincfg"
)

func choiceIdentityDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HE_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-he-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func choiceMultiIntegration() Integration {
	return Integration{ID: "local.search", Config: []ConfigField{{Key: "sources", Label: "搜索来源", Type: "multiselect", Options: []string{"文件", "应用", "书签"}}}, ConfigValues: map[string]any{}}
}

func choiceMultiFocus(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r := mustFind(t, tt, label)
	tt.ClickAt(680, r.Y+r.H/2)
	tt.Frame()
	if !tt.Focused(label) {
		t.Fatalf("checkbox %s did not take focus", label)
	}
}

func TestChoiceIdentityMultiKeyboard(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			a, tt := configIdentityFrame(t, dark, &integration)
			choiceMultiFocus(t, tt, "文件")
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			choiceIdentityDump(t, tt, "multi-keyboard", dark)
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{}) && !configValuesEqual(got, nil) {
				t.Errorf("Space did not clear the focused checkbox: %v", got)
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"文件"}) {
				t.Errorf("Space did not select the focused checkbox: %v", got)
			}
		})
	}
}

func TestChoiceIdentityMultiFocusFollowsOption(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			_, tt := configIdentityFrame(t, dark, &integration)
			choiceMultiFocus(t, tt, "文件")
			integration.Config[0].Options = []string{"应用", "文件", "书签"}
			tt.Frame()
			if !tt.Focused("文件") || tt.Focused("应用") {
				t.Error("multiselect focus moved to another option after reorder")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			choiceIdentityDump(t, tt, "multi-focus", dark)
		})
	}
}

func TestChoiceIdentityPluginCheckboxFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := pluginIdentitySheet()
			sheet.Schema.Fields = []plugincfg.Field{{Key: "sources", Label: "搜索来源", Kind: plugincfg.Checkboxes, Options: []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "apps", Label: "应用"}}}}
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			opts := sheet.Schema.Fields[0].Options
			opts[0], opts[1] = opts[1], opts[0]
			tt.Frame()
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			choiceIdentityDump(t, tt, "plugin-checkbox-focus", dark)
			if got := sheet.Values["sources"]; !configValuesEqual(got, []string{}) {
				t.Errorf("plugin checkbox keyboard changed another option: %v", got)
			}
		})
	}
}
