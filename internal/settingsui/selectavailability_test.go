package settingsui

import (
	"floter/internal/plugincfg"
	"fmt"
	"github.com/egoist/mygo/ui"
	"testing"
)

// The identity fix must preserve scrolling, keyboard navigation and dismissal
// in menus taller than the real plugin window. This is a guard, not a ninth fix.
func TestSelectAvailabilityLongMenuKeyboardAndEscape(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			options := make([]plugincfg.Option, 80)
			for i := range options {
				options[i] = plugincfg.Option{Value: fmt.Sprint(i), Label: fmt.Sprintf("来源 %02d", i)}
			}
			sheet := selectValueSheet("0", options)
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("来源 00"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyEnd)
			tt.Frame()
			r := mustFind(t, tt, "来源 79")
			if r.Y < 0 || r.Y+r.H > 400 {
				t.Errorf("last choice was not scrolled into the window: %v", r)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := sheet.Values["mode"]; got != "79" {
				t.Errorf("long menu selected %v", got)
			}
			if err := tt.Click("来源 79"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyHome)
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			if sheet.Values["mode"] != "79" || tt.HasText("来源 00") {
				t.Error("Escape failed to dismiss without changing the value")
			}
		})
	}
}

func TestSelectAvailabilityEmptyOptions(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			sheet := selectValueSheet("files", nil)
			tt := pluginIdentityFrame(t, dark, &sheet)
			if err := tt.Click("files"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			selectValueDump(t, tt, "empty-menu", dark)
			if tt.Focused("搜索方式") {
				t.Error("empty select still accepts focus and opens a blank menu")
			}
			sheet.Schema.Fields[0].Options = []plugincfg.Option{{Value: "apps", Label: "应用"}}
			tt.Frame()
			if tt.HasText("应用") {
				t.Error("empty menu reopened itself when options arrived")
			}
			if err := tt.Click("files"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyHome)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := sheet.Values["mode"]; got != "apps" {
				t.Errorf("populated select did not recover: %v", got)
			}
		})
	}
}

func TestSelectAvailabilityLockClosesOpenMenu(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "mode", Label: "搜索方式", Type: "select", Required: true, Options: []string{"files", "apps"}}}, ConfigValues: map[string]any{"mode": "files"}}
			a, tt := configIdentityFrame(t, dark, &integration)
			if err := tt.Click("files"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText("apps") {
				t.Fatal("menu did not open")
			}
			a.repairBusy = map[string]bool{integration.ID: true}
			tt.Frame()
			selectValueDump(t, tt, "locked-menu", dark)
			if tt.HasText("apps") {
				t.Error("locked field left its live option popup on screen")
			}
			tt.Key(0, ui.KeyDown)
			tt.Frame()
			if got := a.draft(integration).values["mode"]; got != "files" {
				t.Errorf("locked field changed: %v", got)
			}
			delete(a.repairBusy, integration.ID)
			tt.Frame()
			if tt.HasText("apps") {
				t.Error("unlock resurrected the old popup")
			}
			if err := tt.Click("files"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click("apps"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if got := a.draft(integration).values["mode"]; got != "apps" {
				t.Errorf("unlocked menu did not recover: %v", got)
			}
		})
	}
}
