package shell

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P12-b's visual evidence: the round gives the launcher card's E1 blocks a lit
// face, so the frames that must be looked at are the ones that *have* those
// blocks — a result list with a chosen row and a keycap chord, and the action
// bar's pinned footer with the same chosen block. Both appearances, because the
// light one is where the lit edge is 白压白 and only the counter-side shade
// makes the block stand.
//
// Opt-in, like the other dumps: a test run should not litter /tmp. The
// `before` pair is the same two frames rendered from the commit before this
// round (`git stash` + `FLOTER_PNG_TAG=before`), so the pair is a true
// before/after at the same size.
func TestDumpP12BLitEdgeFrames(t *testing.T) {
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
			// note is the sentence the band carries, "" for the list frames.
			note    string
			warning bool
			results int
		}{
			// The list: the chosen row (E1) plus the trailing keycap chord.
			{"list", "", false, 5},
			// The feedback band: the note and the warning variant, which used
			// to be the only one wearing the lit edge.
			{"note", "Installed Example", false, 3},
			{"warning", "Could not install Example", true, 3},
		} {
			a := New(Options{Store: settings.NewStore(settings.Default())})
			theme := appearance.name
			if err := a.Store.Update(func(s *settings.Settings) { s.Theme = theme }); err != nil {
				t.Fatal(err)
			}
			recentLauncher(a, surface.results)
			if surface.note != "" {
				if surface.warning {
					a.Launcher.WarnFeedback(surface.note)
				} else {
					a.Launcher.Feedback(surface.note)
				}
			}
			tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
			tt.SetDark(appearance.dark)
			tt.Frame()
			tt.Advance(300e6)
			tt.Frame()
			path := filepath.Join("/tmp", fmt.Sprintf("floter-P12-b-%s-%s-%s.png", surface.name, appearance.name, tag))
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
