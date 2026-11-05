package settingsui

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestSelectClearReservedDisplayName(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "mode", Label: "搜索方式", Type: "select", Options: []string{"Not set", "files"}}}, ConfigValues: map[string]any{"mode": "files"}}
			a, tt := configIdentityFrame(t, dark, &integration)
			none := a.copy().ConfigNone
			integration.Config[0].Options[0] = none
			tt.Frame()
			if err := tt.Click("files"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			selectValueDump(t, tt, "reserved-menu", dark)
			tt.Key(0, ui.KeyHome)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			selectValueDump(t, tt, "reserved-cleared", dark)
			if got := a.draft(integration).values["mode"]; got != nil {
				t.Fatalf("reserved display name removed the clear action: got %#v, want nil", got)
			}
			if err := tt.Click(a.copy().ConfigNone); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if got := a.draft(integration).values["mode"]; got != none {
				t.Errorf("real reserved-name option was treated as clear: %#v", got)
			}
		})
	}
}

func TestSelectClearEmptySchemaOption(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "mode", Label: "搜索方式", Type: "select", Options: []string{"", "files"}}}, ConfigValues: map[string]any{"mode": "files"}}
			a, tt := configIdentityFrame(t, dark, &integration)
			if err := tt.Click("files"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			selectValueDump(t, tt, "empty-option-menu", dark)
			tt.Key(0, ui.KeyUp)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			selectValueDump(t, tt, "empty-option-cleared", dark)
			if got := a.draft(integration).values["mode"]; got != nil {
				t.Errorf("empty schema choice did not clear the optional field: got %#v, want nil", got)
			}
		})
	}
}
