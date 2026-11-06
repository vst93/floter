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
