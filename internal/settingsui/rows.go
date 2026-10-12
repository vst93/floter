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
	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		// The group's title is the ladder's title step at 620 in the strong
		// colour, its one-line explanation the body step at 500 in the muted
		// one — `.settings-section__label` / `__hint`.
		ui.Text(c, title).FontSize(t.FontSize + 2).FontWeight(620).TextColor(a.tokens(c).TextStrong)
		if hint != "" {
			ui.Text(c, hint).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted)
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
	// The card wears the reference's plane: the resting control fill, the
	// control edge (not the window's stronger `--input-stroke`), the card
	// radius step and the lens (`--elev-0`). It is a *plane*, never glass —
	// the material belongs to the window shell — and the clip keeps the
	// first and last rows inside the rounded corners.
	card := ui.Column(c).FillWidth().
		Border(1, tokens.ControlEdge).
		Radius(tokens.RadiusMD).
		Background(tokens.Control).
		Clip()
	tokens.Lens(card)
	card.Children(fn)
}

// row draws one row of a card: the label and its optional grey second line
// at the start and the control at the end, with the rule between rows inset
// to the label's own column. It reports whether the label was clicked — the
// label is a hit target that activates the row's control, as the framework's
// own Field does, so a switch row can be toggled from either end.
func (a *App) row(c *ui.Context, label, sublabel string, control func(), below ...func()) bool {
	return a.rowNote(c, label, sublabel, "", control, below...)
}

// rowLines is row with a second line that is itself several lines: the
// sessions list's row keeps the shell's working directory above the state,
// size and start time, as the old page's `.session-manager__lines` did.
func (a *App) rowLines(c *ui.Context, label string, sublabels []string, control func(), below ...func()) bool {
	return a.rowAligned(c, ui.Center, label, sublabels, "", control, below...)
}

// rowNote is row with one more line: a warning-coloured note under the
// description, for the one thing a row has to disclose rather than describe
// (the clipboard's privacy line). An empty note draws nothing.
func (a *App) rowNote(c *ui.Context, label, sublabel, note string, control func(), below ...func()) bool {
	return a.rowAligned(c, ui.Center, label, []string{sublabel}, note, control, below...)
}

// rowTop is rowNote with the control pinned to the row's first line rather
// than centred on it: a row whose copy runs to several lines (a base plugin's
// name, its description and its privacy line) keeps its control beside the
// name, as the reference's `.extensions-base-plugin` did with
// `align-items: flex-start`.
func (a *App) rowTop(c *ui.Context, label, sublabel, note string, control func(), below ...func()) bool {
	return a.rowAligned(c, ui.Start, label, []string{sublabel}, note, control, below...)
}

// rowAligned is the row both helpers share; align places the control across
// the row's axis.
func (a *App) rowAligned(c *ui.Context, align ui.Align, label string, sublabels []string, note string, control func(), below ...func()) bool {
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
		// The row is the reference's own: a 44u floor, a 12u gap, a 9u padding
		// pair, the label at the emphasis step (580, strong) and the grey
		// second line at the body step (500, muted). The press is the control
		// material's own: less opaque than the resting pane, so a row the user
		// is about to activate sinks into the glass.
		row := ui.Row(c).FillWidth().Gap(t.Space(3)).AlignItems(align).
			MinHeight(t.Space(11)).Padding(t.Space(1.5), inset)
		switch {
		case row.Pressed():
			row.Background(t.SurfacePressed)
		case row.Hovered():
			row.Background(t.SurfaceHover)
		}
		row.Children(func() {
			names := ui.Column(c).Grow(1).MinWidth(0).Gap(1)
			names.Children(func() {
				ui.Text(c, label).FontSize(t.FontSize + 1).FontWeight(580).TextColor(tokens.TextStrong)
				for _, sublabel := range sublabels {
					if sublabel == "" {
						continue
					}
					ui.Text(c, sublabel).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted)
				}
				if note != "" {
					ui.Text(c, note).FontSize(t.FontSize).FontWeight(500).TextColor(t.Warning)
				}
			})
			if names.Clicked() {
				clicked = true
			}
			ui.Row(c).Shrink(0).Gap(t.Space(2)).AlignItems(ui.Center).Children(control)
		})
		// What belongs *under* the row but inside the card: an unfolding
		// audit, a configuration form. It keeps the card's own rhythm and its
		// row counter, so a following row still draws its rule.
		for _, extra := range below {
			if extra != nil {
				extra()
			}
		}
	})
	return clicked
}

// sectionWithAction is a section whose title row carries the group's own
// action at its end, baseline-aligned with the title: the reference pane's
// "Restore defaults" idiom (`.settings-section__heading-actions`).
func (a *App) sectionWithAction(c *ui.Context, title, hint, actionLabel string, run func(), fn func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(t.Space(2)).Children(func() {
		heading := ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(t.Space(3))
		heading.Children(func() {
			ui.Text(c, title).FontSize(t.FontSize + 2).FontWeight(620).TextColor(a.tokens(c).TextStrong)
			if hint != "" {
				ui.Text(c, hint).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted).Grow(1)
			}
			if run != nil {
				a.action(c, actionLabel, run)
			}
		})
		fn()
	})
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

// fadeRule draws base.css's `--hairline-fade`: a 1px rule that dissolves into
// the surface at both ends instead of stopping at the frame — the settings
// header's and sidebar's separator. mygo's gradient carries two stops, so the
// fade is three segments: a short transparent-to-hairline lead-in, the solid
// middle, and the mirrored lead-out. vertical draws it top-to-bottom for the
// sidebar's column separator.
func (a *App) fadeRule(c *ui.Context, vertical bool) {
	tokens := a.tokens(c)
	segment := func(from, to ui.Color, lead float32) {
		e := ui.Box(c).Grow(lead)
		angle := float32(90)
		if vertical {
			e, angle = e.Width(1), 180
		} else {
			e = e.Height(1)
		}
		e.Gradient(from, to, angle)
	}
	lead, solid := float32(0.12), float32(0.76)
	if vertical {
		lead, solid = 0.1, 0.8
	}
	var rule ui.Element
	if vertical {
		rule = ui.Column(c).FillHeight().Width(1)
	} else {
		rule = ui.Row(c).FillWidth().Height(1)
	}
	rule.Children(func() {
		segment(ui.Transparent, tokens.Hairline, lead)
		segment(tokens.Hairline, tokens.Hairline, solid)
		segment(tokens.Hairline, ui.Transparent, lead)
	})
}
