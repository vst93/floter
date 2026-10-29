package launcher

import (
	"fmt"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	clipboardpkg "floter/internal/clipboard"
)

// P11-ap · The chips band is transparent to the pointer.
//
// The chips row floats over the result list (the list's top padding scrolls
// away with its content, so rows pass under the field and under the chips).
// Its 20u band is painted with nothing — the rows underneath stay visible — so
// it must not take the pointer: without `PassThrough` the whole band, from the
// chips to the window's right edge, was a dead strip. A row's 40u box is 46u
// apart, so the strip ate half of every row that scrolled under it: the row
// could be read but not hovered, not chosen and not clicked.
//
// The reference never has this shape — `.launcher-filter` is a sibling of
// `.launcher-results` in normal flow (`main:src/App.tsx:3407`), so no row ever
// passes under it. The port's floating field is deliberate, and the same rule
// the terminal's title bar follows (P11-af) is the one that applies: a band
// with no fill of its own leaves the pointer to what it covers.
//
// chipBandScene builds that situation: clipboard mode, forty rows, two wheel
// notches down, the pointer parked inside the band between the chips and the
// hint. It returns the band's own top and bottom too.
func chipBandScene(t *testing.T, dark bool) (*App, *ui.Tester, float32, float32) {
	t.Helper()
	store := clipboardpkg.NewStore(clipboardpkg.FromConfigRoot(t.TempDir()), 500)
	for i := 0; i < 40; i++ {
		if _, _, err := store.AddText(fmt.Sprintf("clip %03d", i)); err != nil {
			t.Fatal(err)
		}
	}
	a := testApp()
	a.Clipboard = store
	a.EnterClipboard()
	tt := renderIn(t, a, dark)
	bounds := tt.Image().Bounds()
	x, y := float32(bounds.Max.X/2), float32(bounds.Max.Y*3/4)
	tt.Move(x, y)
	tt.Frame()
	for i := 0; i < 2; i++ {
		tt.Wheel(x, y, 0, 120)
		tt.Advance(200 * time.Millisecond)
		tt.Frame()
	}
	if first, _ := a.List.Visible(); first == 0 {
		t.Fatalf("the list did not scroll, so no row is under the chips band")
	}
	// The band's own band: the field row plus the breath above it, then the
	// chips row's height — the numbers the launcher charges for the mode's
	// chrome (`height.go` fieldRowUnits/breathUnits, `filtersHeight` = Space(5)).
	spacing := float32(ui.LightTheme().Spacing)
	top := (float32(fieldRowUnits) + float32(breathUnits)) * spacing
	return a, tt, top, top + 5*spacing
}

// P11-bj · The chips band is *above* the scroller, so no row is ever under it.
//
// P11-ap's pin (kept below, second half) was about the band's pointer: the chips
// row floated over the result list and rows passed under it, so the band had to
// let the pointer through. P11-bj removed the premise — the field's band (and a
// mode's chips) is reserved **in the flow** now, so the scroller's clip starts at
// its lower edge and no row is drawn inside the band at all. The pointer
// question answers itself; what is worth pinning is the stronger fact.
//
// The reference never had rows under its filter row either (`.launcher-filter`
// is a sibling of `.launcher-results` in normal flow, `main:src/App.tsx:3407`),
// so this is the port coming back to the reference's shape.
func TestTheChipsBandSitsAboveTheScroller(t *testing.T) {
	a, tt, top, bottom := chipBandScene(t, false)

	// No row of the page is drawn inside the band (they used to be: the list's
	// padding scrolled away with its content). The rows the list *holds* are
	// below its own top edge, and the scroller's box is below the band.
	for _, item := range a.Results() {
		if rect, ok := tt.Find(item.Title); ok && rect.Y < bottom {
			t.Errorf("the row %q is drawn at y=%.0f, inside the chips band (%.0f..%.0f): "+
				"the band is no longer reserved in the flow", item.Title, rect.Y, top, bottom)
		}
	}

	// 2. The chips are still the band's controls: they are the band's own
	// children and stay clickable.
	before := a.clipboardFilter
	if rect, ok := tt.Find("Images"); ok {
		tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
		tt.Frame()
	} else {
		t.Fatalf("the chips row has no Images chip: %v", tt.Texts())
	}
	if a.clipboardFilter == before {
		t.Errorf("clicking the Images chip did not switch the filter (still %v)", before)
	}
}

// TestChipBandHoverDump writes the frame the pin reads: the chips band with no
// row inside it (P11-bj put the band above the scroller).
func TestChipBandHoverDump(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P11-ap-chipband-<appearance>.png")
	}
	forEachAppearance(t, func(t *testing.T, dark bool) {
		a, tt, top, bottom := chipBandScene(t, dark)
		// The pointer is parked below the band first and then inside it, so the
		// picture shows the band with the rows stopped at its lower edge.
		tt.Move(340, 200)
		tt.Advance(200 * time.Millisecond)
		tt.Frame()
		tt.Move(340, top+6)
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		// P11-bj：带子已经在滚动容器之上，带子里没有行可指 —— 这里只留一张图，
		// 记录的正是「带子里不放行」这件事（带子上沿 %.0f，下沿 %.0f）。
		t.Logf("chips band %.0f..%.0f, hovered row %d", top, bottom, a.hoveredRow)
		name := "light"
		if dark {
			name = "dark"
		}
		path := fmt.Sprintf("/tmp/floter-P11-ap-chipband-%s.png", name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s (row in the band %d)", path, a.hoveredRow)
	})
}
