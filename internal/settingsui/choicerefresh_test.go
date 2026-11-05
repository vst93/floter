package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"strings"
	"testing"

	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
)

func choiceRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HH_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hh-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestChoiceRefreshUnknownExternalValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectDynamicSheet()
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			sheet.Values["mode"] = "remote-source"
			tt.Frame()
			choiceRefreshDump(t, tt, "external-unknown", dark)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := sheet.Values["mode"]; got != "remote-source" {
				t.Errorf("Enter silently replaced unknown external value: %v", got)
			}
		})
	}
}

func TestChoiceRefreshBothValuesRemoved(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectDynamicSheet()
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyDown)
			tt.Frame()
			sheet.Schema.Fields[0].Options = []plugincfg.Option{{Value: "notes", Label: "便条"}}
			tt.Frame()
			choiceRefreshDump(t, tt, "both-removed", dark)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := sheet.Values["mode"]; got != "files" {
				t.Errorf("removed current and highlight silently chose neighbour: %v", got)
			}
		})
	}
}

func TestChoiceRefreshRemovedAnchorStaysRetired(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			a, tt := configIdentityFrame(t, dark, &integration)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			integration.Config[0].Options = []string{"应用", "书签"}
			tt.Frame()
			integration.Config[0].Options = []string{"文件", "应用", "书签"}
			tt.Frame()
			if err := tt.ClickWith(ui.Shift, "书签"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			choiceRefreshDump(t, tt, "retired-anchor", dark)
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, []any{"文件", "书签"}) {
				t.Errorf("reappeared option resurrected its old range: %v", got)
			}
		})
	}
}

func TestChoiceRefreshLongPluginCheckbox(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			long := strings.Repeat("工作目录", 50)
			sheet := selectValueSheet("", []plugincfg.Option{{Value: "files", Label: long}, {Value: "apps", Label: "应用"}})
			sheet.Schema.Fields[0].Kind = plugincfg.Checkboxes
			sheet.Values["mode"] = []string{}
			tt := pluginIdentityFrame(t, dark, &sheet)
			choiceRefreshDump(t, tt, "long-checkbox", dark)
			r, next := mustFind(t, tt, long), mustFind(t, tt, "应用")
			if r.X+r.W > 692 || r.H > 25 || next.H == 0 {
				t.Errorf("long checkbox displaced its neighbours: label=%v next=%v", r, next)
			}
			if err := tt.Click("应用"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := sheet.Values["mode"]; !configValuesEqual(got, []string{"apps"}) {
				t.Errorf("checkbox neighbour unreachable: %v", got)
			}
		})
	}
}

func TestChoiceRefreshRadioReorderKeepsFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectDynamicSheet()
			sheet.Schema.Fields[0].Kind = plugincfg.Radio
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("应用"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			opts := sheet.Schema.Fields[0].Options
			opts[0], opts[1] = opts[1], opts[0]
			tt.Frame()
			choiceRefreshDump(t, tt, "radio-reorder", dark)
			if !tt.Focused("应用") {
				t.Error("radio focus moved to a different value after reorder")
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if sheet.Values["mode"] != "apps" {
				t.Errorf("radio Space selected the old position: %v", sheet.Values["mode"])
			}
		})
	}
}

func TestChoiceRefreshRadioRemovedFocus(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectDynamicSheet()
			sheet.Schema.Fields[0].Kind = plugincfg.Radio
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("应用"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			sheet.Values["mode"] = "files"
			opts := sheet.Schema.Fields[0].Options
			sheet.Schema.Fields[0].Options = []plugincfg.Option{opts[0], opts[2]}
			tt.Frame()
			choiceRefreshDump(t, tt, "radio-removed", dark)
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if sheet.Values["mode"] != "files" {
				t.Errorf("removed radio focus activated its replacement: %v", sheet.Values["mode"])
			}
		})
	}
}

func TestChoiceRefreshRadioLongLabel(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			long := strings.Repeat("工作目录", 30)
			sheet := selectValueSheet("apps", []plugincfg.Option{{Value: "files", Label: long}, {Value: "apps", Label: "应用"}})
			sheet.Schema.Fields[0].Kind = plugincfg.Radio
			tt := pluginIdentityFrame(t, dark, &sheet)
			choiceRefreshDump(t, tt, "radio-long", dark)
			r := mustFind(t, tt, long)
			next := mustFind(t, tt, "应用")
			if r.H > 30 || next.H > 30 || next.W < 22 || next.X+next.W > 692 {
				t.Errorf("long radio label inflated or displaced segments: long=%v next=%v", r, next)
			}
			if err := tt.Click(long); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if sheet.Values["mode"] != "files" {
				t.Errorf("long radio cannot be selected: %v", sheet.Values["mode"])
			}
		})
	}
}

func TestChoiceRefreshMultiMovedFocusVisible(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := choiceMultiIntegration()
			for i := 0; i < 15; i++ {
				integration.Config[0].Options = append(integration.Config[0].Options, fmt.Sprintf("来源 %02d", i))
			}
			a, tt := configIdentityFrame(t, dark, &integration)
			choiceMultiFocus(t, tt, "文件")
			natural := mustFind(t, tt, "文件")
			options := integration.Config[0].Options
			integration.Config[0].Options = append(append([]string{}, options[1:]...), "文件")
			tt.Frame()
			choiceRefreshDump(t, tt, "multi-moved-focus", dark)
			r := mustFind(t, tt, "文件")
			label := mustFind(t, tt, "搜索来源")
			center := label.Y + label.H/2
			if r.H != natural.H || r.Y < center-33 || r.Y+r.H > center+33 {
				t.Errorf("reordered focused option is outside its viewport: %v", r)
			}
			tt.Key(0, ui.KeySpace)
			tt.Frame()
			if got := a.draft(integration).values["sources"]; !configValuesEqual(got, nil) {
				t.Errorf("moved focus changed another value: %v", got)
			}
		})
	}
}
