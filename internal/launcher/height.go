package launcher

import (
	"math"
	"runtime"

	"floter/internal/plugincfg"
)

// The launcher window's height is the content's own: the field band, the
// rows the query matched, and the card's frame. A shorter list is a shorter
// window — one deliberate step per row-count change, not a keystroke's
// shake — and the ten-row budget is the tallest it goes. The old build
// resolved the same geometry on the browser side, measuring its own DOM;
// the geometry here is the theme's, so the drawn card and the charged
// window are one decision.

// The launcher's chrome, in the old app's units: the field's row, the breath
// under it, the panel's top inset and its tail, and a row's own floor with and
// without a subtitle. See `launcher.css` (`.collapsed-card__input-row` at 56u,
// `.launcher-bottom`'s 4u/2u padding) and `result-budget.ts`.
const (
	fieldRowUnits   = 14  // 56u
	breathUnits     = 1   // 4u
	rowTwoLineUnits = 10  // 40u (result-budget.ts's ROW_HEIGHT_TWO_LINE)
	rowOneLineUnits = 8.5 // 34u

	// The rows' own registers, as `.launcher-result` declared them: the title
	// is the ladder's title step (two units above the body), the subtitle the
	// body step, and a block heading the body step with its own 6u/4u padding
	// pair. The *line boxes* are the text's own — the framework lays a line at
	// the font's natural ascent, descent and gap, which is not a fixed
	// multiple of the size and cannot be made smaller than the font's own box
	// — so the view measures them and the window is charged what the text
	// takes.
	rowTitleStep   = 2
	headingPadding = 2.5 // 6u above, 4u below

	// TopInsetUnits and TailUnits are the card's own padding above the field's
	// row and below the last one; the shell draws the card, so it reads them
	// here rather than keeping a second copy that could drift from the window's
	// height.
	TopInsetUnits = 1   // 4u
	TailUnits     = 0.5 // 2u

	// FieldRowUnits is the launcher's field-row band, 56u, exported so the
	// settings card's header can name the same band rather than restating 14:
	// the old build pinned the launcher's input row and the settings header to
	// one height (R37, `SEARCH_FIELD_HEIGHT_UNITS`), so switching surfaces does
	// not move the window's first band.
	FieldRowUnits = fieldRowUnits
)

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

