package launcher

import (
	"strings"
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

// The window tracks the content exactly: a growth is immediate and a shrink is
// immediate too. The reference's one-row margin (`resolveLauncherRows`) was
// retired by R66 - it left the window a row too tall whenever a row left, which
// the held modifier's row does on release.
func TestHoldRows(t *testing.T) {
	if got := HoldRows(0, 5); got != 5 {
		t.Errorf("first hold = %d, want 5", got)
	}
	if got := HoldRows(5, 8); got != 8 {
		t.Errorf("a growth is immediate: %d", got)
	}
	// A one-row shrink is not absorbed: the window follows the content down.
	if got := HoldRows(5, 4); got != 4 {
		t.Errorf("a one-row shrink steps down: %d, want 4", got)
	}
	if got := HoldRows(5, 3); got != 3 {
		t.Errorf("a two-row shrink steps down: %d", got)
	}
	// The cap and the floor still hold.
	if got := HoldRows(9, 50); got != rowBudget {
		t.Errorf("over the budget = %d, want %d", got, rowBudget)
	}
	if got := HoldRows(3, 0); got != 1 {
		t.Errorf("an empty list = %d, want the one-row floor", got)
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

// The window is charged with the line boxes the rows actually draw, and a
// row's floor (40u with a subtitle, 34u without) still wins where it is
// taller. A two-line row's content — the title register and the subtitle
// register, with no vertical padding — is charged, and a heading is its own
// band (one body line and the 6u/4u padding pair), not a row's floor.
func TestGeometryChargesTheRowsOwnLineBoxes(t *testing.T) {
	// Two lines at the drawn scales, in the units the view uses.
	two := Geometry{Font: 11, Spacing: 4, Rows: []RowInfo{{Lines: 2}}, Held: 1}.Height()
	one := Geometry{Font: 11, Spacing: 4, Rows: []RowInfo{{Lines: 1}}, Held: 1}.Height()
	if two <= one {
		t.Errorf("a two-line row is %d, not taller than a one-line %d", two, one)
	}
	// The row's floor wins over its own content: 40u with a subtitle.
	// chrome 66u + 40u floor + the platform's margin.
	want := int(roundHalf(66 + 40 + windowInset(4)))
	if two != want {
		t.Errorf("a two-line row is %d, want the 40u floor's %d", two, want)
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
	wantHeading := int(roundHalf(66 + 2.5*4 + 11 + windowInset(4)))
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
	if rect, ok := tt.Find("Restart the computer"); ok {
		first = rect.Y
	}
	if rect, ok := tt.Find("Shut down the computer"); ok {
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

// The captured-output text form is priced per band row, not per text line: the
// old build's `pluginViewRows × ROW_HEIGHT_TWO_LINE`. `pluginTextRows` is the
// count; its three-line floor and ten-row ceiling are the old build's
// `PLUGIN_TEXT_MIN_UNITS` / `PLUGIN_TEXT_MAX_UNITS`.
func TestPluginTextRows(t *testing.T) {
	cases := []struct {
		text  string
		lines int
		rows  int
	}{
		{"", 0, 2},           // no lines: the three-line floor (72u → 2 band rows)
		{"a", 1, 2},          // one line is still under the floor
		{"a\nb", 2, 2},       // two lines, still under the floor
		{"a\nb\nc", 3, 2},    // exactly the floor: 72u → 2 rows
		{"a\nb\nc\nd", 4, 3}, // 96u → 3 rows
		{strings.Repeat("x\n", 99) + "x", 100, 10}, // 600u clamps to 400u → 10 rows
		{strings.Repeat("x\n", 199) + "x", 200, 10},
	}
	for _, tc := range cases {
		if got := strings.Count(tc.text, "\n"); tc.text != "" && got+1 != tc.lines {
			t.Fatalf("the fixture has %d lines, want %d", got+1, tc.lines)
		}
		if got := pluginTextRows(tc.text); got != tc.rows {
			t.Errorf("pluginTextRows(%d lines) = %d, want %d", tc.lines, got, tc.rows)
		}
	}
}

// A status note is its own 30u band and a section title its own 26u band: both
// are shorter than the result row they used to be charged as, and a captured
// output list prices its headings beside its rows rather than inside the
// ten-row budget.
func TestStatusAndSectionBands(t *testing.T) {
	const (
		font    = 14.0
		spacing = 4.0
	)
	// A status note (30u) is shorter than a one-line result row (its 34u
	// floor), and a section band (26u) shorter still.
	status := Geometry{Font: font, Spacing: spacing, Rows: []RowInfo{{Status: true}}, Held: 1}.Height()
	result := Geometry{Font: font, Spacing: spacing, Rows: []RowInfo{{Lines: 1}}, Held: 1}.Height()
	if status >= result {
		t.Errorf("a status row (%d) is not shorter than a result row (%d)", status, result)
	}
	section := Geometry{Font: font, Spacing: spacing, Rows: []RowInfo{{Lines: 1}}, Held: 1, Sections: 1}.Height()
	if section <= result {
		t.Errorf("a section band (%d) did not add to the row (%d)", section, result)
	}
	// Two sections cost twice one.
	one := Geometry{Font: font, Spacing: spacing, Rows: []RowInfo{{Lines: 1}}, Held: 1, Sections: 1}.Height()
	two := Geometry{Font: font, Spacing: spacing, Rows: []RowInfo{{Lines: 1}}, Held: 1, Sections: 2}.Height()
	if got := two - one; got != one-result {
		t.Errorf("a second section added %d, the first %d", got, one-result)
	}
}

// A band row names its own price: the captured text form's block is charged
// `ROW_HEIGHT_TWO_LINE` per band row whatever its text line boxes are, so a
// taller font cannot make the block taller than the rows it stands for.
func TestBandRowsArePricedAsTheirBand(t *testing.T) {
	const spacing = 4.0
	band := Geometry{Font: 14, Spacing: spacing, Rows: []RowInfo{{Band: rowTwoLineUnits}}, Held: 1}.Height()
	twoLine := Geometry{Font: 14, Spacing: spacing, Rows: []RowInfo{{Lines: 2}}, Held: 1}.Height()
	if band != twoLine {
		t.Errorf("a band row is %d, a two-line row %d", band, twoLine)
	}
}

// The action bar is the panel's footer, not one of the ten rows.
//
// The reference draws it as a sibling below the scroller
// (`main:src/launcher/LauncherResults.tsx:1043-1073`) and its budget counts
// results only — `MAX_RESULTS = 10` (`main:src/launcher/result-budget.ts:47-49`),
// 「九项加一个底部的单独进入剪切板的项」, later 「上方一共有 10 项了，应该最大
// 只能有 9 项」 (R19: the ten are the rows *above* the bar). So a full list plus
// a bar is ten results **and** the bar's own band, never nine results and a bar
// the window did not pay for — that bar was the eleventh band, drawn below the
// window's bottom edge, i.e. the 「在终端中打开」 row vanishing exactly when the
// list was full.
func TestTheActionBarIsChargedOutsideTheBudget(t *testing.T) {
	bar := RowInfo{Bar: true}
	rows := func(n int) []RowInfo {
		out := make([]RowInfo, n)
		for i := range out {
			out[i].Lines = 2
		}
		return out
	}

	full := Geometry{Font: 14, Spacing: 4, Rows: rows(rowBudget), Held: rowBudget}.Height()
	// Ten results plus the bar: the ten results keep their own bands and the
	// bar adds its 11.25u band on top.
	withBar := append(rows(rowBudget), bar)
	got := Geometry{Font: 14, Spacing: 4, Rows: withBar, Held: len(withBar)}.Height()
	want := full + int(roundHalf(11.25*4))
	if got != want {
		t.Errorf("ten results plus the bar = %d, want %d (the budget's rows and the bar's own band)", got, want)
	}
	// The bar does not take a result's slot: nine results plus the bar is one
	// result row shorter than the ten above, not two.
	nine := append(rows(rowBudget-1), bar)
	gotNine := Geometry{Font: 14, Spacing: 4, Rows: nine, Held: len(nine)}.Height()
	if gotNine >= got {
		t.Errorf("nine results plus the bar = %d, not shorter than ten plus the bar %d", gotNine, got)
	}
	// A page whose only row is the bar (a URL that matched nothing) pays for
	// that one band and no hint band under it.
	onlyBar := Geometry{Font: 14, Spacing: 4, Rows: []RowInfo{bar}, Held: 1}.Height()
	if onlyBar >= gotNine {
		t.Errorf("a bar-only page = %d, not shorter than nine rows plus a bar %d", onlyBar, gotNine)
	}
}
