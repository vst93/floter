package settingsui

import (
	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
	"testing"
)

func checkParamRemovalRecovery(t *testing.T, last bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.toolParams = []toolParam{{ID: "first", Label: "First", Kind: extensions.ParamText}}
			if !last {
				a.toolParams = append(a.toolParams, toolParam{ID: "second", Label: "Second", Kind: extensions.ParamText})
			}
			tt := localParamsFrame(t, a, dark)
			copy := a.copy()
			r := mustFind(t, tt, copy.ParamsLabel)
			tt.ClickAt(r.X+30, r.Y+r.H+18)
			for n := 0; n < 12 && !tt.Focused(copy.ParamsRemove); n++ {
				tt.Key(0, ui.KeyTab)
			}
			if !tt.Focused(copy.ParamsRemove) {
				t.Fatal("remove button not keyboard reachable")
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			tt.Frame()
			name := "remove-first"
			if last {
				name = "remove-last"
			}
			contentRecoveryDump(t, tt, name, dark)
			if last {
				if !tt.Focused(copy.ParamsAdd) {
					t.Error("last removal lost add focus")
				}
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if len(a.toolParams) != 1 {
					t.Error("last removal cannot add with keyboard")
				}
			} else {
				if !tt.Focused(copy.ParamsLabel) {
					t.Error("first removal lost remaining label focus")
				}
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("Remaining")
				tt.Frame()
				if len(a.toolParams) != 1 || a.toolParams[0].ID != "second" || a.toolParams[0].Label != "Remaining" {
					t.Errorf("first removal lost next input: %+v", a.toolParams)
				}
			}
		})
	}
}

func TestParamRecoveryRemovedLast(t *testing.T)  { checkParamRemovalRecovery(t, true) }
func TestParamRecoveryRemovedFirst(t *testing.T) { checkParamRemovalRecovery(t, false) }
