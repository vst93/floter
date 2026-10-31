package settingsui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
)

// The custom-shortcut section of the Shortcuts page: the bindings the user
// made, plus the row being made.
//
// The row being made is the page's own state (an immediate-mode UI keeps it
// here rather than in a component): the recorded key, the chosen action and,
// for a command line, its text. Nothing is persisted until the binding is
// whole — a key and an action — so an unfinished row can never reach the
// system. The row appears when the reference's "Add" is pressed and leaves with
// its own trash button or with the commit, which is the reference's shape: an
// empty group shows the empty region alone, never an extra half-made row.

// customSection draws the list, the row being made and the empty region.
func (a *App) customSection(c *ui.Context, copy i18n.Settings) {
	entries := a.customShortcuts()
	// A row being made starts on the first action, as the old build's draft
	// row did, so the action can be chosen before the key or the other way
	// round.
	if a.customAction == "" && len(copy.ShortcutsActions) > 0 {
		a.customAction = copy.ShortcutsActions[0].ID
	}
	// The reference's CustomShortcutsSection heading carries an "Add" text
	// action at its end (`ShortcutsPage.tsx:210-216`), the same accent-text
	// idiom the built-in group's "Restore defaults" uses.
	a.sectionWithAction(c, copy.ShortcutsCustom, copy.ShortcutsCustomHint,
		copy.ShortcutsAdd, func() {
			a.beginCustomDraft()
		}, func() {
			// The reference wraps both arms of this group in one card
			// (`ShortcutsPage.tsx:221-301`): the empty region sits *inside* the
			// card's plane, and a draft row is one of the card's rows.
			a.cardLabeled(c, copy.ShortcutsCustom, func() {
				if len(entries) == 0 && !a.customDraftOpen {
					// The reference's SettingsEmpty (`ShortcutsPage.tsx:240-252`):
					// the 22px Keyboard mark, `customShortcutsEmpty` in
					// --text-secondary, the grey hint underneath, and the "Add the
					// first one" text action (＋ mark) as the region's one way out —
					// never a filled button.
					a.emptyState(c, "keyboard", copy.ShortcutsNone, copy.ShortcutsEmptyHint, 0, 0, false, func() {
						a.actionWithGlyph(c, "plus", copy.ShortcutsAddFirst, func() {
							a.beginCustomDraft()
						})
					})
					return
				}
				for index, entry := range entries {
					index, entry := index, entry
					a.customShortcutRow(c, copy, entry, index)
				}
				if a.customDraftOpen {
					a.customDraftRow(c, copy)
				}
			})
		})
}

// customCapturePane is the armed recorder the custom-shortcuts row draws while
// it records: the same `Control` face as the resting recorder, plus the accent
// ring and the accent prompt — an armed state, not a primary action (§6.1 第 10
// 条：静息用 `Control`，只有录制中才用 accent 环). It owns the keyboard for as
// long as it is drawn; a bare Escape gives the recording up and a chord without
// a modifier is not a shortcut at all.
//
// P8-b 重定基线：`RaisedQuiet` 不透面 → `Control`（与静息同一档面）。
func (a *App) customCapturePane(c *ui.Context, copy i18n.Settings, capture func(accelerator string)) {
	t := c.Theme()
	tokens := a.tokens(c)
	pane := ui.Box(c).Focusable().Padding(t.Space(1), t.Space(2)).Radius(tokens.RadiusSM).
		Background(tokens.Control).Border(1, t.Accent).Label(copy.ShortcutRecording)
	pane.Children(func() {
		ui.Text(c, copy.ShortcutRecording).FontSize(t.FontSize).TextColor(t.Accent)
	})
	pane.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind != ui.InputKeyDown {
			return false
		}
		// Escape gives up the recording, but only a bare Escape: a modified
		// Escape is a chord the user may want to record.
		if ev.Key == ui.KeyEscape && ev.Mods == 0 {
			a.customRecording = false
			a.customRecordRow = -1
			return true
		}
		accelerator, ok := shortcuts.FromKey(ev.Mods, ev.Key)
		if !ok {
			return true // a key without a modifier is not a shortcut
		}
		a.customRecording = false
		capture(accelerator)
		return true
	})
	pane.Focus()
}

