package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func checkRunKindFocus(t *testing.T, choice bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			param := Param{ID: "target", Label: "Target", Kind: extensions.ParamText, Default: "12"}
			name := "number-kind-focus"
			if choice {
				param.Kind, param.Default, param.Options = extensions.ParamSelect, "true", []string{"true", "false"}
				name = "boolean-kind-focus"
			}
			a, in := runStateApp(param)
			tt := runRecoveryFrame(t, a, in, dark)
			if choice {
				if err := tt.Click("false"); err != nil {
					t.Fatal(err)
				}
			} else {
				r := mustFind(t, tt, "Target")
				tt.ClickAt(568, r.Y+r.H/2)
			}
			tt.Key(0, ui.KeyTab)
			tt.Key(ui.Shift, ui.KeyTab)
			if choice {
				in.Params[0].Kind = extensions.ParamBoolean
			} else {
				in.Params[0].Kind = extensions.ParamNumber
			}
			tt.Frame()
			tt.Frame()
			validationRefreshDump(t, tt, name, dark)
			if !tt.Focused("Target") {
				t.Error("parameter kind refresh lost current control focus")
			}
			if choice {
				tt.Key(0, ui.KeySpace)
				tt.Frame()
				if a.runValues[in.ID]["target"] != "true" {
					t.Error("new boolean did not accept keyboard toggle")
				}
			} else {
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("24")
				tt.Frame()
				if a.runValues[in.ID]["target"] != "24" {
					t.Errorf("new number did not accept keyboard input: %v", a.runValues[in.ID])
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				tt.Frame()
				if a.runValues[in.ID]["target"] != "12" {
					t.Error("new number undo crossed old editor")
				}
			}
			// The ordinary Cancel path remains available after recovery.
			if err := tt.Click(i18n.For("en").Settings.RunCancel); err != nil {
				t.Fatal(err)
			}
			if a.runOpen[in.ID] {
				t.Error("cancel stopped closing form")
			}
		})
	}
}

func TestRunKindFocusTextToNumber(t *testing.T)    { checkRunKindFocus(t, false) }
func TestRunKindFocusChoiceToBoolean(t *testing.T) { checkRunKindFocus(t, true) }

func TestRunKindFocusDuplicateIDsDoNotShareHandles(t *testing.T) {
	a, in := runStateApp(Param{ID: "same", Label: "First", Kind: extensions.ParamText}, Param{ID: "same", Label: "Second", Kind: extensions.ParamNumber})
	tt := runRecoveryFrame(t, a, in, true)
	in.Params[1].Kind = extensions.ParamBoolean
	tt.Frame()
	if !tt.HasText("First") || !tt.HasText("Second") {
		t.Fatal("duplicate declaration hid a row")
	}
}

func TestRunKindFocusDoesNotStealFromCancel(t *testing.T) {
	a, in := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText})
	tt := runRecoveryFrame(t, a, in, true)
	r := mustFind(t, tt, "Target")
	tt.ClickAt(568, r.Y+r.H/2)
	tt.Key(0, ui.KeyTab)
	if !tt.Focused(i18n.For("en").Settings.RunCancel) {
		t.Fatal("Cancel not keyboard focused")
	}
	in.Params[0].Kind = extensions.ParamNumber
	tt.Frame()
	if !tt.Focused(i18n.For("en").Settings.RunCancel) {
		t.Fatal("unfocused parameter stole Cancel focus")
	}
}

func TestRunKindFocusDuplicateIntegrationsDoNotShareHandles(t *testing.T) {
	a, in := runStateApp(Param{ID: "target", Label: "Target", Kind: extensions.ParamText})
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Children(func() {
			ui.Column(c.Key("first-card")).Children(func() { a.runForm(c, a.copy(), *in) })
			ui.Column(c.Key("second-card")).Children(func() { a.runForm(c, a.copy(), *in) })
		})
	}, 720, 900)
	tt.Frame()
	in.Params[0].Kind = extensions.ParamNumber
	tt.Frame()
	if !tt.HasText("Target") {
		t.Fatal("duplicate integration lost parameter forms")
	}
}
