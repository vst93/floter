package settingsui

import (
	"testing"

	"floter/internal/extensions"
	"floter/internal/i18n"
)

func TestRunFormErrorsBelongToTheirIntegration(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, first := runStateApp(Param{ID: "target", Label: "First target", Kind: extensions.ParamText})
			second := *first
			second.ID = "second"
			second.Params = []Param{{ID: "target", Label: "Second target", Kind: extensions.ParamText}}
			a.runOpen[second.ID] = true
			a.Actions.RunWithParams = func(string, map[string]string) *extensions.ParamFailure {
				return &extensions.ParamFailure{Key: extensions.RunErrorParamInvalid, Param: "target"}
			}
			copy := i18n.For("en").Settings
			ta := runStateFrame(t, a, first, dark)
			if err := ta.Click(copy.RunLabel); err != nil {
				t.Fatal(err)
			}
			ta.Frame()
			firstError := copy.RunParamInvalid("First target")
			if !ta.HasText(firstError) {
				t.Fatal("first form did not show its failure")
			}
			tb := runStateFrame(t, a, &second, dark)
			runStateDump(t, tb, "error-owner", dark)
			if tb.HasText(firstError) {
				t.Error("first run failure leaked into second form")
			}
			if err := tb.Click(copy.RunCancel); err != nil {
				t.Fatal(err)
			}
			ta.Frame()
			if !ta.HasText(firstError) {
				t.Error("canceling second form erased first failure")
			}
			a.runOpen[second.ID] = true
			tb.Frame()
			if err := tb.Click(copy.RunLabel); err != nil {
				t.Fatal(err)
			}
			tb.Frame()
			if !tb.HasText(copy.RunParamInvalid("Second target")) {
				t.Error("second failure missing")
			}
			ta.Frame()
			if !ta.HasText(firstError) {
				t.Error("second run replaced first failure")
			}
		})
	}
}

func TestRunFormInputFollowsParameterIdentity(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a, integration := runStateApp(
				Param{ID: "target", Label: "Target", Kind: extensions.ParamText},
				Param{ID: "note", Label: "Note", Kind: extensions.ParamText},
			)
			tt := runStateFrame(t, a, integration, dark)
			label := mustFind(t, tt, "Target")
			tt.ClickAt(568, label.Y+label.H/2)
			tt.Frame()
			tt.Type("first")
			tt.Frame()
			integration.Params[0], integration.Params[1] = integration.Params[1], integration.Params[0]
			tt.Frame()
			tt.Type("-tail")
			tt.Frame()
			runStateDump(t, tt, "input-identity", dark)
			if got := a.runValues["runner"]["target"]; got != "first-tail" {
				t.Errorf("parameter reorder lost edit target: target=%q note=%q", got, a.runValues["runner"]["note"])
			}
			if got := a.runValues["runner"]["note"]; got != "" {
				t.Errorf("reorder typed into unrelated parameter: %q", got)
			}
			if !tt.Focused("Target") {
				t.Error("parameter reorder moved focus off Target")
			}
		})
	}
}
