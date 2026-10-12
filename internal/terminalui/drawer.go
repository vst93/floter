package terminalui

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"floter/internal/control"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// ToggleDrawer opens or closes the terminal's own appearance drawer: the bar's
// gear flips it, exactly as the reference's `terminalSettingsOpen` did.
func (a *App) ToggleDrawer() { a.Drawer = !a.Drawer }

// CloseDrawer closes the drawer, if it is open. It is the drawer's own close
// control's action.
func (a *App) CloseDrawer() { a.Drawer = false }

// set writes one appearance change through the settings store. The shell's
// store listener hands it to every running session (Refresh) and repaints, so
// the change lands on the terminal the drawer is docked to without the session
// ever being left.
func (a *App) set(mutate func(*settings.Settings)) {
	_ = a.Store.Update(mutate)
}

// drawer is the terminal's appearance strip, docked at the panel's bottom edge
// — a sibling of the canvas, never a layer over it, so opening it shrinks the
// canvas (the PTY is resized) while the frame and the session's scrollback stay
// exactly where they were. The reference's user asked for 「简洁、不繁琐」, so
// the rows are a flat, wrapping grid: no tabs, no sub-headings, no second card.
//
// Only the axes this build can actually apply are drawn. The reference's strip
// also carried the bold mode, the wheel step, select-to-copy, paste-safety and
// the scrollbar switch; the framework's terminal exposes none of those knobs,
// and a control that does nothing is a lie, not a feature (see
// docs/mygo-rewrite-status.md).
func (a *App) drawer(c *ui.Context, copy i18n.Terminal) {
	t := c.Theme()
	tokens := tokensOf(a, c)
	s := a.Store.Snapshot()
	scopy := i18n.For(s.Language).Settings

	strip := ui.Row(c).FillWidth().Shrink(0).
		Padding(t.Space(2.5), t.Space(3)).Gap(t.Space(2.5)).AlignItems(ui.Start).
		Background(tokens.Control).
		BorderWidth(0, 0, 1, 0).BorderColor(tokens.ControlEdge).
		Label(scopy.GroupTerminal)
	strip.Children(func() {
		// The rows are a wrapping grid, as the reference's
		// `repeat(auto-fit, minmax(230px, 1fr))` was: a cell grows to its share
		// of the line and a line holds as many cells as fit, so the strip
		// reflows with the window instead of scrolling.
		grid := ui.Row(c).Grow(1).Wrap().GapX(t.Space(4.5)).GapY(t.Space(1.5))
		grid.Children(func() {
			a.drawerRow(c, t.Space(60), scopy.FontSize, func() {
				value := float64(s.FontSize)
				changed := false
				if ui.Slider(c, &value, float64(settings.MinFontSize), float64(settings.MaxFontSize)).
					Width(t.Space(22)).Changed() {
					changed = true
				}
				ui.Text(c, fmt.Sprintf("%d px", int(math.Round(value)))).
					FontSize(tokens.Emphasis).TextColor(t.TextMuted)
				if changed {
					a.set(func(s *settings.Settings) { s.FontSize = int(math.Round(value)) })
				}
			})
			a.drawerRow(c, t.Space(60), scopy.FontFamily, func() {
				families := FontFamilyOptions(s.FontFamily)
				selected := s.FontFamily
				chosen := ""
				if ui.Select(c, &selected, families).Grow(1).Changed() {
					chosen = selected
				}
				if chosen != "" {
					a.set(func(s *settings.Settings) { s.FontFamily = chosen })
				}
			})
			a.drawerRow(c, t.Space(60), scopy.LineHeight, func() {
				value := s.TerminalLineHeight
				changed := false
				if ui.Slider(c, &value, settings.MinTerminalLineHeight, settings.MaxTerminalLineHeight).
					Width(t.Space(22)).Changed() {
					changed = true
				}
				ui.Text(c, fmt.Sprintf("%.2f\u00d7", value)).
					FontSize(tokens.Emphasis).TextColor(t.TextMuted)
				if changed {
					a.set(func(s *settings.Settings) { s.TerminalLineHeight = value })
				}
			})
			a.drawerRow(c, t.Space(60), scopy.Padding, func() {
				control.Segmented(c, tokens, optionLabels(scopy.Paddings),
					optionIndex(scopy.Paddings, s.TerminalPadding), func(i int) {
						a.set(func(s *settings.Settings) { s.TerminalPadding = scopy.Paddings[i].ID })
					})
			})
			a.drawerRow(c, t.Space(60), scopy.CursorShape, func() {
				control.Segmented(c, tokens, optionLabels(scopy.CursorShapes),
					optionIndex(scopy.CursorShapes, s.CursorShape), func(i int) {
						a.set(func(s *settings.Settings) { s.CursorShape = scopy.CursorShapes[i].ID })
					})
			})
			on := s.CursorBlink
			switched := false
			clicked := a.drawerRow(c, t.Space(60), scopy.CursorBlink, func() {
				if ui.Switch(c, &on).Label(scopy.CursorBlink).Changed() {
					switched = true
				}
			})
			// The row's own label is the switch's name, so clicking it toggles
			// too — unless the switch itself was the target, in which case its
			// own report already carried the new value.
			if clicked && !switched {
				on = !on
			}
			if switched || clicked {
				a.set(func(s *settings.Settings) { s.CursorBlink = on })
			}
			// The palette row is the wide one: the picker beside the miniature
			// terminal that shows what the choice paints, so it asks for a line
			// of its own when the grid would otherwise squeeze the preview.
			a.drawerRow(c, t.Space(95), scopy.Palette, func() {
				selected := optionLabel(scopy.Palettes, s.TerminalTheme)
				chosen := ""
				if ui.Select(c, &selected, optionLabels(scopy.Palettes)).
					Grow(1).Changed() {
					for _, option := range scopy.Palettes {
						if option.Label == selected {
							chosen = option.ID
							break
						}
					}
				}
				// The preview follows the picker in the same frame the choice is
				// made, so a click on the select repaints the block immediately.
				shown := s.TerminalTheme
				if chosen != "" {
					shown = chosen
				}
				PalettePreview(c, tokens, shown, scopy.PalettePreview, t.Space(37))
				if chosen != "" {
					a.set(func(s *settings.Settings) { s.TerminalTheme = chosen })
				}
			})
		})
		// The drawer's own dismiss, in the corner a card's close button lives
		// in: the same 28u icon button the bar's gear/X is, so the module can
		// be dismissed from where it opened or from itself.
		a.barButton(c, "close", copy.SettingsClose, copy.SettingsClose, a.CloseDrawer)
	})
}

