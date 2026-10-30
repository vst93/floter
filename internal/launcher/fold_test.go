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

// darkestIn is the darkest pixel inside a text rectangle: the ink's own
// contrast against the panel face. A row drawn at reduced opacity keeps its
// shape but loses its depth, so its darkest pixel climbs towards the face —
// which is the one number that says "this row has receded".
func darkestIn(img *image.RGBA, rect ui.Rect) int {
	best := -1
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a>>8 == 0 {
				continue
			}
			l := (int(r>>8) + int(g>>8) + int(b>>8)) / 3
			if best < 0 || l < best {
				best = l
			}
		}
	}
	return best
}

// P13-v · 折边之外的那一行**退后**，不是被切。
//
// 列表没有滚动条（用户定的），折边于是是唯一说「下面还有」的地方。它今天
// 的做法是把行硬切：视口下沿落在行的中间，一行字被水平切成两半——实测
// `Walk 09` 的文字盒 24 → 15 px，墨在下沿一刀断掉（`docs/rounds/P13-v-report.md`
// 有前后像素值）。这不是「还有更多」，这是「界面坏了」。
//
// 这条 pin 读两件事：同一行在折边上时墨**更淡**，把它滚进视口整行可见之后
// 墨**更深**。差值就是「退后」这件事本身，与渲染器算出多少像素无关。
func TestTheRowAtTheFoldRecedes(t *testing.T) {
	a := testApp()
	a.SetApps(walkApps(20))
	tt := render(t, a)
	tt.Type("walk")
	tt.Frame()

	folded, ok := tt.Find("Walk 09")
	if !ok {
		t.Fatalf("the tenth row is missing: %q", tt.Texts())
	}
	foldedInk := darkestIn(tt.Image(), folded)

	// Nine steps put the choice on Walk 09, and the list scrolls the choice
	// whole into view (`ScrollIntoView`), which is the same row with its
	// whole 40u box inside the viewport.
	for i := 0; i < 9; i++ {
		tt.Key(0, ui.KeyDown)
	}
	tt.Frame()
	whole, ok := tt.Find("Walk 09")
	if !ok {
		t.Fatalf("the tenth row left the list after the walk")
	}
	wholeInk := darkestIn(tt.Image(), whole)

	if whole.H <= folded.H {
		t.Fatalf("the row's box did not grow when it came whole into view: "+
			"folded %+v, whole %+v", folded, whole)
	}
	t.Logf("folded rect=%+v ink=%d | whole rect=%+v ink=%d", folded, foldedInk, whole, wholeInk)
	if foldedInk <= wholeInk {
		t.Errorf("the row at the fold keeps its full ink: darkest at the fold "+
			"%d, whole in view %d — the fold cuts the row instead of letting "+
			"it recede", foldedInk, wholeInk)
	}
}

// The other half of the same rule: the fold is a **fold**, not a global dim.
// A row wholly inside the viewport keeps the ink it always had.
func TestTheRowsInsideTheViewportKeepTheirInk(t *testing.T) {
	a := testApp()
	a.SetApps(walkApps(20))
	tt := render(t, a)
	tt.Type("walk")
	tt.Frame()

	first, ok := tt.Find("Walk 00")
	if !ok {
		t.Fatalf("the first row is missing: %q", tt.Texts())
	}
	ink := darkestIn(tt.Image(), first)
	if ink < 0 {
		t.Fatalf("the first row carries no ink at %+v", first)
	}
	t.Logf("first row rect=%+v ink=%d", first, ink)
	if ink > 100 {
		t.Errorf("a row wholly inside the viewport has faded: darkest %d, "+
			"want the resting ink (the fold only dims what it cuts)", ink)
	}
}

// And the fold is a *scale*, not a flag: the less of a row the viewport shows,
// the further it has recedes. Two frames, same row, different amounts of it
// visible.
func TestTheLessOfARowTheViewportShowsTheFurtherItRecedes(t *testing.T) {
	inkAt := func(height int) int {
		a := testApp()
		a.SetApps(walkApps(20))
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, InputWindowWidth, height)
		tt.Frame()
		tt.Type("walk")
		tt.Frame()
		rect, ok := tt.Find("Walk 09")
		if !ok {
			t.Fatalf("the tenth row is missing at height %d", height)
		}
		return darkestIn(tt.Image(), rect)
	}
	// The standard frame shows 23 of the row's 40u — the top fifteen pixels of
	// its label; eight pixels shorter shows seven, which is the top of the
	// same glyphs and nothing else.
	most := inkAt(frameHeight(maxResultSlots))
	least := inkAt(frameHeight(maxResultSlots) - 8)
	if least < 0 || most < 0 {
		t.Fatalf("one of the two frames has no ink in the row at all: %d, %d", least, most)
	}
	if least <= most {
		t.Errorf("a row with fifteen pixels showing keeps more ink than one with "+
			"twenty-three: %d vs %d — the fold is a flag, not a scale", least, most)
	}
}

// foldAlpha's own table: whole in view is 1, a cut row is the visible
// fraction, a row scrolled away is 0, and a first frame with no viewport yet
// fades nothing.
func TestFoldAlpha(t *testing.T) {
	viewport := ui.Rect{X: 0, Y: 60, W: 720, H: 383}
	for _, tc := range []struct {
		name string
		box  ui.Rect
		want float64
	}{
		{"wholly inside", ui.Rect{X: 8, Y: 100, W: 704, H: 40}, 1},
		{"cut at the bottom", ui.Rect{X: 8, Y: 420, W: 704, H: 40}, 23.0 / 40},
		{"cut at the top", ui.Rect{X: 8, Y: 40, W: 704, H: 40}, 20.0 / 40},
		{"scrolled away", ui.Rect{X: 8, Y: 460, W: 704, H: 40}, 0},
		{"no viewport yet", ui.Rect{X: 8, Y: 420, W: 704, H: 40}, 1},
	} {
		vp := viewport
		if tc.name == "no viewport yet" {
			vp = ui.Rect{}
		}
		if got := foldAlpha(vp, tc.box); got != tc.want {
			t.Errorf("%s: foldAlpha = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// P13-v visual evidence: the folded page in both appearances. Opt-in
// (`FLOTER_PNG=1`) for the same reason as the other dumps.
func TestDumpP13VFoldPNGs(t *testing.T) {
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
		a.SetApps(walkApps(20))
		tt := render(t, a)
		tt.SetDark(dark)
		tt.Type("walk")
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P13-v-fold-"+appearance+".png")
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
