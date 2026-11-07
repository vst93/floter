package settingsui

import (
	"floter/internal/extensions"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
	"testing"
)

func checkParamRefreshExit(t *testing.T, menu bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			a.toolParams = []toolParam{{ID: "target", Label: "Target", Kind: extensions.ParamText}}
			tt := localParamsFrame(t, a, dark)
			label, name := a.copy().ParamsLabel, "param-editor-removed"
			if menu {
				label, name = a.copy().ParamsKind, "param-menu-removed"
			}
			optionExitFocus(t, tt, label)
			if menu {
				tt.Key(0, ui.KeyDown)
			} else {
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("Edited target")
			}
			a.toolParams = nil
			tt.Frame()
			tt.Frame()
			menuLifetimeDump(t, tt, name, dark)
			if !tt.Focused(a.copy().ParamsAdd) {
				t.Errorf("%s lost Add outlet", name)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if len(a.toolParams) != 1 {
				t.Errorf("%s could not add after refresh", name)
			}
		})
	}
}

func TestMenuExitRemovedParameterEditor(t *testing.T) { checkParamRefreshExit(t, false) }
func TestMenuExitRemovedParameterMenu(t *testing.T)   { checkParamRefreshExit(t, true) }

func checkResidencyDraftExit(t *testing.T, apply bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			if err := a.Store.Update(func(s *settings.Settings) { s.SurfaceResidencySeconds = 0 }); err != nil {
				t.Fatal(err)
			}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				c.Root().Background(c.Theme().Background)
				ui.Column(c).Padding(20).Children(func() { a.residencyRow(c, a.copy()) })
			}, 720, 400)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			tt.Frame()
			label := a.copy().SurfaceResidency
			optionExitFocus(t, tt, label)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyEnd)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if !a.residencyCustomOpen {
				t.Fatal("custom input did not open")
			}
			name := "residency-custom-choice"
			if apply {
				name = "residency-apply-focus"
				optionExitFocus(t, tt, a.copy().SurfaceResidencyCustomLabel)
				tt.Key(ui.Cmd, ui.KeyA)
				tt.Type("30")
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				tt.Frame()
				if a.residencyCustomOpen || a.settings().SurfaceResidencySeconds != 30 {
					t.Fatal("preset commit did not close field")
				}
				menuLifetimeDump(t, tt, name, dark)
				if !tt.Focused(label) {
					t.Error("preset apply removed focused input without returning to selector")
				}
			} else {
				menuLifetimeDump(t, tt, name, dark)
				// The draft choice must remain current so choosing Off again can
				// cancel this input without committing its unsubmitted duration.
				tt.Key(0, ui.KeyDown)
				tt.Key(0, ui.KeyHome)
				tt.Key(0, ui.KeyEnter)
				tt.Frame()
				if a.residencyCustomOpen {
					t.Error("selecting stored Off cannot dismiss custom draft")
				}
				if a.settings().SurfaceResidencySeconds != 0 {
					t.Error("custom draft cancel changed stored duration")
				}
			}
		})
	}
}
