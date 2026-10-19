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
	fieldRowUnits = 14 // 56u
	breathUnits   = 1  // 4u
	// rowUnits is a result row's own box: `40u`. It is **one** box for every
	// row — with a subtitle or without.
	//
	// The reference draws it that way: `.launcher-result` is
	// `height: calc(var(--u) * 40)` (`launcher.css:1099-1104`), and the second
	// constant beside it, `ROW_HEIGHT_COMPACT = 34` (`result-budget.ts:75`), is
	// exported but read by nothing — no rule and no budget charges 34u. The port
	// read the pair as a ladder and drew a one-line row at 34u, four and a half
	// pixels short of the reference's box at the shipped step, which is the
	// pinched row the user saw (「整体小气」) and a window that grew by 6u the
	// moment a row gained a subtitle.
	//
	// One box is also the invariant `result-budget.ts` states: a row gaining a
	// subtitle does not move the window, because every row is charged
	// `ROW_HEIGHT_TWO_LINE` whether the list shows one line or two.
	rowUnits = 10 // 40u
	// launcherStatusUnits is a plugin's status note: the old build's
	// `LAUNCHER_STATUS_UNITS`, 30u -- shorter than a compact result row, and
	// charged by `Geometry.Height` for a row the view marks `Status`.
	launcherStatusUnits = 7.5

	// actionBarUnits is one action-bar row's band: its own 42u floor plus the
	// 3u breath above it, the old build's `LAUNCHER_ACTION_BAR_UNITS` (45u).
	// The footer is charged per row outside the ten-row budget and the view
	// reserves exactly this band at the scroller's bottom edge (see
	// `actionBarFooter`), so the window, the pinned footer and the list's own
	// tail are one number.
	actionBarUnits = 11.25

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
// in pixels at the geometry's own step: Linux and Windows reserve room for the
// card's shadow, macOS needs none. It is the shell's `inset()` in the same
// units — `calc(var(--u) * 10)` a side on Linux, `4u` + `12u` vertically on
// Windows (`base.css:1503,1591-1607`) — so the drawn card and the charged
// window are one decision. The port had these as bare pixels (20 / 16), which
// charged 2px more than the design draws at the shipped `small` step and left
// the card 2px short of the window's own height.
func windowInset(spacing float64) float64 {
	if spacing <= 0 {
		spacing = 4
	}
	switch runtime.GOOS {
	case "linux":
		return 5 * spacing // 10u a side
	case "windows":
		return 4 * spacing // 4u + 12u vertically
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
	// window charges together, as the old build's 45u — the one row the panel
	// charges a breath for.
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
	//
	// It is the *content* count, not the capped one: `Height` caps the results
	// itself and charges the action bar outside that cap (see the footer's own
	// paragraph there), so a capped count would lose the last result to the
	// bar. `App.HeldRows` is the capped count the app keeps for its own
	// accounting and is not this field's value.
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
// above, 2u below) — the rows at their own box, never under the 40u a result
// row draws at, the chips or feedback
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
	// The action bar is the panel's pinned footer and is charged **outside**
	// the ten-row budget.
	//
	// The reference draws it as a sibling below the scroller —
	// `{actionBar && <button className="launcher-action-bar">}` sits after the
	// results div is closed (`main:src/launcher/LauncherResults.tsx:1043-1073`)
	// — and its budget counts results only: `MAX_RESULTS = 10`
	// (`main:src/launcher/result-budget.ts:47-49`), which the user spelled out
	// as 「九项加一个底部的单独进入剪切板的项」 and later 「上方一共有 10 项了，
	// 应该最大只能有 9 项」 (R19) — the rows *above* the bar. The port priced
	// the bar as one of the ten, so a full list lost a match to it and, worse,
	// the bar was the eleventh band the window never paid for: it was drawn
	// below the window's bottom edge, i.e. the 「在终端中打开」 row disappeared
	// exactly when the list was full. A URL or a path draws two bar rows
	// (`barPathRows`), and both are the footer.
	bars := 0
	for i := len(g.Rows) - 1; i >= 0 && g.Rows[i].Bar; i-- {
		bars++
	}
	if bars > g.Held {
		bars = g.Held
	}
	resultsHeld := g.Held - bars
	rows := 0
	if resultsHeld > 0 || bars == 0 {
		// RowCount's one-row floor is the empty page's hint band; a page whose
		// only row is the bar has no hint band to pay for.
		rows = RowCount(resultsHeld)
	}
	drawn := make([]RowInfo, 0, rows+bars)
	for i := 0; i < rows; i++ {
		if i < len(g.Rows) {
			drawn = append(drawn, g.Rows[i])
			continue
		}
		drawn = append(drawn, RowInfo{Lines: 2}) // the worst case: a row with a subtitle
	}
	for j := 0; j < bars; j++ {
		drawn = append(drawn, g.Rows[len(g.Rows)-bars+j])
	}
	for _, info := range drawn {
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
			lines += actionBarUnits * spacing
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
		// A row is its content or its own box, whichever is taller: the old
		// app's rows were one fixed 40u height, subtitle or not, and a one-line
		// row of body text is shorter than that.
		floor := rowUnits * spacing
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
			cap = g.Cap - chrome - windowInset(spacing)
		}
		height = chrome + g.Config.Height(float64(spacing/4), cap)
	}
	height = math.Round(height + windowInset(spacing))
	if g.Cap > 0 && height > g.Cap {
		height = math.Round(g.Cap)
	}
	return int(height)
}
