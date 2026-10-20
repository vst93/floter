package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A window wider than the 720 contract still paints no band beside the card.
//
// Every earlier check of 「搜索页面左右两边各有一大块空白」 rendered the shell
// at the contract's own width, where "the card fills the window" and "the card
// is capped at 720 and centred" are the same picture. The user's window is not
// necessarily 720 wide (a compositor that sizes the toplevel itself, a DPI
// whose client does not divide back to the requested DIP width), and the two
// answers part company exactly there: the cap leaves two blocks of desktop
// inside the window, the fill leaves none. This renders the shipped frame at a
// wider client and reads the painted card's own edges.
func TestTheCardFillsAWindowWiderThanTheContract(t *testing.T) {
	a := appKeysApp()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 1000, 420)
	tt.Frame()

	img := tt.Image()
	b := img.Bounds()
	y := b.Dy() / 2
	// The panel's own fill, read in the middle of the card, and the run of it
	// across the line: the card's margin carries its shadow and the window's
	// transparent root, so "painted at all" would answer the shadow rather
	// than the card. The face's own colour is what spans the card.
	face := img.RGBAAt(b.Dx()/2, y)
	first, last := -1, -1
	for x := b.Min.X; x < b.Max.X; x++ {
		if img.RGBAAt(x, y) == face {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	if first < 0 {
		t.Fatalf("the card's face was not painted across the window's middle line")
	}
	_, rightUnits, _, leftUnits := a.inset()
	spacing := float32(a.Launcher.Spacing)
	wantLeft := iround(leftUnits * spacing)
	wantRight := b.Max.X - 1 - iround(rightUnits*spacing)
	if absi(first-wantLeft) > 2 {
		t.Errorf("the card starts at x=%d, want the platform inset %d (the window is %d wide)",
			first, wantLeft, b.Dx())
	}
	if absi(last-wantRight) > 2 {
		t.Errorf("the card ends at x=%d, want the platform inset %d (the window is %d wide)",
			last, wantRight, b.Dx())
	}
}

// The action bar's own selection face, on the shipped frame.
//
// The user's report names the face as missing: 「应该有选中样式（当前缺失）」.
// The bar is the panel's pinned footer and the one row Enter defaults to for a
// query the field itself answers (a page address, a path, a command line with
// no match), so its selection has to be *painted* — the neutral raised pane the
// reference gave `.launcher-action-bar--selected`
// (`main:src/styles/launcher.css:1579-1583`) — and not merely be the index the
// keyboard would run. This reads the row's own band off the frame: a page
// address puts the bar in the footer with the selection on it, and the band
// differs from the panel's fill above it.
func TestTheActionBarWearsItsSelectionFace(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Type("https://example.com")
	tt.Frame()

	results := a.Launcher.Results()
	bar := -1
	for i, item := range results {
		if item.ID == "bar:url" {
			bar = i
		}
	}
	if bar < 0 {
		t.Fatalf("the page address drew no action bar: %+v", results)
	}
	if a.Launcher.Selected != bar {
		t.Fatalf("the bar is row %d but the selection is on %d", bar, a.Launcher.Selected)
	}
	// The bar's own title is the value the row hands over, so its drawn line
	// is the row's line.
	row, ok := tt.Find("https://example.com")
	if !ok {
		t.Fatalf("the bar's row was not drawn: %q", tt.Texts())
	}
	img := tt.Image()
	y := int(row.Y + row.H/2)
	cardLeft, cardRight := cardEdges(a)
	panel := img.RGBAAt(int(cardLeft)+1, y)
	tinted := 0
	for x := int(cardLeft) + 1; x < int(cardRight); x++ {
		if img.RGBAAt(x, y) != panel {
			tinted++
		}
	}
	if tinted < 60 {
		t.Errorf("the selected bar's band is not painted: %d differing pixels across its line", tinted)
	}
}
