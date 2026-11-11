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

// Shared layout units keep the window charge and the rendered bands in sync.
const (
	fieldRowUnits = 16 // 64px at scale 1
	breathUnits   = 2  // 8px below the search field
	// Results keep the same height with or without a subtitle.
	rowUnits    = 12 // 48px at scale 1
	iconUnits   = 8  // 32px artwork, shared by the field and both result lists
	rowGapUnits = 3  // 12px between the icon and text
	// Enough room for the confirmation row’s 36px buttons and vertical air.
	launcherStatusUnits = 12
	// The footer includes its result row, the air above it and the shell tail.
	actionBarUnits       = rowUnits + actionBarBreathUnits + TailUnits
	actionBarBreathUnits = 0.75

	// The rows' own registers, as `.launcher-result` declared them: the title
	// is the ladder's title step (two units above the body), the subtitle the
	// body step, and a block heading the body step with its own 6u/4u padding
	// pair. The *line boxes* are the text's own — the framework lays a line at
	// the font's natural ascent, descent and gap, which is not a fixed
	// multiple of the size and cannot be made smaller than the font's own box
	// — so the view measures them and the window is charged what the text
	// takes.
	rowTitleStep   = 1
	headingPadding = 2.5 // 6u above, 4u below

	// TopInsetUnits and TailUnits are the card's own padding above the field's
	// row and below the last one; the shell draws the card, so it reads them
	// here rather than keeping a second copy that could drift from the window's
	// height.
	TopInsetUnits = 1   // 4u
	TailUnits     = 0.5 // 2u

	// FieldRowUnits also sizes the settings header so surface switches align.
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
// the height follows the list") - a one-row margin
// left the window a row too tall every time a row left, which is exactly the
// held modifier's row arriving and leaving. The walk to the new height is the
// shell applies once at the end of the frame (see shell.walkLauncherHeight).
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
	return windowInsetOS(runtime.GOOS, spacing)
}

// windowInsetOS is windowInset for a named platform, for the same reason the
// shell's `insetForOS` is: the charged height and the drawn card are one
// decision per platform, and a Windows-only slip in the charged number was
// invisible to the Linux and macOS suites — the platform that runs the test is
// the only one whose branch it read. The values are the shell's own ladder
// (`internal/shell.insetForOS`) summed top-to-bottom, so the window is charged
// exactly the band the card leaves: Linux 10u a side (20u), Windows 4u above
// and 12u below (16u), macOS none.
func windowInsetOS(goos string, spacing float64) float64 {
	if spacing <= 0 {
		spacing = 4
	}
	switch goos {
	case "linux":
		return 5 * spacing // 2 × 10u
	case "windows":
		return 4 * spacing // 4u + 12u
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
	// Bar marks a footer row, charged with its breathing room and shell tail.
	Bar bool
	// Status marks a note that uses the shared status band.
	Status bool
	// Band, when positive, is the row's own band in spacing units, charged
	// instead of the text-line arithmetic. The captured-output text form used
	// it: the block was priced per `ROW_HEIGHT_TWO_LINE` band row, not per text
	// line (the old build's `pluginViewRows × ROW_HEIGHT_TWO_LINE`).
	//
	// It is kept for the *ceiling* the text form still honours, but the block
	// itself is charged by `Block`: see the note there.
	Band float64
	// Block is a row that is one measured block of pixels rather than a line of
	// text or a band of spacing units — the captured-output text form.
	//
	// The old build priced that block in whole 40u band rows because its text
	// was laid out by the browser at whatever the line box came to. This build
	// draws the lines itself and knows the box, so it charges the box: a
	// three-line output is 3 × the body line box plus the block's own breath,
	// not two 40u bands. That is the difference the user read as 「现在显示的
	// 内容并不算很多，但还是触发了滚动条」 — the bands were granular enough to
	// be a whole line short at three, four and five lines, and the page opened
	// scrolled with a line cut off. The floor (three lines) and the ceiling (the
	// ten-band height, `pluginTextMaxUnits`) are unchanged; only the number
	// between them is now the drawn one.
	Block float64
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
	// (charged outside the result budget); a captured-output list prices its
	// rows separately, so its headings are counted here and stand beside the
	// rows rather than eating one of them.
	Sections int
	// OutputHeader is the captured-output page's own header band, in pixels:
	// the title row, its status line, and the breath under them.
	//
	// The band is drawn above the body (`launcher/output.go` `outputBody`: a
	// strip padded `edge` at the top, then the body's own `Space(4)` breath),
	// so a window priced from the rows alone is that strip short. That is the
	// scroll bar the user reported on a short output (「现在显示的内容并不算
	// 很多，但还是触发了滚动条」): three lines of text were charged 145px
	// against the 170px they draw, and a two-row list 116px against 154px.
	//
	// It is pixels rather than units because it is text: the two line boxes are
	// the font's own, measured the way `TitleLine` and `SubtitleLine` are, and
	// no multiple of the spacing step is that number. Zero on every other page.
	OutputHeader float64
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
	//
	// P13-fe（`docs/liquid-glass.md` §6.126）：这两个回退**不进阶梯**，它们不是画出来
	// 的寄存器，而是「还没量过时」的地板——`font` 是 `Geometry.Font`（= `t.FontSize`，
	// 正文档），`+ rowTitleStep` 只是比正文高一点点的一个保守值。真帧里 `TitleLine` /
	// `SubtitleLine` 永远是 `View` 量出来的 `tk.Title` / `tk.Body`，所以这里留偏移量
	// 不会让任何一个字离开阶梯；改成 `tokens` 反而会把设计 token 拉进这个纯几何包。
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
			// Section headings have no result shortcut and do not spend a result slot.
			if g.Rows[i].Heading && rows < len(g.Rows)-bars {
				rows++
			}
			continue
		}
		drawn = append(drawn, RowInfo{Lines: 2}) // the worst case: a row with a subtitle
	}
	for j := 0; j < bars; j++ {
		drawn = append(drawn, g.Rows[len(g.Rows)-bars+j])
	}
	for _, info := range drawn {
		if info.Block > 0 {
			// A block that measured itself: charged its own pixels, the way a
			// row is charged the line boxes the view measured for it.
			lines += info.Block
			continue
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
			lines += actionBarUnits * spacing
			continue
		}
		if info.Status {
			// A status note uses the shared status band,
			// shorter than a compact result row. One constant, not a second
			// hand-written copy: the view draws the same band with it
			// (`FeedbackBand`), so a change to the band moves both.
			lines += launcherStatusUnits * spacing
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
		// `LAUNCHER_STATUS_UNITS` — the same constant the status-note row
		// above is charged with, and the same one the view draws it with.
		feedbackBand = launcherStatusUnits * spacing
	}
	sectionBand := 0.0
	if g.Sections > 0 {
		// A section-title band: one body line and the 6u/4u padding pair, the
		// old build's `LAUNCHER_SECTION_TITLE_UNITS` (26u).
		sectionBand = float64(g.Sections) * (headingPadding*spacing + bodyLine)
	}
	// The captured-output page's header band, drawn above the rows it prices
	// (see Geometry.OutputHeader). Charged here, with the other bands, so a page
	// whose body exactly fills its rows is exactly the window it needs.
	height := chrome + lines + sectionBand + filterBand + feedbackBand + g.OutputHeader
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
	// The shell border consumes two DIPs; round up so fractional rows fit.
	height = math.Ceil(height + windowInset(spacing) + 2)
	if g.Cap > 0 && height > g.Cap {
		height = math.Floor(g.Cap)
	}
	return int(height)
}