// HoldRows resolves the row count the window holds from the count it holds
// and the raw count the content needs. The window tracks the content exactly:
// the reference used to keep a one-row margin (its `resolveLauncherRows`), but
// R43/R66 retired it ("The window tracks the content exactly now; the
// smoothness is the resize animation, not a held height") - a one-row margin
// left the window a row too tall every time a row left, which is exactly the
// held modifier's row arriving and leaving. The walk to the new height is the
// shell's own spring (see shell.walkLauncherHeight).
func HoldRows(held, rows int) int {
	return RowCount(rows)
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

// RowInfo is what the view drew for one row of the list: the text lines it
// carries, and whether it is a block's heading or the action bar rather than a
// result.
type RowInfo struct {
	// Lines are the row's text lines: two for a row with a subtitle, one
	// without.
	Lines float64
	// Heading marks a block's title line, which is charged its own band
	// rather than a result row's floor.
	Heading bool
	// Bar marks the action bar, whose own 3u breath above it and 42u band the
	// window charges together, as the old build's 45u.
	Bar bool
	// Status marks a plugin's status note, which the old build charged its own
	// 30u band (`LAUNCHER_STATUS_UNITS`), shorter than a compact result row.
	Status bool
	// Band, when positive, is the row's own band in spacing units, charged
	// instead of the text-line arithmetic. The captured-output text form uses
	// it: the block is priced per `ROW_HEIGHT_TWO_LINE` band row, not per text
	// line (the old build's `pluginViewRows × ROW_HEIGHT_TWO_LINE`).
	Band float64
}

// Geometry is the measurements one window height is computed from. Every
// number is what the view draws — the theme's own spacing and the text's
// own line box — so the card and the window cannot disagree.
type Geometry struct {
	// Font is the theme's FontSize, the unit every text line is measured
	// with.
	Font float64
	// TitleLine and SubtitleLine are the heights of one line of the row's two
	// text registers, in pixels, as the framework lays them out: the view
	// measures them, so a font whose line box is taller than its size cannot
	// make a row shorter than it draws.
	TitleLine    float64
	SubtitleLine float64
	// Spacing is the theme's Spacing unit, which Space multiplies.
	Spacing float64
	// Rows are what the view drew, in order: each row's own text lines (two
	// for a row with a subtitle, one without) and whether it is a block's
	// heading rather than a result. Rows beyond this slice (a capped list) are
	// charged the worst case, two lines.
	Rows []RowInfo
	// Held is the row count the window is sized for: the rows the content
	// needs (see HoldRows), capped at the budget. Rows beyond the list's own
	// are charged the worst case, so a window holding four rows in a five-row
	// band has the room the next row arrives into.
	Held int
	// Filter says a plugin mode's chips row is drawn under the field, which
	// adds its band to the height (the old build's LAUNCHER_FILTER_UNITS).
	Filter bool
	// Feedback says the docked status row is showing under the list, which
	// adds its band (the old build's LAUNCHER_STATUS_UNITS).
	Feedback bool
	// Cap is the display ceiling in pixels; zero means none.
	Cap float64
	// Config is the open plugin-configuration overlay, which takes the
	// list's place: the window is then the field's row and the sheet's own
	// content, never the rows the list is holding behind it.
	Config *plugincfg.Schema
	// Sections is the number of section-title bands the list prints above its
	// blocks. The ordinary list carries a heading as a `RowInfo.Heading` row
	// (charged inside the ten-row budget); a captured-output list prices its
	// rows separately, so its headings are counted here and stand beside the
	// rows rather than eating one of them.
	Sections int
}

// Height is the window height a geometry asks for: the card's own chrome —
// the field's row (56u), the breath under it (4u) and the panel's insets (4u
// above, 2u below) — the rows at their own heights (a 12u padding pair plus
// their lines, never under the 40u/34u a row draws at), the chips or feedback
// bands when they show, and the platform's transparent margin; never above the
// cap. The old app resolved the same arithmetic on the browser side and wrote
// it as `66 + rows × 40` (see `result-budget.ts`).
func (g Geometry) Height() int {
	spacing := g.Spacing
	if spacing <= 0 {
		spacing = 4
	}
	font := g.Font
	if font <= 0 {
		font = 14
	}

	// The old app's own chrome, in its units (`--u`), which this codebase
	// divides by four: the field's row (56u, the height the settings pages'
	// header band shares), the breath under it (4u), the panel's top inset
	// (4u) and its tail (2u) — `66 + rows × 40` is a window height, exactly as
	// `result-budget.ts` wrote it. The field row is the same band the input is
	// drawn in, so the number the window is measured with is the number the
	// field occupies.
	chrome := (fieldRowUnits + breathUnits + TopInsetUnits + TailUnits) * spacing
	// The row's own vertical padding is zero: `.launcher-result` is a fixed
	// 40u box with its content centred (`align-items: center`), and the app's
	// row is the same — a MinHeight floor with the text centred in it and no
	// padding of its own. Only the horizontal 12u inset is padding.
	rowPadding := 0.0
	// The rows' two line boxes, as the view measured them. A caller that has
	// not measured (the shell before the first frame) is given the font's own
	// size, which is the floor of every real box.
	titleLine := g.TitleLine
	if titleLine <= 0 {
		titleLine = font + rowTitleStep
	}
	subtitleLine := g.SubtitleLine
	if subtitleLine <= 0 {
		subtitleLine = font
	}
	bodyLine := subtitleLine

	lines := 0.0
	rows := RowCount(g.Held)
	for i := 0; i < rows; i++ {
		info := RowInfo{Lines: 2} // the worst case: a row with a subtitle
		if i < len(g.Rows) {
			info = g.Rows[i]
		}
		if info.Band > 0 {
			// A row that names its own band: the captured-output text form's
			// `ROW_HEIGHT_TWO_LINE` block row, charged as the old build charged
			// it, not by the text line boxes it happens to hold.
			lines += info.Band * spacing
			continue
		}
		if info.Bar {
			// The action bar: its own 3u breath and 42u band, which the old
			// build charged together as `LAUNCHER_ACTION_BAR_UNITS`.
			lines += 11.25 * spacing
			continue
		}
		if info.Status {
			// A status note is its own 30u band (`LAUNCHER_STATUS_UNITS`),
			// shorter than a compact result row.
			lines += 7.5 * spacing
			continue
		}
		if info.Heading {
			// A heading is its own band: one body line and the 6u/4u padding
			// pair, as `.launcher-section-title` drew it.
			lines += headingPadding*spacing + bodyLine
			continue
		}
		count := info.Lines
		if count < 1 {
			count = 1
		}
		// A row is its content or its own floor, whichever is taller: the old
		// app's rows were fixed heights — 40u with a subtitle, 34u without —
		// and a one-line row of body text is shorter than 34u.
		floor := rowOneLineUnits * spacing
		if count >= 2 {
			floor = rowTwoLineUnits * spacing
		}
		// The title's line box and the subtitle's, as the view measured them:
		// the number the window is charged is the number the text takes.
		content := rowPadding + titleLine
		if count >= 2 {
			content += subtitleLine
		}
		lines += math.Max(content, floor)
	}
	filterBand := 0.0
	if g.Filter {
		// The chips row's own band plus the breath below it, as the sheet
		// draws: five spacing units at the step.
		filterBand = 5 * spacing
	}
	feedbackBand := 0.0
	if g.Feedback {
		// The docked status row's own band: 30u, the old build's
		// `LAUNCHER_STATUS_UNITS`.
		feedbackBand = 7.5 * spacing
	}
	sectionBand := 0.0
	if g.Sections > 0 {
		// A section-title band: one body line and the 6u/4u padding pair, the
		// old build's `LAUNCHER_SECTION_TITLE_UNITS` (26u).
		sectionBand = float64(g.Sections) * (headingPadding*spacing + bodyLine)
	}
	height := chrome + lines + sectionBand + filterBand + feedbackBand
	if g.Config != nil {
		// The sheet's own content, in the same units: the field's row and the
		// card's insets are the chrome above, so a schema can never grow the
		// window past the band the launcher gave it.
		cap := 0.0
		if g.Cap > 0 {
			cap = g.Cap - chrome - windowInset()
		}
		height = chrome + g.Config.Height(float64(spacing/4), cap)
	}
	height = math.Round(height + windowInset())
	if g.Cap > 0 && height > g.Cap {
		height = math.Round(g.Cap)
	}
	return int(height)
}
