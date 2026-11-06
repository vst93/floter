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
		// A question that no longer has unsaved fields must leave the
		// keyboard path as well as the painted form. Keep the draft itself.
		if a.configDiscard[in.ID] && (in.Orphan || !configOwnerIsHost(in) || !a.configDirty(in, a.draft(in))) {
			a.disarmConfigDiscard(in.ID)
		}
		if in.Orphan && !a.removeBusy[in.ID] {
			delete(a.removeConfirm, in.ID)
			delete(a.removeConfirmAt, in.ID)
			delete(a.removeError, in.ID)
		}
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
	for id := range a.configDiscard {
		if !present[id] {
			// The question is gone; its editable draft is not an answer.
			a.disarmConfigDiscard(id)
		}
	}
	for id := range a.uninstallOpen {
		if !present[id] {
			delete(a.uninstallOpen, id)
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
