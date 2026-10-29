package launcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
)

// P11-bj · The field band is the panel's own surface, and nothing scrolls under
// it.
//
// The field row used to paint its own near-solid face (`tk.FieldFill`) over the
// panel's `tk.Sunken`, with a hairline under it: two planes and a line, which the
// user read as a card floating on the panel (「像一块浮在上面的卡片」「不连续边框」).
// The row paints nothing now, so the band's pixels *are* the panel's pixels; the
// one thing that still marks the boundary is the 1px floor the contract asks for.
//
// The band is also reserved in the flow (a spacer above the scroller), because a
// field with no face of its own is no longer an opaque mask: a row that scrolled
// under it would be read through the query. The second assertion is that fact —
// after a scroll the first drawn row still starts at the band's lower edge.
//
// Mutation checks: put the `Background(tk.FieldFill)` back and the first
// assertion reads the old face (浅 250 vs the panel's 253); drop the spacer and
// the second one finds a row above the edge.
func TestTheFieldBandIsThePanelsOwnSurface(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		a := testApp()
		tt := renderIn(t, a, dark)
		img := tt.Image()
		band := img.RGBAAt(4, 20)
		panel := img.RGBAAt(4, img.Rect.Dy()-6)
		if band != panel {
			t.Errorf("the field band is %v and the panel is %v: the field still paints a face of its own",
				band, panel)
		}
		// The floor is still drawn, and still exactly one pixel: the field's row
		// less one (`launcher.go`'s `Top(fieldRow - 1)`), in the quiet hairline.
		spacing := float64(ui.LightTheme().Spacing)
		floor := int(float64(fieldRowUnits)*spacing) - 1
		above, line, below := img.RGBAAt(4, floor-1), img.RGBAAt(4, floor), img.RGBAAt(4, floor+1)
		if line == above || line == below {
			t.Errorf("the field block's floor at y=%d is %v, the same as its neighbours (%v / %v): "+
				"the block's one pixel is the only boundary it has", floor, line, above, below)
		}
	})
}

func TestNoRowScrollsUnderTheFieldBand(t *testing.T) {
	a := testApp()
	var found []apps.App
	var paths []string
	for i := 0; i < 30; i++ {
		app := apps.App{Name: fmt.Sprintf("app %02d", i), Path: fmt.Sprintf("/x/app%02d", i)}
		found = append(found, app)
		paths = append(paths, app.Path)
	}
	a.SetApps(found)
	a.SetRecent(paths, true)
	tt := renderIn(t, a, false)
	edge := int(float64(fieldRowUnits+breathUnits) * float64(ui.LightTheme().Spacing))
	x, y := float32(tt.Image().Rect.Dx()/2), float32(tt.Image().Rect.Dy()/2)
	for i := 0; i < 2; i++ {
		tt.Wheel(x, y, 0, 120)
		tt.Advance(200 * time.Millisecond)
		tt.Frame()
	}
	if first, _ := a.List.Visible(); first == 0 {
		t.Fatalf("the list did not scroll, so nothing could be read under the band")
	}
	for _, item := range a.Results() {
		rect, ok := tt.Find(item.Title)
		if !ok {
			continue
		}
		if rect.Y < float32(edge) {
			t.Errorf("the row %q is drawn at y=%.0f, above the field band's edge (%d): "+
				"the scroller starts under the band", item.Title, rect.Y, edge)
		}
	}
}
