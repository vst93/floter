package settingsui

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// The grouped-card language the old settings panel used, and macOS System
// Settings with it: a section title with a one-line explanation under it,
// then a flat card of rows — a label (and a grey second line) at the start,
// the control at the end, a rule inset to the label's own column between
// one row and the next. The card is a *plane*, never glass: the material
// belongs to the window shell, and a card that blurred would be a second
// material inside the one sheet the app allows.
//
// Four pieces, no third-party vocabulary: a section (title + hint + cards),
// a card, a row, and the action — the right-aligned accent text the old
// build used for "Edit…" and "Restore defaults". Every page keeps owning
// its own state, handlers and stored keys; these are presentation only.

// tokens resolves the card language's colors for the stored appearance,
// scaled with the interface step exactly as the CSS variables were.
func (a *App) tokens(c *ui.Context) theme.Tokens {
	return theme.For(a.Store.Snapshot(), c.Theme().Dark)
}

// section draws a titled group: the title, its one-line explanation, and
// the cards fn builds under them.
func (a *App) section(c *ui.Context, title, hint string, fn func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(t.Space(1.5)).Children(func() {
		ui.Text(c, title).FontSize(t.FontSize + 1).FontWeight(650).TextColor(a.tokens(c).TextStrong)
		if hint != "" {
			ui.Text(c, hint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
		ui.Column(c).FillWidth().Gap(t.Space(2.5)).Children(fn)
	})
}

// card draws one grouped card: the resting control plane, its hairline
// edge, the card radius, and the clip that keeps the first and last rows
// inside the rounded corners. It is the row counter's own frame: the rows
// fn builds draw the rule *above* themselves except the first, so a card
// never opens or ends on a rule and no caller has to say which row is
// last.
func (a *App) card(c *ui.Context, fn func()) {
	tokens := a.tokens(c)
	saved := a.cardRows
	a.cardRows = 0
	defer func() { a.cardRows = saved }()
	ui.Column(c).FillWidth().
		Border(1, tokens.ControlEdge).
		Radius(tokens.RadiusMD).
		Background(tokens.Control).
		Clip().
		Children(fn)
}

// row draws one row of a card: the label and its optional grey second line
// at the start and the control at the end, with the rule between rows inset
// to the label's own column. It reports whether the label was clicked — the
// label is a hit target that activates the row's control, as the framework's
// own Field does, so a switch row can be toggled from either end.
func (a *App) row(c *ui.Context, label, sublabel string, control func()) bool {
	tokens := a.tokens(c)
	t := c.Theme()
	inset := t.Space(3.5)
	first := a.cardRows == 0
	a.cardRows++
	clicked := false
	ui.Column(c).FillWidth().Children(func() {
		if !first {
			// The row separator, inset on both sides and never running to
			// the card's frame; drawn above so only the card's first row
			// has none.
			ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).
				MarginX(inset).Shrink(0)
		}
		ui.Row(c).FillWidth().Gap(t.Space(3)).AlignItems(ui.Center).
			Padding(t.Space(2.25), inset).Children(func() {
			names := ui.Column(c).Grow(1).MinWidth(0).Gap(1)
			names.Children(func() {
				ui.Text(c, label).FontSize(t.FontSize).FontWeight(580).TextColor(tokens.TextStrong)
				if sublabel != "" {
					ui.Text(c, sublabel).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
				}
			})
			if names.Clicked() {
				clicked = true
			}
			ui.Row(c).Shrink(0).Gap(t.Space(2)).AlignItems(ui.Center).Children(control)
		})
	})
	return clicked
}

// action draws the right-aligned accent text action the reference's rows use
// ("Edit…", "Restore defaults"): the accent spent on text, never on a fill.
func (a *App) action(c *ui.Context, label string, clicked func()) {
	t := c.Theme()
	row := ui.Row(c).Focusable().Padding(t.Space(1), t.Space(1.5)).Radius(a.tokens(c).RadiusSM)
	if row.Hovered() {
		row.Background(t.SurfaceHover)
	}
	row.Children(func() { ui.Text(c, label).FontSize(t.FontSize).TextColor(t.Accent) })
	if row.Clicked() {
		clicked()
	}
}
