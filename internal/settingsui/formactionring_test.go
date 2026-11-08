package settingsui

import (
	"fmt"
	"github.com/egoist/mygo/ui"
	"math"
	"testing"
)

func checkFormActionRing(t *testing.T, name string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, in := runVisibilityApp(t, dark)
			label := a.copy().RunCancel
			if name == "run-submit" {
				label = a.copy().RunLabel
			}
			if name != "run-cancel" && name != "run-submit" {
				in.Params = nil
				a.Actions.RunWithParams = nil
				in.ConfigValues = map[string]any{}
				for i := 0; i < 10; i++ {
					key := fmt.Sprintf("c%d", i)
					in.Config = append(in.Config, ConfigField{Key: key, Label: fmt.Sprintf("设置 %d", i+1), Type: "text", Default: "default"})
					in.ConfigValues[key] = "stored"
				}
				a.configOpen = map[string]bool{in.ID: true}
				a.Actions.SaveConfiguration = func(string, map[string]any, func(error)) { t.Error("ring triggered save") }
				a.Actions.CopyConfiguration = func(string, map[string]any, func(error)) { t.Error("ring triggered copy") }
				a.Actions.ExportConfiguration = func(string, map[string]any, func(error)) { t.Error("ring triggered export") }
				label = a.copy().ConfigDefaults
				switch name {
				case "config-save":
					label = a.copy().ConfigSave
				case "config-copy":
					label = a.copy().ConfigCopy
				case "config-export":
					label = a.copy().ConfigExport
				}
			}
			tt := toolVisibilityFrame(t, a, dark)
			if len(in.Config) > 0 {
				a.draft(*in).values["c0"] = "mine"
				tt.Frame()
			}
			for i := 0; i < 60 && !tt.Focused(label); i++ {
				tt.Key(0, ui.KeyTab)
				tt.Frame()
			}
			if !tt.Focused(label) {
				t.Fatalf("cannot focus %s", label)
			}
			outputBoundsDump(t, tt, name, dark)
			r := mustFind(t, tt, label)
			if name != "config-copy" && name != "config-export" {
				// actionButton's discoverable word is centered inside its 30u
				// face; Find returns that word, not the enclosing control.
				r.Y -= (30 - r.H) / 2
				r.H = 30
			}
			for side, p := range map[string][2]int{"top": {int(r.X + r.W/2), int(r.Y - 2)}, "bottom": {int(r.X + r.W/2), int(math.Round(float64(r.Y + r.H + 1)))}} {
				red, _, blue := rgbAt(tt.Image(), p[0], p[1])
				if blue-red < 20 {
					t.Errorf("%s %s ring missing: rect=%+v blue-red=%d", name, side, r, blue-red)
				}
			}
			runVisibilityWheel(t, a, tt)
		})
	}
}

func TestFormActionRunCancelRing(t *testing.T) { checkFormActionRing(t, "run-cancel") }
func TestFormActionRunSubmitRing(t *testing.T) { checkFormActionRing(t, "run-submit") }
