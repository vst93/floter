// Package control holds the reference's own control materials that more than
// one surface draws. The settings panel and the terminal's appearance drawer
// both offer the same small set of choices, and a choice must not wear two
// shapes depending on which surface it is on: the segmented control lives here
// so there is one implementation, not one per caller.
package control

import (
	"math"

	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// Segmented draws centered choices in a quiet track, with a neutral selected
// face. labels[i] stands for option i; apply receives the chosen index.
func Segmented(c *ui.Context, tokens theme.Tokens, labels []string, current int, apply func(int)) {
	segmented(c, tokens, labels, current, apply, false, false)
}

// SegmentedFill is Segmented at the row's full width: the reference's stacked
// segmented row (`.settings-row--stacked .settings-row__control` plus the
// inline track's `repeat(auto-fit, minmax(0, 1fr))`), where the track spans the
// card and the segments share the line. The trailing form sizes to its labels;
// this one is the choice under its own label.
func SegmentedFill(c *ui.Context, tokens theme.Tokens, labels []string, current int, apply func(int), keys ...[]string) ui.Element {
	return segmented(c, tokens, labels, current, apply, true, false, keys...)
}

// SegmentedGlassFill keeps the material picker's API while sharing the same
// label alignment and geometry as every other segmented control.
func SegmentedGlassFill(c *ui.Context, tokens theme.Tokens, labels []string, current int, apply func(int)) {
	segmented(c, tokens, labels, current, apply, true, true)
}

// segmented is the one implementation every form shares; fill only changes how
// the track and its segments take their width.
func segmented(c *ui.Context, tokens theme.Tokens, labels []string, current int, apply func(int), fill, glass bool, keys ...[]string) ui.Element {
	t := c.Theme()
	// The settings-side track's own inset: `.settings-options--inline { padding:
	// calc(var(--u) * 2); gap: calc(var(--u) * 2) }` (`settings.css:1016-1022`).
	selected := current
	track := SegmentedIndex(c, tokens, &selected, labels, fill, glass, t.Space(0.5), t.Space(0.5), keys...)
	if track.Changed() {
		apply(selected)
	}
	return track
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
// (`extensions.css:1752`), so it passes its own pair in. Both use the shared
// control radius and a distinct chosen face.
func SegmentedIndex(c *ui.Context, tokens theme.Tokens, selected *int, labels []string, fill, glass bool, pad, gap float32, keys ...[]string) ui.Element {
	t := c.Theme()
	parts := ui.SegmentedBase(c, selected, len(labels))
	track := parts.Track
	focus := ui.Local(track, "segment.focus", func() ui.Handle { return ui.Handle{} })
	hadFocus := focus.Focused(c)
	// A quiet rounded track leaves the material detail on the chosen segment.
	track.Shrink(0).Padding(pad).Gap(gap).Radius(tokens.RadiusSM).
		Background(tokens.Control)
	if fill {
		track.FillWidth()
	}
	track.Border(1, tokens.HairlineSoft)
	// Both standard options and material stops center their labels.
	inset := t.Space(2)
	track.Children(func() {
		for i, label := range labels {
			i, label := i, label
			// Dynamic schemas identify a segment by value, so a reorder or
			// removal cannot hand its keyboard focus to a different choice.
			if len(keys) > 0 && i < len(keys[0]) {
				c.Key(keys[0][i])
			}
			// Keep a usable hit area even at the smallest interface scale.
			segment := parts.Segment(i).Padding(0, inset).MinHeight(max(36, t.Space(9))).
				Radius(max(0, tokens.RadiusSM-pad-1)).Border(1, ui.Transparent).AlignItems(ui.Center).Justify(ui.Center).Label(label)
			if segment.Focused() {
				segment.Bind(focus)
			}
			if fill {
				width, _ := c.MeasureText(0, ui.Span{Text: label, Size: tokens.Body, Weight: 400})
				segment.Basis(float32(math.Ceil(float64(width))) + 2*inset + 2).Grow(1).MinWidth(0)
			}
			switch {
			case i == *selected:
				// The selected face stays neutral. The framework draws a focus
				// ring only when this segment receives keyboard focus.
				segment.Background(tokens.Thumb).Border(1, tokens.HairlineSoft)
			case segment.Pressed():
				// P13-gt · 第三档。未选中的那一格此前只有悬停（`ControlHover`）：
				// 按住的那一帧与悬停帧**逐像素相同**，于是「我这一下按到了吗」在
				// 墨上没有答案，而分段控件正是要「按下去选这一格」的东西。
				//
				// 面取控制阶梯的第三格 `ControlPress`（悬停之下、静息之上）——
				// 与同族的 `pageButton` 同一句。**不收盒子**：这一格的盒子就是
				// 它的命中框，`Scale` 会把点击点从指针下挪走（§6.151 的 7-10 在
				// 终端浮条上量到过同样的账），而分段控件是整块轨道里一个像素
				// 都不能丢的地方。
				segment.Background(tokens.ControlPress)
			case segment.Hovered():
				segment.Background(tokens.ControlHover)
			}
			// Every segment's label is the reference's `.settings-option__label`:
			// `color: var(--text-strong); font-weight: 580` for every segment,
			// chosen or not (`main:src/styles/settings.css:1111-1120`). The chosen
			// state is carried by the neutral selected face above,
			// never by dimming the label: an unchosen segment is not muted text.
			segment.Children(func() {
				ui.Text(c, label).FontSize(tokens.Body).FontWeight(400).TextColor(tokens.TextStrong).SingleLine().Ellipsis("…").MinWidth(0)
			})
		}
	})
	// Retiring a focused segment must not choose its replacement. Keep a
	// neutral Tab origin only while the track owns focus; ordinary radio
	// navigation and its single Tab stop otherwise remain unchanged.
	if hadFocus && !c.Resolve(*focus).Valid() {
		track.Focus()
	}
	if track.Focused() {
		track.Focusable().FocusGroup(0)
	}
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
	// AxisSelectCopy is the copy-on-select switch (`terminal_select_copy`): a
	// selection that ends lands on the clipboard instead of waiting for the
	// copy chord. The reference keeps it on the strip
	// (`main:src/settings/TerminalAppearance.tsx:328-338`, `page: "strip"`), so
	// it is a drawer row and not a card row — the settings card is the few
	// controls the user called important.
	//
	// It is here because the setting was *wired and invisible*: the terminal
	// surface read it (`internal/terminalui/options` → `Options.CopyOnSelect`,
	// and `Refresh` → `SetCopyOnSelect` for a running session) while no view
	// ever offered it, so a user could only reach it by editing settings.json —
	// the same 「磁盘上有、UI 上没有」 shape that used to be recorded against the
	// four dead knobs.
	AxisSelectCopy AppearanceAxis = "select_copy"
	// AxisPasteSafe is the safe-paste switch: it belongs to the drawer alone
	// (the reference's `page: "strip"`), because the rule it turns on is a
	// terminal-input rule, not a look.
	AxisPasteSafe AppearanceAxis = "paste_safe"
)

// AppearanceAxes is the full schema, in the order the drawer paints it.
func AppearanceAxes() []AppearanceAxis {
	return []AppearanceAxis{
		AxisFontFamily, AxisFontSize, AxisLineHeight, AxisPadding,
		AxisCursorShape, AxisCursorBlink, AxisPalette, AxisSelectCopy,
		AxisPasteSafe,
	}
}

// OnSettingsCard reports whether the settings page's "Terminal appearance"
// card carries the axis. R55: the card keeps only the few controls the user
// called important — the family, the size, the line height and the palette —
// while the terminal's own drawer keeps the full set. Hiding a row on the card
// never touches persistence: a value set from the drawer keeps applying.
func OnSettingsCard(axis AppearanceAxis) bool {
	switch axis {
	case AxisPadding, AxisCursorShape, AxisCursorBlink, AxisSelectCopy, AxisPasteSafe:
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
