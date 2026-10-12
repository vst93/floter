package launcher

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// The keys the launcher's search field answers itself, before the text editor
// sees them.
//
// The framework decides a key in this order (`ui`'s keyDownPhysical): the
// focused element's HandleInput, then the editor's own `wants`, then the
// shortcut registrations. A `Shortcut` registration therefore can never reach
// a key the editor wants — and a Backspace with any modifier is one of them
// (`editor.wants` answers true for it whenever the buffer is non-empty, and
// for a token field with a modifier always). `Element.HandleInput` on the
// field itself is the one seam that runs first, which is why the old build's
// two field-scoped gestures live here rather than in `modeShortcuts`:
//
//   - `CmdOrCtrl+Backspace` (⌘⌫ / Ctrl+⌫) arms and confirms the selected
//     history row's inline delete (the two-step rule of the old build's
//     `history-actions.ts`);
//   - `Ctrl+Shift+Backspace` empties the typed-command history.
//
// Both are launcher-only: the terminal keeps the keyboard in its own mode and
// the shell is waiting for both keys.
//
// The bare Backspace is the field's own gesture too, but only for one state: a
// plugin mode whose needle is already empty has nothing left to delete, so the
// press leaves the mode (see `modeExitMods` and the old build's
// `pluginModeExitOnBackspace`).
//
// Everything else returns false, so the field keeps its own editing.

// historyClearMods is Ctrl+Shift, spelled out rather than taken from the app
// modifier: the old build bound the history-clearing chord to a literal
// Control, and macOS keeps it off ⌘ so a hand trimming a command line never
// empties the history by accident.
const historyClearMods = ui.Ctrl | ui.Shift

// historyDeleteMods is the old build's `HISTORY_DELETE_SHORTCUT`,
// `CmdOrCtrl+Backspace`: ⌘⌫ on macOS, Ctrl+⌫ elsewhere — the app modifier, the
// same one the rest of the launcher's commands use. It is deliberately not a
// member of the settings' shortcut map: the user never asked to rebind it.
var historyDeleteMods = ui.Cmd

// modeExitMods are the modifiers that make a Backspace an *editing* gesture
// rather than the mode-leaving one: ⌘/⌃/⌥ all belong to the editor (delete to
// the start of the line, delete a word, delete a word on macOS), so a hand
// trimming text is never thrown out of the mode. Shift is deliberately absent —
// the old build's rule read `metaKey || ctrlKey || altKey` and nothing else, so
// ⇧⌫ is the plain key. Super is Command on macOS and the Windows key elsewhere;
// both spellings are listed because `ui.Cmd` is only one of them per platform.
const modeExitMods = ui.Super | ui.Ctrl | ui.Alt

// fieldKeys takes the launcher's own field keys. It is attached to the search
// field with HandleInput, so it runs while the field has the keyboard and
// before the editor turns a Backspace into an edit.
func (a *App) fieldKeys(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown {
		return false
	}
	// An armed delete is cancelled by any key except the one that confirms it
	// — the old build's rule, and the reason a stray press cannot be confirmed
	// later by an unrelated keystroke. Escape stops there instead of going on
	// to leave the mode, exactly as the old handler did (the field's key is
	// read before the view's).
	if a.armedDelete != "" {
		if !(ev.Key == ui.KeyBackspace && ev.Mods == historyDeleteMods) {
			a.armedDelete = ""
			return ev.Key == ui.KeyEscape
		}
	}
	if ev.Key != ui.KeyBackspace {
		return false
	}
	// Ctrl+Shift+Backspace empties the typed-command history.
	if ev.Mods == historyClearMods {
		a.clearQueryHistory()
		return true
	}
	// The two-step delete of the selected history row, claimed only while a
	// history mode owns the field and nothing is typed in it — the old build's
	// structural mitigation for putting the key on the app modifier, so a hand
	// trimming text keeps the editor's own gesture.
	if ev.Mods == historyDeleteMods {
		return a.historyDeleteKey()
	}
	// The bare key, on a field with nothing left in it: the old build's R32
	// rule — backspacing the needle to empty *stops* in the mode, and the press
	// after that leaves it (「退格删空即停」). The editor still owns the press
	// that empties the field, because `Query` is only empty once it has; this
	// branch is the one after it.
	if ev.Mods&modeExitMods == 0 && a.Query == "" && a.leavePluginMode() {
		return true
	}
	return false
}

// pluginModeActive reports whether a plugin mode owns the field: the old
// build's four scopes (the browser, the clipboard and the calculator modes,
// and an extension command's argument mode) plus this build's files mode, which
// a drop or the `files ` trigger opens. The ordinary search and the
// captured-output view are not modes.
func (a *App) pluginModeActive() bool {
	return a.files || a.calculatorMode || a.browser || a.clipboard || a.mode != nil
}

