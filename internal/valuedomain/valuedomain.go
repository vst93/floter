// Package valuedomain is the one list of ids for every setting floter stores
// as a name.
//
// A setting like `theme` or `terminal_theme` is a choice from a closed set: the
// loader has to know which ids a file may hold, the picker has to know which
// rows to offer, and the renderer has to know which ones it can paint. Before
// P11-w those three answers lived in five packages — `internal/i18n` spelled
// the picker's tables out as `[]Option` literals in both languages,
// `internal/settings` kept four slices and a factor map, `internal/terminalui`
// and `internal/glassmap` keyed their colour/effect tables by the same ids, and
// `internal/shell` plus `internal/settingsui` each re-answered "is this the
// light app icon" with `== "light"`.
//
// They agree today. The change that makes them disagree is adding one id — a
// ninth palette, a fifth glass stop — and then the picker offers a row the
// loader throws away, or stores an id the renderer silently falls back on
// (`terminalui.themeFor` treats an unknown palette as `inherit`, so a missing
// colour spec is not an error, it is the wrong colour). One list, read by
// every face, is the shape that cannot diverge.
//
// The order of each list is the order the picker shows, because that is the
// only order anyone can observe: membership checks never look at it. The
// tables that carry *values* for these ids (a palette's colours, a glass
// stop's blur) stay with the package that paints them, and each is pinned to
// this list by a test rather than by a second copy of the ids.
package valuedomain

// The shipped id domains, in picker order.
var (
	// AppIcons are the two menu-bar/window icons the app ships; dark is the
	// default, and NormalizeAppIcon is the only thing that decides.
	AppIcons = []string{"dark", "light"}

	// Themes are the window appearances; `auto` follows the desktop.
	Themes = []string{"auto", "light", "dark"}

	// GlassSteps are the material stops; `off` is a real stored value that
	// removes the material, not the absence of one.
	GlassSteps = []string{"off", "frosted", "regular", "liquid"}

	// Scales are the interface sizes.
	Scales = []string{"tiny", "small", "default", "large"}

	// CursorShapes are the terminal's caret shapes.
	CursorShapes = []string{"beam", "block", "underline"}

	// Paddings are the terminal's inset steps.
	Paddings = []string{"compact", "regular", "relaxed"}

	// Palettes are the terminal colour overrides; `inherit` follows the app.
	Palettes = []string{"inherit", "contrast", "paper", "ink", "fog", "forest", "dusk", "mist", "amber"}

	// BoldModes are the two ways a bold run can be drawn. It has no picker,
	// so it has no label table either — the id list is still the answer to
	// "what may this file hold".
	BoldModes = []string{"font", "bright"}
)

// The browser plugin's three vocabularies. They are stored inside the
// `browser_plugin` block rather than as top-level fields, but a stored value
// is the same problem: the picker offers an id, the loader has to accept
// exactly those, and the browser package has to know what each one means.
const (
	// BrowserSortRelevance is the launcher's own ranking — the order the
	// merge produced. It is the default and the browser's zero value.
	BrowserSortRelevance    = "relevance"
	BrowserSortRecent       = "recent"
	BrowserSortAlphabetical = "alphabetical"
	BrowserSortVisits       = "visits"
)

// BrowserSortOrders are how browser results are ordered, in picker order.
var BrowserSortOrders = []string{
	BrowserSortRelevance,
	BrowserSortRecent,
	BrowserSortAlphabetical,
	BrowserSortVisits,
}

const (
	// BrowserFieldAll matches the title and the address.
	BrowserFieldAll   = "all"
	BrowserFieldTitle = "title"
	BrowserFieldURL   = "url"
)

// BrowserSearchFields are what a browser query is matched against, in picker
// order.
var BrowserSearchFields = []string{BrowserFieldAll, BrowserFieldTitle, BrowserFieldURL}

// The calculator plugin's copy modes: what Enter copies from a history row.
const (
	CalculatorCopyFull   = "full"
	CalculatorCopyResult = "result"
)

// CalculatorCopyModes are the copy modes, in picker order.
var CalculatorCopyModes = []string{CalculatorCopyFull, CalculatorCopyResult}

// ScaleFactors is how big each interface size is. It is the definition of the
// step, not a settings detail, so it sits beside the ids it names; a step
// without a factor would silently scale the interface to nothing.
var ScaleFactors = map[string]float64{
	"tiny":    0.8,
	"small":   0.9,
	"default": 1.0,
	"large":   1.1,
}

// DefaultAppIcon is the icon a stored value lands on unless it says otherwise.
const DefaultAppIcon = "dark"

// Has reports whether id is one of the allowed ids. Only an exact match
// counts: these ids are stored strings, and "Dark" is not "dark".
func Has(ids []string, id string) bool {
	for _, candidate := range ids {
		if id == candidate {
			return true
		}
	}
	return false
}

// NormalizeAppIcon maps any stored app-icon value onto the two the app ships.
// The icon is an `Extra` key, so nothing in `internal/settings` normalizes it
// on load; this is that normalization, and both the tray (which picks the
// bytes) and the settings picker (which picks the row) read it here.
func NormalizeAppIcon(name string) string {
	if Has(AppIcons, name) {
		return name
	}
	return DefaultAppIcon
}
