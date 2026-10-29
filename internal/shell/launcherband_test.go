package shell

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The two named items, measured on the **shipped** frame rather than on the
// launcher drawn alone.
//
// Both reports have been answered in code for several rounds, and every guard
// written for them rendered `launcher.App` on its own, at a fixed height. The
// frame the user actually sees is the shell's: the launcher is a child of the
// card, the card sits inside the platform's inset, and the panel's height is
// the one the shell charged. A footer that is "the list's last row" inside the
// launcher can still be painted at the top of what the user sees if the band
// accounting disagrees with the panel — so the assertions below read the
// composed frame's own rects and pixels.
//
// The reference's shape, which these pin:
//
//   - the action bar is `.launcher-action-bar`, a **sibling** of
//     `.launcher-results` (`main:src/launcher/LauncherResults.tsx:1043-1073`):
//     the panel's last band, never a row that a scroll can push under the
//     results;
//   - a typed query draws **no** held row (R64,
//     `main:src/launcher/terminal-row.ts:77-85`): holding the modifier only
//     moves the *selection* to that footer — 「按住时默认加到底部」;
//   - releasing it takes the selection back off the footer — 「松手就消失」 —
//     while the footer itself stays, because it is the field's hand-off and not
//     a gesture row;
//   - the held footer is *drawn* selected (「应该有选中样式（当前缺失）」): the
//     band's pixels change on the press edge and go back on the release edge.

// bandImage is a copy of the tester's frame, so two frames can be compared.
func bandImage(tt *ui.Tester) *image.RGBA {
	src := tt.Image()
	out := image.NewRGBA(src.Bounds())
	copy(out.Pix, src.Pix)
	return out
}

// bandDiff counts the pixels two frames disagree on inside the given box.
func bandDiff(a, b *image.RGBA, x0, y0, x1, y1 int) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if a.RGBAAt(x, y) != b.RGBAAt(x, y) {
				n++
			}
		}
	}
	return n
}

// TestTheActionBarIsThePanelsLastBandOverATypedQuery is the 「固定在第一项」
// report, read off the shipped frame: with a query typed, the footer is below
// every result row and sits in the panel's own bottom band.
func TestTheActionBarIsThePanelsLastBandOverATypedQuery(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Type("ter")
	tt.Frame()

	result, ok := tt.Find("Terminal")
	if !ok {
		t.Fatalf("the matched row was not drawn: %q", tt.Texts())
	}
	bar, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatalf("the action bar was not drawn: %q", tt.Texts())
	}
	if bar.Y < result.Y+result.H {
		t.Errorf("the action bar sits at y=%.1f, above the result ending at y=%.1f: the footer is not the list's last band",
			bar.Y, result.Y+result.H)
	}

	// The panel's own bottom edge: the window less the platform's inset (the
	// shell hands `inset` in the reference's units, so it is read back through
	// the theme's `Space`) and the card's 1px stroke.
	_, _, bottomUnits, _ := a.inset()
	spacing := float32(a.Launcher.Spacing)
	panelBottom := float32(420) - bottomUnits*spacing - 1
	if bottom := bar.Y + bar.H; bottom > panelBottom+1 {
		t.Errorf("the footer ends at y=%.1f, past the panel's %.1f edge: the band is not charged by the window",
			bottom, panelBottom)
	}
	// ...and it is the *last* band, not merely somewhere below the results: a
	// footer floating in the middle would leave the panel's bottom empty, which
	// is the same misplacement read from the other side.
	if slack := panelBottom - (bar.Y + bar.H); slack > bar.H {
		t.Errorf("the footer ends %.1f above the panel's bottom edge (%.1f > one row %.1f): it is not the pinned band",
			slack, panelBottom, bar.H)
	}
}

