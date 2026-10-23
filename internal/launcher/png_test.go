package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// The P8-e visual evidence: the captured-output panel with its **chosen row**
// in the picture — the one plane §1 C re-routed from `AccentTintQuiet` to
// `RowSelected` — in both appearances.
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps: a test run
// should not litter /tmp; the round that produced the report ran it once with
// the switch on.
func TestDumpOutputPanelPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		tt := runListOutput(t, a, outputRowsWithActions(6))
		// `render` hands the tester the framework's theme, which is the one the
		// shell installs on the real window; the panel's fills come from the
		// app's own tokens, so the dump is the app's own appearance either way.
		// The two frames are told apart by the store's theme, printed beside the
		// path for the report.
		tokens := theme.ForPreferences(a.settings(), dark, ui.Preferences{})
		t.Logf("RowSelected=%v (dark=%v)", tokens.RowSelected, dark)
		path := filepath.Join("/tmp", "floter-p8e-output-"+appearance+".png")
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
