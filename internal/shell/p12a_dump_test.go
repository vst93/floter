package shell

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/settingsui"
	"floter/internal/terminalui"
)

// P12-a's visual evidence: the round changes eight token values, so the frames
// it touches are the two surfaces whose chrome is *made* of those tokens —
// the launcher card (which owns the sheen, the cast and the control lens) and
// the settings Sessions page (which owns grouped cards and fields). Opt-in, like
// the other dumps: a test run should not litter /tmp.
//
// The frames are byte-identical to P11-cq's dump at the same size, so the pair
// is a true before/after: copy P11-cq's `-after` files in as this round's
// `-before` and diff them.
func TestDumpP12AElevationFrames(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"dark", true}, {"light", false}} {
		for _, surface := range []struct {
			name string
			surf Surface
			page int
			at   func(a *App)
		}{
			{"sessions", SurfaceSettings, settingsui.PageSessions, func(a *App) {
				for i := 0; i < 16; i++ {
					a.Terminal.Sessions = append(a.Terminal.Sessions, &terminalui.Session{
						ID:      uint64(i + 1),
						Title:   fmt.Sprintf("session %d", i+1),
						Term:    &terminal.Terminal{},
						Created: time.Now().Add(-time.Duration(i) * time.Minute),
					})
				}
				a.Terminal.Active = 0
			}},
			{"launcher", SurfaceLauncher, settingsui.PageGeneral, func(a *App) {}},
		} {
			a := New(Options{Store: settings.NewStore(settings.Default())})
			surface.at(a)
			a.Surf, a.Settings.Page = surface.surf, surface.page
			tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
			tt.SetDark(appearance.dark)
			tt.Frame()
			tt.Frame()
			path := filepath.Join("/tmp", "floter-P12-a-"+surface.name+"-"+appearance.name+"-"+tag+".png")
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			img := tt.Image()
			t.Logf("%s (%dx%d)", path, img.Rect.Dx(), img.Rect.Dy())
		}
	}
}