// TestTheHeldModifierMovesTheSelectionToTheFooterAndBack is the gesture half:
// a typed query adds no row, the press edge selects the footer, the release
// edge returns the choice to the page's default, and the footer's band is
// painted differently while it is selected.
func TestTheHeldModifierMovesTheSelectionToTheFooterAndBack(t *testing.T) {
	a := appKeysAppWithRecents()
	tt := appKeysRender(t, a)
	tt.Type("ter")
	tt.Frame()

	rowsUp := len(a.Launcher.Results())
	result, ok := tt.Find("Terminal")
	if !ok {
		t.Fatalf("the matched row was not drawn: %q", tt.Texts())
	}
	bar, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatalf("the action bar was not drawn: %q", tt.Texts())
	}
	up := bandImage(tt)
	if a.Launcher.Selected != 0 {
		t.Fatalf("the typed query starts on %d, want the first result", a.Launcher.Selected)
	}

	// The press edge: no new row (R64 — a typed query draws no held row), the
	// selection moves to the footer, and the footer's band is repainted.
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	held := bandImage(tt)
	if got := len(a.Launcher.Results()); got != rowsUp {
		t.Errorf("the modifier changed the list from %d rows to %d: a typed query draws no held row", rowsUp, got)
	}
	barIndex := len(a.Launcher.Results()) - 1
	if a.Launcher.Selected != barIndex {
		t.Errorf("the held modifier left the selection on %d, want the footer %d", a.Launcher.Selected, barIndex)
	}
	barX0, barX1 := int(bar.X), int(bar.X+bar.W)
	barY0, barY1 := int(bar.Y-2), int(bar.Y+bar.H+2)
	if n := bandDiff(up, held, barX0, barY0, barX1, barY1); n == 0 {
		t.Errorf("the held footer's band is painted exactly as the unselected one: no selected style")
	}
	// The rows above keep their pixels: the press edge moves the choice, it does
	// not relayout the list.
	resY0, resY1 := int(result.Y-2), int(result.Y+result.H+2)
	if n := bandDiff(up, held, barX0, resY0, barX1, resY1); n != 0 {
		t.Errorf("holding the modifier repainted %d pixels of the result row: the press edge moved the list", n)
	}

	// The release edge: the choice goes back to the page's default and the band
	// is painted as it was.
	tt.HoldModifiers(0)
	tt.Frame()
	if a.Launcher.Selected != 0 {
		t.Errorf("the release left the selection on %d, want the first result", a.Launcher.Selected)
	}
	if n := bandDiff(up, bandImage(tt), barX0, barY0, barX1, barY1); n != 0 {
		t.Errorf("%d pixels of the footer's band still carry the held selection after the release", n)
	}
}

// TestTheHeldRowOnlyExistsOnTheEmptyField is the other half of 「不是固定项」:
// the gesture row the user described is a row of the *empty* page, appended
// last, and it is gone the moment the field holds text — so it can never be
// "fixed" above a typed list.
func TestTheHeldRowOnlyExistsOnTheEmptyField(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	emptyRows := len(a.Launcher.Results())

	// The modifier adds no row at all now: its line is the panel's pinned band
	// (P11-bo), so the empty page keeps its row count and gains the band's
	// sentence instead.
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if got := len(a.Launcher.Results()); got != emptyRows {
		t.Fatalf("the held empty page has %d rows, want the page's own %d", got, emptyRows)
	}
	if !tt.HasText("Open a terminal session") {
		t.Fatalf("the held empty page drew no band: %q", tt.Texts())
	}
	if a.Launcher.Selected != 0 {
		t.Errorf("the modifier moved the selection to %d, want the page's own 0", a.Launcher.Selected)
	}

	// Typed, the same gesture adds nothing: the footer takes the selection
	// instead (see the test above), so no row of a typed list is ever the
	// gesture's.
	tt.HoldModifiers(0)
	tt.Frame()
	tt.Type("ter")
	tt.Frame()
	typedRows := len(a.Launcher.Results())
	tt.HoldModifiers(ui.Cmd)
	tt.Frame()
	if got := len(a.Launcher.Results()); got != typedRows {
		t.Errorf("the typed page grew from %d rows to %d under the modifier", typedRows, got)
	}
	for _, item := range a.Launcher.Results() {
		if item.ID == "system-terminal-bare" {
			t.Errorf("the gesture row is drawn over a typed query: %v", item.ID)
		}
	}
}