// leavePluginMode leaves whichever plugin mode owns the field, and reports
// whether there was one. It is the one ladder both ways out read — Escape in
// the view, and an empty field's Backspace here — so the order can never be
// written down twice and drift: the files mode, then the calculator, the
// browser, the clipboard, and last an extension command's arguments.
func (a *App) leavePluginMode() bool {
	switch {
	case a.files:
		a.leaveFiles()
	case a.calculatorMode:
		a.leaveCalculator()
	case a.browser:
		a.leaveBrowser()
	case a.clipboard:
		a.leaveClipboard()
	case a.mode != nil:
		a.leaveCommand()
	default:
		return false
	}
	return true
}

// clearQueryHistory empties the typed-command history and says so. The field
// keeps the draft the user is writing: only the remembered lines go.
func (a *App) clearQueryHistory() {
	a.History = nil
	a.historyIndex = -1
	a.draftBeforeHistory = ""
	a.Feedback(a.copy().HistoryCleared)
}

// historyDeleteKey arms the selected history row, or confirms an already-armed
// one. It reports whether the key belonged to the list: a press the list does
// not claim falls through to the editor, which is what keeps ⌘⌫ a "delete to
// the start of the line" while the user is writing.
func (a *App) historyDeleteKey() bool {
	if !a.historyDeleteClaimable() {
		return false
	}
	item, ok := a.selectedHistoryRow()
	if !ok {
		return false
	}
	if a.armedDelete == item.ID {
		a.armedDelete = ""
		a.deleteHistoryRow(item.ID)
		return true
	}
	// A different row re-arms on the new one: the confirmation never transfers.
	a.armedDelete = item.ID
	return true
}

// historyDeleteClaimable reports whether the delete key may be claimed right
// now: a history mode owns the field, the field holds nothing, and the
// selection sits on a runnable history row.
//
// `Query` is the mode's own needle — the trigger word was consumed on entry —
// so any typed character means "the user is editing" and the field's native
// gesture must win. With an empty field there is no text to delete, so the arm
// cannot swallow a real edit.
func (a *App) historyDeleteClaimable() bool {
	if !a.clipboard && !a.calculatorMode {
		return false
	}
	if strings.TrimSpace(a.Query) != "" {
		return false
	}
	_, ok := a.selectedHistoryRow()
	return ok
}

// selectedHistoryRow is the clipboard entry or calculation the selection is
// on: the rows the delete key may act upon. The calculator's answer row (the
// pending expression, not a stored entry) and every other kind of row are not
// one of them.
func (a *App) selectedHistoryRow() (Item, bool) {
	results := a.Results()
	if a.Selected < 0 || a.Selected >= len(results) {
		return Item{}, false
	}
	item := results[a.Selected]
	if item.clip == nil && item.calc == nil {
		return Item{}, false
	}
	return item, true
}

// deleteHistoryRow removes the history row with the given id, and hands the
// selection to the row that took its place: the neighbour steps into the slot,
// and removing the last row selects the new last one — the old build's
// `selectionAfterRemoval`, the user's 「邻居接管选中，不跳顶」. The list never
// jumps back to the top.
//
// The row is named by id rather than by index so a list that shifted under the
// arm (a clipboard watcher adding an entry, a filter chip narrowing) can never
// turn a confirmation into the removal of a row the user was not looking at.
func (a *App) deleteHistoryRow(id string) {
	results := a.Results()
	index := -1
	for i, item := range results {
		if item.ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	item := results[index]
	copy := a.copy()
	switch {
	case item.clip != nil && a.Clipboard != nil:
		if err := a.Clipboard.Remove(item.clip.ID); err != nil {
			a.WarnFeedback(copy.ClipboardDeleteFailed)
			return
		}
		a.toast = copy.ClipboardDeleted
	case item.calc != nil && a.Calculator != nil:
		if err := a.Calculator.Delete(item.calc.ID); err != nil {
			a.WarnFeedback(copy.CalculatorDeleteFailed)
			return
		}
		a.toast = copy.CalculatorDeleted
		a.refreshCalculator()
	default:
		return
	}
	a.Selected, a.chosenRow = selectionAfterRemoval(index, len(results)), -1
}

// selectionAfterRemoval is where the choice lands once the row at index leaves
// a list that had length rows: the neighbour takes the slot, the last row
// hands the choice to the new last one, and a list of one starts over.
func selectionAfterRemoval(index, length int) int {
	if length <= 1 {
		return 0
	}
	return max(0, min(index, length-2))
}

// DisarmDelete cancels a half-finished history delete, for a caller that knows
// the list has left the user's hands: the launcher surface being left, or the
// window losing the keyboard. The old build disarmed on a window blur and on
// leaving the mode for the same reason — a confirmation that waits for its
// second press must not outlive the moment it was asked for.
func (a *App) DisarmDelete() { a.armedDelete = "" }

// ArmedDelete is the id of the history row waiting for its confirming press,
// or the empty string. It exists for a caller outside the launcher (the
// shell's own tests) to see the arm the key set.
func (a *App) ArmedDelete() string { return a.armedDelete }
