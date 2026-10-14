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
	return theme.ForPreferences(a.Store.Snapshot(), c.Theme().Dark, c.Preferences())
}

// section draws a titled group: the title, its one-line explanation, and
// the cards fn builds under them.
func (a *App) section(c *ui.Context, title, hint string, fn func()) {
	a.sectionFoot(c, title, hint, nil, fn)
}

// sectionFoot is section with the group's own footnotes under its cards. The
// reference drew a group's explanations as `.settings-section__hint` lines at
// the *end* of the section when they spoke for the whole group rather than for
// one row (the material section's glass and transparency notes), so the two
// cards above them carry no repeated per-row hint.
func (a *App) sectionFoot(c *ui.Context, title, hint string, footnotes []string, fn func()) {
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
		for _, note := range footnotes {
			if note != "" {
				ui.Text(c, note).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted)
			}
		}
	})
}

// card draws one grouped card: the resting control plane, its hairline
// edge, the card radius, and the clip that keeps the first and last rows
// inside the rounded corners. It is the row counter's own frame: the rows
// fn builds draw the rule *above* themselves except the first, so a card
// never opens or ends on a rule and no caller has to say which row is
// last.
func (a *App) card(c *ui.Context, fn func()) {
	a.cardLabeled(c, "", fn)
}

// cardLabeled is card with the group's accessible name, the reference's
// `aria-label` on a card whose visible title is the section heading above it
// (the transparency card inside the material section).
func (a *App) cardLabeled(c *ui.Context, label string, fn func()) {
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
	if label != "" {
		card.Label(label)
	}
	tokens.Lens(card)
	card.Children(fn)
}

// actionButton draws one of the card's action buttons — the reference's
// `.extensions-action-button`, the raised lens rung (`--elev-0`) every
// settings-card button carried. The framework's `Button` is the flat control
// pane the app builds every button from; the lens is the same four inset
// layers the card wears, and `--glass-lens-scale` grades it with the glass
// stop, so moving the stop is visible on the buttons and not only in the
// frame (the user's 「特别是各种控件按钮」).
func (a *App) actionButton(c *ui.Context, label string) ui.Element {
	return a.tokens(c).Lens(ui.Button(c, label))
}

// switchControl draws one switch: the framework's Switch inside the recessed
// lens (base.css's `.settings-switch` with `--elev-track`), the groove the
// user looks into. The inset stack is added to the track the framework draws,
// so the whole toggle is graded by the glass stop.
func (a *App) switchControl(c *ui.Context, on *bool) ui.Element {
	return a.tokens(c).LensInset(ui.Switch(c, on))
}

// segmentedControl draws one of the custom-tool form's segmented pickers
// (the reference's `.extension-custom-mode`): the framework's `Segmented`
// inside the recessed lens, the same groove a switch's track is.
func (a *App) segmentedControl(c *ui.Context, index *int, labels ...string) ui.Element {
	return a.tokens(c).LensInset(ui.Segmented(c, index, labels...))
}

// row draws one row of a card: the label and its optional grey second line
// at the start and the control at the end, with the rule between rows inset
// to the label's own column. It reports whether the label was clicked — the
// label is a hit target that activates the row's control, as the framework's
// own Field does, so a switch row can be toggled from either end.
func (a *App) row(c *ui.Context, label, sublabel string, control func(), below ...func()) bool {
	return a.rowNote(c, label, sublabel, "", control, below...)
}

// rowLine is one grey line under a row's label. Plain text is painted in
// the row's muted grey; a line may instead be several segments, each in a
// role's own colour — the sessions meta line, whose state word is the
// accent while its session runs and the quiet grey once it has exited.
type rowLine struct {
	Text     string
	Segments []rowSegment
}

// rowSegment is one piece of a row's meta line.
type rowSegment struct {
	Text string
	Role rowSegmentRole
}

// rowSegmentRole names the colour a segment is painted in, so a line can be
// built without a theme and painted with one.
type rowSegmentRole uint8

const (
	// rowSegmentTertiary is the quiet grey the old `.session-manager__meta`
	// line was drawn in: the screen size and the start time, and the state
	// word of a session that has exited.
	rowSegmentTertiary rowSegmentRole = iota
	// rowSegmentAccent is the live accent `.session-state` used while the
	// session is running.
	rowSegmentAccent
)

// rowLines is row with a second line that is itself several lines: the
// sessions list's row keeps the shell's working directory above the state,
// size and start time, as the old page's `.session-manager__lines` did.
func (a *App) rowLines(c *ui.Context, label string, sublabels []rowLine, control func(), below ...func()) bool {
	return a.rowAligned(c, ui.Center, label, sublabels, "", control, below...)
}

// rowNote is row with one more line: a warning-coloured note under the
// description, for the one thing a row has to disclose rather than describe
// (the clipboard's privacy line). An empty note draws nothing.
func (a *App) rowNote(c *ui.Context, label, sublabel, note string, control func(), below ...func()) bool {
	return a.rowAligned(c, ui.Center, label, []rowLine{{Text: sublabel}}, note, control, below...)
}

// rowTop is rowNote with the control pinned to the row's first line rather
// than centred on it: a row whose copy runs to several lines (a base plugin's
// name, its description and its privacy line) keeps its control beside the
// name, as the reference's `.extensions-base-plugin` did with
// `align-items: flex-start`.
func (a *App) rowTop(c *ui.Context, label, sublabel, note string, control func(), below ...func()) bool {
	return a.rowAligned(c, ui.Start, label, []rowLine{{Text: sublabel}}, note, control, below...)
}

// rowAligned is the row both helpers share; align places the control across
// the row's axis.
func (a *App) rowAligned(c *ui.Context, align ui.Align, label string, sublabels []rowLine, note string, control func(), below ...func()) bool {
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
					if len(sublabel.Segments) > 0 {
						// A meta line is the old page's `.session-manager__meta`: a
						// wrapping flex row of its own facts, each a span with its own
						// colour, so the state word can be the accent while the size
						// and start time stay quiet.
						meta := ui.Row(c).FillWidth().Wrap().GapX(t.Space(2.5)).GapY(t.Space(1))
						meta.Children(func() {
							for _, segment := range sublabel.Segments {
								color := tokens.TextTertiary
								if segment.Role == rowSegmentAccent {
									color = t.Accent
								}
								ui.Text(c, segment.Text).FontSize(t.FontSize).FontWeight(500).TextColor(color)
							}
						})
						continue
					}
					if sublabel.Text == "" {
						continue
					}
					ui.Text(c, sublabel.Text).FontSize(t.FontSize).FontWeight(500).TextColor(t.TextMuted)
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

// actionRow is a card row with no label: a full-width band that draws the
// row's separating hairline and padding and lets control fill it. The local-
// tool form's content-action row (a cluster of buttons, no label of its own)
// is its one caller, so the card's row rhythm — the rule above every row but
// the first — stays identical to the labelled rows beside it.
func (a *App) actionRow(c *ui.Context, control func()) {
	tokens := a.tokens(c)
	t := c.Theme()
	inset := t.Space(3.5)
	first := a.cardRows == 0
	a.cardRows++
	ui.Column(c).FillWidth().Children(func() {
		if !first {
			ui.Box(c).FillWidth().Height(1).Background(tokens.Hairline).MarginX(inset).Shrink(0)
		}
		ui.Column(c).FillWidth().Padding(t.Space(1.5), inset).Children(control)
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
