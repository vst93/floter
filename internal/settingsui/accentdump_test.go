package settingsui

import (
	"image/png"
	"os"
	"testing"
)

// TestDumpP11ALAccentPNGs is the opt-in screenshot dump for P11-al (F2's root:
// the accent hue). It is skipped unless FLOTER_PNG is set, so it never counts
// as a pass in the ordinary suite — the same contract as the launcher's other
// dumps.
func TestDumpP11ALAccentPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to dump the accent screenshots")
	}
	for _, appearance := range appearanceNames {
		a := New(newStore(t), Actions{})
		tt, tokens := renderIn(t, a, 720, 580, appearance.dark)
		img := tt.Image()
		path := "/tmp/floter-P11-al-accent-" + appearance.name + ".png"
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
		mark := mustFind(t, tt, "General")
		x := int(mark.X) + 10
		y := int(mark.Y+mark.H/2) - 3
		p := imagePixel(img, x, y)
		t.Logf("%s: %s accent=%v chosen-mark-at-(%d,%d)=%v", appearance.name, path, tokens.Theme.Accent, x, y, p)
	}
}
