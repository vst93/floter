package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/calculator"
	"floter/internal/i18n"
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
