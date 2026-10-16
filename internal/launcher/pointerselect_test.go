package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
)

// R21 · The pointer takes the selection over on *entry*, the reference's own
// rule (`main:src/launcher/LauncherResults.tsx:859-862`: `onPointerEnter={() =>
// onSelectResult(index)}`), and there is **no** separate pointer-only pane on
// top of the selection — that second highlight was the conflict the user
// reported (`main:src/styles/launcher.css:1653-1663`). The port had the
// opposite: the keyboard's choice stayed put and the pointer painted its own
// `--glass-control-hover` band, so two rows could be lit at once and Enter ran
// the one the pointer was not on.
func TestThePointerTakesTheSelectionOnEntry(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Frame()
	if a.Selected != 0 {
		t.Fatalf("the page did not start on row 0: selected=%d", a.Selected)
	}
	// Entering "Open terminal" (row 1) moves the choice to it.
	rect, ok := tt.Find("Open terminal")
	if !ok {
		t.Fatalf("no terminal row: %q", tt.Texts())
	}
	tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	tt.Frame()
	if a.Selected != 1 {
		t.Errorf("the pointer did not select the row it entered: selected=%d want 1", a.Selected)
	}
	// Resting on the row keeps the choice there, and — the R21 half that
	// matters — a keyboard step is not undone by a pointer that sits still on
	// the row the selection just left.
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.Selected != 2 {
		t.Errorf("the arrow step was undone by the resting pointer: selected=%d want 2", a.Selected)
	}
	tt.Frame()
	if a.Selected != 2 {
		t.Errorf("a later frame re-asserted the pointer's row: selected=%d want 2", a.Selected)
	}
	// Entering another row moves it again.
	other, ok := tt.Find("Quit floter")
	if !ok {
		t.Fatalf("no quit row: %q", tt.Texts())
	}
	tt.Move(other.X+other.W/2, other.Y+other.H/2)
	tt.Frame()
	if a.Selected != 7 {
		t.Errorf("entering another row did not select it: selected=%d want 7", a.Selected)
	}
}

// The entry edge is not a re-assert: a row the pointer is already resting on
// does not fight the keyboard, so the two devices share one state — the whole
// point of the reference's `onPointerEnter` over `onMouseMove`.
func TestARestingPointerDoesNotFightTheKeyboard(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	rect, ok := tt.Find("Open terminal")
	if !ok {
		t.Fatalf("no terminal row: %q", tt.Texts())
	}
	tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	tt.Frame()
	if a.Selected != 1 {
		t.Fatalf("the pointer did not select the row it entered: %d", a.Selected)
	}
	// Up moves back to row 0 while the pointer still rests on row 1.
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if a.Selected != 0 {
		t.Errorf("Up from the pointer's row = %d, want 0", a.Selected)
	}
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if a.Selected != 0 {
		t.Errorf("the resting pointer re-asserted its row: selected=%d want 0", a.Selected)
	}
}

// The pointer never lands on a row the keyboard cannot act on: a heading and a
// plugin status note are labels, not choices (the reference's
// `unavailable || !interactive` guard), so entering one leaves the selection
// where it was.
func TestThePointerSkipsRowsTheKeyboardCannotActOn(t *testing.T) {
	a := testApp()
	var found []apps.App
	for i := 0; i < 3; i++ {
		found = append(found, apps.App{Name: "Recent " + string(rune('A'+i)), Path: "/Applications/R" + string(rune('A'+i)) + ".app"})
	}
	a.SetApps(found)
	a.SetRecent([]string{found[0].Path, found[1].Path, found[2].Path}, true)
	tt := render(t, a)
	tt.Frame()
	// The empty page draws the built-ins, then a "Recent" heading, then the
	// recents: row 8 is the heading.
	heading := a.copy().RecentSection
	rect, ok := tt.Find(heading)
	if !ok {
		t.Fatalf("no recent heading: %q", tt.Texts())
	}
	before := a.Selected
	tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	tt.Frame()
	if a.Selected != before {
		t.Errorf("the pointer selected the heading: selected=%d want %d", a.Selected, before)
	}
}
