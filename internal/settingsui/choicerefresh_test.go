package settingsui

import (
	"image/png"
	"os"
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
