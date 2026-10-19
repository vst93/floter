package launcher

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/calculator"
	"floter/internal/drops"
	"floter/internal/i18n"
	"floter/internal/settings"
)

// The two keys the search field answers before the editor sees them: the
// history row's two-step delete (⌘⌫ / Ctrl+⌫) and the chord that empties the
// typed-command history (⌃⇧⌫). The field is a live text input, so a bare
// Backspace must keep editing it — the modifier is what makes the intent
// deliberate, and the delete is claimed only while the field is empty.

// deleteKey presses the app modifier's Backspace, whatever it is on the
// platform under test.
func deleteKey(tt *ui.Tester) {
	tt.Key(historyDeleteMods, ui.KeyBackspace)
}

func TestHistoryDeleteKeyArmsAndConfirmsTheSelectedClipboardRow(t *testing.T) {
	a, store := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()

	if got := len(store.Entries()); got != 2 {
		t.Fatalf("entries = %d, want 2", got)
	}
	armed := a.Results()[a.Selected].ID

	// The first press arms the row: the muted note stands in for the trailing
	// chrome, and nothing is removed yet.
	deleteKey(tt)
	tt.Frame()
	if a.armedDelete != armed {
		t.Fatalf("armed = %q, want %q", a.armedDelete, armed)
	}
	if !tt.HasText(i18n.For("en").Launcher.DeleteConfirm) {
		t.Fatalf("the armed note is missing: %q", tt.Texts())
	}
	if got := len(store.Entries()); got != 2 {
		t.Errorf("the first press removed a row: %d entries", got)
	}
	// The key was the list's, not the field's: an empty field has nothing to
	// edit, so it must not have been read as one either.
	if a.Query != "" {
		t.Errorf("the delete key typed into the field: %q", a.Query)
	}

	// The second press confirms.
	deleteKey(tt)
	tt.Frame()
	if got := len(store.Entries()); got != 1 {
		t.Fatalf("the confirm did not land: %d entries", got)
	}
	if a.armedDelete != "" {
		t.Errorf("the arm survived its own confirmation: %q", a.armedDelete)
	}
	if !a.clipboard {
		t.Error("deleting a row left the mode")
	}
	if !tt.HasText(i18n.For("en").Launcher.ClipboardDeleted) {
		t.Errorf("the feedback line is missing: %q", tt.Texts())
	}
}

// The calculator mode's rows carry the same key.
func TestHistoryDeleteKeyDeletesACalculation(t *testing.T) {
	a := testApp()
	fixture := &calculatorFixture{entries: []calculator.Entry{
		{ID: "1", Expression: "10*3", Result: "30", CreatedAt: 2},
	}}
	a.Calculator = fixture
	tt := render(t, a)
	a.EnterCalculator()
	tt.Frame()

	deleteKey(tt)
	tt.Frame()
	deleteKey(tt)
	tt.Frame()
	if len(fixture.deleted) != 1 || fixture.deleted[0] != "1" {
		t.Errorf("deleted %v", fixture.deleted)
	}
}

// The key belongs to the list only while the field holds nothing: a hand
// trimming what it typed keeps the editor's own gesture (⌘⌫ deletes to the
// start of the line on macOS, Ctrl+⌫ deletes a word elsewhere), and a row is
// never armed by it.
func TestHistoryDeleteKeyLeavesATypedFieldItsOwnGesture(t *testing.T) {
	a, store := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()
	tt.Type("first clip")
	tt.Frame()

	deleteKey(tt)
	tt.Frame()
	if a.armedDelete != "" {
		t.Errorf("a typed field armed the delete: %q", a.armedDelete)
	}
	if got := len(store.Entries()); got != 2 {
		t.Errorf("a typed field removed a row: %d entries", got)
	}
	// The editor took the press as an edit, as it always has.
	if a.Query == "first clip" {
		t.Errorf("the field did not edit: %q", a.Query)
	}
}

