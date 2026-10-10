package launcher

// The plugin list modes' filter axes, declared once. Each axis is the ordered
// set of values its chips row and its Tab cycle walk, with the word each chip
// prints — one fact, so the values and their words cannot drift apart. The
// axis carries no predicate: what a value *selects* is a fact about the mode's
// own rows, and it lives where they are built.

import (
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	clipboardpkg "floter/internal/clipboard"
	"floter/internal/i18n"
)

// FilterAxis is one dimension a mode's list can be filtered by.
type FilterAxis struct {
	// Values are the choices, in display order.
	Values []string
	// Label is the word a value's chip prints.
	Label func(copy i18n.Launcher, value string) string
}

// Cycle moves one step through the axis, wrapping at both ends: 1 for Tab and
// -1 for Shift+Tab, the contract every mode shares. A choice off the axis
// lands on the first rather than trapping the field.
func (a FilterAxis) Cycle(current string, direction int) string {
	index := -1
	for i, value := range a.Values {
		if value == current {
			index = i
			break
		}
	}
	if index < 0 {
		return a.Values[0]
	}
	length := len(a.Values)
	next := ((index+direction)%length + length) % length
	return a.Values[next]
}

// LabelOf is the word for a value, or the value itself off the axis.
func (a FilterAxis) LabelOf(copy i18n.Launcher, value string) string {
	if a.Label == nil {
		return value
	}
	return a.Label(copy, value)
}

// The browser mode's range chips: the whole answer, or one of the three
// sources a result can come from.
const (
	filterAll       = "all"
	filterFavorites = "favorites"
	filterBookmarks = "bookmarks"
	filterHistory   = "history"
	filterTabs      = "tabs"
	filterText      = "text"
	filterImage     = "image"
	filterLink      = "link"
	filterFiles     = "files"
)

// BrowserFilterAxis is the browser mode's range filter.
var BrowserFilterAxis = FilterAxis{
	Values: []string{filterAll, filterBookmarks, filterHistory, filterTabs},
	Label: func(copy i18n.Launcher, value string) string {
		switch value {
		case filterBookmarks:
			return copy.BrowserBookmarks
		case filterHistory:
			return copy.BrowserHistory
		case filterTabs:
			return copy.BrowserTabs
		default:
			return copy.BrowserAll
		}
	},
}

// ClipboardFilterAxis is the clipboard mode's six chips: the mode's own two
// words, then the four kinds a clipboard entry can be.
var ClipboardFilterAxis = FilterAxis{
	Values: []string{filterAll, filterFavorites, filterText, filterImage, filterLink, filterFiles},
	Label: func(copy i18n.Launcher, value string) string {
		switch value {
		case filterFavorites:
			return copy.ClipboardFilterFavorites
		case filterText:
			return copy.ClipboardTypeText
		case filterImage:
			return copy.ClipboardTypeImage
		case filterLink:
			return copy.ClipboardTypeLink
		case filterFiles:
			return copy.ClipboardTypeFiles
		default:
			return copy.ClipboardFilterAll
		}
	},
}

// CalculatorFilterAxis is the calculator's two chips.
var CalculatorFilterAxis = FilterAxis{
	Values: []string{filterAll, filterFavorites},
	Label: func(copy i18n.Launcher, value string) string {
		if value == filterFavorites {
			return copy.CalculatorFilterFavorites
		}
		return copy.CalculatorFilterAll
	},
}

// filterAxisFor is the active mode's axis and its current value, or ok false
// when no mode draws chips (the ordinary search page, an external command, or
// a mode whose list is not on screen).
func (a *App) filterAxisFor() (FilterAxis, string, bool) {
	switch {
	case a.browser:
		return BrowserFilterAxis, a.browserFilter, true
	case a.clipboard:
		return ClipboardFilterAxis, a.clipboardFilter, true
	case a.calculatorMode:
		return CalculatorFilterAxis, a.calculatorFilter, true
	}
	return FilterAxis{}, "", false
}

