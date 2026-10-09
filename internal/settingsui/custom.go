package settingsui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
)

// The custom-shortcut section of the Shortcuts page: the bindings the user
// made, plus one row that makes a new one.
//
// The row being made is the page's own state (an immediate-mode UI keeps it
// here rather than in a component): the recorded key, the chosen action and,
// for a command line, its text. Nothing is persisted until the binding is
// whole — a key and an action — so an unfinished row can never reach the
// system.

// customSection draws the list and the new-binding row.
func (a *App) customSection(c *ui.Context, copy i18n.Settings) {
	t := c.Theme()
	entries := a.customShortcuts()
	// A row being made starts on the first action, as the old build's draft
	// row did, so the action can be chosen before the key or the other way
	// round.
	if a.customAction == "" && len(copy.ShortcutsActions) > 0 {
		a.customAction = copy.ShortcutsActions[0].ID
	}
	ui.Fieldset(c, copy.ShortcutsCustom, func() {
		ui.Text(c, copy.ShortcutsCustomHint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		if len(entries) == 0 {
			ui.Text(c, copy.ShortcutsNone).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		}
		for index, entry := range entries {
			index, entry := index, entry
			row := ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).Padding(t.Space(1), 0)
			row.Children(func() {
				ui.Text(c, shortcuts.Display(entry.Key)).Width(160).FontSize(t.FontSize)
				a.customActionPicker(c, copy, entry.Action, func(action string) {
					a.updateCustomShortcut(index, settings.CustomShortcut{Key: entry.Key, Action: action})
				})
				if ui.Button(c, copy.ShortcutsRemove).Clicked() {
					a.removeCustomShortcut(index)
				}
			})
		}

		if a.customFeedback != "" {
			ui.Text(c, a.customFeedback).FontSize(t.FontSize - 1).TextColor(t.Danger)
		}

		row := ui.Row(c).FillWidth().Gap(t.Space(2)).AlignItems(ui.Center).Padding(t.Space(1), 0)
		row.Children(func() {
			if a.customRecording {
				capture := ui.Box(c).Focusable().Padding(t.Space(1), t.Space(2)).Radius(t.Radius).
					Background(t.Surface).Border(1, t.Accent).Label(copy.ShortcutRecording)
				capture.Children(func() {
					ui.Text(c, copy.ShortcutRecording).FontSize(t.FontSize)
				})
				capture.HandleInput(func(ev ui.InputEvent) bool {
					if ev.Kind != ui.InputKeyDown {
						return false
					}
					if ev.Key == ui.KeyEscape {
						a.customRecording = false
						return true
					}
					accelerator, ok := shortcuts.FromKey(ev.Mods, ev.Key)
					if !ok {
						return true // a key without a modifier is not a shortcut
					}
					a.customRecording = false
					a.customKey = accelerator
					return true
				})
				capture.Focus()
			} else {
				label := a.customKey
				if label == "" {
					label = copy.ShortcutsRecordKey
				} else {
					label = shortcuts.Display(label)
				}
				if ui.Button(c, label).Clicked() {
					a.customRecording = true
				}
			}
			a.customActionPicker(c, copy, a.customAction, func(action string) {
				a.customAction = action
			})
			if a.customAction == commandAction {
				command := a.customCommand
				ui.TextInput(c, &command).Placeholder(copy.ShortcutsCommandHint).
					Label(copy.ShortcutsCommand).Grow(1)
				a.customCommand = command
			}
			if ui.Button(c, copy.ShortcutsAdd).Clicked() {
				a.addCustomShortcut()
			}
		})
	})
}

// commandAction is the picker value that means "a command line I type": it is
// not an action string, so it can never be stored as one.
const commandAction = "command"

// customActionPicker draws the action choice: one drop-down listing the
// plugins, the app's actions and the command-line slot.
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
	if ui.Select(c, &selected, labels).Changed() {
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
	a.customKey, a.customCommand = "", ""
}

// updateCustomShortcut rewrites one row's action.
func (a *App) updateCustomShortcut(index int, entry settings.CustomShortcut) {
	entries := append([]settings.CustomShortcut{}, a.customShortcuts()...)
	if index < 0 || index >= len(entries) {
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
	entries = append(entries[:index], entries[index+1:]...)
	a.commitCustomShortcuts(entries)
}

// commitCustomShortcuts hands the list to the shell, which persists it and
// registers the keys; a key the system refused is reported on the page rather
// than left looking bound.
func (a *App) commitCustomShortcuts(entries []settings.CustomShortcut) {
	if a.Actions.SetCustomShortcuts == nil {
		return
	}
	rejections := a.Actions.SetCustomShortcuts(entries)
	a.customFeedback = ""
	if len(rejections) > 0 {
		copy := i18n.For(a.Store.Snapshot().Language).Settings
		a.customFeedback = copy.ShortcutsRejected(rejections[0].Key, rejections[0].Reason)
	}
}