// The ordinary page has no history rows, and the browser and command modes
// have their own lists: the key is the clipboard's and the calculator's.
func TestHistoryDeleteKeyLeavesOtherSurfacesAlone(t *testing.T) {
	a, store := clipboardApp(t)
	tt := render(t, a)
	tt.Frame()
	deleteKey(tt)
	tt.Frame()
	if a.armedDelete != "" {
		t.Errorf("the ordinary page armed a delete: %q", a.armedDelete)
	}
	if got := len(store.Entries()); got != 2 {
		t.Errorf("the ordinary page removed a row: %d entries", got)
	}
}

// The calculator's answer row — the pending expression, not a stored entry —
// is not a history row and cannot be armed.
func TestHistoryDeleteKeySkipsTheCalculatorAnswerRow(t *testing.T) {
	a := testApp()
	fixture := &calculatorFixture{entries: []calculator.Entry{
		{ID: "1", Expression: "10*3", Result: "30", CreatedAt: 2},
	}}
	a.Calculator = fixture
	tt := render(t, a)
	a.EnterCalculator()
	tt.Frame()
	tt.Type("2+2")
	tt.Frame()
	if a.Selected != 0 {
		t.Fatalf("selected = %d, want the answer row", a.Selected)
	}
	if item, ok := a.selectedHistoryRow(); ok {
		t.Fatalf("the answer row reads as a history row: %+v", item)
	}
	deleteKey(tt)
	tt.Frame()
	if a.armedDelete != "" {
		t.Errorf("the answer row armed a delete: %q", a.armedDelete)
	}
	if len(fixture.deleted) != 0 {
		t.Errorf("the answer row deleted something: %v", fixture.deleted)
	}
}

// An armed delete is cancelled by any key but the one that confirms it, so a
// stray press cannot be confirmed minutes later by an unrelated keystroke.
func TestAnArmedDeleteIsCancelledByAnyOtherKey(t *testing.T) {
	a, store := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()

	deleteKey(tt)
	tt.Frame()
	if a.armedDelete == "" {
		t.Fatal("the first press did not arm")
	}
	// A plain letter cancels it, and lands in the field as it always does.
	tt.TypeKey(0, ui.KeyX, "x")
	tt.Frame()
	if a.armedDelete != "" {
		t.Errorf("typing did not disarm: %q", a.armedDelete)
	}
	if got := len(store.Entries()); got != 2 {
		t.Errorf("typing removed a row: %d entries", got)
	}
	if a.Query != "x" {
		t.Errorf("the letter did not reach the field: %q", a.Query)
	}
}

