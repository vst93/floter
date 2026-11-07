package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

func validationRefreshDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IL_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-il-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkDeclarationDefaultRefresh(t *testing.T, kind, corrected string) {
	t.Helper()
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			tool := callbackTool("weather")
			tool.Params = []extensions.ParamDefinition{{ID: "target", Label: "Target", Kind: kind, Flag: "--target", Options: []string{"fast", "slow"}}}
			a.beginEdit("local.weather", tool)
			calls := 0
			a.Actions.EditCustomTool = func(_ string, sent CustomTool) {
				calls++
				if err := extensions.ValidateParams(sent.Params); err != nil {
					t.Errorf("invalid declaration reached save: %v", err)
				}
			}
			tt := declarationRefreshFrame(t, a, dark)
			copy := a.copy()
			r := mustFind(t, tt, copy.ParamsDefault)
			tt.ClickAt(r.X+30, r.Y+r.H+18)
			if !tt.Focused(copy.ParamsDefault) {
				t.Fatal("default field not focused")
			}
			tt.Type("invalid")
			tt.Frame()
			tt.Frame()
			validationRefreshDump(t, tt, "default-"+kind, dark)
			if !tt.HasText(copy.RunParamInvalid(copy.ParamsDefault)) {
				t.Error("invalid default has no inline reason")
			}
			if err := tt.Click(copy.LocalToolSave); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 0 || !a.toolOpen {
				t.Errorf("invalid %s default dispatched %d saves, open=%v", kind, calls, a.toolOpen)
				return
			}
			r = mustFind(t, tt, copy.ParamsDefault)
			tt.ClickAt(r.X+30, r.Y+r.H+18)
			tt.Key(ui.Cmd, ui.KeyA)
			tt.Type(corrected)
			tt.Frame()
			tt.Frame()
			if tt.HasText(copy.RunParamInvalid(copy.ParamsDefault)) {
				t.Error("corrected default kept obsolete reason")
			}
			if err := tt.Click(copy.LocalToolSave); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if calls != 1 || a.toolOpen {
				t.Errorf("corrected declaration did not save once: calls=%d open=%v", calls, a.toolOpen)
			}
		})
	}
}

func TestDeclarationRefreshNumberDefault(t *testing.T) {
	checkDeclarationDefaultRefresh(t, extensions.ParamNumber, "12")
}
func TestDeclarationRefreshBooleanDefault(t *testing.T) {
	checkDeclarationDefaultRefresh(t, extensions.ParamBoolean, "true")
}
func TestDeclarationRefreshSelectDefault(t *testing.T) {
	checkDeclarationDefaultRefresh(t, extensions.ParamSelect, "fast")
}
func TestDeclarationRefreshPlaceholderExit(t *testing.T) { checkDeclarationPlaceholderExit(t) }
func checkDeclarationPlaceholderExit(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.toolParams = []toolParam{{ID: "target", Kind: extensions.ParamText, Placeholder: "记住此提示", Flag: "--target"}}
			tt := localParamsFrame(t, a, dark)
			copy := a.copy()
			r := mustFind(t, tt, copy.ParamsPlaceholder)
			tt.ClickAt(r.X+30, r.Y+r.H+18)
			tt.Key(0, ui.KeyTab)
			tt.Key(ui.Shift, ui.KeyTab)
			if !tt.Focused(copy.ParamsPlaceholder) {
				t.Fatal("placeholder not keyboard focused")
			}
			a.toolParams[0].Kind = extensions.ParamBoolean
			tt.Frame()
			tt.Frame()
			validationRefreshDump(t, tt, "placeholder-exit", dark)
			if !tt.Focused(copy.ParamsKind) {
				t.Error("hidden placeholder lost kind focus")
			}
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyHome)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if a.toolParams[0].Kind != extensions.ParamText || a.toolParams[0].Placeholder != "记住此提示" {
				t.Errorf("placeholder keyboard recovery failed: %+v", a.toolParams[0])
			}
		})
	}
}

func declarationRefreshFrame(t *testing.T, a *App, dark bool) *ui.Tester {
	t.Helper()
	ground := realPageGround(t, dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Children(func() { a.extensionMetrics(func() { a.localToolSection(c, a.copy()) }) })
	}, 720, 1300)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}
