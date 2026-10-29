package launcher

import (
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"floter/internal/settings"
)

// The P11-bj visual evidence: the search page's field band against the panel it
// sits in, in both appearances. Opt-in for the same reason as the other dumps.
func TestDumpTheSearchPagesFieldBand(t *testing.T) {
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
		tt := renderIn(t, a, dark)
		img := tt.Image()
		path := filepath.Join("/tmp", "floter-P11-bj-search-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		// The band the field paints and the band the list paints, sampled in
		// one column well clear of any glyph.
		for _, y := range []int{0, 1, 2, 3, 4, 6, 8, 54, 55, 56, 57} {
			row := ""
			for _, px := range []int{0, 1, 2, 3, 4, 6, 8, 12, 700, 716, 718, 719} {
				c := img.RGBAAt(px, y)
				row += " " + strconv.Itoa(px) + ":" + strconv.Itoa(int(c.R))
			}
			t.Logf("%s y=%d %s", appearance, y, row)
		}
		t.Logf("%s (%dx%d) %s", appearance, img.Rect.Dx(), img.Rect.Dy(), path)
	}
}
