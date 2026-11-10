package settingsui

import (
	"floter/internal/i18n"
	"floter/internal/theme"
	"github.com/egoist/mygo/ui"
)

// Status always has words and a distinct mark as well as semantic color.
func (a *App) integrationStatus(c *ui.Context, copy i18n.Settings, item Integration) {
	t, tokens := a.cardTheme(c), a.tokens(c)
	label, glyph, ink := item.state(copy), "power", tokens.TextSecondary
	// Needs-attention states use amber; damage remains red. The general
	// warning token is deliberately coral, too close to damage for this list.
	attention := ui.Hex("#a85b00")
	if t.Dark {
		attention = ui.Hex("#efb453")
	}
	switch {
	case item.Orphan:
		glyph, ink = "info", attention
	case item.Broken:
		glyph, ink = "alert", t.Danger
	case !item.Enabled:
	case !item.RuntimeAvailable:
		label, glyph, ink = copy.RuntimeUnavailable, "triangle-alert", attention
	default:
		glyph, ink = "check", t.Success
	}
	ink = theme.ReadableOnGlass(ink, t.Background, float32(a.settings().MainOpacity)/100, 4.5)
	ui.Row(c).Gap(t.Space(1)).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, mustGlyph(glyph)).Size(t.Space(3.5), t.Space(3.5)).TextColor(ink)
		labelNode := ui.Text(c, label).FontSize(tokens.Caption).FontWeight(500).TextColor(ink).SingleLine().Ellipsis("…").MaxWidth(t.Space(34))
		tokens.Tip(c, labelNode, label)
	})
}
