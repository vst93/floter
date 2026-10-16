package launcher

import (
	"strings"
	"time"

	"floter/internal/calc"
	"floter/internal/calculator"
	"floter/internal/i18n"
)

// The calculator mode: the trigger word, the pending expression, and the two
// filters the Tab key cycles.
//
// The field's text is the pending expression, exactly as the old build had it:
// Enter evaluates a *fresh* expression (and records it), and Enter on an
// already-evaluated expression or on a history row copies that row per the
// plugin's copy mode.

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
	a.modeWord = firstWord(query)
	a.Query = ""
	a.calculatorFilter = calculatorFilterAll
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
	// The same rule as the clipboard's: an armed delete leaves with the mode.
	a.disarmDelete()
	a.Selected, a.chosenRow = 0, -1
}

// calculatorQuery is what the user typed after the mode word.
func (a *App) calculatorQuery() string {
	return a.Query
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

// CalculatorFetchLimit is how deep the calculator mode's held history goes:
// the reference's inline mode raises the output view's ten-row group ceiling to
// `MAX_CALCULATOR_MAX_ITEMS` (`MAX_CALCULATOR_MAX_ITEMS = 500`,
// `calculator.ts:250`) and passes that as `calculatorModeRows`'s `limit`
// (`useLauncherCatalog.ts:871-878`), which the mode spends *after* the chip
// filter (`plugins/calculator/mode.ts:79-80`, `matched.slice(0, limit)`). The
// reference's own `CALCULATOR_FETCH_LIMIT` is `MAX_RESULTS`, the same ten-row
// box ceiling the clipboard's `CLIPBOARD_FETCH_LIMIT` is, and is deliberately
// not replicated for the same reason (see `ClipboardFetchLimit`).
//
// The history store's own `maxItems` bounds the *stored* list and is a setting
// the user can raise to `calculator.MaxMaxItems`, so this cap and the store's
// are the same number: the mode cannot be handed more rows than the store keeps
// unless favourites outlive the cap. Pinning it here — the store's own maximum
// — is what makes the two ceilings agree rather than leaving the mode's reach to
// depend on which setting the user chose.
const CalculatorFetchLimit = calculator.MaxMaxItems

// calculatorItems is the calculator mode's list: the pending expression's
// answer first, then the history rows the chip keeps, capped at
// `CalculatorFetchLimit` the way the reference's `calculatorModeRows` spends its
// `limit` after the chip filter.
//
// The history is deliberately *not* filtered by the field's text: the field
// is the pending expression, and filtering the list by it would hide every
// past calculation exactly while the user is writing a new one. The chip
// (all / favorites) is the list's only filter, as the old build's own mode
// had it.
func (a *App) calculatorItems() []Item {
	expression := strings.TrimSpace(strings.TrimPrefix(a.calculatorQuery(), "="))
	out := make([]Item, 0, len(a.calculatorFound)+1)

	// A fresh expression offers its answer, which Enter evaluates, records and
	// copies. The expression is the title that gives up room and the answer the
	// trailing value, complete at the row's right edge — R50's register, the
	// same row the history rows and the ordinary page's own answer use.
	if expression != "" {
		if value, err := calc.Eval(expression); err == nil {
			result := calc.Format(value)
			out = append(out, Item{
				ID:       "calc:" + expression,
				Title:    expression,
				trailing: result,
				Run:      func() { a.runCalculation(expression, result) },
			})
		}
	}

	copy := a.copy()
	now := time.Now()
	history := 0
	for _, entry := range a.calculatorFound {
		if a.calculatorFilter == calculatorFilterFavorites && !entry.Favorite {
			continue
		}
		// The reference's own order: the chip narrows the set first and the
		// limit is spent on what survives (`mode.ts:79-80`), so a chip that
		// keeps few rows cannot be starved by the ones it drops.
		if history >= CalculatorFetchLimit {
			break
		}
		history++
		entry := entry
		// A favourited row shows its state through the star *control* at its
		// trailing edge, never a `★` badge in the key slot: the reference's
		// own row carries no such text, and printing both made a starred entry
		// wear two stars (the clipboard row's twin).
		out = append(out, Item{
			ID:       "calc:" + entry.ID,
			Title:    entry.Expression,
			Detail:   calculatorRowTime(entry, now, copy),
			trailing: entry.Result,
			Run:      func() { a.copyCalculation(entry) },
			calc:     &entry,
		})
	}
	// An empty list says its own thing through the list's empty state (see
	// emptyMessage), not through the feedback row: the row is for what an
	// *action* did.
	return out
}

// calculatorRowTime is a history row's compact time, the old build's
// `calculatorTime`: a clock for an entry made today, the translated word for
// one made yesterday, and a short date beyond that. It is the row's subtitle
// slot — the result lives at the row's right edge.
func calculatorRowTime(entry calculator.Entry, now time.Time, copy i18n.Launcher) string {
	created := entry.Time()
	if created.IsZero() {
		return ""
	}
	switch {
	case sameCalendarDay(created, now):
		return created.Format("15:04")
	case sameCalendarDay(created, now.AddDate(0, 0, -1)):
		return copy.CalculatorYesterday
	case created.Year() == now.Year():
		return created.Format("01-02")
	default:
		return created.Format("2006-01-02")
	}
}

// sameCalendarDay reports whether two instants fall on the same local day.
func sameCalendarDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
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
