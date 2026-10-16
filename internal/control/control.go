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
	segmented(c, tokens, labels, current, apply, false, false)
}

// SegmentedFill is Segmented at the row's full width: the reference's stacked
// segmented row (`.settings-row--stacked .settings-row__control` plus the
// inline track's `repeat(auto-fit, minmax(0, 1fr))`), where the track spans the
// card and the segments share the line. The trailing form sizes to its labels;
// this one is the choice under its own label.
func SegmentedFill(c *ui.Context, tokens theme.Tokens, labels []string, current int, apply func(int)) {
	segmented(c, tokens, labels, current, apply, true, false)
}

// SegmentedGlassFill is the material picker's own variant of the full-width
// form: the reference's `.glass-intensity__segments` (`main:src/styles/
// settings.css:792-799`) is the *only* segmented track whose labels are
// centered rather than left-aligned as a row, and the only one whose horizontal
// inset tightens from 9u to 6u. CSS's own comment names the reason — "A stop's
// label is the effect's name; centered because the segment is a choice, not a
// row" — so the base form keeps the row reading and this one carries the choice
// reading. The rest of the segment material (30u floor, raised pane, accent
// keyline) is unchanged.
func SegmentedGlassFill(c *ui.Context, tokens theme.Tokens, labels []string, current int, apply func(int)) {
	segmented(c, tokens, labels, current, apply, true, true)
}

// segmented is the one implementation every form shares; fill only changes how
// the track and its segments take their width, glass only the label's alignment
// and the segment's horizontal inset.
func segmented(c *ui.Context, tokens theme.Tokens, labels []string, current int, apply func(int), fill, glass bool) {
	t := c.Theme()
	// The settings-side track's own inset: `.settings-options--inline { padding:
	// calc(var(--u) * 2); gap: calc(var(--u) * 2) }` (`settings.css:1016-1022`).
	selected := current
	track := SegmentedIndex(c, tokens, &selected, labels, fill, glass, t.Space(0.5), t.Space(0.5))
	if track.Changed() {
		apply(selected)
	}
}

// SegmentedIndex is `segmented` over the caller's own index pointer: it draws
// the reference's track and segments and returns the track, whose Changed
// reports a new choice. It is the same material `Segmented`/`SegmentedFill`
// draw — one body, so the extension panel's own picker (which needs the pointer
// form, because it writes the chosen index back into its draft) cannot drift
// into a second shape for the same choice.
//
// `pad` is the track's own inset from its segments and `gap` the room between
// them. The settings pages' track is `2u` for both; the extension panel's
// `.extension-custom-mode` writes a literal 3px pad and a 2px gap
// (`extensions.css:1752`), so it passes its own pair in. The shape — the 999px
// stadium and the accent-keyline chosen pane — is the same for both.
func SegmentedIndex(c *ui.Context, tokens theme.Tokens, selected *int, labels []string, fill, glass bool, pad, gap float32) ui.Element {
	t := c.Theme()
	parts := ui.SegmentedBase(c, selected, len(labels))
	track := parts.Track
	// The track is the reference's stadium: a 999px corner, not the ladder's
	// control step (`.settings-options--inline { border-radius: 999px }`,
	// `settings.css:1016-1029`; `.extension-custom-mode { border-radius: 999px }`,
	// `extensions.css:1752`).
	track.Shrink(0).Padding(pad).Gap(gap).Radius(999).
		Background(tokens.Track)
	if fill {
		track.FillWidth()
	}
	// The track is a groove the user looks into: the recessed lens, the same
	// stack the raised panes carry read backwards (see `theme.Tokens.LensInset`,
	// base.css's `--elev-track` / `--glass-lens-stack-inset`).
	tokens.LensInset(track)
	// The bare `.settings-option` writes `text-align: left` (`settings.css:1060`)
	// with `padding: 0 9u` (`:1055`), so a segment is a row: its label starts at
	// the inset and reads left to right. Only `.glass-intensity__segments`
	// flips it to `text-align: center; justify-items: center` and tightens the
	// inset to `0 6u` (`:792-799`).
	inset := t.Space(2.25) // 9u
	justify := ui.Start
	if glass {
		inset = t.Space(1.5) // 6u
		justify = ui.Center
	}
	track.Children(func() {
		for i, label := range labels {
			i, label := i, label
			// The segment's box is the reference's `.settings-option`: a
			// `min-height` of 30u (R43's tightened step) with a `padding: 0 9u`
			// — the height comes from the floor, not from vertical padding, so
			// every segment is one box high whether its label is short or long.
			// `AlignItems(ui.Center)` is what puts the line box on the middle.
			segment := parts.Segment(i).Padding(0, inset).MinHeight(t.Space(7.5)).
				Radius(999).AlignItems(ui.Center).Justify(justify).Label(label)
			if fill {
				segment.Grow(1)
			}
			switch {
			case i == *selected:
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
			// Every segment's label is the reference's `.settings-option__label`:
			// `color: var(--text-strong); font-weight: 580` for every segment,
			// chosen or not (`main:src/styles/settings.css:1111-1120`). The chosen
			// state is carried by the raised pane and the accent keyline above,
			// never by dimming the label: an unchosen segment is not muted text.
			segment.Children(func() {
				ui.Text(c, label).FontSize(tokens.Emphasis).FontWeight(580).TextColor(tokens.TextStrong)
			})
		}
	})
	return track
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
