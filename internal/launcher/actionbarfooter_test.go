package launcher

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
)

// R-next · The action bar is the panel's **pinned footer**, not a first row.
//
// The user's report, verbatim: 「输入框中有输入内容时出现的"在终端中打开"项：现在
// 固定在第一项（错误）」. The old build drew `.launcher-action-bar` as a sibling of
// the scroller, *below* it — the bar is the field's own hand-off and the list
// sits above it — where this build had folded it into the list's head as a
// `bar:` row. This pins the position: every match comes first, the bar closes
// the list.
func TestTheActionBarClosesTheListNotHeadsIt(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Type("term")
	tt.Frame()

	results := a.Results()
	if len(results) < 2 {
		t.Fatalf("the list is too short to pin an order: %v", ids(results))
	}
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the action bar is not the list's last row: %v", ids(results))
	}
	if strings.HasPrefix(results[0].ID, "bar:") {
		t.Errorf("the action bar heads the list: %v", ids(results))
	}
	if results[bar-1].ID != "terminal" {
		t.Errorf("the match did not come before the bar: %v", ids(results))
	}
	// The bar is the *footer*: it must be the last thing drawn on the panel, so
	// its row box sits below every matched row's.
	barRect, ok := tt.Find("Run in terminal")
	if !ok {
		t.Fatal("the bar row was not drawn")
	}
	topRect, ok := tt.Find("Open terminal")
	if !ok {
		t.Fatal("the matched row was not drawn")
	}
	if barRect.Y <= topRect.Y {
		t.Errorf("the bar is drawn above the match: bar y=%.0f, match y=%.0f", barRect.Y, topRect.Y)
	}
}

// R-next · The footer is not a numbered result and cannot take a number: the
// old build ran `resultShortcutSlots` over `displayedResults`, which excludes
// the bar. With the bar at the list's *head* it did not matter; as the footer
// it is the row a numbering over the matched rows would otherwise leave a
// number for.
func TestTheActionBarTakesNoNumber(t *testing.T) {
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	tt := render(t, a)
	tt.Type("git")
	tt.Frame()

	results := a.Results()
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the bar is not the list's last row: %v", ids(results))
	}
	if _, ok := a.numbers[bar]; ok {
		t.Errorf("the action bar was numbered: %v", a.numbers)
	}
	if got := a.numbers[0]; got != 1 {
		t.Errorf("the first result's number = %d, want 1 (%v)", got, a.numbers)
	}
	// The held modifier's own row can never reach a number either — it is the
	// one row the numbered family cannot reach (the reference's R67).
	b := testApp()
	tb := render(t, b)
	tb.HoldModifiers(ui.Cmd)
	tb.Frame()
	bresults := b.Results()
	if bresults[len(bresults)-1].ID != bareTerminalRowID {
		t.Fatalf("the held row is not the list's last line: %v", ids(bresults))
	}
	if _, ok := b.numbers[len(bresults)-1]; ok {
		t.Errorf("the held row was numbered: %v", b.numbers)
	}
}

// R-next · The footer takes the selection on the command line the catalogue
// also matches, and it is what makes the row's selected face reachable at all.
//
// The user's second report is that the item had no selected style. It had none
// because it was never the selected row: the bar was drawn at the list's head
// *beside* a match that owned the choice. The reference hands Enter to the bar
// for exactly this query (`shouldDefaultToActionBar`: a shell command line with
// no runnable command plan), and this build inherited that rule
// (`defaultsToShell`) — the placement was the only thing hiding it.
func TestTheActionBarTakesTheSelectionOnACommandLine(t *testing.T) {
	a := testApp()
	// The `git` row is a PATH tool: runnable, but not a *command plan*, which
	// is the distinction `shouldDefaultToActionBar` turns on.
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	ran := 0
	a.Actions.RunInTerminal = func([]string) { ran++ }
	tt := render(t, a)
	tt.Type("git")
	tt.Frame()

	results := a.Results()
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the bar is not the list's last row: %v", ids(results))
	}
	if a.Selected != bar {
		t.Errorf("the selection is on row %d, want the footer %d (%v)", a.Selected, bar, ids(results))
	}
	// The drawn row carries the selection, so its band is tinted against the
	// panel's own fill at the same height — the visible "selected style" the
	// report said was missing.
	rect, ok := tt.Find("Run in terminal")
	if !ok {
		t.Fatal("the selected footer was not drawn")
	}
	img := tt.Image()
	y := int(rect.Y + rect.H/2)
	if got, want := img.RGBAAt(10, y), img.RGBAAt(3, y); got == want {
		t.Errorf("the selected footer's band is not tinted: %v at x=10, %v at x=3", got, want)
	}
	// Enter runs the footer, not the tool row above it: the bar is the field's
	// own hand-off and it is the row the choice is on.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if ran != 1 {
		t.Errorf("Enter ran the shell row %d times, want once (it is the selected row)", ran)
	}
}

// R-next · A page where a match keeps the choice leaves the footer untinted —
// "selected" is a state the row has only when Enter would run it.
func TestAnUnselectedActionBarStaysQuiet(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Type("term")
	tt.Frame()

	results := a.Results()
	bar := len(results) - 1
	if !strings.HasPrefix(results[bar].ID, "bar:") {
		t.Fatalf("the bar is not the list's last row: %v", ids(results))
	}
	if a.Selected == bar {
		t.Fatalf("the match did not keep the selection: selected=%d bar=%d", a.Selected, bar)
	}
	rect, ok := tt.Find("Run in terminal")
	if !ok {
		t.Fatal("the bar row was not drawn")
	}
	img := tt.Image()
	y := int(rect.Y + rect.H/2)
	if got, want := img.RGBAAt(10, y), img.RGBAAt(3, y); got != want {
		t.Errorf("the unselected footer is tinted: %v at x=10, %v at x=3", got, want)
	}
}

// R-next · The field row and the result rows share the reference's text column.
//
// The user's first report is 「搜索页面左右两边各有一大块空白」. The horizontal
// insets had been written in the wrong unit — a `Space(n)` is **four** reference
// units (`--u * 4n`; `internal/theme/theme.go`), so the reference's
// `.collapsed-card__input-row { padding: 0 16u }` and `.launcher-result
// { padding: 0 12u }` (`src/styles/launcher.css:212-215,1099-1114`) are
// `Space(4)` and `Space(3)`, not `Space(16)` and `Space(12)`. At the shipped
// step this build was drawing 57.6px of side padding where the design asks for
// 14.4 — a query at 64u and row titles at 120u instead of 16 and 54.
func TestTheFieldAndTheRowsShareTheReferenceTextColumn(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	// The empty field's caret sits on the row's own 16u inset.
	if caret, ok := tt.TextCaret(); ok {
		if got := int(caret.X); got != 16 {
			t.Errorf("the field's caret starts at x=%d, want the reference's 16", got)
		}
	} else {
		t.Fatal("the field's caret was not drawn")
	}
	// A result row's title starts on 16u + the 28u icon + the 10u gap = 54u.
	if r, ok := tt.Find("Open settings"); ok {
		if got := int(r.X); got != 54 {
			t.Errorf("a result title starts at x=%d, want the reference's 54", got)
		}
	} else {
		t.Fatal("the first result's title was not drawn")
	}
}
