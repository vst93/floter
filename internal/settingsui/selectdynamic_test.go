package settingsui

import (
	"floter/internal/plugincfg"
	"github.com/egoist/mygo/ui"
	"testing"
)

func selectDynamicSheet() PluginSheet {
	return selectValueSheet("files", []plugincfg.Option{{Value: "files", Label: "文件"}, {Value: "apps", Label: "应用"}, {Value: "notes", Label: "便条"}})
}

func TestSelectDynamicReorderKeepsHighlightedValue(t *testing.T) {
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
			opts := sheet.Schema.Fields[0].Options
			opts[0], opts[1] = opts[1], opts[0]
			tt.Frame()
			selectValueDump(t, tt, "reordered-menu", dark)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			selectValueDump(t, tt, "reordered-choice", dark)
			if got := sheet.Values["mode"]; got != "apps" {
				t.Errorf("menu reorder moved the keyboard choice: got %v, want apps", got)
			}
		})
	}
}

func TestSelectDynamicRemovedHighlightDoesNotChooseNeighbour(t *testing.T) {
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
			opts := sheet.Schema.Fields[0].Options
			sheet.Schema.Fields[0].Options = []plugincfg.Option{opts[0], opts[2]}
			tt.Frame()
			selectValueDump(t, tt, "removed-menu", dark)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			selectValueDump(t, tt, "removed-choice", dark)
			if got := sheet.Values["mode"]; got != "files" {
				t.Errorf("removed highlight selected an unrequested neighbour: got %v, want files", got)
			}
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyEnd)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := sheet.Values["mode"]; got != "notes" {
				t.Errorf("menu did not recover after removal: %v", got)
			}
		})
	}
}
