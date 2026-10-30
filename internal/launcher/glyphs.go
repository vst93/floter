package launcher

// The row glyphs: the small marks that sit on a result row's icon plate. They
// are the old build's own vocabulary (lucide's shapes), kept as a closed set
// here so a row cannot invent a glyph the rest of the list does not speak.
//
// Each is a stroked 24×24 path drawn at the plate's glyph size, so it reads at
// 28 DIPs and stays crisp at every interface step.

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/browser"
	clipboardpkg "floter/internal/clipboard"
	"floter/internal/settings"
)

// glyphSource is one icon's own SVG, as lucide draws it: a 24×24 viewBox,
// stroked outlines, no fill.
func glyphSource(body string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`
}

// The glyph set, parsed once: a parse failure would be a programming error, so
// the sources are trusted at the boundary that owns them.
var (
	glyphTerminal = ui.MustParseSVG([]byte(glyphSource(`<polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/>`)))
	glyphClock    = ui.MustParseSVG([]byte(glyphSource(`<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>`)))
	glyphFile     = ui.MustParseSVG([]byte(glyphSource(`<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/>`)))
	glyphFolder   = ui.MustParseSVG([]byte(glyphSource(`<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>`)))
	glyphGlobe    = ui.MustParseSVG([]byte(glyphSource(`<circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/>`)))
	glyphBookmark = ui.MustParseSVG([]byte(glyphSource(`<path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/>`)))
	glyphStar     = ui.MustParseSVG([]byte(glyphSource(`<path d="M11.5 3.1a.5.5 0 0 1 1 0l2.2 4.4 4.9.7a.5.5 0 0 1 .3.9l-3.6 3.4.9 4.9a.5.5 0 0 1-.8.5L12 15.6l-4.4 2.3a.5.5 0 0 1-.8-.5l.9-4.9L4.1 9.1a.5.5 0 0 1 .3-.9l4.9-.7z"/>`)))
	// glyphStarFilled is the same lucide Star with its interior painted, the
	// reference's favourited face (`LauncherResults.tsx:987-991` fills the path
	// with `currentColor` when the entry is starred, `none` otherwise). It is a
	// second source rather than a property toggle because the framework's icons
	// carry no runtime fill switch — `ui.Icon` paints whatever the SVG says, in
	// the text colour.
	glyphStarFilled = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="16" height="16" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M11.5 3.1a.5.5 0 0 1 1 0l2.2 4.4 4.9.7a.5.5 0 0 1 .3.9l-3.6 3.4.9 4.9a.5.5 0 0 1-.8.5L12 15.6l-4.4 2.3a.5.5 0 0 1-.8-.5l.9-4.9L4.1 9.1a.5.5 0 0 1 .3-.9l4.9-.7z"/></svg>`))
	glyphImage      = ui.MustParseSVG([]byte(glyphSource(`<rect width="18" height="18" x="3" y="3" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.1-3.1a2 2 0 0 0-2.8 0L6 21"/>`)))
	glyphLink       = ui.MustParseSVG([]byte(glyphSource(`<path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7"/><path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7"/>`)))
	glyphText       = ui.MustParseSVG([]byte(glyphSource(`<path d="M4 7V4h16v3"/><path d="M9 20h6"/><path d="M12 4v16"/>`)))
	glyphCalculator = ui.MustParseSVG([]byte(glyphSource(`<rect width="16" height="20" x="4" y="2" rx="2"/><line x1="8" x2="16" y1="6" y2="6"/><line x1="8" x2="8" y1="14" y2="14"/><line x1="12" x2="12" y1="14" y2="14"/><line x1="16" x2="16" y1="14" y2="14"/><line x1="8" x2="8" y1="18" y2="18"/><line x1="12" x2="12" y1="18" y2="18"/><line x1="16" x2="16" y1="18" y2="18"/>`)))
	glyphSettings   = ui.MustParseSVG([]byte(glyphSource(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.9.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.9 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.9.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.9V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>`)))
	glyphPower      = ui.MustParseSVG([]byte(glyphSource(`<path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.8 0"/>`)))
	glyphRestart    = ui.MustParseSVG([]byte(glyphSource(`<path d="M21 12a9 9 0 1 1-2.6-6.4"/><path d="M21 3v6h-6"/>`)))
	glyphAlert      = ui.MustParseSVG([]byte(glyphSource(`<circle cx="12" cy="12" r="10"/><line x1="12" x2="12" y1="8" y2="12"/><line x1="12" x2="12.01" y1="16" y2="16"/>`)))
	// glyphInfo is lucide's Info, the mark the reference puts in a status
	// line's icon column (`.launcher-status__icon` drew `<InfoIcon size={14}/>`,
	// `LauncherResults.tsx:752-754`).
	glyphInfo   = ui.MustParseSVG([]byte(glyphSource(`<circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/>`)))
	glyphWindow = ui.MustParseSVG([]byte(glyphSource(`<rect width="20" height="16" x="2" y="4" rx="2"/><path d="M6 8h.01"/><path d="M10 8h.01"/>`)))
	// glyphSearch is lucide's Search: the mark Raycast leads its search row
	// with, 16px at stroke 2 in the secondary ink (`docs/raycast-design.md`
	// §4.2). The launcher's field drew no mark at all before this slice.
	glyphSearch = ui.MustParseSVG([]byte(glyphSource(`<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>`)))
	glyphClose  = ui.MustParseSVG([]byte(glyphSource(`<path d="M18 6 6 18"/><path d="m6 6 12 12"/>`)))
	// glyphChevronRight and glyphChevronDown are lucide's two disclosure marks,
	// the pair the reference's run-output block leads and closes with
	// (`ExtensionRow.tsx:516-527` opens the folded affordance with a 12px
	// `ChevronRight`, `:483-497` closes the open block with a 14px
	// `ChevronDown`).
	glyphChevronRight = ui.MustParseSVG([]byte(glyphSource(`<path d="m9 18 6-6-6-6"/>`)))
	glyphChevronDown  = ui.MustParseSVG([]byte(glyphSource(`<path d="m6 9 6 6 6-6"/>`)))
	// glyphPin is lucide's Pin, the reference's detach button for external
	// commands (R84 — `.collapsed-card__settings` with a `<Pin size={16}/>`).
	// The pin puts the plugin page's command + current argv into its own
	// window that never follows the launcher's summon/hide.
	glyphPin = ui.MustParseSVG([]byte(glyphSource(`<path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1z"/>`)))
	// The toast marks: the old build's own kind icons (its `ToastStack` drew
	// lucide's AlertCircle / TriangleAlert / Check at 15px). They belong to the
	// same closed set the settings sidebar reads, so the shell's toast viewport
	// leads with the app's vocabulary rather than its own paths. `alert` is
	// lucide's AlertCircle and already in the set; these two are the other ends
	// of the same trio.
	glyphTriangleAlert = ui.MustParseSVG([]byte(glyphSource(`<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 20h16a2 2 0 0 0 1.73-2Z"/><path d="M12 9v4"/><path d="M12 17h.01"/>`)))
	glyphCheck         = ui.MustParseSVG([]byte(glyphSource(`<path d="M20 6 9 17l-5-5"/>`)))
	// glyphCopy is lucide's Copy, the mark the reference's deep-link copy
	// action drew at 13px (`DeepLinkRow.tsx:47-58`). The clipboard mark above
	// is a different shape: it has a clip and a board, where this one is the
	// two stacked pages the copy action itself uses.
	glyphCopy      = ui.MustParseSVG([]byte(glyphSource(`<rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/>`)))
	glyphClipboard = ui.MustParseSVG([]byte(glyphSource(`<rect width="8" height="4" x="8" y="2" rx="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>`)))
	// glyphSquareTerminal is lucide's SquareTerminal, the mark the reference's
	// Sessions page used both for a session row and for the resting empty state
	// (`SessionsPage.tsx:151,186`): the terminal idea inside its own rounded
	// square, where `glyphTerminal` above is the bare prompt.
	glyphSquareTerminal = ui.MustParseSVG([]byte(glyphSource(`<path d="m7 11 2-2-2-2"/><path d="M11 13h4"/><rect width="18" height="18" x="3" y="3" rx="2"/>`)))
	// glyphRefresh is lucide's RefreshCw, the mark the reference drew while the
	// sessions list was being fetched (`SessionsPage.tsx:148`). Its two arrows
	// are a different shape from `glyphRestart`'s single loop.
	glyphRefresh = ui.MustParseSVG([]byte(glyphSource(`<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>`)))
	// glyphLoaderCircle is lucide's LoaderCircle (lucide-react 1.31.0's own
	// path), the reference's in-flight mark: the broken ring an extension row's
	// repair action swaps its Wrench / RefreshCw for while the work runs
	// (`src/extensions/ExtensionRow.tsx:305,325,364`).
	glyphLoaderCircle = ui.MustParseSVG([]byte(glyphSource(`<path d="M21 12a9 9 0 1 1-6.219-8.56"/>`)))
	// glyphPackage is lucide's Package (lucide-react 1.31.0's own path), the
	// mark the reference's installed-integrations list drew when it was empty
	// (`ExtensionsPanel.tsx:2462`, `<Package size={20} strokeWidth={2}/>`).
	glyphPackage = ui.MustParseSVG([]byte(glyphSource(`<path d="M11 21.73a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73z"/><path d="M12 22V12"/><polyline points="3.29 7 12 12 20.71 7"/><path d="m7.5 4.27 9 5.15"/>`)))
	// glyphKeyboard is lucide's Keyboard (lucide-react 1.31.0's own path), the
	// 22px mark the reference's custom-shortcuts empty state drew
	// (`ShortcutsPage.tsx:241`, `<Keyboard size={22} strokeWidth={1.6}/>`).
	glyphKeyboard = ui.MustParseSVG([]byte(glyphSource(`<rect width="20" height="16" x="2" y="4" rx="2"/><path d="M6 8h.01"/><path d="M10 8h.01"/><path d="M14 8h.01"/><path d="M18 8h.01"/><path d="M8 12h.01"/><path d="M12 12h.01"/><path d="M16 12h.01"/><path d="M7 16h10"/>`)))
	// glyphPlus is lucide's Plus, the mark the reference's text action led
	// with as a `<span class="settings-reset__icon">＋</span>` glyph
	// (`ShortcutsPage.tsx:230-236,247-252`).
	glyphPlus = ui.MustParseSVG([]byte(glyphSource(`<path d="M5 12h14"/><path d="M12 5v14"/>`)))
	// glyphArrowUpDown is lucide's ArrowUpDown, the mark the reference's
	// Sessions sort control led with (`SessionsPage.tsx:123-125`,
	// `<ArrowUpDown size={13} strokeWidth={1.8}/>`): an up arrow beside a down
	// one, the two directions the control flips between.
	glyphArrowUpDown = ui.MustParseSVG([]byte(glyphSource(`<path d="m21 16-4 4-4-4"/><path d="M17 20V4"/><path d="m3 8 4-4 4 4"/><path d="M7 4v16"/>`)))
	// glyphSlidersHorizontal is lucide's SlidersHorizontal (lucide-react
	// 1.31.0's own path), the mark the reference's General settings page led
	// with (`App.tsx:2904`, `["general", SlidersHorizontal]`): three tracks
	// with an offset knob on each.
	glyphSlidersHorizontal = ui.MustParseSVG([]byte(glyphSource(`<path d="M10 5H3"/><path d="M12 19H3"/><path d="M14 3v4"/><path d="M16 17v4"/><path d="M21 12h-9"/><path d="M21 19h-5"/><path d="M21 5h-7"/><path d="M8 10v4"/><path d="M8 12H3"/>`)))
	// glyphBlocks is lucide's Blocks (lucide-react 1.31.0's own path), the
	// mark the reference's Integrations settings page led with (`App.tsx:2907`,
	// `["integrations", Blocks]`): one block stacked against two others, the
	// pieces a plugin platform is built from.
	glyphBlocks = ui.MustParseSVG([]byte(glyphSource(`<path d="M10 22V7a1 1 0 0 0-1-1H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-5a1 1 0 0 0-1-1H2"/><rect width="8" height="8" x="14" y="2" rx="1"/>`)))
	// glyphPlay is lucide's Play, the mark the reference's Sessions page put on
	// its per-row resume button (`SessionsPage.tsx:211-213`,
	// `<Play size={14} strokeWidth={1.9}/>`): one triangle pointing right, the
	// word-picture for "bring this session back on screen". Its `stroke-width`
	// is the reference's own 1.9, kept as the source's literal.
	glyphPlay = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"><polygon points="6 3 20 12 6 21 6 3"/></svg>`))
	// glyphTrash is lucide's Trash2, the mark the reference's removal
	// confirmation bar led its danger button with (`RemovalConfirmation.tsx:47`,
	// `<Trash2 size={14} />`): a bin with its lid and two vertical lines, the
	// word-picture for deletion.
	glyphTrash = ui.MustParseSVG([]byte(glyphSource(`<path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><line x1="10" x2="10" y1="11" y2="17"/><line x1="14" x2="14" y1="11" y2="17"/>`)))
	// glyphUnplug is lucide's Unplug, the mark the reference's removal bar
	// used for a system integration (`RemovalConfirmation.tsx:47`,
	// `<Unplug size={14} />`): two plugs pulled apart, the word-picture for
	// disconnecting from the OS.
	glyphUnplug = ui.MustParseSVG([]byte(glyphSource(`<path d="m19 5 3-3"/><path d="m2 22 3-3"/><path d="M6.3 20.3a2.4 2.4 0 0 0 3.4 0L12 18l-6-6-2.3 2.3a2.4 2.4 0 0 0 0 3.4Z"/><path d="m2 6 6 6 2.3-2.3a2.4 2.4 0 0 0 0-3.4L8.7 4a2.4 2.4 0 0 0-3.4 0Z"/><path d="m12 12 6 6 2.3-2.3a2.4 2.4 0 0 0 0-3.4L18.7 10a2.4 2.4 0 0 0-3.4 0Z"/>`)))
	// glyphFileDown is lucide's FileDown, the mark the reference's config
	// toolbar led its export icon-button with (`ExtensionsPanel.tsx:2995`,
	// `<FileDown size={14} strokeWidth={2}/>`): a page with its downward arrow,
	// the word-picture for "write this to a file". A mark of its own because
	// the copy button beside it is `Copy`; the two icon-buttons must not
	// speak the same shape.
	glyphFileDown = ui.MustParseSVG([]byte(glyphSource(`<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M12 18v-6"/><path d="m9 15 3 3 3-3"/>`)))
	// glyphRotateCcw is lucide's RotateCcw, the mark the reference's config
	// form's "Restore defaults" pill led with (`ExtensionsPanel.tsx:3017`,
	// `<RotateCcw size={14} strokeWidth={2}/>`): a counter-clockwise turn,
	// the word-picture for "put it back the way it was". It is a different
	// shape from `glyphRestart`'s single-loop arrow.
	glyphRotateCcw = ui.MustParseSVG([]byte(glyphSource(`<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/>`)))
)

// rowGlyph picks the mark for a row from what the row is: an application shows
// its own icon (no glyph), and everything else shows the one the old build's
// `launcher-result__icon` slot used.
func rowGlyph(item Item) (*ui.SVG, bool) {
	switch {
	case item.thumb != nil, item.icon != nil:
		return nil, false // an image, not a glyph
	case item.clip != nil:
		switch clipboardKindOf(item.clip) {
		case filterImage:
			return glyphImage, true
		case filterFiles:
			return glyphFile, true
		case filterLink:
			return glyphLink, true
		default:
			return glyphText, true
		}
	case item.web != nil:
		// The reference's `BrowserRowIcon` (LauncherResults.tsx:290-294): a
		// bookmark leads with the bookmark mark, a history entry with the
		// clock (a page *at a time*), and only a row from a plugin that
		// predates this field falls through to the shared globe.
		switch item.web.Kind {
		case browser.KindBookmark:
			return glyphBookmark, true
		case browser.KindHistory:
			return glyphClock, true
		}
		return glyphGlobe, true
	case item.tab != nil:
		return glyphWindow, true
	case item.calc != nil:
		return glyphCalculator, true
	case item.entry != nil:
		return glyphTerminal, true
	}
	// The files page's three actions are the path action bar's three actions, so
	// they wear the same three marks (see the `bar:path-*` cases below). Their
	// ids carry the dropped path after the action (`drop:open:/x/y`, `files.go`),
	// so they are read by prefix rather than by the equality switch. Before P13-f
	// none of them matched anything and every row fell through to the tail — the
	// recents' clock: a page of five dropped files led with fifteen history marks,
	// the one glyph none of those rows is.
	switch {
	case strings.HasPrefix(item.ID, "drop:open:"):
		return glyphFolder, true // `bar:path-open`: hand it to the system's opener
	case strings.HasPrefix(item.ID, "drop:cd:"):
		return glyphTerminal, true // `bar:path-cd`: a terminal in its directory
	case strings.HasPrefix(item.ID, "drop:copy:"):
		return glyphFile, true // `bar:path-copy`: its own path, onto the clipboard
	case item.ID == "drop:more":
		// The tail row that collapses the rest of the drop *reveals* rows more
		// (`files.go`'s `Run` only sets `dropsExpanded`): a disclosure mark, not
		// a launch.
		return glyphChevronRight, true
	}
	switch item.ID {
	case "bar:url":
		return glyphGlobe, true
	case "bar:shell", "bar:path-cd":
		return glyphTerminal, true
	case "bar:path-open":
		return glyphFolder, true
	case "bar:path-copy":
		return glyphFile, true
		// The held modifier's own row is the old build's bare-terminal row: it carries
		// the same action as the terminal system row, so it leads with the terminal
		// glyph (`.launcher-result__icon--system` → `SystemActionIcon`, see R64). The
		// id falls through to the recents' clock below without this case, which is
		// exactly the icon the reference did not draw.
	case "system-terminal-bare":
		return glyphTerminal, true
	// The built-in command rows. Their ids carry no `cmd:` prefix (see
	// `commands()` in search.go): the port's glyph switch once tested
	// `cmd:settings` and friends, which matched nothing, so every built-in
	// row fell through to the recents' clock — the icon the reference did
	// not draw. The marks follow the reference's own `SystemActionIcon` /
	// command-icon split: terminal → the chevron, browser → the globe,
	// clipboard → the type mark, calculator → the calculator, the two power
	// actions → the restart arrow and the power mark. The list is the
	// reference's six rows — settings and quit are tray/menu rows and never
	// reach here (`commands()` in search.go).
	case "terminal":
		return glyphTerminal, true
	case "browser":
		return glyphGlobe, true
	case "clipboard":
		return glyphClipboard, true
	case "calculator":
		return glyphCalculator, true
	case "power-restart":
		return glyphRestart, true
	case "power-shutdown":
		return glyphPower, true
	}
	if strings.HasPrefix(item.ID, "tool:") || item.alias != "" {
		return glyphTerminal, true
	}
	return glyphClock, true
}

// clipboardKindOf is the chip kind for the clipboard entry a row shows.
func clipboardKindOf(entry *clipboardpkg.Entry) string {
	if entry == nil {
		return filterText
	}
	return clipboardEntryKind(*entry)
}

// iconPlate is the 28u cell every row leads with: the resting control fill, a
// small radius and a hairline shadow. A power action's plate is warm rather
// than the neutral fill — the one launcher entry that cannot be undone by
// closing a window, as the old build's `--system-icon-surface` said
// (`.launcher-result__icon--system`). The warm face is the *system-icon plane*
// token, not the warning ink at a low alpha: the port had quoted
// `t.Warning.Alpha(0.14)`, which paints the label's own hue as the plate and
// is the wrong colour on both appearances.
func (a *App) iconPlate(c *ui.Context, warm bool) ui.Element {
	tokens := a.tokens(c)
	t := c.Theme()
	plate := ui.Box(c).Size(t.Space(7), t.Space(7)).Radius(tokens.RadiusXS).
		AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
	if warm {
		plate.Background(tokens.IconSurface)
	} else {
		plate.Background(tokens.Control)
	}
	// Raycast's plate is flat: the control lens is capped at 0.04 by the
	// palette (`docs/raycast-design.md` §4.1), so the stack is still the stop's
	// own (`--glass-lens-scale` grades it) but no longer reads as a lit pane of
	// glass.
	return tokens.Lens(plate)
}

// enterBadge is the key the action bar's row runs on: the select-result
// binding's modifiers with Enter, formatted as a key badge — `⌘↩` on macOS,
// `Ctrl+↩` elsewhere, exactly as the old build's action bar printed it.
func (a *App) enterBadge() string {
	return a.badge(a.shortcut(settings.ShortcutSelectResult), "enter")
}

// outputGlyph maps a list-protocol row's icon name onto the glyph set. The
// vocabulary is closed (the protocol's own `RowIcons`), so a name outside it
// draws no mark rather than a guessed one.
func outputGlyph(name string) (*ui.SVG, bool) {
	switch name {
	case "link":
		return glyphLink, true
	case "file":
		return glyphFile, true
	case "folder":
		return glyphFolder, true
	case "globe":
		return glyphGlobe, true
	case "star":
		return glyphStar, true
	case "clock":
		return glyphClock, true
	case "text":
		return glyphText, true
	case "image":
		return glyphImage, true
	case "command":
		return glyphTerminal, true
	}
	return nil, false
}

// Glyph returns one of the shared marks by name, for the surfaces outside the
// launcher that lead with the same vocabulary (the settings sidebar, the
// integration rows). The set is the module's own, so a caller cannot invent a
// mark the rest of the app does not speak.
func Glyph(name string) (*ui.SVG, bool) {
	switch name {
	case "terminal":
		return glyphTerminal, true
	case "settings":
		return glyphSettings, true
	case "globe":
		return glyphGlobe, true
	case "text":
		return glyphText, true
	case "calculator":
		return glyphCalculator, true
	case "window":
		return glyphWindow, true
	case "close":
		return glyphClose, true
	case "chevron-right":
		return glyphChevronRight, true
	case "chevron-down":
		return glyphChevronDown, true
	case "pin":
		return glyphPin, true
	case "clipboard":
		return glyphClipboard, true
	case "clock":
		return glyphClock, true
	case "star":
		return glyphStar, true
	case "star-filled":
		return glyphStarFilled, true
	case "power":
		return glyphPower, true
	case "alert":
		return glyphAlert, true
	case "info":
		return glyphInfo, true
	case "sliders-horizontal":
		return glyphSlidersHorizontal, true
	case "blocks":
		return glyphBlocks, true
	case "triangle-alert":
		return glyphTriangleAlert, true
	case "check":
		return glyphCheck, true
	case "copy":
		return glyphCopy, true
	case "square-terminal":
		return glyphSquareTerminal, true
	case "refresh":
		return glyphRefresh, true
	case "loader-circle":
		return glyphLoaderCircle, true
	case "package":
		return glyphPackage, true
	case "keyboard":
		return glyphKeyboard, true
	case "plus":
		return glyphPlus, true
	case "trash":
		return glyphTrash, true
	case "play":
		return glyphPlay, true
	case "unplug":
		return glyphUnplug, true
	case "file-down":
		return glyphFileDown, true
	case "rotate-ccw":
		return glyphRotateCcw, true
	case "arrow-up-down":
		return glyphArrowUpDown, true
	case "file":
		return glyphFile, true
	case "folder":
		return glyphFolder, true
	case "link":
		return glyphLink, true
	case "image":
		return glyphImage, true
	case "bookmark":
		return glyphBookmark, true
	case "search":
		return glyphSearch, true
	}
	return nil, false
}

// iconCell draws an application's own artwork (or a clipboard thumbnail) in
// the row's icon cell: 23u of the plate's 28u, scaled past the cell and
// cropped by it.
//
// The oversize is the old build's own compensation: macOS `.icns` artwork
// carries a margin of its own, and at 23u inside a 28u plate the mark read as
// 「很单薄」 beside the plated glyphs. `transform: scale(1.42)` with
// `object-fit: cover` pulled that margin out and filled the cell; a clipped box
// around an oversized image is the same picture here.
func iconCell(c *ui.Context, t *ui.Theme, bitmap *ui.Bitmap) {
	side := t.Space(5.75)
	cell := ui.Box(c).Size(side, side).Radius(t.Space(0.75)).Clip().
		AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
	cell.Children(func() {
		ui.Image(c, bitmap).Size(side*1.42, side*1.42)
	})
}
