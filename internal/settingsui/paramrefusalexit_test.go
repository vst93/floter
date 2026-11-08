package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func TestToolRefusalExitParameter(t *testing.T) {
	for _, field := range []string{"id", "duplicate", "options", "flag", "badflag", "number", "boolean", "select"} {
		for _, dark := range []bool{false, true} {
			t.Run(field+"/"+appearanceName(dark), func(t *testing.T) {
				a := copyLifecycleApp(t, dark)
				a.beginEdit("", callbackTool("weather"))
				first := toolParam{ID: "first", Label: "首项", Kind: extensions.ParamText}
				bad := toolParam{ID: "second", Label: "需要修正", Kind: extensions.ParamText}
				copy := a.copy()
				target, fixed := copy.ParamsID, "second"
				switch field {
				case "id":
					bad.ID = "bad id"
				case "duplicate":
					bad.ID = "first"
				case "options":
					bad.Kind = extensions.ParamSelect
					target, fixed = copy.ParamsOptions, "small\nlarge"
				case "flag":
					bad.Kind = extensions.ParamBoolean
					target, fixed = copy.ParamsFlag, "--enabled"
				case "badflag":
					bad.Flag = "--bad flag"
					target, fixed = copy.ParamsFlag, "--value"
				case "number":
					bad.Kind, bad.Default = extensions.ParamNumber, "oops"
					target, fixed = copy.ParamsDefault, "12"
				case "boolean":
					bad.Kind, bad.Default, bad.Flag = extensions.ParamBoolean, "maybe", "--enabled"
					target, fixed = copy.ParamsDefault, "true"
				case "select":
					bad.Kind, bad.Default, bad.Options = extensions.ParamSelect, "missing", "small\nlarge"
					target, fixed = copy.ParamsDefault, "large"
				}
				a.toolParams = []toolParam{first, bad}
				var submitted []CustomTool
				a.Actions.ConnectLocalTool = func(v CustomTool) { submitted = append(submitted, v) }
				tt := toolRecoveryFrame(t, a, dark)
				if err := tt.Click(copy.LocalToolConnect); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				toolExitDump(t, tt, "refusal-param-"+field, dark)
				if len(submitted) != 0 {
					t.Fatal("invalid declaration was submitted")
				}
				if !tt.Focused(target) {
					t.Errorf("%s declaration refusal lost its field", field)
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type(fixed)
				tt.Frame()
				if err := tt.Click(copy.LocalToolConnect); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if len(submitted) != 1 {
					t.Errorf("%s declaration cannot correct and retry", field)
					return
				}
				if len(submitted[0].Params) != 2 || submitted[0].Params[0].ID != "first" || submitted[0].Params[0].Label != "首项" {
					t.Errorf("correction changed a different declaration: %+v", submitted[0].Params)
				}
			})
		}
	}
}
