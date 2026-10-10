package launcher

import (
	"math"
	"runtime"
)

// The launcher window's height is the content's own: the field band, the
// rows the query matched, and the card's frame. A shorter list is a shorter
// window — one deliberate step per row-count change, not a keystroke's
// shake — and the ten-row budget is the tallest it goes. The old build
// resolved the same geometry on the browser side, measuring its own DOM;
// the geometry here is the theme's, so the drawn card and the charged
// window are one decision.

// rowBudget is the rows the window holds at most: a broad query scrolls
// inside the window instead of growing it past a screen.
const rowBudget = 10

// RowCount is the rows the window has to hold, capped at the budget and
// floored at one (an empty query still draws the hint band).
func RowCount(results int) int {
	if results < 1 {
		return 1
	}
	if results > rowBudget {
		return rowBudget
	}
	return results
}

// Hysteresis is how many rows below the held count the content must fall
// before the window steps down. One absorbs the 1↔2 boundary flap: a
// keystroke that deletes one row of two leaves the window where it is, and
// the next growth finds the room already there.
const Hysteresis = 1

// HoldRows resolves the row count the window holds from the count it holds
// and the raw count the content needs. Growing is immediate; shrinking
// waits for a row of margin.
func HoldRows(held, rows int) int {
	target := RowCount(rows)
	current := RowCount(held)
	if target >= current {
		return target
	}
	if target < current-Hysteresis {
		return target
	}
	return current
}

// windowInset is the transparent margin the window carries around the card,
// in pixels: Linux and Windows reserve room for the card's shadow, macOS
// needs none. The same numbers the shell's inset() draws with.
func windowInset() float64 {
	switch runtime.GOOS {
	case "linux":
		return 20 // 10 DIPs a side
	case "windows":
		return 16 // 4 + 12 vertical
	default:
		return 0
	}
}

// Geometry is the measurements one window height is computed from. Every
// number is what the view draws — the theme's own spacing and the text's
// own line box — so the card and the window cannot disagree.
type Geometry struct {
	// Font is the theme's FontSize, the unit every text line is measured
	// with.
	Font float64
	// Spacing is the theme's Spacing unit, which Space multiplies.
	Spacing float64
	// RowLines are the text lines of each visible row: two for a row with a
	// subtitle, one without. Rows beyond this slice (a capped list) are
	// charged the worst case, two lines.
	RowLines []float64
	// Held is the row count the window is sized for (with hysteresis): the
	// band. Rows beyond the list's own are charged the worst case, so a
	// window holding four rows in a five-row band has the room the next
	// row arrives into.
	Held int
	// Filter says a plugin mode's chips row is drawn under the field, which
	// adds its band to the height (the old build's LAUNCHER_FILTER_UNITS).
	Filter bool
	// Cap is the display ceiling in pixels; zero means none.
	Cap float64
}

// Height is the window height a geometry asks for: the card's own band —
// the field row (Space 7), the edge above the first row (Space 2), the
// rows at their own heights (Space 1.5 padding a side plus their lines)
// and the card's padding pair (Space 2.5 a side) — plus the platform
// inset, and never above the cap.
func (g Geometry) Height() int {
	spacing := g.Spacing
	if spacing <= 0 {
		spacing = 4
	}
	font := g.Font
	if font <= 0 {
		font = 14
	}
	const lineLeading = 1.4 // the text's own line box, as the theme draws it
	fieldRow := 7 * spacing
	edge := 2 * spacing
	rowPadding := 3 * spacing
	cardPadding := 5 * spacing

	lines := 0.0
	rows := RowCount(g.Held)
	for i := 0; i < rows; i++ {
		count := 2.0 // the worst case: a row with a subtitle
		if i < len(g.RowLines) {
			count = g.RowLines[i]
			if count < 1 {
				count = 1
			}
		}
		lines += rowPadding + float64(count)*font*lineLeading
	}
	filterBand := 0.0
	if g.Filter {
		// The chips row's own band plus the breath below it, as the sheet
		// draws: five spacing units at the step.
		filterBand = 5 * spacing
	}
	height := math.Round(fieldRow + edge + lines + cardPadding + filterBand + windowInset())
	if g.Cap > 0 && height > g.Cap {
		height = math.Round(g.Cap)
	}
	return int(height)
}
