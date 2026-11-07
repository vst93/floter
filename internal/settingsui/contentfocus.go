package settingsui

import "github.com/egoist/mygo/ui"

// Content actions return to their own editable value when the host withdraws
// the operation. Handles belong to this form and this view, not another tool.
type contentFocusState struct {
	button, source ui.Handle
}

func (a *App) contentFocus(c *ui.Context, action string) *contentFocusState {
	key := struct {
		app    *App
		form   *int
		action string
	}{a, a.toolFormKey, action}
	return ui.Local(c.Root(), key, func() contentFocusState { return contentFocusState{} })
}

func (s *contentFocusState) restore(c *ui.Context) {
	if s.button.FocusWithin(c) {
		if source := c.Resolve(s.source); source.Valid() && !source.IsDisabled() {
			source.Focus()
		}
	}
}
