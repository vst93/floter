package launcher

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// inkIn counts the pixels of a text rectangle that are darker than the panel's
// own face. A sentence drawn **under** an opaque block contributes none: the
// pixels inside its box are that block's face, not its glyphs.
func inkIn(img *image.RGBA, rect ui.Rect) int {
	n := 0
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a>>8 == 0 {
				continue
			}
			if r>>8 < 200 && g>>8 < 200 && b>>8 < 200 {
				n++
			}
		}
	}
	return n
}

// P11-ag · The empty page's sentence has to be *on screen*.
//
// The panel is the whole card's height and the field row floats over its top
// (`launcher.fieldRow`, 56u, opaque). The empty state used to be a bare
// `Column` with no band of its own, so `Center()` put the sentence on the
// card's top edge (y=12–30 in this frame) — underneath the field block. Every
// text-tree pin (`TestNoResultsShowsTheEmptyMessage`, `TestLauncherShowsTheEmptyState`)
// asked `HasText`, which reads the built tree rather than the raster, so a
// page that rendered **no** sentence still passed.
//
// The pin therefore reads two things: ink inside the sentence's own box (a
// hidden sentence has none), and the box's place — centred in the band the
// list would occupy, i.e. below the field row plus its breath, above the
// panel's bottom.
func TestTheEmptyPageSaysSoUnderTheField(t *testing.T) {
	a := testApp()
	a.Query = "zzzz"
	tt := render(t, a)

	rect, ok := tt.Find("No results")
	if !ok {
		t.Fatalf("the empty message is missing: %q", tt.Texts())
	}
	if ink := inkIn(tt.Image(), rect); ink == 0 {
		t.Fatalf("the empty page's sentence carries no ink at %+v: it is painted "+
			"under the field block, so the window reads as blank", rect)
	}
	// The band the empty state belongs to is the list's: top = the field row
	// plus its breath (one Space unit is four reference units, see frameHeight),
	// bottom = the panel's own 3u inset.
	bandTop := float32(fieldRowUnits+breathUnits) * 4
	height := float32(frameHeight(maxResultSlots))
	want := (bandTop + (height - 3*4)) / 2
	if got := rect.Y + rect.H/2; got < want-6 || got > want+6 {
		t.Errorf("the sentence is centred at y=%v, want %v (the band under the field)", got, want)
	}
	if rect.Y < bandTop {
		t.Errorf("the sentence starts at y=%v, inside the field band that ends at %v", rect.Y, bandTop)
	}
}

// P11-ag visual evidence: the empty page in both appearances, written to /tmp
// so the report can point at a file. Opt-in (`FLOTER_PNG=1`) for the same
// reason as the other dumps: a test run should not litter /tmp.
func TestDumpP11AGEmptyPagePNGs(t *testing.T) {
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
		a.Query = "zzzz"
		tt := render(t, a)
		tt.SetDark(dark)
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P11-ag-empty-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s", path)
	}
}
