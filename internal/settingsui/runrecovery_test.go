package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func recoveryDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HT_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-ht-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunRecoveryNewParameterDefaults(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, integration := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "original"})
			tt := runStateFrame(t, a, integration, dark)
			a.runValues["runner"]["target"] = "my edit"
			integration.Params = append(integration.Params,
				Param{ID: "format", Label: "Format", Kind: extensions.ParamSelect, Default: "json", Options: []string{"text", "json"}},
				Param{ID: "limit", Label: "Limit", Kind: extensions.ParamNumber, Default: "12"},
			)
			tt.Frame()
			tt.Frame()
			recoveryDump(t, tt, "new-parameter", dark)
			if got := a.runValues["runner"]; got["target"] != "my edit" || got["format"] != "json" || got["limit"] != "12" {
				t.Errorf("new parameters did not receive defaults without replacing edits: %v", got)
			}
		})
	}
}

func TestRunRecoveryRemovedParameterCannotReachCommand(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, integration := runStateApp(
				Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "current"},
				Param{ID: "obsolete", Label: "Obsolete", Kind: extensions.ParamText, Default: "old"},
			)
			tt := runStateFrame(t, a, integration, dark)
			integration.Params = integration.Params[:1]
			tt.Frame()
			calls := 0
			a.Actions.RunWithParams = func(_ string, values map[string]string) *extensions.ParamFailure {
				calls++
				if len(values) != 1 || values["target"] != "current" {
					t.Errorf("removed parameter reached command: %v", values)
				}
				return nil
			}
			if err := tt.Click(i18n.For("en").Settings.RunLabel); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 {
				t.Errorf("runs=%d, want 1", calls)
			}
			integration.Params = append(integration.Params, Param{ID: "obsolete", Label: "Re-added", Kind: extensions.ParamText, Default: "fresh"})
			tt.Frame()
			tt.Frame()
			if got := a.runValues["runner"]["obsolete"]; got != "fresh" {
				t.Errorf("re-added parameter resurrected old answer: %q", got)
			}
		})
	}
}