// filtersVisible reports whether the chips row is on screen. It is the plugin
// *list's* own filter, so it follows the list: no row while the output view
// holds the field (the list is not there to filter).
func (a *App) filtersVisible() bool {
	if a.output != nil {
		return false
	}
	_, _, ok := a.filterAxisFor()
	return ok
}

// cycleFilter moves the active mode's filter one step, as Tab does.
func (a *App) cycleFilter(direction int) {
	switch {
	case a.browser:
		a.browserFilter = BrowserFilterAxis.Cycle(a.browserFilter, direction)
	case a.clipboard:
		a.clipboardFilter = ClipboardFilterAxis.Cycle(a.clipboardFilter, direction)
	case a.calculatorMode:
		a.calculatorFilter = CalculatorFilterAxis.Cycle(a.calculatorFilter, direction)
	}
}

// filtersHeight is the chips row's own band under the field, in theme units:
// the chips' padding pair plus a line box. Zero when no row is drawn.
func (a *App) filtersHeight(c *ui.Context) float32 {
	if !a.filtersVisible() {
		return 0
	}
	return c.Theme().Space(5)
}

// filtersRow draws the chips row the plugin list modes carry under the field,
// floating there as the field does: the mode's filter axis with the chosen
// chip lit. Clicking a chip chooses it.
func (a *App) filtersRow(c *ui.Context, top float32) {
	if !a.filtersVisible() {
		return
	}
	axis, current, ok := a.filterAxisFor()
	if !ok {
		return
	}
	copy := a.copy()
	t := c.Theme()
	ui.Row(c).Absolute().Top(top).Left(0).Right(0).Height(a.filtersHeight(c)).
		Padding(0, t.Space(2)).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
		for _, value := range axis.Values {
			value := value
			chip := ui.Row(c).Focusable().Shrink(0).
				Padding(t.Space(0.75), t.Space(2)).Radius(t.Radius).
				Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
			if value == current {
				chip.Background(t.Accent.Alpha(0.14))
			} else if chip.Hovered() {
				chip.Background(t.SurfaceHover)
			}
			label := axis.LabelOf(copy, value)
			color := t.TextMuted
			if value == current {
				color = t.Text
			}
			chip.Children(func() { ui.Text(c, label).FontSize(t.FontSize - 1).TextColor(color) })
			if chip.Clicked() {
				a.setFilter(value)
			}
		}
	})
}

// setFilter chooses one value of the active mode's axis.
func (a *App) setFilter(value string) {
	switch {
	case a.browser:
		a.browserFilter = value
	case a.clipboard:
		a.clipboardFilter = value
	case a.calculatorMode:
		a.calculatorFilter = value
	}
}

// clipboardFilterAllows reports whether an entry passes the clipboard mode's
// chosen chip: the two mode words (all, favorites) and the four kinds a
// clipboard entry can be. A link is a text entry whose whole value is a page
// address — the old build's own classification, so the chip, the row's type
// word and its icon always agree.
func (a *App) clipboardFilterAllows(entry clipboardpkg.Entry) bool {
	switch a.clipboardFilter {
	case filterFavorites:
		return entry.Favorite
	case filterText:
		return clipboardEntryKind(entry) == filterText
	case filterImage:
		return entry.Kind == clipboardpkg.KindImage
	case filterLink:
		return clipboardEntryKind(entry) == filterLink
	case filterFiles:
		return entry.Kind == clipboardpkg.KindFiles
	default:
		return true
	}
}

// clipboardEntryKind is an entry's chip kind: image and files are the stored
// kinds, and text splits into link (a page address) and text.
func clipboardEntryKind(entry clipboardpkg.Entry) string {
	switch entry.Kind {
	case clipboardpkg.KindImage:
		return filterImage
	case clipboardpkg.KindFiles:
		return filterFiles
	}
	if isPageAddress(entry.Text) {
		return filterLink
	}
	return filterText
}

// isPageAddress reports whether text is a single page address.
func isPageAddress(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t\n") {
		return false
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// FiltersVisible reports whether a plugin mode's chips row is on screen, for
// the shell's window sizing.
func (a *App) FiltersVisible() bool { return a.filtersVisible() }
