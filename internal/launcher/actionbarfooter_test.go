package launcher

import (
	"image/color"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
)

// panelFill is the light panel's own fill the bar rows are drawn on: a pixel
// holding it means "nothing is painted here", which is what a rounded corner
// leaves at the box's own corner.
var panelFill = color.RGBA{R: 254, G: 254, B: 254, A: 255}

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

// R-next · The bar row's two text registers follow the reference's split: the
// *title* is the string the action hands over (the command line, the URL, the
// path) and the *subtitle* is the action's own word.
//
// `main:src/launcher/LauncherResults.tsx:1067-1068` prints
// `{actionBar.value}` in `.launcher-action-bar__title` and `{actionBar.label}`
// in `.launcher-action-bar__subtitle`; `.launcher-action-bar__title`
// (`main:src/styles/launcher.css:1632-1638`) is the monospace register,
// because the string it holds is code. The port had the two swapped: the
// action word was the title (and so wore the monospace) while the command line
// was the subtitle — the visible result of which is that the row reads
// "Run in terminal" in the code face and the line you are about to run in the
// interface one.
func TestTheActionBarTitleIsTheValueNotTheActionWord(t *testing.T) {
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	tt := render(t, a)
	tt.Type("git")
	tt.Frame()

	results := a.Results()
	var bar *Item
	for i := range results {
		if strings.HasPrefix(results[i].ID, "bar:") {
			bar = &results[i]
		}
	}
	if bar == nil {
		t.Fatalf("the bar row is missing: %v", ids(results))
	}
	// The title is the typed line; the subtitle is the action's word. The
	// word must not be the row's title (it belongs to the subtitle slot).
	if bar.Title != "git" {
		t.Errorf("the bar's title = %q, want the typed line %q", bar.Title, "git")
	}
	if bar.Detail != "Run in terminal" {
		t.Errorf("the bar's subtitle = %q, want the action word %q", bar.Detail, "Run in terminal")
	}
	// Both texts are drawn, and the value's box is the wide one — the action
	// word sits under it in the subtitle slot.
	valueRect, ok := tt.Find("git")
	if !ok {
		t.Fatal("the bar's value was not drawn")
	}
	wordRect, ok := tt.Find("Run in terminal")
	if !ok {
		t.Fatal("the bar's action word was not drawn")
	}
	if wordRect.Y <= valueRect.Y {
		t.Errorf("the action word is not the subtitle: value y=%.0f, word y=%.0f",
			valueRect.Y, wordRect.Y)
	}

	// A URL query prints the address as the title and "Open in browser" as the
	// subtitle, the same split.
	b := testApp()
	tb := render(t, b)
	tb.Type("https://example.com/docs")
	tb.Frame()
	var urlBar *Item
	for i := range b.Results() {
		if b.Results()[i].ID == "bar:url" {
			bar := b.Results()[i]
			urlBar = &bar
		}
	}
	if urlBar == nil {
		t.Fatalf("the url bar is missing: %v", ids(b.Results()))
	}
	if urlBar.Title != "https://example.com/docs" || urlBar.Detail != "Open in browser" {
		t.Errorf("the url bar = (%q, %q), want (value, action word)",
			urlBar.Title, urlBar.Detail)
	}
}

// R-next · The footer's top corners are square when a list sits above it.
//
// The reference welds the pinned bar to the scroller with
// `.launcher-results + .launcher-action-bar { border-radius: 0 0
// var(--radius-md) var(--radius-md) }` (`main:src/styles/launcher.css:1556-1562`):
// the bar keeps its own full rounding only when no result list precedes it (a
// URL that matched nothing). The port is a single in-list row family, so "a
// list precedes it" is "some row is drawn above the footer" — a bar at the
// list's head keeps the rounding, a bar after a match squares its top corners.
func TestTheActionBarSquaresItsTopCornersOverAList(t *testing.T) {
	// The bar after a match: its top-left corner is already the row's own fill.
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	ta := render(t, a)
	ta.Type("git")
	ta.Frame()

	// The bar at the list's head (a URL with no match): its top-left corner is
	// still the panel's fill, i.e. the corner is rounded away.
	b := testApp()
	tb := render(t, b)
	tb.Type("https://example.com/nope")
	tb.Frame()

	corner := func(tt *ui.Tester) (top int, px color.RGBA) {
		img := tt.Image()
		// Find the bar band's top edge just inside its left inset (x=6), then
		// read the corner pixel one step further out (x=4, the panel's own 4u
		// inset): a squared corner paints the row's fill there, a rounded one
		// leaves the panel (or its faint edge) showing under the arc.
		top = -1
		for y := 50; y < 200; y++ {
			c := img.RGBAAt(6, y)
			if panelDistance(c) > 8 {
				top = y
				break
			}
		}
		if top < 0 {
			t.Fatalf("the bar band was not found: %q", tt.Texts())
		}
		return top, img.RGBAAt(4, top)
	}

	barTop, squared := corner(ta)
	if panelDistance(squared) <= 8 {
		t.Errorf("the footer over a list is rounded at its top-left corner (y=%d): %v", barTop, squared)
	}
	_, rounded := corner(tb)
	if panelDistance(rounded) > 8 {
		t.Errorf("the footer with no list above it is squared at its top-left corner: %v over panel %v",
			rounded, panelFill)
	}
}

// panelDistance is how far a pixel is from the panel's own fill, in summed
// channels: 0 means "nothing is painted here", a row's fill reads as tens.
func panelDistance(c color.RGBA) int {
	d := int(c.R) - int(panelFill.R)
	if d < 0 {
		d = -d
	}
	g := int(c.G) - int(panelFill.G)
	if g < 0 {
		g = -g
	}
	if g > d {
		d = g
	}
	b := int(c.B) - int(panelFill.B)
	if b < 0 {
		b = -b
	}
	if b > d {
		d = b
	}
	return d
}
