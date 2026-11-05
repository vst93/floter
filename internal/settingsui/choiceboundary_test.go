package settingsui

import (
	"image/png"
	"os"
	"strings"
	"testing"

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
			if count != 1 {
				t.Errorf("same multi value drawn %d times, want once", count)
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
