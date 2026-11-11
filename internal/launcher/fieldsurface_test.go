package launcher

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"floter/internal/apps"
)

// P13-dr visual evidence: the field row and its block drawn at the ruler the app
// ships, in both appearances. Opt-in (`FLOTER_PNG=1`) like every other dump.
//
// The two numbers this round pins live above the list, so the dump is the whole
// frame: the field's text column (54u = 48.6) and the block's floor (49) are
// both readable off it, and the same frame on the stock ruler would read 54 / 55.
func TestDumpP13DRFieldRowPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := testApp()
		recentPage(a, 4)
		tt := renderShipped(t, a, dark)
		path := filepath.Join("/tmp", "floter-P13-dr-field-"+appearance+".png")
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
// P13-dr 重定基线：`floor` 旧值 55 → **49**，取景从 `renderIn`（库存尺）换成
// `renderShipped`（出货尺）。旧值 = `int(14 × 4.0) - 1`，是框架库存主题的乘
// 积；app 画的是 `int(14 × 3.6) - 1`（§6.86 第 1 行：单位算术）。
func TestTheFieldBandIsThePanelsOwnSurface(t *testing.T) {
	forEachAppearance(t, func(t *testing.T, dark bool) {
		a := testApp()
		tt := renderShipped(t, a, dark)
		img := tt.Image()
		band := img.RGBAAt(4, 20)
		panel := img.RGBAAt(4, img.Rect.Dy()-6)
		if band != panel {
			t.Errorf("the field band is %v and the panel is %v: the field still paints a face of its own",
				band, panel)
		}
		// The floor is still drawn, and still exactly one pixel: the field's row
		// less one (`launcher.go`'s `Top(TopInsetUnits + fieldRow - 1)`), in the
		// quiet hairline. P19 重定基线：面板自己的 4u 上内边距此前由外壳的
		// `Padding` 支付（那一圈就是用户报的「四周内边」），现在由面板自己支付，
		// 于是带子的顶与它的底线一起下移 `TopInsetUnits`，带子的高度与
		// 窗口的高度链一个数没动。
		spacing, _, _, _ := designMetrics()
		floor := int(float64(TopInsetUnits+fieldRowUnits)*spacing) - 1
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
	// P11-bk: the empty page is ten recents at most, and ten rows do not
	// overflow this frame — the built-in commands used to pad the list. A
	// typed query keeps the same claim (rows never start above the band) over
	// a list that really does scroll.
	a.SetQuery("app")
	tt := renderShipped(t, a, false)
	spacing, _, _, _ := designMetrics()
	edge := int(float64(fieldRowUnits+breathUnits) * spacing)
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
