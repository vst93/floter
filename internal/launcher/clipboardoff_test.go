package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// clipboardOff turns the history off in the app's settings, the state the
// plugin's own switch produces.
func clipboardOff(t *testing.T, a *App) {
	t.Helper()
	err := a.Store.Update(func(s *settings.Settings) {
		state := settings.ClipboardOf(*s)
		state.Enabled = false
		s.SetClipboard(state)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The clipboard mode with the plugin switched off says so: the reference
// answers the off switch *before* it reads the history
// (`useLauncherCatalog.ts:814-816`), emitting one
// `clipboardStatusRow("clipboard-disabled", "clipboard.pageUnavailable")`
// instead of the list. The mode must not fall through to "nothing copied yet"
// — a different, and false, statement about the same screen.
func TestTheClipboardModeSaysWhenTheHistoryIsOff(t *testing.T) {
	a, store := clipboardApp(t)
	if _, _, err := store.AddText("a third clip"); err != nil {
		t.Fatal(err)
	}
	clipboardOff(t, a)

	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()

	copy := a.copy()
	if !tt.HasText(copy.ClipboardDisabled) {
		t.Fatalf("the switched-off note is missing: %q", tt.Texts())
	}
	if tt.HasText(copy.ClipboardEmpty) {
		t.Errorf("a switched-off history claimed nothing was copied: %q", tt.Texts())
	}
	// The history is not listed: the note is the whole mode.
	for _, text := range []string{"first clip", "second clip with a secret", "a third clip"} {
		if tt.HasText(text) {
			t.Errorf("the history row %q survived the off switch: %q", text, tt.Texts())
		}
	}
	results := a.Results()
	if len(results) != 1 {
		t.Fatalf("the mode holds %d rows, want the note alone: %+v", len(results), results)
	}
	note := results[0]
	// The id is the reference's own (`clipboard-disabled`), and the row is a
	// status line: information, not a door.
	if note.ID != ClipboardDisabled {
		t.Errorf("note id = %q, want %q", note.ID, ClipboardDisabled)
	}
	if !note.status {
		t.Errorf("the note is not marked as a status line: %+v", note)
	}
	if note.Run != nil {
		t.Errorf("the note is runnable: %+v", note)
	}
	// No number: the rows the modifier-plus-digit family can reach are the
	// runnable ones, and a note is not one.
	if number, ok := a.numbers[0]; ok {
		t.Errorf("the note took the number %d: %v", number, a.numbers)
	}
}

// The note's condition is the plugin's own switch: with the history on, the
// same screen lists the history and says nothing about being off.
func TestTheClipboardNoteOnlyAppearsWhenTheHistoryIsOff(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if tt.HasText(a.copy().ClipboardDisabled) {
		t.Errorf("an enabled history showed the off note: %q", tt.Texts())
	}
	if !tt.HasText("first clip") {
		t.Errorf("the enabled history is not listed: %q", tt.Texts())
	}
}

// The note is charged the notes' own band rather than a result row's floor:
// the reference's `launcherRowHeightUnits` prices a `status` item at
// `LAUNCHER_STATUS_UNITS` (`App.tsx:2165-2166`), which `Geometry.Height`
// charges for the row `windowRows` marks `Status`.
func TestTheClipboardOffNoteIsChargedTheStatusBand(t *testing.T) {
	a, _ := clipboardApp(t)
	clipboardOff(t, a)
	a.EnterClipboard()
	tt := render(t, a)
	tt.Frame()
	if len(a.Rows) != 1 {
		t.Fatalf("the window table is %+v, want one row", a.Rows)
	}
	if !a.Rows[0].Status {
		t.Errorf("the note is not priced as a status note: %+v", a.Rows[0])
	}
	// It draws as a note, not as a block's heading: the two are priced
	// differently (a heading is a section band, a note its own 30u).
	if a.Rows[0].Heading {
		t.Errorf("the note is drawn as a section heading: %+v", a.Rows[0])
	}
	// The note's own band is `launcherStatusUnits` spacing units, shorter than
	// a one-line result row's floor: the same note drawn as an ordinary row
	// would be charged `rowUnits` and the window would be the taller.
	geometry := func(rows []RowInfo) Geometry {
		return Geometry{
			Font: a.Font, Spacing: a.Spacing, Rows: rows, Held: len(rows),
			TitleLine: a.TitleLine, SubtitleLine: a.SubtitleLine,
		}
	}
	note := geometry(a.Rows).Height()
	plain := geometry([]RowInfo{{Lines: 1}}).Height()
	if note != plain-int((rowUnits-launcherStatusUnits)*a.Spacing) {
		t.Errorf("the note prices the window at %d; a status note is %d short of a one-line row (%d)",
			note, int((rowUnits-launcherStatusUnits)*a.Spacing), plain)
	}
	// And it is a band row in the count the window holds, not a row plus a
	// band: one note is one row of the ten-row budget.
	if a.HeldRows != 1 {
		t.Errorf("the note holds %d rows, want 1", a.HeldRows)
	}
}

// The note is read off the settings, not off the history store: the setting
// turns the watcher off, not the file, so a mode that trusted the store's mere
// presence would list a history the app has stopped recording. The note is
// therefore drawn even when there is no store at all.
func TestTheClipboardOffNoteSurvivesNoStore(t *testing.T) {
	a, _ := clipboardApp(t)
	clipboardOff(t, a)
	a.Clipboard = nil
	a.EnterClipboard()
	tt := render(t, a)
	tt.Frame()
	if !tt.HasText(a.copy().ClipboardDisabled) {
		t.Fatalf("the note is missing with no store: %q", tt.Texts())
	}
}
