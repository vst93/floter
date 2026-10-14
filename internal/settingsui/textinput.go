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
