package settingsui

import "github.com/egoist/mygo/ui"

type noticeTargetsKey struct{ app *App }

// A row cannot return focus after discovery stops building it. Compare this
// page's consecutive lists while the previous frame still owns the keyboard,
// and retire only its UI handles; callbacks retain their in-flight work.
func (a *App) restoreMissingNoticeFocus(c *ui.Context, integrations []Integration) {
	previous := ui.Local(c.Root(), noticeTargetsKey{a}, func() map[string]bool { return nil })
	present := make(map[string]bool, len(integrations))
	for _, in := range integrations {
		present[in.ID] = true
	}
	for id := range *previous {
		if present[id] {
			continue
		}
		for _, kind := range []string{"save", "field", "transfer", "editor", "discard", "repair", "toggle", "disable", "remove", "uninstall", "run", "run-opener", "command", "output"} {
			focus := a.noticeFocus(c, id, kind)
			focus.restore(c, false, c.Resolve(*noticeExit(c)))
			*focus = noticeFocusState{}
		}
	}
	*previous = present
}

// Questions belong to an installed target and the conditions that made the
// question applicable. A refresh can withdraw either without an answer.
// Retire invalid questions before Escape; keep in-flight work with its callback.
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
		if !a.disableBusy[in.ID] && (!in.Enabled || in.Broken || in.Orphan) {
			delete(a.disableConfirm, in.ID)
			delete(a.disableConfirmAt, in.ID)
			delete(a.disableError, in.ID)
		}
		if !a.uninstallBusy[in.ID] && (in.Orphan || !in.componentizedRemoval()) {
			delete(a.uninstallOpen, in.ID)
			a.resetUninstallDraft(in.ID)
			delete(a.uninstallError, in.ID)
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
	panel           ui.Handle
}

// The header's existing return control remains a safe outlet when a refresh
// leaves no usable action for the former question's target.
func noticeExit(c *ui.Context) *ui.Handle {
	return ui.Local(c.Root(), "settings.notice-exit", func() ui.Handle { return ui.Handle{} })
}

// Only the operation that owns the keyboard needs a return path. Repeated
// integration IDs can draw several rows; an unfocused sibling must neither
// bind the same handle twice nor replace the focused row's identity.
func (a *App) rememberOperationFocus(c *ui.Context, id, kind string, entry ui.Element) ui.Element {
	if entry.FocusWithin() {
		entry.Bind(&a.noticeFocus(c, id, kind).panel)
	}
	return entry
}

func (a *App) restoreNotice(c *ui.Context, id, kind string, open bool, opener ui.Element) {
	if !opener.Valid() || opener.IsDisabled() {
		opener = c.Resolve(*noticeExit(c))
	}
	// The run form can retire far from its disclosure in a scrolled list.
	// Reveal only the existing return event; surviving focus keeps its scroll.
	if kind == "run" && !open && a.noticeFocus(c, id, kind).panel.FocusWithin(c) {
		opener.ScrollIntoView()
	}
	a.noticeFocus(c, id, kind).restore(c, open, opener)
}

func (a *App) noticeFocus(c *ui.Context, id, kind string) *noticeFocusState {
	key := struct {
		app      *App
		id, kind string
	}{a, id, kind}
	return ui.Local(c.Root(), key, func() noticeFocusState { return noticeFocusState{} })
}

func (s *noticeFocusState) restore(c *ui.Context, open bool, opener ui.Element) {
	if !open && (s.cancel.Focused(c) || s.confirm.Focused(c) || s.panel.FocusWithin(c)) {
		opener.Focus()
	}
}
