package settingsui

import (
	"errors"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func externalResultDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13JK_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-jk-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestExternalConfigAcceptanceRetiresSaveFailure(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			in := Integration{ID: "local.external", Config: []ConfigField{{Key: "path", Label: "工作目录", Type: "path"}}, ConfigValues: map[string]any{"path": "/workspace/before"}}
			a, tt := configIdentityFrame(t, dark, &in)
			a.Actions.SaveConfiguration = func(_ string, _ map[string]any, done func(error)) {
				done(errors.New("storage temporarily unavailable"))
			}
			configIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/accepted")
			tt.Frame()
			if err := tt.Click(a.copy().ConfigSave); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if !tt.HasText("storage temporarily unavailable") {
				t.Fatal("save did not fail")
			}
			in.ConfigValues["path"] = "/workspace/accepted"
			tt.Frame()
			tt.Frame()
			externalResultDump(t, tt, "external-save-error", dark)
			if tt.HasText("storage temporarily unavailable") {
				t.Error("external acceptance left an obsolete save failure")
			}
			if !tt.HasText(a.copy().ConfigSavedState) {
				t.Error("accepted draft does not read Saved")
			}
		})
	}
}

func checkExternalConfigValidity(t *testing.T, number bool) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			max, maxLength := 10.0, 4
			field := ConfigField{Key: "value", Label: "结果上限", Type: "number", Maximum: &max}
			var initial, next, valid any = 15.0, 20.0, 5.0
			name := "external-number-validity"
			if !number {
				field = ConfigField{Key: "value", Label: "保存名称", Type: "text", MaxLength: &maxLength}
				initial, next, valid = "before-long", "after-long", "ok"
				name = "external-text-validity"
			}
			in := Integration{ID: "local.external", Config: []ConfigField{field}, ConfigValues: map[string]any{"value": initial}}
			a, tt := configIdentityFrame(t, dark, &in)
			configIdentityFocus(t, tt, field.Label)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			problem := a.copy().ConfigOutOfRange
			if !number {
				problem = a.copy().ConfigOutOfLength
			}
			if !tt.HasText(problem) {
				t.Fatal("initial validation did not occur")
			}
			in.ConfigValues["value"] = next
			tt.Frame()
			tt.Frame()
			externalResultDump(t, tt, name, dark)
			if !tt.HasText(problem) {
				t.Error("external invalid replacement erased the current field problem")
			}
			if got := a.draft(in).values["value"]; got != next {
				t.Errorf("external value hidden: %v", got)
			}
			in.ConfigValues["value"] = valid
			tt.Frame()
			tt.Frame()
			if tt.HasText(problem) {
				t.Error("valid replacement did not recover")
			}
		})
	}
}

func TestExternalConfigNumberRetainsCurrentValidity(t *testing.T) {
	checkExternalConfigValidity(t, true)
}
func TestExternalConfigTextRetainsCurrentValidity(t *testing.T) {
	checkExternalConfigValidity(t, false)
}

func TestExternalConfigAcceptedSelectionEndsRange(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			field := ConfigField{Key: "sources", Label: "数据来源", Type: "multiselect", Options: []string{"alpha", "beta", "gamma"}}
			in := Integration{ID: "local.external", Config: []ConfigField{field}, ConfigValues: map[string]any{"sources": []any{}}}
			a, tt := configIdentityFrame(t, dark, &in)
			if err := tt.Click("alpha"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			in.ConfigValues["sources"] = []any{"alpha"}
			tt.Frame()
			if err := tt.ClickWith(ui.Shift, "gamma"); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			externalResultDump(t, tt, "external-selection-range", dark)
			if got := a.draft(in).values["sources"]; !configValuesEqual(got, []any{"alpha", "gamma"}) {
				t.Errorf("accepted external selection retained old range: %v", got)
			}
		})
	}
}

func TestExternalConfigAcceptanceKeepsUnfinishedSibling(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			in := Integration{ID: "local.external", Config: []ConfigField{
				{Key: "path", Label: "工作目录", Type: "path"},
				{Key: "limit", Label: "结果上限", Type: "number"},
			}, ConfigValues: map[string]any{"path": "/workspace/before", "limit": nil}}
			a, tt := configIdentityFrame(t, dark, &in)
			a.Actions.SaveConfiguration = func(_ string, _ map[string]any, done func(error)) { done(errors.New("save was refused")) }
			configIdentityFocus(t, tt, "工作目录")
			tt.Type("/workspace/accepted")
			tt.Frame()
			if err := tt.Click(a.copy().ConfigSave); err != nil {
				t.Fatal(err)
			}
			configIdentityFocus(t, tt, "结果上限")
			tt.Type("1e")
			tt.Frame()
			in.ConfigValues["path"] = "/workspace/accepted"
			tt.Frame()
			if !tt.HasText("save was refused") || refreshRead(tt) != "1e" {
				t.Error("accepting one field retired the unfinished form's failure or input")
			}
		})
	}
}