// Escape disarms and stops there: the first press must not also leave the mode.
func TestEscapeDisarmsWithoutLeavingTheMode(t *testing.T) {
	a, store := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()

	deleteKey(tt)
	tt.Frame()
	if a.armedDelete == "" {
		t.Fatal("the press did not arm")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.armedDelete != "" {
		t.Errorf("Escape did not disarm: %q", a.armedDelete)
	}
	if !a.clipboard {
		t.Error("Escape while armed left the mode as well")
	}
	if got := len(store.Entries()); got != 2 {
		t.Errorf("Escape removed a row: %d entries", got)
	}
}

// Removing a row hands the choice to the row that took its place — the old
// build's 「邻居接管选中，不跳顶」: the list never jumps back to the top.
func TestDeletingAHistoryRowKeepsTheNeighbourSelected(t *testing.T) {
	a, store := clipboardApp(t)
	if _, _, err := store.AddText("third clip"); err != nil {
		t.Fatal(err)
	}
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()
	if got := len(store.Entries()); got != 3 {
		t.Fatalf("entries = %d, want 3", got)
	}

	// The middle row: its neighbour steps into the slot.
	a.Selected = 1
	middle := a.Results()[1].ID
	deleteKey(tt)
	tt.Frame()
	deleteKey(tt)
	tt.Frame()
	if got := len(store.Entries()); got != 2 {
		t.Fatalf("entries = %d, want 2", got)
	}
	if a.Selected != 1 {
		t.Errorf("selected = %d after removing the middle row, want 1", a.Selected)
	}
	if a.Selected < len(store.Entries()) && a.Results()[a.Selected].ID == middle {
		t.Error("the removed row is still under the choice")
	}

	// The last row: the new last one takes the choice rather than the list
	// jumping to the top.
	a.Selected = 1
	deleteKey(tt)
	tt.Frame()
	deleteKey(tt)
	tt.Frame()
	if got := len(store.Entries()); got != 1 {
		t.Fatalf("entries = %d, want 1", got)
	}
	if a.Selected != 0 {
		t.Errorf("selected = %d after removing the last row, want 0", a.Selected)
	}
}

// The arm names its row by id, so a list that shifted under it (a clipboard
// watcher adding an entry) cannot turn a confirmation into the removal of a
// row the user was not looking at: the press re-arms on the row now under the
// choice.
func TestAnArmedDeleteNeverConfirmsOnADifferentRow(t *testing.T) {
	a, store := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()
	armed := a.Results()[0].ID

	deleteKey(tt)
	tt.Frame()
	if a.armedDelete != armed {
		t.Fatalf("armed = %q, want %q", a.armedDelete, armed)
	}

	// A new entry arrives at the top, shifting the armed row one place down.
	if _, _, err := store.AddText("third clip"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	deleteKey(tt)
	tt.Frame()
	if got := len(store.Entries()); got != 3 {
		t.Fatalf("the shifted list confirmed on a row: %d entries", got)
	}
	if a.armedDelete != a.Results()[0].ID {
		t.Errorf("armed = %q, want the row now under the choice %q", a.armedDelete, a.Results()[0].ID)
	}
	if a.armedDelete == armed {
		t.Error("the arm transferred to the shifted row")
	}
}

// A half-finished delete does not outlive its mode: leaving and coming back
// with the same row under the choice must not let one press confirm an arm the
// user had already walked away from.
func TestAnArmedDeleteLeavesWithTheMode(t *testing.T) {
	a, store := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()

	deleteKey(tt)
	tt.Frame()
	if a.armedDelete == "" {
		t.Fatal("the press did not arm")
	}

	a.ResetQuery()
	tt.Frame()
	if a.armedDelete != "" {
		t.Fatalf("the arm survived leaving the mode: %q", a.armedDelete)
	}

	// Back in the mode with the same row under the choice: the first press
	// arms rather than confirms.
	a.EnterClipboard()
	tt.Frame()
	deleteKey(tt)
	tt.Frame()
	if got := len(store.Entries()); got != 2 {
		t.Errorf("one press after re-entry removed a row: %d entries", got)
	}
	if a.armedDelete == "" {
		t.Error("the press after re-entry did not arm")
	}
}

// Ctrl+Shift+Backspace empties the typed-command history and says so. The
// chord is launcher-only and takes a literal Control, so the terminal keeps
// the key for the shell.
func TestCtrlShiftBackspaceClearsTheQueryHistory(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	a.History = []string{"open terminal", "clip"}
	a.historyIndex = 0
	a.draftBeforeHistory = "half typed"
	tt.Frame()

	tt.Key(historyClearMods, ui.KeyBackspace)
	tt.Frame()
	if len(a.History) != 0 {
		t.Errorf("history = %v, want empty", a.History)
	}
	if a.historyIndex != -1 || a.draftBeforeHistory != "" {
		t.Errorf("the history walk was left open: %d %q", a.historyIndex, a.draftBeforeHistory)
	}
	if !tt.HasText(i18n.For("en").Launcher.HistoryCleared) {
		t.Errorf("the feedback line is missing: %q", tt.Texts())
	}
	// The field's own text is not the history's: only the remembered lines go.
	if a.Query != "" {
		t.Errorf("the field was emptied as well: %q", a.Query)
	}
}

// The two keys are the field's, not the window's: a shortcut registration
// would be asked after the editor, which is why they live on the field.
func TestTheFieldOwnsTheHistoryKeys(t *testing.T) {
	if historyDeleteMods != ui.Cmd {
		t.Errorf("historyDeleteMods = %v, want the app modifier %v", historyDeleteMods, ui.Cmd)
	}
	if historyClearMods != ui.Ctrl|ui.Shift {
		t.Errorf("historyClearMods = %v, want Ctrl+Shift", historyClearMods)
	}
	// The rule the claim rests on, pinned: the neighbour takes the slot, the
	// last row hands the choice to the new last one, and a list of one starts
	// over.
	for _, tc := range []struct{ index, length, want int }{
		{0, 3, 0}, {1, 3, 1}, {2, 3, 1}, {0, 1, 0}, {4, 5, 3},
	} {
		if got := selectionAfterRemoval(tc.index, tc.length); got != tc.want {
			t.Errorf("selectionAfterRemoval(%d, %d) = %d, want %d", tc.index, tc.length, got, tc.want)
		}
	}
}

// The bare Backspace, when a plugin mode owns a field with nothing left in it,
// is the deliberate "there is nothing left to delete" that leaves the mode —
// the old build's R32 rule (「退格删空即停」: the press that empties the needle
// stops in the mode, the press after it walks out). It is the third key the
// field answers before the editor: the editor never sees a press that has no
// edit to make.
func TestEmptyFieldBackspaceLeavesTheClipboardMode(t *testing.T) {
	a, store := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the mode did not open")
	}

	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if a.clipboard {
		t.Error("an empty field's Backspace did not leave the mode")
	}
	// The press is a way out, not an edit or an execution: the history is
	// exactly as it was, and no row ran.
	if got := len(store.Entries()); got != 2 {
		t.Errorf("leaving the mode removed an entry: %d", got)
	}
	if testRuns["copy"] != 0 {
		t.Errorf("leaving the mode ran a row: %v", testRuns)
	}
}

// The rule's two steps, in the order the user performs them: the press that
// empties the needle is the editor's (the mode keeps its ground), and only the
// press after it leaves.
func TestOnlyThePressAfterTheEmptyOneLeavesTheMode(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()

	tt.Type("a")
	tt.Frame()
	if a.Query != "a" {
		t.Fatalf("Query = %q, want the typed needle", a.Query)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the press that emptied the needle left the mode")
	}
	if a.Query != "" {
		t.Fatalf("the press did not edit the field: %q", a.Query)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if a.clipboard {
		t.Error("the press after the field emptied did not leave the mode")
	}
}

// A bare Backspace with a needle still in the field is the editor's, not the
// mode's: only an empty field means "nothing left to delete".
func TestBackspaceWithANeedleKeepsTheEditorAndTheMode(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()

	tt.Type("xyz")
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if !a.clipboard {
		t.Fatal("a Backspace with text in the field left the mode")
	}
	if a.Query != "xy" {
		t.Errorf("Query = %q, want the editor's own edit", a.Query)
	}
}

// Every plugin mode answers the empty field's Backspace, and it is one ladder:
// the files mode a drop opens, the calculator, the browser, the clipboard and an
// extension command's argument mode. The captured output view is not one of
// them — Escape closes it, and a Backspace in its field edits.
func TestEmptyFieldBackspaceLeavesEveryPluginMode(t *testing.T) {
	cases := []struct {
		name string
		open func(*App)
		in   func(*App) bool
	}{
		{"clipboard", func(a *App) { a.EnterClipboard() }, func(a *App) bool { return a.clipboard }},
		{"browser", func(a *App) { a.EnterBrowser() }, func(a *App) bool { return a.browser }},
		{"calculator", func(a *App) { a.EnterCalculator() }, func(a *App) bool { return a.calculatorMode }},
		{"files", func(a *App) { a.EnterFiles() }, func(a *App) bool { return a.files }},
		{"command", func(a *App) { a.enterCommand(a.Commands[0]) }, func(a *App) bool { return a.mode != nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			if tc.name == "command" {
				a = commandApp()
			}
			tt := render(t, a)
			tc.open(a)
			tt.Frame()
			if !tc.in(a) {
				t.Fatalf("%s: the mode did not open", tc.name)
			}
			tt.Key(0, ui.KeyBackspace)
			tt.Frame()
			if tc.in(a) {
				t.Errorf("%s: an empty field's Backspace did not leave the mode", tc.name)
			}
			if !a.pluginModeActive() {
				return
			}
			t.Errorf("%s: a mode is still active after the press", tc.name)
		})
	}
}

// The exported predicate the shell's residency rule reads is the same fact the
// field's own Backspace reads: a mode owns the field. The ordinary search page
// and the captured-output view are not modes.
func TestPluginModeActiveNamesTheModes(t *testing.T) {
	a := testApp()
	if a.PluginModeActive() {
		t.Fatal("the ordinary search page reported a plugin mode")
	}
	for _, open := range []func(*App){
		func(a *App) { a.EnterClipboard() },
		func(a *App) { a.EnterBrowser() },
		func(a *App) { a.EnterCalculator() },
		func(a *App) { a.EnterFiles() },
	} {
		b := testApp()
		open(b)
		if !b.PluginModeActive() {
			t.Errorf("PluginModeActive = false with %T active", open)
		}
		if b.PluginModeActive() != b.pluginModeActive() {
			t.Errorf("the exported predicate and the internal one disagree")
		}
	}
	cmd := commandApp()
	cmd.enterCommand(cmd.Commands[0])
	if !cmd.PluginModeActive() {
		t.Error("an extension command's argument mode is not a plugin mode")
	}
}

// ClearDropped forgets the rows the files mode lists, the half of a fresh
// summon that takes the previous interaction's drop with it.
func TestClearDroppedForgetsTheRows(t *testing.T) {
	a := testApp()
	a.SetDropped([]drops.File{{Path: "/tmp/a.txt", Name: "a.txt", Directory: "/tmp"}})
	a.EnterFiles()
	if len(a.Dropped) != 1 {
		t.Fatalf("the drop did not land: %d rows", len(a.Dropped))
	}
	a.ClearDropped()
	if len(a.Dropped) != 0 {
		t.Errorf("ClearDropped left %d rows", len(a.Dropped))
	}
}

// The editor's own modifiers keep their gestures: ⌘/⌃/⌥ + Backspace is a
// deletion, so it must never walk the user out of a mode — ⌘⌫ on a history row
// is the two-step delete, and the field's own line-editing gestures are the
// rest.
func TestModifiedBackspaceKeepsItsOwnGesture(t *testing.T) {
	for _, mods := range []ui.Modifiers{ui.Cmd, ui.Super, ui.Ctrl, ui.Alt} {
		a, store := clipboardApp(t)
		tt := render(t, a)
		a.EnterClipboard()
		tt.Frame()
		tt.Key(mods, ui.KeyBackspace)
		tt.Frame()
		if !a.clipboard {
			t.Errorf("%v+Backspace left the mode", mods)
		}
		if got := len(store.Entries()); got != 2 {
			t.Errorf("%v+Backspace removed an entry: %d", mods, got)
		}
	}
}

// Shift is not one of the editor's modifiers in this rule — the old build read
// `metaKey || ctrlKey || altKey` and nothing else — so ⇧⌫ is the plain key and
// leaves the mode.
func TestShiftBackspaceIsThePlainKey(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()
	if modeExitMods&ui.Shift != 0 {
		t.Fatal("the rule blocks Shift, which the old build did not")
	}
	tt.Key(ui.Shift, ui.KeyBackspace)
	tt.Frame()
	if a.clipboard {
		t.Error("Shift+Backspace did not leave the mode")
	}
}

// The ordinary search is not a mode: an empty field's Backspace edits nothing
// and leaves the window alone, exactly as it always did.
func TestEmptyFieldBackspaceOnTheOrdinaryPageChangesNothing(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if a.Query != "" {
		t.Errorf("Query = %q, want it untouched", a.Query)
	}
	if a.pluginModeActive() {
		t.Error("the ordinary page reads as a mode")
	}
	if _, hidden := testRuns["dismiss"]; hidden {
		t.Errorf("the press hid the window: %v", testRuns)
	}
}

// disarmArmClock installs a clock the test drives by hand, so the
// history-delete window can be exercised without waiting five seconds. The
// launcher reads `a.clock` at the moment it arms and at every frame, so moving
// the returned time between frames is what a real wait does.
func disarmArmClock(a *App) *time.Time {
	now := time.Unix(1_700_000_000, 0)
	a.clock = func() time.Time { return now }
	return &now
}

// The arm is not forever: the old build's `HISTORY_DELETE_CONFIRM_MS` window
// disarms a row that has waited too long for its confirming press, so a
// confirmation asked for one moment never outlives it.
func TestTheArmedDeleteExpiresOnItsOwn(t *testing.T) {
	a, store := clipboardApp(t)
	now := disarmArmClock(a)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()

	deleteKey(tt)
	tt.Frame()
	if a.armedDelete == "" {
		t.Fatal("the first press did not arm")
	}
	if !tt.HasText(i18n.For("en").Launcher.DeleteConfirm) {
		t.Fatalf("the armed note is missing: %q", tt.Texts())
	}

	// A wait longer than the window, with nothing else happening: the next
	// frame disarms the row and the muted note leaves with it.
	*now = now.Add(historyDeleteConfirm + time.Second)
	tt.Frame()
	if a.armedDelete != "" {
		t.Errorf("the arm outlived its window: %q", a.armedDelete)
	}
	if tt.HasText(i18n.For("en").Launcher.DeleteConfirm) {
		t.Errorf("the armed note outlived its window: %q", tt.Texts())
	}
	if got := len(store.Entries()); got != 2 {
		t.Errorf("the timeout removed a row: %d entries", got)
	}
}

// A press that arrives after the window does not delete: the lapsed arm is
// re-armed on the same row, exactly as the old build's `reduceHistoryDelete`
// re-armed rather than confirming past `HISTORY_DELETE_CONFIRM_MS`.
func TestAPressAfterTheWindowReArmsInsteadOfDeleting(t *testing.T) {
	a, store := clipboardApp(t)
	now := disarmArmClock(a)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()
	armed := a.Results()[a.Selected].ID

	deleteKey(tt)
	tt.Frame()

	*now = now.Add(historyDeleteConfirm + time.Second)
	deleteKey(tt)
	tt.Frame()
	if got := len(store.Entries()); got != 2 {
		t.Errorf("a press after the window deleted a row: %d entries", got)
	}
	if a.armedDelete != armed {
		t.Errorf("armed = %q, want the same row re-armed %q", a.armedDelete, armed)
	}
}

// Inside the window the second press still confirms: the timeout bounds how
// long an arm waits, it does not change the gesture.
func TestAConfirmInsideTheWindowStillDeletes(t *testing.T) {
	a, store := clipboardApp(t)
	now := disarmArmClock(a)
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()

	deleteKey(tt)
	tt.Frame()
	*now = now.Add(2 * time.Second)
	deleteKey(tt)
	tt.Frame()
	if got := len(store.Entries()); got != 1 {
		t.Errorf("the confirm inside the window did not land: %d entries", got)
	}
	if a.armedDelete != "" {
		t.Errorf("the arm survived its confirmation: %q", a.armedDelete)
	}
}

// The ladder is written once: Escape and the empty field's Backspace leave the
// same mode for the same reason, and `leavePluginMode` answers "was there one"
// for the ordinary page.
func TestLeavePluginModeIsTheOnlyLadder(t *testing.T) {
	a := testApp()
	if a.leavePluginMode() {
		t.Error("the ordinary page reported a mode")
	}
	for _, open := range []func(*App){
		func(a *App) { a.EnterClipboard() },
		func(a *App) { a.EnterBrowser() },
		func(a *App) { a.EnterCalculator() },
		func(a *App) { a.EnterFiles() },
		func(a *App) { a.enterCommand(a.Commands[0]) },
	} {
		a := commandApp()
		open(a)
		if !a.pluginModeActive() {
			t.Fatalf("the mode did not open: %+v", a)
		}
		if !a.leavePluginMode() {
			t.Error("the ladder reported no mode while one was open")
		}
		if a.pluginModeActive() {
			t.Error("the ladder left a mode open")
		}
		if a.leavePluginMode() {
			t.Error("the ladder reported a second mode")
		}
	}
}

// The settings page sells the launcher's own keys as rebindable, and the field
// is the seam that keeps that promise: the framework asks the focused
// element's HandleInput before the editor's `wants`, so a chord recorded as an
// `Alt`+letter — which macOS's editor types as `ñ` for ⌥N — still answers.
// The press is claimed before the editor, so the action runs and the field
// keeps the text it had.
func TestTheFieldsOwnBindingsAnswerThroughTheField(t *testing.T) {
	a := testApp()
	a.Query = "app"
	tt := render(t, a)
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutNewCommand, "Alt+N")
		s.SetShortcut(settings.ShortcutOpenSettings, "Alt+Shift+O")
	}); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// The new command: on the launcher surface the shell answers it by hiding
	// the window, which is the launcher's own Dismiss.
	tt.Key(ui.Alt, ui.KeyN)
	tt.Frame()
	if got := testRuns["dismiss"]; got != 1 {
		t.Errorf("the rebound new command hid the window %d times, want 1", got)
	}
	if a.Query != "app" {
		t.Errorf("the chord typed into the field: %q", a.Query)
	}

	// Open settings, on the same seam.
	tt.Key(ui.Alt|ui.Shift, ui.KeyO)
	tt.Frame()
	if got := testRuns["settings"]; got != 1 {
		t.Errorf("the rebound open-settings opened %d times, want 1", got)
	}
	if a.Query != "app" {
		t.Errorf("the chord typed into the field: %q", a.Query)
	}

	// A chord the user gave up answers nothing: ⌥⇧P was never bound.
	tt.Key(ui.Alt|ui.Shift, ui.KeyP)
	tt.Frame()
	if testRuns["dismiss"] != 1 || testRuns["settings"] != 1 {
		t.Errorf("an unbound chord ran something: %v", testRuns)
	}
}

