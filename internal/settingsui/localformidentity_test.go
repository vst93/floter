package settingsui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/settings"
)

func TestLocalFormNewToolHasItsOwnTextHistory(t *testing.T) {
	a := localDraftApp()
	a.beginEdit("first", CustomTool{Name: "First", Command: "first"})
	tt := localDraftFrame(t, a, true)
	localFormFocus(t, tt, a.copy().LocalToolName)
	tt.Type("Changed first")
	tt.Frame()
	a.beginEdit("second", CustomTool{Name: "Second", Command: "second"})
	tt.Frame()
	localFormFocus(t, tt, a.copy().LocalToolName)
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if a.toolName != "Second" {
		t.Errorf("new tool inherited text undo: %q", a.toolName)
	}
	tt.Type("Changed second")
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if a.toolName != "Second" {
		t.Errorf("current tool lost its own undo: %q", a.toolName)
	}
}

func TestLocalFormLocalePreservesParameterInput(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := localDraftApp()
			a.toolParams = []toolParam{{ID: "target", Label: "Target", Kind: extensions.ParamText}}
			tt := localParamsFrame(t, a, dark)
			r := mustFind(t, tt, a.copy().ParamsLabel)
			tt.ClickAt(r.X+40, r.Y+r.H+3+15)
			tt.Frame()
			if !tt.Focused(a.copy().ParamsLabel) {
				t.Fatal("parameter label did not take focus")
			}
			tt.Key(ui.Cmd, ui.KeyA)
			if err := a.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Type("Destination")
			tt.Frame()
			localFormDump(t, tt, "parameter-locale", dark)
			if a.toolParams[0].Label != "Destination" {
				t.Errorf("locale switch lost parameter edit: %q", a.toolParams[0].Label)
			}
		})
	}
}
