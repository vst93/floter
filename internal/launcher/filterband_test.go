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

func TestTheChipsBandLeavesTheRowsBeneathItToThePointer(t *testing.T) {
	a, tt, top, bottom := chipBandScene(t, false)

	// A row really is painted through the band: the first row title that falls
	// inside it. Without this the assertions below would hold on an empty band.
	painted := false
	first, last := a.List.Visible()
	for _, item := range a.Results()[first:min(last+1, len(a.Results()))] {
		if rect, ok := tt.Find(item.Title); ok && rect.Y < bottom {
			painted = true
		}
	}
	if !painted {
		t.Fatalf("no row is drawn in the chips band (%.1f..%.1f), so there is nothing to test", top, bottom)
	}

	// 1. The band finds the row under it. x=340 is between the chips and the
	// hint; 660 is over the hint's own words, whose box is the row's whole
	// trailing span.
	for _, px := range []float32{340, 660} {
		for py := top + 1; py < bottom; py += 3 {
			a.hoveredRow = -1
			tt.Move(px, py)
			tt.Advance(10 * time.Millisecond)
			tt.Frame()
			if a.hoveredRow < 0 {
				t.Errorf("the pointer over the chips band at (%.0f, %.0f) finds no row: "+
					"the band is taking the pointer from the rows it covers", px, py)
			}
		}
	}

	// 2. The chips are still the band's controls: passing the pointer through
	// the band must not pass it through them either.
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

// TestChipBandHoverDump writes the frame the pin reads: the pointer inside the
// chips band, the row under it carrying its chosen pill. Before the fix the
// same frame has no pill there — the band took the pointer.
func TestChipBandHoverDump(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P11-ap-chipband-<appearance>.png")
	}
	forEachAppearance(t, func(t *testing.T, dark bool) {
		a, tt, top, _ := chipBandScene(t, dark)
		// The frame has to differ, not just be hovered: the wheel leaves the choice
		// on the row under the pointer, so both builds would paint the same pill.
		// Park the pointer on a row *below* the band first (the pointer enters it,
		// so the choice is there), then move it into the band: the fix moves the
		// pill into the band (the pointer enters the row it covers), the old build
		// leaves it below. Move the pointer, never the wheel, so the list's offset
		// is the same in both builds.
		tt.Move(340, 200)
		tt.Advance(200 * time.Millisecond)
		tt.Frame()
		below := a.hoveredRow
		tt.Move(340, top+6)
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
		if a.hoveredRow < 0 {
			t.Fatalf("no row under the band to photograph")
		}
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
		t.Logf("wrote %s (row below the band %d, row in the band %d)", path, below, a.hoveredRow)
	})
}
