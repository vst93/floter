package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func conflictDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HP_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hp-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConflictRecoveryTextUsesCurrentSavedValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "directory", Label: "工作目录", Type: "path"}
			in := Integration{ID: "local.conflict", Config: []ConfigField{field}, ConfigValues: map[string]any{field.Key: "/old"}}
			a, tt := configIdentityFrame(t, dark, &in)
			configIdentityFocus(t, tt, field.Label)
			tt.Type("/local")
			tt.Frame()
			in.ConfigValues[field.Key] = "/external"
			tt.Frame()
			if got := refreshRead(tt); got != "/local" {
				t.Fatalf("conflict lost real edit: %q", got)
			}
			tt.Type("/external")
			tt.Frame()
			conflictDump(t, tt, "text-current", dark)
			if a.configDirty(in, a.draft(in)) {
				t.Error("returning to external text still says Unsaved changes")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type("/old")
			tt.Frame()
			if !a.configDirty(in, a.draft(in)) {
				t.Error("obsolete original text falsely says Saved")
			}
		})
	}
}

func TestConflictRecoveryNumberDefaultUsesCurrentSavedValue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "limit", Label: "结果上限", Type: "number", Default: 5.0}
			in := Integration{ID: "local.conflict", Config: []ConfigField{field}, ConfigValues: map[string]any{field.Key: 5.0}}
			a, tt := configIdentityFrame(t, dark, &in)
			configIdentityFocus(t, tt, field.Label)
			tt.Type("1e")
			tt.Frame()
			in.ConfigValues[field.Key] = 40.0
			tt.Frame()
			if got := refreshRead(tt); got != "1e" {
				t.Fatalf("conflict lost partial number: %q", got)
			}
			if err := tt.Click(a.copy().ConfigDefaults); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			conflictDump(t, tt, "number-default", dark)
			if !a.configDirty(in, a.draft(in)) {
				t.Error("restoring obsolete default falsely says Saved")
			}
			configIdentityFocus(t, tt, field.Label)
			tt.Type("40.0")
			tt.Frame()
			if a.configDirty(in, a.draft(in)) {
				t.Error("returning to external number still says Unsaved changes")
			}
		})
	}
}
