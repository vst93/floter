package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func schemaRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HI_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hi-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSchemaRefreshConfigTypeRetiresHalfInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "value", Label: "配置值", Type: "number"}}, ConfigValues: map[string]any{"value": float64(5)}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, "配置值")
			tt.Type("1e")
			tt.Frame()
			integration.Config[0].Type = "text"
			tt.Frame()
			schemaRefreshDump(t, tt, "config-type-draft", dark)
			if a.draft(integration).liveText(integration.Config[0], "") != "" || tt.HasText(a.copy().ConfigNumber) {
				t.Errorf("text field inherited the number half-input or error: %q", tt.Texts())
			}
			configIdentityFocus(t, tt, "配置值")
			tt.Type("搜索")
			tt.Frame()
			if got := a.draft(integration).values["value"]; got != "搜索" {
				t.Errorf("new text editor could not recover: %v", got)
			}
		})
	}
}

func TestSchemaRefreshConfigTypeRetiresUndo(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			integration := Integration{ID: "local.search", Config: []ConfigField{{Key: "value", Label: "配置值", Type: "text"}}, ConfigValues: map[string]any{"value": "12"}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, "配置值")
			tt.Type("34")
			tt.Frame()
			integration.Config[0].Type = "number"
			tt.Frame()
			configIdentityFocus(t, tt, "配置值")
			tt.Key(ui.Cmd, ui.KeyZ)
			tt.Frame()
			schemaRefreshDump(t, tt, "config-type-undo", dark)
			if got := a.draft(integration).values["value"]; got != "34" {
				t.Errorf("number field inherited text undo and wrote an unrequested value: %v", got)
			}
			tt.Type("56")
			tt.Frame()
			if got := a.draft(integration).values["value"]; got != float64(56) {
				t.Errorf("new number editor could not recover: %v", got)
			}
		})
	}
}