// customRecorder is one row's key recorder: the reference's
// `.shortcut-recorder` pane, showing the chord in the monospace stack while it
// rests and the armed prompt while it records. index is the row it records
// into; draftRow names the row being made.
func (a *App) customRecorder(c *ui.Context, copy i18n.Settings, key string, index int) {
	if a.customRecording && a.customRecordRow == index {
		a.customCapturePane(c, copy, func(accelerator string) {
			a.captureCustomKey(copy, accelerator)
		})
		return
	}
	// The reference passes `formatShortcut(row.key)` straight through, so a row
	// without a key yet shows an empty keys slot (the button's accessible name
	// is `settings.shortcut.record`, `ShortcutRecorder.tsx:67-78`).
	a.shortcutRecorder(c, copy.ShortcutRecord, shortcuts.Display(key), func() {
		a.customRecording = true
		a.customRecordRow = index
		a.customDuplicate, a.customDuplicateRow = "", customDraftRow
	})
}

// captureCustomKey applies a chord the custom section recorded. A saved row
// takes the key itself (the reference's `capture(id, next)`); the draft row
// being made takes it into the page's own draft state. A key another custom row
// already holds is refused on the spot — the reference's `localRejection` — and
// never reaches the system.
func (a *App) captureCustomKey(copy i18n.Settings, accelerator string) {
	entries := a.customShortcuts()
	if a.customRecordRow < 0 || a.customRecordRow >= len(entries) {
		keys := make([]string, len(entries))
		for i, entry := range entries {
			keys[i] = entry.Key
		}
		if shortcuts.Duplicate(accelerator, keys, -1) != "" {
			a.customDuplicate, a.customDuplicateRow = accelerator, customDraftRow
			return
		}
		a.customDuplicate, a.customDuplicateRow = "", customDraftRow
		a.customKey = accelerator
		return
	}
	keys := make([]string, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}
	if shortcuts.Duplicate(accelerator, keys, a.customRecordRow) != "" {
		a.customDuplicate, a.customDuplicateRow = accelerator, a.customRecordRow
		return
	}
	a.customDuplicate = ""
	// The in-place command draft belongs to the old key; the new binding starts
	// from the stored action.
	delete(a.customEdits, entries[a.customRecordRow].Key)
	entries[a.customRecordRow] = settings.CustomShortcut{
		Key:    accelerator,
		Action: entries[a.customRecordRow].Action,
	}
	a.commitCustomShortcuts(entries)
}

// customWarning words one row's refused key, as the reference's row sublabel
// did (`ShortcutsPage.tsx:262-274`): the two reasons have their own sentence
// (`settings.customShortcutsConflict` / `settings.customShortcutsOccupied`) and
// the warning belongs to the row that was being recorded — never to the whole
// page. row is the row's index, or -1 for the row being made, which is how the
// reference's local rejection was keyed (`localRejection?.id === row.id`).
func (a *App) customWarning(copy i18n.Settings, key string, row int) string {
	if a.customDuplicate != "" && a.customDuplicateRow == row {
		return copy.ShortcutsConflict
	}
	if strings.TrimSpace(key) == "" {
		return ""
	}
	for _, rejection := range a.customRejections {
		if !shortcuts.Equal(rejection.Key, key) {
			continue
		}
		if rejection.Reason == CustomShortcutReasonOccupied {
			return copy.ShortcutsOccupied
		}
		return copy.ShortcutsConflict
	}
	return ""
}