// TestTheCardReachesTheWindowsSideEdges is the 「搜索页面左右两边各有一大块
// 空白」 report read on the shipped frame: the list's own rect reaches the
// panel's edges, with no band of the window's colour beside it. The wider
// window is the case that report was written for — a card capped at the 720
// contract and centred inside a wider window leaves exactly two such bands,
// and `holdContractWidth` cannot act on a headless frame, so this reads the
// card's own width rule (fill the window) rather than the resize that usually
// keeps the two equal.
func TestTheCardReachesTheWindowsSideEdges(t *testing.T) {
	base := appKeysApp()
	_, right, _, left := base.inset()
	for _, width := range []int{720, 1000} {
		app := appKeysAppWithRecents()
		tt := ui.NewTester(func(c *ui.Context) { app.View(c) }, width, 420)
		tt.Frame()
		spacing := float32(app.Launcher.Spacing)
		list, ok := tt.Find("Launcher results")
		if !ok {
			t.Fatalf("the list was not drawn at %d wide: %q", width, tt.Texts())
		}
		leftGap := list.X - left*spacing
		rightGap := float32(width) - right*spacing - (list.X + list.W)
		if leftGap > 3 {
			t.Errorf("window %d wide: the list starts %.1f past the panel's left edge — a blank band beside the card",
				width, leftGap)
		}
		if rightGap > 3 {
			t.Errorf("window %d wide: the list ends %.1f short of the panel's right edge — a blank band beside the card",
				width, rightGap)
		}
		// ...and the face is painted at that edge, so the band is not merely
		// "the list thinks it is wide": the pixel under the list's first
		// column is the panel's own.
		if _, _, _, alpha := tt.Image().At(int(list.X)+2, int(list.Y+list.H/2)).RGBA(); alpha>>8 < 100 {
			t.Errorf("window %d wide: the panel is not painted under the list's first column (alpha %d)",
				width, alpha>>8)
		}
	}
}

// TestTheFooterIsNeverTheFirstRowOfATypedPage is 「现在固定在第一项」 and 「不是
// 固定项」 read off the shipped frame: a typed page's footer sits under every
// result row, and a query the catalogue does not answer draws no footer at all
// — the reference's `visibleActionBar` gate reads `launcherResults.length > 0`
// (`main:src/App.tsx:1543-1550`), so a command line with no match has no row
// above the field's own hand-off to sit under.
func TestTheFooterIsNeverTheFirstRowOfATypedPage(t *testing.T) {
	a := appKeysApp()
	tt := appKeysRender(t, a)
	tt.Type("term")
	tt.Frame()
	bar, ok := tt.Find("Run in shell")
	if !ok {
		t.Fatalf("the action bar was not drawn over a matched query: %q", tt.Texts())
	}
	result, ok := tt.Find("Terminal")
	if !ok {
		t.Fatalf("the matched row was not drawn: %q", tt.Texts())
	}
	if bar.Y <= result.Y {
		t.Errorf("the footer starts at y=%.1f, not below the first result at y=%.1f: it is drawn as a first row",
			bar.Y, result.Y)
	}

	// A URL is the footer's own page: the bar is the only row, and it is the
	// page's default selection — the selected style is the row's own index.
	a.Launcher.Query = "https://example.com"
	a.Launcher.Selected = 0
	tt.Frame()
	if results := a.Launcher.Results(); len(results) != 1 || results[0].ID != "bar:url" {
		t.Fatalf("the URL page is %v, want the single bar row", results)
	}
	if a.Launcher.Selected != 0 {
		t.Errorf("the URL page selects row %d, want the bar it is", a.Launcher.Selected)
	}

	// No match, no footer: the shell fallback is a control *for a matched
	// list*, never a fixed row of its own.
	a.Launcher.Query = "git status --short"
	a.Launcher.Selected = 0
	tt.Frame()
	if _, ok := tt.Find("Run in shell"); ok {
		t.Errorf("an unmatched command line drew the footer: %q", tt.Texts())
	}
}
