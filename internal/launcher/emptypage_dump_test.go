package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/settings"
)

// The P11-bk visual evidence: the empty search page before and after it stopped
// leading with the six built-in commands, in both appearances. Opt-in for the
// same reason as the other dumps (`FLOTER_PNG=1`).
func TestDumpTheEmptyPageShowsOnlyTheRecents(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		recentPage(a, 6)
		tt := renderIn(t, a, dark)
		tt.Frame()
		img := tt.Image()
		path := filepath.Join("/tmp", "floter-P11-bk-emptypage-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s rows: %v", appearance, tt.Texts())
		t.Logf("%s (%dx%d) %s", appearance, img.Rect.Dx(), img.Rect.Dy(), path)
	}
}

// The same page with a word typed: the built-in commands are answers now, and
// this frame is what the report's "still reachable" half shows.
func TestDumpTheTypedPageFindsTheBuiltInCommand(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	a := testApp()
	recentPage(a, 6)
	a.SetQuery("restart")
	tt := renderIn(t, a, false)
	tt.Frame()
	img := tt.Image()
	path := filepath.Join("/tmp", "floter-P11-bk-typed-restart-light.png")
	f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	t.Logf("typed rows: %v", tt.Texts())
}
