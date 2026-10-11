package launcher

import (
	"testing"
)

// The row count the window holds: capped at the budget, floored at one.
func TestRowCount(t *testing.T) {
	if got := RowCount(0); got != 1 {
		t.Errorf("RowCount(0) = %d, want 1 (the hint band)", got)
	}
	if got := RowCount(-3); got != 1 {
		t.Errorf("RowCount(-3) = %d", got)
	}
	if got := RowCount(7); got != 7 {
		t.Errorf("RowCount(7) = %d", got)
	}
	if got := RowCount(50); got != rowBudget {
		t.Errorf("RowCount(50) = %d, want the budget", got)
	}
}

// Growing is immediate; shrinking waits for a row of margin.
func TestHoldRows(t *testing.T) {
	if got := HoldRows(0, 5); got != 5 {
		t.Errorf("first hold = %d, want 5", got)
	}
	if got := HoldRows(5, 8); got != 8 {
		t.Errorf("a growth is immediate: %d", got)
	}
	// One row below the held count is absorbed.
	if got := HoldRows(5, 4); got != 5 {
		t.Errorf("a one-row shrink is absorbed: %d", got)
	}
	// Two rows below steps down.
	if got := HoldRows(5, 3); got != 3 {
		t.Errorf("a two-row shrink steps down: %d", got)
	}
}

// The height is the card's own band: the field row, the edge, the rows and
// the frame, at the theme's values. A list of one-line rows is shorter than
// a list of two-line rows; a held count with no list yet prices the worst
// case, so the room the next row arrives into is already there.
func TestGeometryHeight(t *testing.T) {
	base := Geometry{Font: 14, Spacing: 4, Rows: []RowInfo{{Lines: 1}}, Held: 1}.Height()
	if base <= 0 {
		t.Fatalf("height = %d", base)
	}
	// A second line makes the window taller by one line box.
	two := Geometry{Font: 14, Spacing: 4, Rows: []RowInfo{{Lines: 2}}, Held: 1}.Height()
	if two <= base {
		t.Errorf("a two-line row = %d, want taller than a one-line %d", two, base)
	}
	// A held band prices rows the list does not have yet.
	band := Geometry{Font: 14, Spacing: 4, Rows: []RowInfo{{Lines: 1}}, Held: 5}.Height()
	if band <= base {
		t.Errorf("a five-row band = %d, want taller than one row %d", band, base)
	}
	// The cap binds.
	capped := Geometry{Font: 14, Spacing: 4, Rows: []RowInfo{{Lines: 2}}, Held: rowBudget, Cap: 300}.Height()
	if capped != 300 {
		t.Errorf("capped height = %d, want 300", capped)
	}
	// Defaults fill the theme's own values in.
	zero := Geometry{}
	if zero.Height() <= 0 {
		t.Error("a zero geometry still has a height")
	}
	// Ten rows stay within the budget's arithmetic: the height grows by
	// exactly one row per row.
	prev := 0
	for rows := 1; rows <= rowBudget; rows++ {
		height := Geometry{Font: 14, Spacing: 4, Rows: []RowInfo{{Lines: 2}}, Held: rows}.Height()
		if prev > 0 && height-prev <= 0 {
			t.Errorf("row %d = %d, not taller than %d", rows, height, prev)
		}
		prev = height
	}
}

// The window is charged with the line boxes the rows actually draw: the title
// register at 1.2, the subtitle at 1.15, and a heading the body step at 1.4
// with its own padding pair. A row's floor (42u with a subtitle, 34u without)
// still wins where it is taller.
func TestGeometryChargesTheRowsOwnLineBoxes(t *testing.T) {
	// Two lines at the drawn scales, in the units the view uses.
	two := Geometry{Font: 11, Spacing: 4, Rows: []RowInfo{{Lines: 2}}, Held: 1}.Height()
	one := Geometry{Font: 11, Spacing: 4, Rows: []RowInfo{{Lines: 1}}, Held: 1}.Height()
	if two <= one {
		t.Errorf("a two-line row is %d, not taller than a one-line %d", two, one)
	}
	// The row's floor wins over its own content: 42u with a subtitle.
	// chrome 66u + 42u floor + the platform's margin.
	want := int(roundHalf(66 + 42 + windowInset()))
	if two != want {
		t.Errorf("a two-line row is %d, want the 42u floor's %d", two, want)
	}
	// A heading is charged its own band — one body line at 1.4 and the 6u/4u
	// padding pair — not a row's floor, so a list of headings is shorter than
	// a list of rows.
	heading := Geometry{Font: 11, Spacing: 4, Rows: []RowInfo{{Heading: true}}, Held: 1}.Height()
	if heading >= one {
		t.Errorf("a heading is %d, not shorter than a one-line row %d", heading, one)
	}
	// The heading's own line box is the body one (a caller that has not
	// measured is given the font's own size, which is the floor of a real box).
	wantHeading := int(roundHalf(66 + 2.5*4 + 11 + windowInset()))
	if heading != wantHeading {
		t.Errorf("a heading is %d, want %d", heading, wantHeading)
	}
}

func roundHalf(v float64) float64 { return float64(int(v + 0.5)) }

// The pitch between two drawn rows is the height the window charges for a row:
// a list of n rows is n times the pitch taller than a list of n-1, and the rows
// on screen are where that arithmetic says they are.
func TestDrawnPitchIsTheChargedRow(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Frame()
	// Two adjacent rows of the same shape (both carry a subtitle).
	var first, second float32
	if rect, ok := tt.Find("Open settings"); ok {
		first = rect.Y
	}
	if rect, ok := tt.Find("Open terminal"); ok {
		second = rect.Y
	}
	if first == 0 || second == 0 {
		t.Fatalf("the rows are not on screen: %q", tt.Texts())
	}
	pitch := float64(second - first)

	measured := func(held int) Geometry {
		rows := make([]RowInfo, held)
		for i := range rows {
			rows[i] = RowInfo{Lines: 2}
		}
		return Geometry{
			Font: a.Font, Spacing: a.Spacing, Rows: rows, Held: held,
			TitleLine: a.TitleLine, SubtitleLine: a.SubtitleLine,
		}
	}
	one := measured(1).Height()
	two := measured(2).Height()
	charged := float64(two - one)
	if diff := pitch - charged; diff > 1.5 || diff < -1.5 {
		t.Errorf("the drawn pitch is %.1f, the window charges %.1f per row", pitch, charged)
	}
}