// The numbered family rides the same seam: a rebind to an `Alt`+digit spelling
// still runs the row the badge prints, and the field keeps its text.
func TestTheNumberedFamilyAnswersThroughTheField(t *testing.T) {
	a := testApp()
	found := make([]apps.App, 0, 3)
	for i := 0; i < 3; i++ {
		found = append(found, apps.App{Name: fmt.Sprintf("App %02d", i), Path: fmt.Sprintf("/Applications/App %02d.app", i)})
	}
	a.SetApps(found)
	ran := []string{}
	a.Actions.OpenApp = func(app apps.App) { ran = append(ran, app.Name) }

	tt := render(t, a)
	a.Query = "app"
	tt.Frame()
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutSelectResult, "Alt+1")
	}); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	tt.Key(ui.Alt, ui.Key3)
	tt.Frame()
	if len(ran) != 1 || ran[0] != "App 02" {
		t.Fatalf("ran %v, want the third row", ran)
	}
	if a.Query != "app" {
		t.Errorf("the chord typed into the field: %q", a.Query)
	}

	// The digit the family no longer holds answers nothing: ⌥5 is unbound.
	tt.Key(ui.Alt, ui.Key5)
	tt.Frame()
	if len(ran) != 1 {
		t.Errorf("an unbound digit ran a row: %v", ran)
	}
}
