package launcher

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/settings"
)

// The P11-ab visual evidence: the search row's new leading mark in both
// appearances, so the report can point at a file and at a pixel rather than at
// a description. What the two frames have to show is the magnifier in the rows'
// own icon cell (16u..44u) and the query column moved onto the rows' title
// column (54u).
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps: a test run
// should not litter /tmp.
func TestDumpP11ABSearchRowPNGs(t *testing.T) {
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
		path := filepath.Join("/tmp", "floter-P11-ab-searchrow-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		// The card is translucent (the window shows the desktop through it) and
		// a tester has no desktop behind it: composite onto a neutral one so the
		// muted mark is a readable pixel rather than a premultiplied alpha.
		frame := image.NewRGBA(image.Rect(0, 0, tt.Image().Rect.Dx(), tt.Image().Rect.Dy()))
		backdrop := color.RGBA{R: 0xDF, G: 0xDF, B: 0xE4, A: 0xFF}
		if dark {
			backdrop = color.RGBA{R: 0x1C, G: 0x1C, B: 0x20, A: 0xFF}
		}
		draw.Draw(frame, frame.Rect, &image.Uniform{C: backdrop}, image.Point{}, draw.Src)
		draw.Draw(frame, frame.Rect, tt.Image(), image.Point{}, draw.Over)
		if err := png.Encode(f, frame); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s (%dx%d) dark=%v", path, frame.Rect.Dx(), frame.Rect.Dy(), dark)
	}
}
