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

	clipboardpkg "floter/internal/clipboard"
	"floter/internal/theme"
)

// glyphSource is one icon's own SVG, as lucide draws it: a 24×24 viewBox,
// stroked outlines, no fill.
func glyphSource(body string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`
}

// The glyph set, parsed once: a parse failure would be a programming error, so
// the sources are trusted at the boundary that owns them.
var (
	glyphTerminal   = ui.MustParseSVG([]byte(glyphSource(`<polyline points="4 17 10 11 4 5"/><line x1="12" x2="20" y1="19" y2="19"/>`)))
	glyphClock      = ui.MustParseSVG([]byte(glyphSource(`<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>`)))
	glyphFile       = ui.MustParseSVG([]byte(glyphSource(`<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/>`)))
	glyphFolder     = ui.MustParseSVG([]byte(glyphSource(`<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>`)))
	glyphGlobe      = ui.MustParseSVG([]byte(glyphSource(`<circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/>`)))
	glyphBookmark   = ui.MustParseSVG([]byte(glyphSource(`<path d="m19 21-7-4-7 4V5a2 2 0 0 1 2-2h10a2 2 0 0 1 2 2v16z"/>`)))
	glyphStar       = ui.MustParseSVG([]byte(glyphSource(`<path d="M11.5 3.1a.5.5 0 0 1 1 0l2.2 4.4 4.9.7a.5.5 0 0 1 .3.9l-3.6 3.4.9 4.9a.5.5 0 0 1-.8.5L12 15.6l-4.4 2.3a.5.5 0 0 1-.8-.5l.9-4.9L4.1 9.1a.5.5 0 0 1 .3-.9l4.9-.7z"/>`)))
	glyphImage      = ui.MustParseSVG([]byte(glyphSource(`<rect width="18" height="18" x="3" y="3" rx="2"/><circle cx="9" cy="9" r="2"/><path d="m21 15-3.1-3.1a2 2 0 0 0-2.8 0L6 21"/>`)))
	glyphLink       = ui.MustParseSVG([]byte(glyphSource(`<path d="M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7"/><path d="M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7"/>`)))
	glyphText       = ui.MustParseSVG([]byte(glyphSource(`<path d="M4 7V4h16v3"/><path d="M9 20h6"/><path d="M12 4v16"/>`)))
	glyphCalculator = ui.MustParseSVG([]byte(glyphSource(`<rect width="16" height="20" x="4" y="2" rx="2"/><line x1="8" x2="16" y1="6" y2="6"/><line x1="8" x2="8" y1="14" y2="14"/><line x1="12" x2="12" y1="14" y2="14"/><line x1="16" x2="16" y1="14" y2="14"/><line x1="8" x2="8" y1="18" y2="18"/><line x1="12" x2="12" y1="18" y2="18"/><line x1="16" x2="16" y1="18" y2="18"/>`)))
	glyphSettings   = ui.MustParseSVG([]byte(glyphSource(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.9.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.9 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.9.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.9V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>`)))
	glyphPower      = ui.MustParseSVG([]byte(glyphSource(`<path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.8 0"/>`)))
	glyphRestart    = ui.MustParseSVG([]byte(glyphSource(`<path d="M21 12a9 9 0 1 1-2.6-6.4"/><path d="M21 3v6h-6"/>`)))
	glyphWindow     = ui.MustParseSVG([]byte(glyphSource(`<rect width="20" height="16" x="2" y="4" rx="2"/><path d="M6 8h.01"/><path d="M10 8h.01"/>`)))
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
		if item.web.Kind == "bookmark" {
			return glyphBookmark, true
		}
		return glyphGlobe, true
	case item.tab != nil:
		return glyphWindow, true
	case item.calc != nil:
		return glyphCalculator, true
	case item.entry != nil:
		return glyphTerminal, true
	}
	switch item.ID {
	case "cmd:terminal":
		return glyphTerminal, true
	case "cmd:settings":
		return glyphSettings, true
	case "cmd:browser":
		return glyphGlobe, true
	case "cmd:clipboard":
		return glyphText, true
	case "cmd:calculator":
		return glyphCalculator, true
	case "cmd:power-restart":
		return glyphRestart, true
	case "cmd:power-shutdown":
		return glyphPower, true
	case "cmd:quit":
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
// closing a window, as the old build's `--system-icon-surface` said.
func (a *App) iconPlate(c *ui.Context, warm bool) ui.Element {
	tokens := theme.For(a.settings(), c.Theme().Dark)
	t := c.Theme()
	plate := ui.Box(c).Size(t.Space(7), t.Space(7)).Radius(tokens.RadiusSM).
		AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
	if warm {
		plate.Background(t.Warning.Alpha(0.14))
	} else {
		plate.Background(tokens.Control)
	}
	return plate
}
