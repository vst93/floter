package shell

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-g's visual evidence: the round moves the toast onto the same throw the
// other four floating layers already use (`theme.ElevThrow`), and gives the
// launcher's 20u strip the contact layer and the lit rim it never had. The
// frames that must be looked at are therefore the two placements — a
// full-height surface (settings, top right, `--elev-3`) and the collapsed
// launcher (bottom left, `--elev-3-compact`) — in both appearances.
//
// Opt-in, like the other dumps: a test run should not litter /tmp. The `before`
// pair is the same two frames rendered from the commit before this round
// (`git stash` + `FLOTER_PNG_TAG=before`), so the pair is a true before/after
// at the same size.
func TestDumpP13GToastFrames(t *testing.T) {
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
			open func(*App)
		}{
			// Full height: the toast hangs from the top right under the header.
			{"settings", func(a *App) { a.Open(SurfaceSettings) }},
			// Collapsed: one toast pinned to the bottom-left 20u band.
			{"launcher", func(a *App) { a.Surf = SurfaceLauncher }},
		} {
			a := New(Options{Store: settings.NewStore(settings.Default())})
			theme := appearance.name
			if err := a.Store.Update(func(s *settings.Settings) { s.Theme = theme }); err != nil {
				t.Fatal(err)
			}
			now := time.Unix(0, 0)
			a.toastClock = func() time.Time { return now }
			surface.open(a)
			tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
			tt.SetDark(appearance.dark)
			tt.Frame()
			a.warnNotice("Where am I")
			tt.Frame()
			// Settle the entrance before the picture: the throw is what this
			// round changed, not the travel.
			now = now.Add(toastEnter)
			tt.Frame()
			path := filepath.Join("/tmp",
				fmt.Sprintf("floter-P13-g-%s-%s-%s.png", surface.name, appearance.name, tag))
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s %s: wrote %s", surface.name, appearance.name, path)
		}
	}
}
