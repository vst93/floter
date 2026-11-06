package settingsui

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func checkControlRefreshClosedSelect(t *testing.T, removed bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			in := Integration{ID: "local.controls", Config: []ConfigField{{Key: "mode", Label: "搜索方式", Type: "select", Required: true, Options: []string{"文件", "应用", "便条"}}}, ConfigValues: map[string]any{"mode": "文件"}}
			a, tt := configIdentityFrame(t, dark, &in)
			if err := tt.Click("文件"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyEscape)
			tt.Frame()
			if !tt.Focused("搜索方式") || tt.HasText("应用") {
				t.Fatal("closed select did not retain keyboard focus")
			}
			name := "select-external"
			if removed {
				in.Config[0].Options = []string{"应用", "便条"}
				name = "select-removed"
			} else {
				in.ConfigValues["mode"] = "远程来源"
			}
			tt.Frame()
			tt.Frame()
			controlRefreshDump(t, tt, name, dark)
			if !tt.Focused("搜索方式") {
				t.Error("closed select refresh lost keyboard focus")
			}
			if tt.HasText("应用") || a.configDirty(in, a.draft(in)) {
				t.Error("refresh opened a menu or changed an untouched value")
			}
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyEnd)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			controlRefreshDump(t, tt, name+"-recovered", dark)
			if got := a.draft(in).values["mode"]; got != "便条" {
				t.Errorf("refreshed select cannot recover with keyboard: %v, want 便条", got)
			}
			if !tt.Focused("搜索方式") {
				t.Error("select recovery lost focus")
			}
		})
	}
}

func TestControlRefreshClosedSelectExternalValue(t *testing.T) {
	checkControlRefreshClosedSelect(t, false)
}

func TestControlRefreshClosedSelectRemovedValue(t *testing.T) {
	checkControlRefreshClosedSelect(t, true)
}
