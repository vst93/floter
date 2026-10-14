package launcher

import (
	"fmt"
	"testing"

	"floter/internal/calculator"
)

// TestTheCalculatorModeHoldsTheReferenceFetchDepth pins the calculator mode's
// held set to the reference's own ceiling: the inline launcher passes
// `MAX_CALCULATOR_MAX_ITEMS` as `calculatorModeRows`'s `limit`
// (`useLauncherCatalog.ts:871-878`), and the store's capacity cannot exceed the
// same number, so the mode's cap and the storage's are one.
func TestTheCalculatorModeHoldsTheReferenceFetchDepth(t *testing.T) {
	if CalculatorFetchLimit != calculator.MaxMaxItems {
		t.Fatalf("CalculatorFetchLimit = %d, want the store's own maximum %d",
			CalculatorFetchLimit, calculator.MaxMaxItems)
	}
	if CalculatorFetchLimit != 500 {
		t.Fatalf("CalculatorFetchLimit = %d, want 500", CalculatorFetchLimit)
	}

	entries := make([]calculator.Entry, CalculatorFetchLimit+100)
	for i := range entries {
		// The newest first, exactly as the store hands them over.
		entries[i] = calculator.Entry{
			ID: fmt.Sprintf("e%d", i), Expression: fmt.Sprintf("%d+1", i),
			Result: fmt.Sprintf("%d", i+1), CreatedAt: int64(len(entries) - i),
		}
	}
	a := testApp()
	a.Calculator = &calculatorFixture{entries: entries}
	a.RefreshCalculator()
	a.EnterCalculator()

	rows := a.Results()
	if len(rows) != CalculatorFetchLimit {
		t.Fatalf("the mode holds %d rows, want %d", len(rows), CalculatorFetchLimit)
	}
	// The newest entry leads the list.
	if rows[0].ID != "calc:e0" {
		t.Errorf("the first row is %q, want the newest entry", rows[0].ID)
	}
}

// TestTheCalculatorChipFilterSeesTheWholeHeldSet is the same rule the browser
// and clipboard caps follow: the chip narrows the set first and the ceiling is
// spent on what survives (`plugins/calculator/mode.ts:79-80`). A favourites
// view whose only favourite sits past the cap still shows it.
func TestTheCalculatorChipFilterSeesTheWholeHeldSet(t *testing.T) {
	entries := make([]calculator.Entry, CalculatorFetchLimit+50)
	for i := range entries {
		entries[i] = calculator.Entry{
			ID: fmt.Sprintf("e%d", i), Expression: fmt.Sprintf("%d+1", i),
			Result: fmt.Sprintf("%d", i+1), CreatedAt: int64(len(entries) - i),
		}
	}
	// The one favourite is the oldest entry, i.e. the last the cap would reach.
	entries[len(entries)-1].Favorite = true

	a := testApp()
	a.Calculator = &calculatorFixture{entries: entries}
	a.RefreshCalculator()
	a.EnterCalculator()

	// The favourites chip is the second filter Tab cycles to.
	a.cycleFilter(1)
	if a.calculatorFilter != calculatorFilterFavorites {
		t.Fatalf("the chip is %q, want favourites", a.calculatorFilter)
	}
	rows := a.Results()
	if len(rows) != 1 {
		t.Fatalf("the favourites view holds %d rows, want 1: %+v", len(rows), rows)
	}
	if rows[0].ID != fmt.Sprintf("calc:e%d", len(entries)-1) {
		t.Errorf("the favourite row is %q, want the oldest entry", rows[0].ID)
	}
}