// customShortcutRow is one saved binding. The reference's row puts the recorder
// and the trash button in the label slot and the action picker in the control
// slot (`ShortcutsPage.tsx:262-298`); the old port left the label empty and
// pushed all three into the trailing slot, so every saved row read as a run of
// controls pinned to the right of nothing.
func (a *App) customShortcutRow(c *ui.Context, copy i18n.Settings, entry settings.CustomShortcut, index int) {
	tokens := a.tokens(c)
	t := a.cardTheme(c)
	action := entry.Action
	commandLine := !strings.HasPrefix(action, "plugin:") && !strings.HasPrefix(action, "action:")
	a.cardRow(c, ui.Center, nil, rowFace{}, func() {
		a.customActionPicker(c, copy, action, func(next string) {
			a.updateCustomShortcut(index, settings.CustomShortcut{Key: entry.Key, Action: next})
		})
		if commandLine {
			// A stored command line is the reference's editable
			// `input.settings-select.custom-shortcut__command`
			// (`ShortcutsPage.tsx:288-330`): the field wears the box of the
			// select beside it — a 28u pill cut into the glass (`settings.css:923-943`)
			// — and Enter stores the trimmed text as the row's action
			// (`commitCommand`, `:204-216`). What is typed lives in
			// `customEdits` between frames, as the reference kept it on the row
			// (`row.command`), so a half-typed command is not overwritten by the
			// stored one.
			text := action
			if draft, ok := a.customEdits[entry.Key]; ok {
				text = draft
			}
			field := a.textInput(c, &text).Label(copy.ShortcutsCommand).
				Placeholder(copy.ShortcutsCommandHint).Width(220).
				MinHeight(t.Space(7)).Padding(0, t.Space(2.5)).Radius(999)
			if field.Changed() {
				if a.customEdits == nil {
					a.customEdits = map[string]string{}
				}
				a.customEdits[entry.Key] = text
			}
			if field.Submitted() {
				a.commitCustomCommand(index, text)
			}
		}
	}, func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			a.customRecorder(c, copy, entry.Key, index)
			// The reference's trash is the sessions page's 28u danger icon
			// button (`session-manager__icon-button--danger`), not a text button
			// (`ShortcutsPage.tsx:275-286`): a transparent square whose ink turns
			// `--text-warning` under the pointer.
			a.sessionIconButton(c, "trash", copy.ShortcutsRemove, true, false, false, func() {
				a.removeCustomShortcut(index)
			})
		})
		if warning := a.customWarning(copy, entry.Key, index); warning != "" {
			ui.Text(c, warning).FontSize(tokens.Caption).FontWeight(500).TextColor(c.Theme().Warning)
		}
	})
}

// customDraftRow is the row being made: the same shape as a saved row, muted
// while it is not whole. Its trash dismisses the draft instead of deleting a
// saved binding, which is the reference's one row shape for both states.
func (a *App) customDraftRow(c *ui.Context, copy i18n.Settings) {
	tokens := a.tokens(c)
	complete := a.customRowComplete()
	face := rowFace{}
	face.Muted = !complete
	a.cardRow(c, ui.Center, nil, face, func() {
		a.customActionPicker(c, copy, a.customAction, func(action string) {
			a.customAction = action
		})
		if a.customAction == commandAction {
			command := a.customCommand
			a.textInput(c, &command).Placeholder(copy.ShortcutsCommandHint).
				Label(copy.ShortcutsCommand).Grow(1)
			a.customCommand = command
		}
		// The heading's own "Add" opens this row; this button commits it, so it
		// wears its own word rather than the same one twice on the page.
		if a.pageButton(c, copy.ShortcutsBind).Clicked() {
			a.addCustomShortcut()
		}
	}, func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			a.customRecorder(c, copy, a.customKey, -1)
			a.sessionIconButton(c, "trash", copy.ShortcutsRemove, true, false, false, func() {
				a.cancelCustomDraft()
			})
		})
		if warning := a.customWarning(copy, a.customKey, customDraftRow); warning != "" {
			ui.Text(c, warning).FontSize(tokens.Caption).FontWeight(500).TextColor(c.Theme().Warning)
		}
	})
}

// beginCustomDraft puts the reference's draft row on screen and arms its
// recorder: "Add" records a new custom shortcut, as its own hint promises.
func (a *App) beginCustomDraft() {
	a.customDraftOpen = true
	a.customRecording = true
	a.customRecordRow = -1
	a.customDuplicate = ""
}

// cancelCustomDraft drops the row being made without touching the stored list.
func (a *App) cancelCustomDraft() {
	a.customDraftOpen = false
	a.customRecording = false
	a.customRecordRow = -1
	a.customKey, a.customCommand = "", ""
	a.customDuplicate, a.customDuplicateRow = "", customDraftRow
}

// commandAction is the picker value that means "a command line I type": it is
// not an action string, so it can never be stored as one.
const commandAction = "command"

// customDraftRow names the row being made, where a row index is expected.
const customDraftRow = -1

// customActionPicker draws the action choice: one drop-down listing the
// plugins, the app's actions and the command-line slot. Its accessible name is
// the reference's `settings.customShortcutsAction`.
func (a *App) customActionPicker(c *ui.Context, copy i18n.Settings, current string, apply func(string)) {
	options := copy.ShortcutsActions
	value := current
	if value == "" {
		value = options[0].ID
	}
	index := -1
	labels := make([]string, len(options))
	for i, option := range options {
		labels[i] = option.Label
		if option.ID == value {
			index = i
		}
	}
	if index < 0 {
		// A stored action that is not in the picker (a command line, or an
		// action a later build dropped) reads as the command-line slot so the
		// control never shows a value it does not hold.
		index = len(options) - 1
	}
	selected := labels[index]
	chosen := ""
	if a.fieldFace(c, ui.Select(c, &selected, labels).Radius(999).Label(copy.ShortcutsAction)).Changed() {
		for _, option := range options {
			if option.Label == selected {
				chosen = option.ID
				break
			}
		}
	}
	if chosen != "" && chosen != current {
		apply(chosen)
	}
}

