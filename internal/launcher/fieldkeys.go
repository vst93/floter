package launcher

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/deadline"
	"floter/internal/settings"
	"floter/internal/shortcuts"
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

// historyStarMods is the history star's chord, the reference's `CmdOrCtrl+D`
// (`main:src/launcher.ts:461` `CLIPBOARD_FAVORITE_SHORTCUT`, `:521`
// `CALCULATOR_FAVORITE_SHORTCUT`): ⌘D on macOS, Ctrl+D elsewhere — the app
// modifier, not both at once. It is mode-local (one surface, one meaning) and
// deliberately not in the settings' shortcut map.
//
// It lives here rather than in `modeShortcuts` because the editor would take
// it first: macOS reads Ctrl+D as delete-char, so a `Shortcut` registration
// never saw the press and the star never lit (P7-iv).
var historyStarMods = ui.Cmd

// historyDeleteConfirm is the old build's `HISTORY_DELETE_CONFIRM_MS`: an armed
// row waits this long for its confirming press, then disarms itself (the old
// handler armed a timer at `HISTORY_DELETE_CONFIRM_MS + 50`). Stillness must
// not leave a destructive confirmation waiting for a stray second press.
const historyDeleteConfirm = 5 * time.Second

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
	// mygo 0.8 also delivers physical-only keys, including a modifier pressed
	// before the second delete chord. Those must not cancel its confirmation.
	if ev.Kind != ui.InputKeyDown || ev.Key == ui.KeyUnknown {
		return false
	}
	// The user's own bindings come first: a chord the settings page sold the
	// user is theirs even where the editor would type it (see appChordKeys).
	if a.appChordKeys(ev) {
		// Any key but the confirming one cancels an armed delete, chords
		// included — the old build's rule, kept in one place.
		a.disarmDelete()
		return true
	}
	// The history star's own key, claimed only while a history mode owns the
	// field — elsewhere ⌘D/Ctrl+D keeps whatever it meant. The view runs it on
	// the frame that follows, where the row it stars is the one on screen.
	if ev.Key == ui.KeyD && ev.Mods == historyStarMods && (a.calculatorMode || a.clipboard) {
		a.pendingFavorite = true
		a.disarmDelete()
		return true
	}
	// An armed delete is cancelled by any key except the one that confirms it
	// — the old build's rule, and the reason a stray press cannot be confirmed
	// later by an unrelated keystroke. Escape stops there instead of going on
	// to leave the mode, exactly as the old handler did (the field's key is
	// read before the view's).
	if a.armedDelete != "" {
		if !(ev.Key == ui.KeyBackspace && ev.Mods == historyDeleteMods) {
			a.disarmDelete()
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

// appChordKeys claims the user's rebindable chords on the field itself, and
// runs them there.
//
// The framework decides a key in this order (`ui`'s keyDownPhysical): the
// focused element's HandleInput, then the editor's own `wants`, then the
// `Shortcut` registrations. The editor wants every `Option`+printable on
// macOS — ⌥N types ñ — so a binding the user recorded as `Alt+N` never
// reached the shortcut layer at all: the settings page offered the chord and
// the app never answered it. The field is the one seam in front of the editor,
// so the launcher's own bindings are matched here.
//
// The chords are the ones the launcher's own view answers — the app keys
// (`appShortcuts`) and the numbered result family — with one substitution the
// shell makes: on the launcher surface the new-command chord hides the window
// (`shell.appShortcuts` runs before the launcher's view), which is
// `Actions.Dismiss`. Everything the user did not bind is left to the editor:
// ⌥N still types ñ.
func (a *App) appChordKeys(ev ui.InputEvent) bool {
	m := a.settings()
	for _, action := range []string{
		settings.ShortcutNewCommand,
		settings.ShortcutOpenExternalTerminal,
		settings.ShortcutOpenSettings,
	} {
		if !chordPressed(ev, settings.Shortcut(m, action)) {
			continue
		}
		a.pendingChord = action
		return true
	}
	digit, ok := chordDigit(ev, m)
	if !ok {
		return false
	}
	a.pendingDigit, a.pendingDigitSet = digit, true
	return true
}

// chordPressed reports whether the event is exactly the chord the accelerator
// spells: the same modifiers, in the same set, and the same key.
func chordPressed(ev ui.InputEvent, accelerator string) bool {
	mods, key, ok := shortcuts.Parse(accelerator)
	return ok && ev.Mods == mods && ev.Key == key
}

// chordDigit reports which digit of the result family the event is, if any.
// The family is the settings' select_result binding, so a rebind moves every
// digit with it and a chord nobody bound answers nothing.
func chordDigit(ev ui.InputEvent, m settings.Settings) (int, bool) {
	for digit := 0; digit <= 9; digit++ {
		if chordPressed(ev, settings.SelectResultDigit(m, digit)) {
			return digit, true
		}
	}
	return 0, false
}

// pluginModeActive reports whether a plugin mode owns the field: the old
// build's four scopes (the browser, the clipboard and the calculator modes,
// and an extension command's argument mode) plus this build's files mode, which
// a drop or the `files ` trigger opens. The ordinary search and the
// captured-output view are not modes.
func (a *App) pluginModeActive() bool {
	return a.files || a.calculatorMode || a.browser || a.clipboard || a.mode != nil
}

// PluginModeActive reports whether a plugin mode owns the field, for the
// shell's residency rule: a plugin mode is one of the surfaces that survives a
// dismissal for the residency window (see src/surface-residency.ts).
func (a *App) PluginModeActive() bool { return a.pluginModeActive() }

// leavePluginMode leaves whichever plugin mode owns the field, and reports
// whether there was one. It is the one ladder both ways out read — Escape in
// the view, and an empty field's Backspace here — so the order can never be
// written down twice and drift: the files mode, then the calculator, the
// browser, the clipboard, and last an extension command's arguments.
func (a *App) leavePluginMode() bool {
	// A pinned page has no way back to the search list. It is the plugin the
	// user detached — 「独立出来的窗口不应该能回退到搜索页，不能通过删除、esc
	// 回退，只能整个退出」 — so the ladder that would walk it back to the search
	// list stops here: Escape, an empty field's Backspace and ⌘W all reach this
	// one function, and all of them now do nothing. The window's own close is
	// the only exit.
	if a.pinned {
		return false
	}
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
	now := a.now()
	// The confirming press counts only inside the window; after it the arm has
	// lapsed, so the press re-arms on the same row rather than deleting (the
	// old build's `reduceHistoryDelete`, whose `same` branch also required
	// `now - armedAt <= HISTORY_DELETE_CONFIRM_MS`).
	if a.armedDelete == item.ID && now.Sub(a.armedDeleteAt) <= historyDeleteConfirm {
		a.disarmDelete()
		a.deleteHistoryRow(item)
		return true
	}
	// A different row — or the same one after the window — re-arms on the new
	// one: the confirmation never transfers.
	a.armedDelete, a.armedDeleteAt = item.ID, now
	return true
}

// expireArmedDelete disarms an arm that has waited longer than the window. It
// is called once per frame from the launcher's View, and arms one wake at the
// deadline while an arm is live so it is reached even in a still window — the
// same shape as the docked status row's own clock, and the one the toast below
// uses (P11-bq: a rebuild per refresh was 120 of them a second for an arm that
// only changes once).
func (a *App) expireArmedDelete(c *ui.Context) {
	if a.armedDelete == "" {
		return
	}
	waited := a.now().Sub(a.armedDeleteAt)
	if waited > historyDeleteConfirm {
		a.disarmDelete()
		return
	}
	deadline.Wake(c, historyDeleteConfirm-waited)
}

// now is the launcher's clock: the injected one when a test set it, the wall
// clock otherwise.
func (a *App) now() time.Time {
	if a.clock != nil {
		return a.clock()
	}
	return time.Now()
}

// disarmDelete drops the arm and its clock in one place, so the two can never
// drift apart.
func (a *App) disarmDelete() {
	a.armedDelete, a.armedDeleteAt = "", time.Time{}
	a.armedDeleteAnnounced = false
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
//
// It reads the table the frame drew (`App.drawn`) — the row on screen, which is
// the row the user is looking at when the key lands.
func (a *App) selectedHistoryRow() (Item, bool) {
	if a.Selected < 0 || a.Selected >= len(a.drawn) {
		return Item{}, false
	}
	item := a.drawn[a.Selected]
	if item.clip == nil && item.calc == nil {
		return Item{}, false
	}
	return item, true
}

// deleteHistoryRow removes the history row it is handed, and hands the
// selection to the row that took its place: the neighbour steps into the slot,
// and removing the last row selects the new last one — the old build's
// `selectionAfterRemoval`, the user's 「邻居接管选中，不跳顶」. The list never
// jumps back to the top.
//
// The row arrives as the `Item` the frame drew, and the *arm* is what makes it
// safe: the arm holds the row's id, so a list that shifted under it (a
// clipboard watcher adding an entry, a filter chip narrowing) cannot turn a
// confirmation into the removal of a row the user was not looking at — the
// confirming press re-reads the frame's table and only matches by id (see
// historyDeleteKey). The caller therefore never re-walks the catalog to find
// the row again.
func (a *App) deleteHistoryRow(item Item) {
	index := -1
	for i, drawn := range a.drawn {
		if drawn.ID == item.ID {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	length := len(a.drawn)
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
	a.Selected, a.chosenRow = selectionAfterRemoval(index, length), -1
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
func (a *App) DisarmDelete() { a.disarmDelete() }

// ArmedDelete is the id of the history row waiting for its confirming press,
// or the empty string. It exists for a caller outside the launcher (the
// shell's own tests) to see the arm the key set.
func (a *App) ArmedDelete() string { return a.armedDelete }

// PinPage makes this launcher a pinned page: it keeps whatever mode it is in
// and refuses to leave it (see `leavePluginMode`). The shell's detached window
// sets it on the second launcher it builds for a detached plugin page.
func (a *App) PinPage() { a.pinned = true }

// LeavePluginMode returns the launcher to its ordinary search page, from any
// mode: the panel's own way out when the page it was showing has been pinned
// into a window of its own (see shell.detachSurface).
//
// It is `leavePluginMode` with the caller's spelling, so the ladder of modes
// stays in one place: a second copy of that switch here would let a new mode be
// forgotten in one of the two.
func (a *App) LeavePluginMode() { a.leavePluginMode() }

// PluginTitle is the name of the page a mode is showing — the plugin's own
// name, or the command's — and the empty string on the ordinary search page.
// It is what a pinned page calls itself in its window title.
func (a *App) PluginTitle() string {
	switch {
	case a.mode != nil:
		if name := a.mode.Command.Name; name != "" {
			return name
		}
		return a.mode.Command.ID
	case a.clipboard:
		return a.copy().ScopeClipboard
	case a.browser:
		return a.copy().ScopeBrowser
	case a.calculatorMode:
		return a.copy().ScopeCalculator
	case a.files:
		return a.copy().FilesSection
	}
	return ""
}

// AdoptPage takes the page `from` is showing — its mode, its filter chip and
// the text in its field — so a second launcher can draw the same page while the
// first one goes back to the search list.
//
// It is a *page*, not a state: the selection, the scroll offset, the armed
// delete and the query history stay with the instance that owns them. A pinned
// page starts at its own first row, which is the rule every second window in
// this app follows (`showOutput` resets its scroller for the same reason).
func (a *App) AdoptPage(from *App) {
	if from == nil {
		return
	}
	a.mode, a.modeWord = nil, ""
	a.clipboard, a.browser, a.calculatorMode, a.files = false, false, false, false
	switch {
	case from.mode != nil:
		entry := *from.mode
		a.mode = &entry
		a.modeWord = from.modeWord
	case from.clipboard:
		a.clipboard, a.clipboardFilter = true, from.clipboardFilter
		a.modeWord = from.modeWord
	case from.browser:
		a.browser, a.browserFilter = true, from.browserFilter
		a.modeWord = from.modeWord
	case from.calculatorMode:
		a.calculatorMode, a.calculatorFilter = true, from.calculatorFilter
		a.modeWord = from.modeWord
	case from.files:
		a.files = true
		a.modeWord = from.modeWord
	}
	// The page's *content* comes across too, not just its mode: the needle, the
	// chosen row, the scroll the user had walked to and the mode's own data. The
	// user's 「独立切换时要保留当前的操作结果，最好就是把插件本体独立出来，不影响接着
	// 操作」 is exactly this — the pinned page must open where the panel was, with
	// the same rows under the same choice, so the next keystroke continues the
	// operation instead of starting it again.
	a.Query = from.Query
	a.Selected, a.chosenRow = from.Selected, from.chosenRow
	// The caret stays where it was. `pendingCaret` would put it at the end of
	// the field on the next frame, which is a visible change to a page the user
	// only asked to *move* — 「除了形式变化，其他东西应该都不动」. It is set only
	// when the field's own text is being replaced from outside (a trigger word
	// being consumed, a mode being entered); a pin moves the window, not the
	// caret.
	a.pendingCaret = false
	// The default-selection marker comes across with the choice it guards, so
	// the first frame of the pinned page does not re-run `defaultSelection` and
	// move the highlight off the row the user was on.
	a.defaultedQuery = from.defaultedQuery
	// The typed-command history and the line it is showing: ↓ on a pinned page
	// walks the same history it walked in the panel.
	a.History = from.History
	a.historyIndex = from.historyIndex
	a.draftBeforeHistory = from.draftBeforeHistory
	// The rows the page was showing, so the pinned window opens on the same
	// list under the same choice rather than on a re-fetch that could come back
	// different. The mode's own data comes with it for the same reason.
	a.drawn = from.drawn
	a.calculatorFound = from.calculatorFound
	a.browserFound = from.browserFound
	a.Output = from.Output
	a.output = from.output
	a.OutputList = from.OutputList
	a.List = from.List
	// The armed delete belongs to the row it was armed on. A pinned page starts
	// with none: a half-finished confirmation is a gesture in the panel, and a
	// second window must not be able to complete it.
	a.armedDelete, a.armedDeleteAt, a.armedDeleteAnnounced = "", time.Time{}, false
}

// PageKey is the identity of the page the launcher is showing: the plugin's own
// id, or the empty string on the ordinary search page (which is not a page the
// user can pin).
//
// It exists because every plugin page — and the search list itself — is the one
// `SurfaceLauncher` surface: a window keyed on the surface could not tell two
// plugin pages apart, so pinning a second one raised the first one's window
// instead of opening its own.
func (a *App) PageKey() string {
	switch {
	case a.mode != nil:
		return "command\x00" + a.mode.IntegrationID + "\x00" + a.mode.Command.ID
	case a.clipboard:
		return "clipboard"
	case a.browser:
		return "browser"
	case a.calculatorMode:
		return "calculator"
	case a.files:
		return "files"
	}
	return ""
}
