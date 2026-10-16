package settingsui

import (
	"github.com/egoist/mygo/ui"

	"floter/internal/theme"
)

// textInput is the settings surface's one text field: the framework's field at
// the surface's own `--u` metric, wearing the reference's *field* face instead
// of the framework's control face.
//
// The reference writes a text field's material as `--glass-field` under the
// `--glass-field-shadow` inset stack — `.settings-select`, `.residency-control
// __input` (`settings.css:919-940,964-977`), `.extension-config-field input,
// select` (`extensions.css:2402-2417`), `.extension-custom-form input`
// (`extensions.css:1770-1781`) all say it — while the framework's `ui.TextInput`
// paints its own `--glass-control` resting pane (`theme.Tokens.Control`, which
// is what `ui.Theme.Surface` carries) with the stronger `--input-stroke` edge.
// `.glass-field` is the thinner rung of the same ladder (dark 0.03 against the
// control's 0.055, light 0.5 against the control's 0.035 — `base.css:517,880
// vs :894`), so a field the framework drew read a step *heavier* than the slot
// the reference asked for, everywhere one appeared.
//
// The `↑`/`↓` rule is the settings surface's own: ↑/↓ never walk the sidebar
// while the keyboard is in a text field, whether the field is single- or
// multi-line — the reference's `isArrowKeyEditableTarget`
// (src/settings-nav.ts:24-38) treats every INPUT and TEXTAREA alike.
//
// mygo's editor only claims ↑/↓ for a multiline field (`editor.wants`,
// ui/editor_navigation.go:46-47: `return ed.multiline || m != 0`), so a
// single-line TextInput lets the surface's window-level shortcut take them.
// The field therefore claims them itself: an element-scoped shortcut sits in
// the focus chain, and `routeKeys` (ui/input.go:730) gives a key to the
// innermost element handling it, so a focused field wins over the window.
func (a *App) textInput(c *ui.Context, value *string) ui.Element {
	e := a.fieldFace(c, ui.TextInput(c, value))
	e.Shortcut(0, ui.KeyUp)
	e.Shortcut(0, ui.KeyDown)
	return e
}

// fieldFace lays the reference's field material over an element: the
// `--glass-field` fill, the `--glass-control-edge` hairline and the
// `--glass-field-shadow` inset stack (`theme.Tokens.FieldSlot`). It is the one
// place a field's material is decided, so a config input, a run parameter, a
// residency field and a shortcut command cannot drift into two faces.
func (a *App) fieldFace(c *ui.Context, e ui.Element) ui.Element {
	tokens := a.tokens(c)
	e = e.Background(tokens.Field).Border(1, tokens.ControlEdge)
	return tokens.FieldSlot(e)
}

// extensionFieldMetrics is the literal box one of the extension panel's text
// fields wears. The reference writes the panel's fields in px, one step apart:
// a config field is a 28px floor with 4px/8px padding and a 999px pill
// (`.extension-config-field input`, extensions.css:2402-2417), while a custom
// form field — the local-tool form and the parameter editor — is a 30px floor
// with 3px/7px padding and the same pill (`.extension-custom-form input`,
// extensions.css:1770-1781).
type extensionFieldMetrics struct{ minHeight, padY, padX float32 }

var (
	configFieldMetrics = extensionFieldMetrics{minHeight: 28, padY: 4, padX: 8}
	customFieldMetrics = extensionFieldMetrics{minHeight: 30, padY: 3, padX: 7}
)

// aliasFieldMetrics is the command-alias editor's own box: the reference's
// `.extension-command-alias__input` (`extensions.css:2281-2290`) — a
// `padding: 4px 8px` box with the *ladder's* `--radius-xs` (6px) corner and the
// `--text-caption` step. It is deliberately neither the config pill (28/4/8/
// 999) nor the custom pill (30/3/7/999): the alias is a caption-scale inline
// box, so it keeps the small corner and the panel's caption register, and its
// 4/8 padding is written in literal px like the rest of `extensions.css`.
type aliasFieldMetrics struct{ padY, padX, radius float32 }

var commandAliasMetrics = aliasFieldMetrics{padY: 4, padX: 8, radius: theme.RadiusXS}

// extensionField is the framework's text field at the extension panel's own
// metric. The reference's panel is written in literal px and does not follow
// the interface step the way the settings pages' `--u` metrics do, so inside
// `extensionMetrics` the field is pinned to `m` and keeps the same box at every
// step; on the settings pages, where `cardUnit` is zero, the field keeps the
// framework's metrics and scales with the step as the reference's `--u` fields
// do. Every extension-panel entry point for a text field goes through here, so
// a config field, a run parameter, a local-tool input and a parameter-editor
// input cannot drift into two boxes.
func (a *App) extensionField(c *ui.Context, value *string, m extensionFieldMetrics) ui.Element {
	e := a.textInput(c, value)
	if a.cardUnit > 0 {
		e = e.MinHeight(m.minHeight).Padding(m.padY, m.padX).Radius(999)
	}
	return e
}

// aliasField draws one command's alias editor: the reference's
// `.extension-command-alias__input` (`extensions.css:2281-2290`) — a caption
// register, `--hairline`-edged, `--glass-control`-filled box with the ladder's
// 6px corner and a literal `4px 8px` padding. The box is pinned the same way
// `extensionField` pins its fields, so the alias keeps one box at every
// interface step and does not follow the settings pages' `--u`.
func (a *App) aliasField(c *ui.Context, value *string) ui.Element {
	e := ui.TextInput(c, value)
	tokens := a.tokens(c)
	if a.cardUnit > 0 {
		m := commandAliasMetrics
		e = e.MinHeight(tokens.Caption+m.padY*2).
			Padding(m.padY, m.padX).
			Radius(m.radius)
	}
	e = e.FontSize(tokens.Caption)
	// The alias field is the panel's one control filled with the *control*
	// rung (`.extension-command-alias__input { background: var(--glass-control);
	// border: 1px solid var(--hairline) }`), not the field rung the other panel
	// fields wear: it is an inline command-chip, not a settings slot.
	return e.Background(tokens.Control).Border(1, tokens.Hairline)
}
