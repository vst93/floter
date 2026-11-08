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

func (s *contentFocusState) restore(c *ui.Context) bool {
	if s.button.FocusWithin(c) {
		if source := c.Resolve(s.source); source.Valid() && !source.IsDisabled() {
			source.Focus()
			return true
		}
	}
	return false
}

// Revealing is part of returning to an input, not a continuous focus follower.
// Once the button gives up focus, subsequent scrolling stays with the user.
func (s *contentFocusState) restoreVisible(c *ui.Context) {
	if s.restore(c) {
		c.Resolve(s.source).ScrollIntoView()
	}
}

// Only the focused member of a mode owns its exit. The mode picker is a
// neutral return point when those inputs and actions are withdrawn together.
func (a *App) rememberModeContent(c *ui.Context, script bool, entry ui.Element) ui.Element {
	kind := "mode-program"
	if script {
		kind = "mode-script"
	}
	if entry.FocusWithin() {
		entry.Bind(&a.contentFocus(c, kind).button)
	}
	return entry
}

// Deep links share labels, so their return targets are keyed by the URL.
func (a *App) linkFocus(c *ui.Context, link string) *contentFocusState {
	key := struct {
		app  *App
		link string
	}{a, link}
	return ui.Local(c.Root(), key, func() contentFocusState { return contentFocusState{} })
}
