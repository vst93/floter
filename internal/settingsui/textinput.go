package settingsui

import "github.com/egoist/mygo/ui"

// textInput is ui.TextInput with the settings surface's arrow rule applied:
// ↑/↓ never walk the sidebar while the keyboard is in a text field, whether
// the field is single- or multi-line — the reference's
// `isArrowKeyEditableTarget` (src/settings-nav.ts:24-38) treats every INPUT
// and TEXTAREA alike.
//
// mygo's editor only claims ↑/↓ for a multiline field (`editor.wants`,
// ui/editor_navigation.go:46-47: `return ed.multiline || m != 0`), so a
// single-line TextInput lets the surface's window-level shortcut take them.
// The field therefore claims them itself: an element-scoped shortcut sits in
// the focus chain, and `routeKeys` (ui/input.go:730) gives a key to the
// innermost element handling it, so a focused field wins over the window.
func textInput(c *ui.Context, value *string) ui.Element {
	e := ui.TextInput(c, value)
	e.Shortcut(0, ui.KeyUp)
	e.Shortcut(0, ui.KeyDown)
	return e
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
	e := textInput(c, value)
	if a.cardUnit > 0 {
		e = e.MinHeight(m.minHeight).Padding(m.padY, m.padX).Radius(999)
	}
	return e
}
