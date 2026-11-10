package shell

import (
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

func TestSolidOpacityDisablesAndRestoresMaterialOnSurfaceChanges(t *testing.T) {
	s := settings.Default()
	s.MainOpacity, s.TerminalOpacity, s.GlassStep = 100, 66, "liquid"
	a := New(Options{Store: settings.NewStore(s)})
	for _, surface := range []Surface{SurfaceSettings, SurfaceTerminal, SurfaceLauncher, SurfaceTerminal} {
		a.Open(surface)
		want := "off"
		if surface == SurfaceTerminal {
			want = "liquid"
		}
		if a.lastGlassStep != want {
			t.Fatalf("surface %v material %q, want %q", surface, a.lastGlassStep, want)
		}
	}
	for _, opacity := range []uint8{100, 83, 100, 66} {
		if err := a.Store.Update(func(s *settings.Settings) { s.TerminalOpacity = opacity }); err != nil {
			t.Fatal(err)
		}
		want := "liquid"
		if opacity == 100 {
			want = "off"
		}
		if a.lastGlassStep != want {
			t.Fatalf("terminal %d%% kept material %q", opacity, a.lastGlassStep)
		}
	}
}

func TestSolidOpacityPaintsOpaqueInterior(t *testing.T) {
	for _, appearance := range []string{"light", "dark"} {
		for _, surface := range []Surface{SurfaceLauncher, SurfaceSettings, SurfaceTerminal} {
			s := settings.Default()
			s.Theme, s.MainOpacity, s.TerminalOpacity = appearance, 100, 100
			a := New(Options{Store: settings.NewStore(s)})
			a.Surf = surface
			recentLauncher(a, 5)
			tt := ui.NewTester(a.View, 720, 580)
			tt.Frame()
			if surface == SurfaceLauncher {
				tt.SetSize(720, a.launcherTarget)
				tt.Frame()
			}
			img := tt.Image()
			// Leave the rounded outline and window shadow out of the interior.
			for y := 20; y < img.Rect.Dy()-20; y++ {
				for x := 20; x < img.Rect.Dx()-20; x++ {
					if alpha := img.Pix[y*img.Stride+x*4+3]; alpha != 255 {
						t.Fatalf("%s/%v pixel %d,%d alpha %d at 100%%", appearance, surface, x, y, alpha)
					}
				}
			}
		}
	}
}
