package launcher

import (
	"strings"
	"testing"

	"floter/internal/calculator"
)

// The calculator mode's Enter belongs to the field's text: an expression this
// build refuses must say why, and must never fall through to the history row
// the selection happens to sit on. These pins drive `activate` — the one entry
// point every Enter (key, click, action bar) reaches — so they cover the wiring
// rather than a helper.

// calculatorEnterApp is a mode open over a two-row history, with the copy
// action recorded.
func calculatorEnterApp(t *testing.T) (*App, *calculatorFixture, *string) {
	t.Helper()
	a := testApp()
	fixture := &calculatorFixture{entries: []calculator.Entry{
		{ID: "1", Expression: "10*3", Result: "30", CreatedAt: 2},
		{ID: "2", Expression: "sqrt(16)", Result: "4", CreatedAt: 1, Favorite: true},
	}}
	a.Calculator = fixture
	a.CopyMode = calculator.CopyFull
	copied := ""
	a.Actions.Copy = func(text string) { copied = text; testRuns["copy"]++ }
	a.EnterCalculator()
	return a, fixture, &copied
}

func TestABrokenSumSaysWhyInsteadOfCopyingAHistoryRow(t *testing.T) {
	a, fixture, copied := calculatorEnterApp(t)
	a.Query = "1+"
	a.Selected = 0
	if rows := a.Results(); len(rows) < 2 {
		t.Fatalf("the history did not show: %d rows", len(rows))
	}

	a.activate(a.Results())

	if *copied != "" {
		t.Errorf("a broken sum copied %q — Enter fell through to a history row", *copied)
	}
	if len(fixture.added) != 0 {
		t.Errorf("a broken sum was recorded: %v", fixture.added)
	}
	if !a.calculatorMode {
		t.Error("the mode closed on a refused expression; it must stay open")
	}
	if a.toast != "Invalid expression" {
		t.Errorf("feedback = %q, want %q", a.toast, "Invalid expression")
	}
	if !a.toastWarning {
		t.Error("a refusal is a warning, not a neutral note")
	}
}

func TestADivisionByZeroSaysItHasNoFiniteValue(t *testing.T) {
	a, fixture, copied := calculatorEnterApp(t)
	for _, query := range []string{"1/0", "1%0", "sqrt(-1)"} {
		a.Query = query
		a.Selected = 0
		a.toast = ""
		a.activate(a.Results())
		if *copied != "" {
			t.Errorf("%q copied %q", query, *copied)
		}
		if a.toast != "Result is not a finite number (division by zero?)" {
			t.Errorf("%q feedback = %q", query, a.toast)
		}
		if !a.calculatorMode {
			t.Errorf("%q closed the mode", query)
		}
	}
	if len(fixture.added) != 0 {
		t.Errorf("a non-finite sum was recorded: %v", fixture.added)
	}
}

func TestALongSumIsRefusedAndNotRecorded(t *testing.T) {
	a, fixture, copied := calculatorEnterApp(t)
	// 301 code points: past `calc.MaxExpressionLength` (256).
	a.Query = "1" + strings.Repeat("+1", 150)
	a.Selected = 0
	a.activate(a.Results())

	if *copied != "" {
		t.Errorf("an over-long sum copied %q", *copied)
	}
	if len(fixture.added) != 0 {
		t.Errorf("an over-long sum was recorded: %v", fixture.added)
	}
	if a.toast != "Expression is too long" {
		t.Errorf("feedback = %q, want %q", a.toast, "Expression is too long")
	}
	if !a.calculatorMode {
		t.Error("the mode closed on a refused expression")
	}
}

// TestTheModeStillCopiesTheRowTheSelectionIsOn keeps the existing contract: a
// field that holds a sum still has its answer row on the list, and moving the
// selection onto a history row means Enter copies *that* row. An empty field is
// the reference's "run" case (`calculatorEnterAction` returns "run" for it), so
// it copies the selected history row too.
func TestTheModeStillCopiesTheRowTheSelectionIsOn(t *testing.T) {
	a, _, copied := calculatorEnterApp(t)
	a.Query = "2+2"
	a.Selected = 1 // the first history row, not the pending answer
	if rows := a.Results(); len(rows) < 2 || !pendingCalculation(rows[0]) {
		t.Fatalf("expected a pending answer row above the history: %d rows", len(rows))
	}
	a.activate(a.Results())
	if *copied != "10*3 = 30" {
		t.Errorf("copied %q, want the chosen history row", *copied)
	}

	// An empty field is not an expression: Enter runs the selection.
	a, _, copied = calculatorEnterApp(t)
	a.Query = ""
	a.Selected = 0
	a.activate(a.Results())
	if *copied != "10*3 = 30" {
		t.Errorf("an empty field copied %q, want the selected history row", *copied)
	}
}

// TestAFreshCalculationCopiesTheEntrysOwnLine pins the one producer of
// `expression = result`: a fresh calculation copies what the entry it becomes
// would copy, so the two can never print two spellings.
func TestAFreshCalculationCopiesTheEntrysOwnLine(t *testing.T) {
	a, fixture, copied := calculatorEnterApp(t)
	a.Query = " =1+1 "
	a.Selected = 0
	a.activate(a.Results())

	if len(fixture.added) != 1 {
		t.Fatalf("recorded %v, want one row", fixture.added)
	}
	// The stored expression is the normal form, not the typed text: the "=" key
	// and the stray spaces are not part of the calculation's identity.
	if fixture.added[0] != [2]string{"1+1", "2"} {
		t.Errorf("recorded %v, want [{1+1 2}]", fixture.added)
	}
	if *copied != a.calculationText("1+1", "2") {
		t.Errorf("copied %q, want the entry's own line", *copied)
	}
	if *copied != "1+1 = 2" {
		t.Errorf("copied %q, want %q", *copied, "1+1 = 2")
	}
}
