package settingsui

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func checkModeRecovery(t *testing.T, field string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tool := callbackTool("weather")
			tool.VersionArgs, tool.Description = "--version", "Weather provider"
			a.beginEdit("local.weather", tool)
			tt := toolCallbackFrame(t, a, dark)
			copy := a.copy()
			label := copy.LocalToolBackground
			if field == "platform" {
				label = customPlatforms[0].Label
			}
			if field == "advanced" {
				label = copy.LocalToolAdvanced
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			tt.Key(ui.Shift, ui.KeyTab)
			beforePlatform := a.toolPlatforms[customPlatforms[0].ID]
			beforeAdvanced := a.toolAdvanced
			a.toolMode = routeScript
			tt.Frame()
			contentRecoveryDump(t, tt, "mode-"+field, dark)
			if !tt.Focused(label) {
				t.Errorf("mode refresh lost %s focus", field)
			}
			if field == "output" {
				tt.Key(0, ui.KeyLeft)
			} else {
				tt.Key(0, ui.KeySpace)
			}
			tt.Frame()
			if field == "output" && a.toolOutput != "" {
				t.Errorf("mode refresh blocked output keyboard: %q", a.toolOutput)
			}
			if field == "platform" && a.toolPlatforms[customPlatforms[0].ID] == beforePlatform {
				t.Error("mode refresh blocked platform keyboard")
			}
			if field == "advanced" && a.toolAdvanced == beforeAdvanced {
				t.Error("mode refresh blocked disclosure keyboard")
			}
		})
	}
}

func TestContentRecoveryModeOutput(t *testing.T)   { checkModeRecovery(t, "output") }
func TestContentRecoveryModePlatform(t *testing.T) { checkModeRecovery(t, "platform") }
