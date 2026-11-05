package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

func localFormDump(t *testing.T, tt *ui.Tester, name string, dark bool) {
	t.Helper()
	if tag := os.Getenv("FLOTER_P13HC_DUMP"); tag != "" {
		f, err := os.Create("/tmp/floter-P13-hc-" + name + "-" + appearanceName(dark) + "-" + tag + ".png")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func localFormFocus(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	r := mustFind(t, tt, label)
	tt.ClickAt(r.X+r.W+35, r.Y+r.H/2)
	tt.Frame()
	if !tt.Focused(label) {
		t.Fatalf("%s did not take focus at %+v", label, r)
	}
	tt.Key(ui.Cmd, ui.KeyA)
}

func TestLocalFormCommandSurvivesModeSwitch(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(appearanceName(dark), func(t *testing.T) {
			a := localDraftApp()
			a.toolCommand = "weather"
			tt := localDraftFrame(t, a, dark)
			localFormFocus(t, tt, a.copy().LocalToolCommand)
			a.toolMode = routeScript
			tt.Frame()
			tt.Type("forecast")
			tt.Frame()
			localFormDump(t, tt, "command-mode", dark)
			if a.toolCommand != "forecast" {
				t.Errorf("mode switch lost command edit: %q", a.toolCommand)
			}
		})
	}
}

func TestLocalFormVersionSurvivesModeSwitch(t *testing.T) {
	a := localDraftApp()
	a.toolVersion = "1.2.3"
	tt := localDraftFrame(t, a, true)
	localFormFocus(t, tt, a.copy().LocalToolVersion)
	a.toolMode = routeScript
	tt.Frame()
	tt.Type("2.0.0")
	tt.Frame()
	if a.toolVersion != "2.0.0" {
		t.Errorf("mode switch lost version edit: %q", a.toolVersion)
	}
}
