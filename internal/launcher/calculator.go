package launcher

import (
	"strings"

	"floter/internal/calc"
	"floter/internal/calculator"
)

// The calculator mode: the trigger word, an expression or the history's
// needle, and the two filters the Tab key cycles.
//
// The field's text is both the pending expression and the history's search
// needle, exactly as the old build had it: Enter evaluates a *fresh*
// expression (and records it), and Enter on an already-evaluated expression or
// on a history row copies that row per the plugin's copy mode.

// calculatorWord is what the field starts with in the mode; the other
// spellings enter it just as well.
const calculatorWord = "calc"

// calculatorWords are the trigger words, as the old build accepted them.
var calculatorWords = []string{"calc", "calculator", "计算器", "计算", "="}

// The two filters, in the order Tab cycles them; `all` is the shipped one.
const (
	calculatorFilterAll       = "all"
	calculatorFilterFavorites = "favorites"
)

// CalculatorSource is the calculator history the launcher shows. The launcher
// reads it when the mode opens and after every change; the store behind it is
// bounded (at most a few hundred rows), so filtering locally is cheap and a
// keystroke never has to wait for a round trip.
type CalculatorSource interface {
	Entries() []calculator.Entry
	Add(expression, result string) (calculator.Entry, error)
	ToggleFavorite(id string) (bool, error)
	Delete(id string) error
}

// EnterCalculator opens the calculator history mode.
func (a *App) EnterCalculator() { a.enterCalculator() }

// InCalculatorMode reports whether the calculator mode is open.
func (a *App) InCalculatorMode() bool { return a.calculatorMode }

// enterCalculator starts the calculator mode.
func (a *App) enterCalculator() {
	a.enterCalculatorWord(calculatorWord + " ")
}

// enterCalculatorWord opens the calculator mode with the field's text.
func (a *App) enterCalculatorWord(query string) {
	a.mode = nil
	a.clipboard = false
	a.browser = false
	a.calculatorMode = true
	a.calculatorFilter = calculatorFilterAll
	a.Query = query
	a.Selected, a.chosenRow = 0, -1
	a.pendingCaret = true
	a.refreshCalculator()
}

// leaveCalculator returns to the search.
func (a *App) leaveCalculator() {
	if !a.calculatorMode {
		return
	}
	a.calculatorMode = false
	a.Query = ""
	a.Selected, a.chosenRow = 0, -1
}

// calculatorQuery is what the user typed after the mode word.
func (a *App) calculatorQuery() string {
	words := splitArgs(a.Query)
	if len(words) <= 1 {
		return ""
	}
	return strings.TrimSpace(strings.Join(words[1:], " "))
}

// RefreshCalculator re-reads the history from the source, for a caller that
// changed it (a settings change, a favourite toggle).
func (a *App) RefreshCalculator() { a.refreshCalculator() }

// refreshCalculator re-reads the history from the source.
func (a *App) refreshCalculator() {
	if a.Calculator == nil {
		a.calculatorFound = nil
		return
	}
	a.calculatorFound = a.Calculator.Entries()
}

// calculatorItems is the calculator mode's list: the pending expression's
// answer first, then the history rows matching the needle and the filter.
func (a *App) calculatorItems() []Item {
	needle := a.calculatorQuery()
	expression := strings.TrimSpace(strings.TrimPrefix(needle, "="))
	out := make([]Item, 0, len(a.calculatorFound)+1)

	// A fresh expression offers its answer, which Enter evaluates, records and
	// copies.
	if expression != "" {
		if value, err := calc.Eval(expression); err == nil {
			result := calc.Format(value)
			out = append(out, Item{
				ID:     "calc:" + expression,
				Title:  result,
				Detail: expression,
				Run:    func() { a.runCalculation(expression, result) },
			})
		}
	}

	terms := strings.Fields(strings.ToLower(needle))
	for _, entry := range a.calculatorFound {
		if a.calculatorFilter == calculatorFilterFavorites && !entry.Favorite {
			continue
		}
		if !calculatorMatches(entry, terms) {
			continue
		}
		entry := entry
		shortcut := ""
		if entry.Favorite {
			shortcut = "★"
		}
		out = append(out, Item{
			ID:       "calc:" + entry.ID,
			Title:    entry.Expression,
			Detail:   entry.Result,
			Shortcut: shortcut,
			Run:      func() { a.copyCalculation(entry) },
			calc:     &entry,
		})
	}
	// An empty list says its own thing through the list's empty state (see
	// emptyMessage), not through the feedback row: the row is for what an
	// *action* did.
	return out
}

// runCalculation records a fresh calculation and copies it.
func (a *App) runCalculation(expression, result string) {
	if a.Calculator != nil {
		if _, err := a.Calculator.Add(expression, result); err != nil {
			a.WarnFeedback(StringsFor(a.settings().Language).CalculatorRecordFailed)
		} else {
			a.refreshCalculator()
		}
	}
	if a.Actions.Copy != nil {
		a.Actions.Copy(a.calculationText(expression, result))
	}
	a.leaveCalculator()
	a.Hide()
}

// copyCalculation copies a history row per the plugin's copy mode.
func (a *App) copyCalculation(entry calculator.Entry) {
	if a.Actions.Copy != nil {
		a.Actions.Copy(entry.Text(a.CopyMode))
	}
	a.leaveCalculator()
	a.Hide()
}

// calculationText is what Enter copies for a fresh calculation: the whole line
// or the result alone, by the same setting a history row obeys.
func (a *App) calculationText(expression, result string) string {
	if a.CopyMode == calculator.CopyResult {
		return result
	}
	return expression + " = " + result
}

// toggleCalculatorFavorite stars or unstars the chosen row.
func (a *App) toggleCalculatorFavorite() {
	if a.Calculator == nil {
		return
	}
	results := a.Results()
	if a.Selected < 0 || a.Selected >= len(results) {
		return
	}
	item := results[a.Selected]
	if item.calc == nil {
		return
	}
	if _, err := a.Calculator.ToggleFavorite(item.calc.ID); err != nil {
		a.toast = StringsFor(a.settings().Language).CalculatorFavoriteFailed
	}
	a.refreshCalculator()
}

// cycleCalculatorFilter moves to the other filter, as Tab does.
func (a *App) cycleCalculatorFilter() {
	if a.calculatorFilter == calculatorFilterAll {
		a.calculatorFilter = calculatorFilterFavorites
		return
	}
	a.calculatorFilter = calculatorFilterAll
}

// calculatorMatches reports whether every term appears in the entry.
func calculatorMatches(entry calculator.Entry, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(entry.Expression + " " + entry.Result)
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}