// customRowComplete reports whether the draft row is whole — both a key and an
// action are set — so the reference's muted style can be applied to an
// unfinished draft, as `settings-row--muted` did (`settings.css:564-569`).
func (a *App) customRowComplete() bool {
	if strings.TrimSpace(a.customKey) == "" {
		return false
	}
	if a.customAction == "" || a.customAction == commandAction {
		return strings.TrimSpace(a.customCommand) != ""
	}
	return true
}

// customShortcuts is the stored list.
func (a *App) customShortcuts() []settings.CustomShortcut {
	if a.Actions.CustomShortcuts == nil {
		return nil
	}
	return a.Actions.CustomShortcuts()
}

// addCustomShortcut stores the row being made, when it is whole, and reports
// what the system refused.
func (a *App) addCustomShortcut() {
	action := a.customAction
	if action == "" || action == commandAction {
		command := strings.TrimSpace(a.customCommand)
		if command == "" {
			return
		}
		action = command
	}
	if strings.TrimSpace(a.customKey) == "" {
		return
	}
	entries := append(append([]settings.CustomShortcut{}, a.customShortcuts()...),
		settings.CustomShortcut{Key: a.customKey, Action: action})
	a.commitCustomShortcuts(entries)
	a.customDraftOpen = false
	a.customKey, a.customCommand = "", ""
}

// commitCustomCommand writes a saved row's edited command line. The
// reference's `commitCommand` (`ShortcutsPage.tsx:204-216`) trimmed what the
// field held and left the row untouched when that was empty or the action it
// already carried; otherwise the text became the row's stored action. The
// draft is dropped by `updateCustomShortcut`, so a committed row reads the
// stored value back from the settings rather than the text just typed.
func (a *App) commitCustomCommand(index int, text string) {
	next := strings.TrimSpace(text)
	entries := a.customShortcuts()
	if index < 0 || index >= len(entries) || next == "" || next == entries[index].Action {
		return
	}
	a.updateCustomShortcut(index, settings.CustomShortcut{Key: entries[index].Key, Action: next})
}

// updateCustomShortcut rewrites one row's action. The picker's
// "Command line…" slot is a mode, not an action: the reference's `setAction`
// turned the row into a command draft (action empty, a text field visible)
// instead of storing the sentinel, so choosing it here carries the row's key
// into the draft and drops the binding rather than persisting the literal word
// `command` — which would otherwise run a shell command named `command`.
func (a *App) updateCustomShortcut(index int, entry settings.CustomShortcut) {
	entries := append([]settings.CustomShortcut{}, a.customShortcuts()...)
	if index < 0 || index >= len(entries) {
		return
	}
	// Whatever was typed into the row's command field is committed (or dropped
	// with the binding); the row must not keep showing a draft over a new
	// action, as the reference's `setAction` cleared `row.command` too.
	delete(a.customEdits, entries[index].Key)
	if entry.Action == commandAction {
		a.customKey, a.customCommand, a.customAction = entry.Key, "", commandAction
		a.customDraftOpen = true
		entries = append(entries[:index], entries[index+1:]...)
		a.commitCustomShortcuts(entries)
		return
	}
	entries[index] = entry
	a.commitCustomShortcuts(entries)
}

// removeCustomShortcut drops one row.
func (a *App) removeCustomShortcut(index int) {
	entries := append([]settings.CustomShortcut{}, a.customShortcuts()...)
	if index < 0 || index >= len(entries) {
		return
	}
	delete(a.customEdits, entries[index].Key)
	entries = append(entries[:index], entries[index+1:]...)
	a.commitCustomShortcuts(entries)
}

// commitCustomShortcuts hands the list to the shell, which persists it and
// registers the keys; a key the system refused is reported on its own row
// rather than left looking bound.
func (a *App) commitCustomShortcuts(entries []settings.CustomShortcut) {
	if a.Actions.SetCustomShortcuts == nil {
		return
	}
	a.customRejections = a.Actions.SetCustomShortcuts(entries)
}
