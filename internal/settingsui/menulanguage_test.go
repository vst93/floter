package settingsui

import (
	"image/png"
	"os"
	"testing"

	"floter/internal/extensions"
	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func menuLifetimeDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13IS_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-is-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func checkMenuLanguage(t *testing.T, residency bool) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := copyLifecycleApp(t, dark)
			if err := a.Store.Update(func(s *settings.Settings) { s.Language = "en"; s.SurfaceResidencySeconds = 0 }); err != nil {
				t.Fatal(err)
			}
			a.toolParams = []toolParam{{ID: "target", Label: "Target", Kind: extensions.ParamText}}
			var tt *ui.Tester
			label, name := a.copy().ParamsKind, "param-language"
			if residency {
				label, name = a.copy().SurfaceResidency, "residency-language"
				tt = ui.NewTester(func(c *ui.Context) {
					c.SetTheme(a.tokens(c).Theme)
					c.Root().Background(c.Theme().Background)
					ui.Column(c).Padding(20).Children(func() { a.residencyRow(c, a.copy()) })
				}, 720, 400)
				tt.SetDark(dark)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
			} else {
				tt = localParamsFrame(t, a, dark)
			}
			optionExitFocus(t, tt, label)
			tt.Key(0, ui.KeyDown)
			tt.Key(0, ui.KeyDown)
			if err := a.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			tt.Frame()
			menuLifetimeDump(t, tt, name, dark)
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			if residency {
				if got := a.settings().SurfaceResidencySeconds; got != residencyPresetSeconds[0] {
					t.Errorf("translation lost residency highlight: %d, want %d", got, residencyPresetSeconds[0])
				}
				label = a.copy().SurfaceResidency
			} else {
				if got := a.toolParams[0].Kind; got != extensions.ParamNumber {
					t.Errorf("translation lost parameter highlight: %s", got)
				}
				label = a.copy().ParamsKind
			}
			if !tt.Focused(label) {
				t.Error("translation lost trigger focus")
			}
		})
	}
}

func TestMenuLanguageParameter(t *testing.T) { checkMenuLanguage(t, false) }
func TestMenuLanguageResidency(t *testing.T) { checkMenuLanguage(t, true) }
