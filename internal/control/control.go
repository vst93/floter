// Package control holds the reference's own control materials that more than
// one surface draws. The settings panel and the terminal's appearance drawer
// both offer the same small set of choices, and a choice must not wear two
// shapes depending on which surface it is on: the segmented control lives here
// so there is one implementation, not one per caller.
package control

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// Segmented draws the reference's inline segmented control
// (`.settings-options--inline`): a pill track holding pill segments, the chosen
// one a neutral raised pane with an accent keyline — a *state of the control*,
// not a primary action, so it spends no accent fill. The framework's own
// Segmented is a rounded rectangle with a darkened chosen segment; this is the
// reference's shape and material.
//
// labels[i] stands for option i; apply runs with the index the user chose.
func Segmented(c *ui.Context, tokens theme.Tokens, labels []string, current int, apply func(int)) {
	t := c.Theme()
	track := ui.Row(c).Shrink(0).Padding(t.Space(0.5)).Gap(t.Space(0.5)).Radius(999).
		Background(tokens.Track)
	// The track is a groove the user looks into: the recessed lens, the same
	// stack the raised panes carry read backwards (see `theme.Tokens.LensInset`,
	// base.css's `--elev-track` / `--glass-lens-stack-inset`).
	tokens.LensInset(track)
	track.Children(func() {
		for i, label := range labels {
			i, label := i, label
			segment := ui.Row(c).Focusable().Padding(t.Space(0.75), t.Space(2.25)).
				Radius(999).AlignItems(ui.Center).Justify(ui.Center).Label(label)
			switch {
			case i == current:
				// The chosen segment is a raised pane: the resting raised lens
				// (`--elev-0`) plus the accent keyline that says "chosen", and
				// deliberately not an accent fill. The framework's `ui.Button`-
				// less pane the whole card language builds on is a flat tint;
				// the lens is what makes the chosen slot read as glass.
				segment.Background(tokens.ControlHover)
				tokens.Lens(segment)
				segment.InsetShadow(0, 0, 0, 1, tokens.AccentEdge)
			case segment.Hovered():
				segment.Background(tokens.ControlHover)
			}
			color := tokens.TextStrong
			weight := 580
			if i != current {
				color = t.TextMuted
				weight = 500
			}
			segment.Children(func() {
				ui.Text(c, label).FontSize(tokens.Emphasis).FontWeight(weight).TextColor(color)
			})
			if segment.Clicked() {
				apply(i)
			}
		}
	})
}

// AppearanceAxis names one of the terminal's appearance controls. Two surfaces
// paint these — the settings page's "Terminal appearance" card and the
// terminal's own appearance drawer — from one settings file, so the axis is
// the identity the pair shares: the reference's schema carried the same idea
// as a `page: "both" | "card" | "strip"` field on every row. Deciding here,
// once, which surface shows which axis is what keeps the two views from
// drifting into two truths about the same control (R42/R55).
type AppearanceAxis string

// The shipped axes, in the order the drawer paints them (the reference's strip
// order). The settings card keeps the same relative order; see
// SettingsCardAxes.
const (
	AxisFontFamily  AppearanceAxis = "font_family"
	AxisFontSize    AppearanceAxis = "font_size"
	AxisLineHeight  AppearanceAxis = "line_height"
	AxisPadding     AppearanceAxis = "padding"
	AxisCursorShape AppearanceAxis = "cursor_shape"
	AxisCursorBlink AppearanceAxis = "cursor_blink"
	AxisPalette     AppearanceAxis = "palette"
	// AxisPasteSafe is the safe-paste switch: it belongs to the drawer alone
	// (the reference's `page: "strip"`), because the rule it turns on is a
	// terminal-input rule, not a look.
	AxisPasteSafe AppearanceAxis = "paste_safe"
)

// AppearanceAxes is the full schema, in the order the drawer paints it.
func AppearanceAxes() []AppearanceAxis {
	return []AppearanceAxis{
		AxisFontFamily, AxisFontSize, AxisLineHeight, AxisPadding,
		AxisCursorShape, AxisCursorBlink, AxisPalette, AxisPasteSafe,
	}
}

// OnSettingsCard reports whether the settings page's "Terminal appearance"
// card carries the axis. R55: the card keeps only the few controls the user
// called important — the family, the size, the line height and the palette —
// while the terminal's own drawer keeps the full set. Hiding a row on the card
// never touches persistence: a value set from the drawer keeps applying.
func OnSettingsCard(axis AppearanceAxis) bool {
	switch axis {
	case AxisPadding, AxisCursorShape, AxisCursorBlink, AxisPasteSafe:
		return false
	}
	return true
}

// SettingsCardAxes is the subset the settings card paints, in the full
// schema's order. It is derived from OnSettingsCard, so the card cannot list a
// row the rule says belongs to the drawer.
func SettingsCardAxes() []AppearanceAxis {
	var out []AppearanceAxis
	for _, axis := range AppearanceAxes() {
		if OnSettingsCard(axis) {
			out = append(out, axis)
		}
	}
	return out
}
