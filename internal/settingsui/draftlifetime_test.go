package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func lifetimeDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HO_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ho-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkRevertedConfig(t *testing.T, kind, original, edit, external string, initial, next any) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "value", Label: "配置值", Type: kind}
			integration := Integration{ID: "local.lifetime", Config: []ConfigField{field}, ConfigValues: map[string]any{"value": initial}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, field.Label)
			tt.Type(edit)
			tt.Frame()
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type(original)
			tt.Frame()
			if a.configDirty(integration, a.draft(integration)) {
				t.Fatal("reverted edit is still dirty")
			}
			integration.ConfigValues[field.Key] = next
			tt.Frame()
			lifetimeDump(t, tt, "reverted-"+kind, dark)
			if got := refreshRead(tt); got != external {
				t.Errorf("reverted %s masks host update: %q, want %q", kind, got, external)
			}
			if !tt.Focused(field.Label) {
				t.Error("host update stole field focus")
			}
			tt.Type(edit)
			tt.Frame()
			integration.ConfigValues[field.Key] = initial
			tt.Frame()
			if got := refreshRead(tt); got != edit {
				t.Errorf("real edit was discarded: %q", got)
			}
		})
	}
}

func TestDraftLifetimeRevertedTextFollowsHost(t *testing.T) {
	checkRevertedConfig(t, "text", "/workspace/initial", "/workspace/local", "/workspace/external", "/workspace/initial", "/workspace/external")
}

func TestDraftLifetimeRevertedNumberFollowsHost(t *testing.T) {
	checkRevertedConfig(t, "number", "5.0", "12", "40", 5.0, 40.0)
}

func TestDraftLifetimeRemovedNumberRetiresPartialInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "limit", Label: "结果上限", Type: "number"}
			integration := Integration{ID: "local.lifetime", Config: []ConfigField{field}, ConfigValues: map[string]any{"limit": 5.0}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, field.Label)
			tt.Type("1e")
			tt.Frame()
			integration.Config = nil
			tt.Frame()
			integration.Config = []ConfigField{field}
			tt.Frame()
			lifetimeDump(t, tt, "removed-number", dark)
			configIdentityFocus(t, tt, field.Label)
			if got := refreshRead(tt); got != "" {
				t.Errorf("removed number resurrected partial text: %q", got)
			}
			if tt.HasText(a.copy().ConfigNumber) {
				t.Error("removed number resurrected old parse error")
			}
			tt.Type("8")
			tt.Frame()
			if a.draft(integration).values[field.Key] != 8.0 {
				t.Error("readded field cannot be edited")
			}
		})
	}
}

func TestDraftLifetimeHostAcknowledgesMatchingEdit(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "directory", Label: "工作目录", Type: "text"}
			integration := Integration{ID: "local.lifetime", Config: []ConfigField{field}, ConfigValues: map[string]any{"directory": "/workspace/old"}}
			a, tt := configIdentityFrame(t, dark, &integration)
			configIdentityFocus(t, tt, field.Label)
			tt.Type("/workspace/shared")
			tt.Frame()
			integration.ConfigValues[field.Key] = "/workspace/shared"
			tt.Frame()
			lifetimeDump(t, tt, "host-ack", dark)
			if a.configDirty(integration, a.draft(integration)) || tt.HasText(a.copy().ConfigUnsaved) {
				t.Error("host-confirmed edit still says Unsaved changes")
			}
			integration.ConfigValues[field.Key] = "/workspace/latest"
			tt.Frame()
			if got := refreshRead(tt); got != "/workspace/latest" {
				t.Errorf("host-confirmed edit masks next generation: %q", got)
			}
		})
	}
}
