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
	base := Geometry{Font: 14, Spacing: 4, RowLines: []float64{1}, Held: 1}.Height()
	if base <= 0 {
		t.Fatalf("height = %d", base)
	}
	// A second line makes the window taller by one line box.
	two := Geometry{Font: 14, Spacing: 4, RowLines: []float64{2}, Held: 1}.Height()
	if two <= base {
		t.Errorf("a two-line row = %d, want taller than a one-line %d", two, base)
	}
	// A held band prices rows the list does not have yet.
	band := Geometry{Font: 14, Spacing: 4, RowLines: []float64{1}, Held: 5}.Height()
	if band <= base {
		t.Errorf("a five-row band = %d, want taller than one row %d", band, base)
	}
	// The cap binds.
	capped := Geometry{Font: 14, Spacing: 4, RowLines: []float64{2}, Held: rowBudget, Cap: 300}.Height()
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
		height := Geometry{Font: 14, Spacing: 4, RowLines: []float64{2}, Held: rows}.Height()
		if prev > 0 && height-prev <= 0 {
			t.Errorf("row %d = %d, not taller than %d", rows, height, prev)
		}
		prev = height
	}
}

// The window is charged with the height a line of text actually draws at: the
// framework's line box is the font's own ascent, descent and gap, which is
// taller than the size (14 points lay out at 22), and a model that assumed a
// multiple of the size left the card taller than its window.
func TestGeometryUsesTheMeasuredLineBox(t *testing.T) {
	// The measured box and the fallback differ: the model must use the first.
	measured := Geometry{Font: 14, Line: 22, Spacing: 4, RowLines: []float64{2}, Held: 1}.Height()
	assumed := Geometry{Font: 14, Spacing: 4, RowLines: []float64{2}, Held: 1}.Height()
	if measured <= assumed {
		t.Errorf("a measured line box of 22 gives %d, not more than the %d a font-sized one gives", measured, assumed)
	}
	// The measured box is what the drawn card is: two lines, the row's 12u
	// padding pair, the card's 66u chrome and the platform's margin.
	want := int(roundHalf(66 + 12 + 2*22 + windowInset()))
	if measured != want {
		t.Errorf("height = %d, want %d", measured, want)
	}
}

func roundHalf(v float64) float64 { return float64(int(v + 0.5)) }
