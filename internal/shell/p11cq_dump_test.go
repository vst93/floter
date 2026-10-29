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

// P11-cq's zero-pixel evidence: the round changes only *where* the settings
// value is read from, never what it holds, so a frame drawn before and after
// must be identical byte for byte. The Sessions page at sixteen sessions — the
// ceiling the page enforces — is the frame that used to take 32 settings
// copies, so it is the one dumped; the launcher rides along because its
// appearance resolve is the other read that moved. Opt-in, like the other
// dumps: a test run should not litter /tmp.
func TestDumpP11CQSessionFrames(t *testing.T) {
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
						// A real session always has a non-zero ID (`nextSessionID`
						// hands one out at creation); zero means "a session built
						// by hand" and is also the list's "nothing armed" value,
						// so a fixture that left it zero would draw every row's
						// close control in its armed face.
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
			path := filepath.Join("/tmp", "floter-P11-cq-"+surface.name+"-"+appearance.name+"-"+tag+".png")
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
