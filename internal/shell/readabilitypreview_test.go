package shell

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/settings"
	"github.com/egoist/mygo/ui"
)

// The desktop must be part of the preview: a transparent PNG alone hides
// the very failure this test records. Native blur is deliberately absent.
func TestGlassReadabilityPreview(t *testing.T) {
	dir := os.Getenv("FLOTER_READABILITY_PNG")
	if dir == "" {
		t.Skip("set FLOTER_READABILITY_PNG to render black/white composites")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, appearance := range []string{"light", "dark"} {
		for index, opacity := range []uint8{72, 86, 100} {
			for _, page := range []struct {
				name    string
				surface Surface
			}{{"launcher", SurfaceLauncher}, {"settings", SurfaceSettings}, {"opacity", SurfaceSettings}, {"terminal", SurfaceTerminal}} {
				s := settings.Default()
				s.Theme, s.Language, s.GlassStep = appearance, "zh", "liquid"
				s.MainOpacity, s.TerminalOpacity = opacity, []uint8{66, 83, 100}[index]
				shownOpacity := opacity
				if page.surface == SurfaceTerminal {
					shownOpacity = s.TerminalOpacity
				}
				a := New(Options{Store: settings.NewStore(s)})
				a.Surf = page.surface
				recentLauncher(a, 5)
				tt := ui.NewTester(a.View, 720, 580)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				tt.Frame()
				if page.surface == SurfaceLauncher {
					tt.SetSize(720, a.launcherTarget)
				}
				if page.name == "opacity" {
					a.Settings.Body.Y = a.Settings.Body.MaxY
				}
				tt.Frame()
				for _, backdrop := range []uint8{0, 255} {
					src := tt.Image()
					dst := image.NewRGBA(src.Bounds())
					draw.Draw(dst, dst.Bounds(), image.NewUniform(color.RGBA{backdrop, backdrop, backdrop, 255}), image.Point{}, draw.Src)
					draw.Draw(dst, dst.Bounds(), src, image.Point{}, draw.Over)
					name := fmt.Sprintf("%s-%s-%d-bg%d.png", page.name, appearance, shownOpacity, backdrop)
					f, err := os.Create(filepath.Join(dir, name))
					if err != nil {
						t.Fatal(err)
					}
					encodeErr, closeErr := png.Encode(f, dst), f.Close()
					if encodeErr != nil || closeErr != nil {
						t.Fatalf("write preview: %v / %v", encodeErr, closeErr)
					}
				}
			}
		}
	}
}
