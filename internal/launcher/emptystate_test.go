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
	tt := renderShipped(t, a, false)

	rect, ok := tt.Find("No results")
	if !ok {
		t.Fatalf("the empty message is missing: %q", tt.Texts())
	}
	if ink := inkIn(tt.Image(), rect); ink == 0 {
		t.Fatalf("the empty page's sentence carries no ink at %+v: it is painted "+
			"under the field block, so the window reads as blank", rect)
	}
	// The band the empty state belongs to is the list's: top = the field row
	// plus its breath (one Space unit is four reference units, and one reference
	// unit is `designUnit()` = 0.9 at the shipped step), bottom = the panel's own
	// 3u inset.
	//
	// P13-dt 重定基线：带顶 60 → **54**、帧高 488 → **439**，且中线原来带 ±6 的容差。
	// 实测中线 241.10 与带子的几何中线 241.10 是同一个数，所以容差换成了
	// 十分之一像素的整数相等。
	bandTop := float32(fieldRowUnits+breathUnits) * 4 * float32(designUnit())
	height := float32(themedFrameHeight(maxResultSlots))
	// P13-fd 重定基线：居中的带底从「卡底 − `Space(3)`（3u = 10.8）」改成「卡底 −
	// 面板自己的 tail（`TailUnits` = 2u = 1.8）」，与列表的下内边距同一个数（§6.125）。
	want := (float64(bandTop) + float64(height) - TailUnits*4*designUnit()) / 2
	if got := tenths(float64(rect.Y + rect.H/2)); got != tenths(want) {
		t.Errorf("the sentence is centred at y=%.2f (%.1f), want %.2f (%.1f) "+
			"(the band under the field, %.2f..%.2f)",
			rect.Y+rect.H/2, float64(got)/10, want, float64(tenths(want))/10,
			bandTop, float64(height)-3*4*designUnit())
	}
	if got, top := tenths(float64(rect.Y)), tenths(float64(bandTop)); got < top {
		t.Errorf("the sentence starts at y=%.2f (%.1f), inside the field band that ends at %.2f (%.1f)",
			rect.Y, float64(got)/10, bandTop, float64(top)/10)
	}
}

// P11-ag visual evidence: the empty page in both appearances, written to /tmp
// so the report can point at a file. Opt-in (`FLOTER_PNG=1`) for the same
// reason as the other dumps: a test run should not litter /tmp.
//
// P13-dt：取景从 `render`（库存尺帧 488）换到 `renderShipped`（出货尺帧 439）——
// pin 量的那个中线属于这张帧，图也必须是这张帧。
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
		tt := renderShipped(t, a, dark)
		path := filepath.Join("/tmp", "floter-P13-dt-empty-"+appearance+".png")
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
