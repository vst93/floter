package settingsui

import (
	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestParamRecoveryOptionsWithdrawn(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.toolParams = []toolParam{{ID: "target", Label: "Target", Kind: extensions.ParamSelect, Options: "one\ntwo"}}
			tt := localParamsFrame(t, a, dark)
			copy := a.copy()
			r := mustFind(t, tt, copy.ParamsOptions)
			tt.ClickAt(r.X+30, r.Y+r.H+18)
			tt.Frame()
			if !tt.Focused(copy.ParamsOptions) {
				t.Fatal("options did not take focus")
			}
			tt.Key(0, ui.KeyTab)
			tt.Key(ui.Shift, ui.KeyTab)
			if !tt.Focused(copy.ParamsOptions) {
				t.Fatal("options keyboard roundtrip lost focus")
			}
			a.toolParams[0].Kind = extensions.ParamText
			tt.Frame()
			tt.Frame()
			contentRecoveryDump(t, tt, "options-withdrawn", dark)
			if !tt.Focused(copy.ParamsKind) {
				t.Error("withdrawn options lost kind focus")
			}
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyEnd)
			tt.Key(0, ui.KeyUp)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if a.toolParams[0].Kind != extensions.ParamSelect || a.toolParams[0].Options != "one\ntwo" {
				t.Errorf("options keyboard recovery failed: %+v", a.toolParams[0])
			}
		})
	}
}