// drawerRow is one appearance row: its label at the start and its control at
// the end, the pair a cell of the strip's grid. minWidth is the cell's floor —
// the width below which the grid moves the row to the next line rather than
// squeezing the control. It reports whether the row itself was clicked, so a
// row whose label is the control's own name (the switch) can act on it, as the
// settings panel's rows do.
func (a *App) drawerRow(c *ui.Context, minWidth float32, label string, build func()) bool {
	t := c.Theme()
	tokens := tokensOf(a, c)
	row := ui.Row(c).Grow(1).MinWidth(minWidth).MinHeight(t.Space(7.5)).
		Gap(t.Space(2.5)).AlignItems(ui.Center)
	row.Children(func() {
		ui.Text(c, label).FontSize(t.FontSize).FontWeight(560).
			TextColor(tokens.TextSecondary).SingleLine().Shrink(0)
		slot := ui.Row(c).Grow(1).Gap(t.Space(2)).AlignItems(ui.Center).Justify(ui.End)
		slot.Children(build)
	})
	return row.Clicked()
}

// optionLabels is a copy table's choices as the labels the segmented control
// draws.
func optionLabels(options []i18n.Option) []string {
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
	}
	return labels
}

// optionLabel is the label of the choice whose id is `id`, the id itself when
// the table does not name it (a hand-edited value the picker must still show).
func optionLabel(options []i18n.Option, id string) string {
	for _, option := range options {
		if option.ID == id {
			return option.Label
		}
	}
	return id
}

// optionIndex is the position of the choice with this id, 0 when the table does
// not name it.
func optionIndex(options []i18n.Option, id string) int {
	for i, option := range options {
		if option.ID == id {
			return i
		}
	}
	return 0
}
