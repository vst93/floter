package settingsui

import "github.com/egoist/mygo/ui"

// Questions belong to an installed target. A refresh can remove that target
// without going through the question's answer. Retire the invisible question
// before routing Escape; leave any in-flight operation to its own callback.
func (a *App) retireMissingConfirmations() {
	if a.Integrations == nil || !a.anyConfirmOpen() {
		return
	}
	present := make(map[string]bool)
	for _, in := range a.installedIntegrations() {
		present[in.ID] = true
	}
	for id := range a.disableConfirm {
		if !present[id] {
			delete(a.disableConfirm, id)
			delete(a.disableConfirmAt, id)
			delete(a.disableError, id)
		}
	}
	for id := range a.removeConfirm {
		if !present[id] {
			delete(a.removeConfirm, id)
			delete(a.removeConfirmAt, id)
			delete(a.removeError, id)
		}
	}
}

// Each question remembers its two answers across builds. Escape and expiry
// run before the row draws, so focus must be read from the committed frame.
// Keeping this on the UI root makes the handles live only as long as the view.
type noticeFocusState struct {
	cancel, confirm ui.Handle
}

func (a *App) noticeFocus(c *ui.Context, id, kind string) *noticeFocusState {
	key := struct {
		app      *App
		id, kind string
	}{a, id, kind}
	return ui.Local(c.Root(), key, func() noticeFocusState { return noticeFocusState{} })
}

func (s *noticeFocusState) restore(c *ui.Context, open bool, opener ui.Element) {
	if !open && (s.cancel.Focused(c) || s.confirm.Focused(c)) {
		opener.Focus()
	}
}
