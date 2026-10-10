package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/drops"
	"floter/internal/extensions"
	"floter/internal/tools"

	clipboardpkg "floter/internal/clipboard"
	"floter/internal/i18n"
)

// The axis cycle wraps at both ends and lands on the first value for a choice
// off the axis.
func TestFilterAxisCycle(t *testing.T) {
	axis := FilterAxis{Values: []string{"a", "b", "c"}}
	cases := []struct {
		current   string
		direction int
		want      string
	}{
		{"a", 1, "b"}, {"b", 1, "c"}, {"c", 1, "a"},
		{"a", -1, "c"}, {"c", -1, "b"}, {"b", -1, "a"},
		{"off", 1, "a"},
	}
	for _, c := range cases {
		if got := axis.Cycle(c.current, c.direction); got != c.want {
			t.Errorf("Cycle(%q, %d) = %q, want %q", c.current, c.direction, got, c.want)
		}
	}
}

// The three axes are the ones the old build declared, in order.
func TestFilterAxesMatchTheOldBuild(t *testing.T) {
	want := map[string][]string{
		"browser":   {filterAll, filterBookmarks, filterHistory, filterTabs},
		"clipboard": {filterAll, filterFavorites, filterText, filterImage, filterLink, filterFiles},
	}
	if got := BrowserFilterAxis.Values; len(got) != len(want["browser"]) {
		t.Errorf("browser = %v", got)
	} else {
		for i, v := range want["browser"] {
			if got[i] != v {
				t.Errorf("browser[%d] = %q, want %q", i, got[i], v)
			}
		}
	}
	if got := ClipboardFilterAxis.Values; len(got) != len(want["clipboard"]) {
		t.Errorf("clipboard = %v", got)
	} else {
		for i, v := range want["clipboard"] {
			if got[i] != v {
				t.Errorf("clipboard[%d] = %q, want %q", i, got[i], v)
			}
		}
	}
	if got := CalculatorFilterAxis.Values; len(got) != 2 || got[0] != filterAll || got[1] != filterFavorites {
		t.Errorf("calculator = %v", got)
	}
}

// Every value of every axis has a word, in both languages: a chip that prints
// nothing is the drift the axis exists to prevent.
func TestEveryChipHasAWord(t *testing.T) {
	for _, language := range []string{"en", "zh"} {
		copy := i18n.For(language).Launcher
		for _, axis := range []FilterAxis{BrowserFilterAxis, ClipboardFilterAxis, CalculatorFilterAxis} {
			for _, value := range axis.Values {
				word := axis.LabelOf(copy, value)
				if word == "" || word == value {
					t.Errorf("%s: %q has no word", language, value)
				}
			}
		}
	}
}

// The clipboard kind split: a link is a text entry whose whole value is a
// page address, and the stored kinds pass through.
func TestClipboardEntryKind(t *testing.T) {
	cases := []struct {
		entry clipboardpkg.Entry
		want  string
	}{
		{clipboardpkg.Entry{Kind: clipboardpkg.KindText, Text: "hello world"}, filterText},
		{clipboardpkg.Entry{Kind: clipboardpkg.KindText, Text: "https://example.com/a"}, filterLink},
		{clipboardpkg.Entry{Kind: clipboardpkg.KindText, Text: "see https://example.com"}, filterText},
		{clipboardpkg.Entry{Kind: clipboardpkg.KindImage}, filterImage},
		{clipboardpkg.Entry{Kind: clipboardpkg.KindFiles, Paths: []string{"/tmp/a"}}, filterFiles},
	}
	for _, c := range cases {
		if got := clipboardEntryKind(c.entry); got != c.want {
			t.Errorf("kind(%q, %q) = %q, want %q", c.entry.Kind, c.entry.Text, got, c.want)
		}
	}
}

// The list's own section headings: a heading row is a label the keyboard steps
// over and never numbers, and a block's first row prints its title above
// itself.
func TestSectionHeadings(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{{Name: "Editor", Path: "/Applications/Editor.app"}})
	a.SetRecent([]string{"/Applications/Editor.app"}, true)
	// A taller window: the recents and their heading sit below the built-ins.
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) },
		InputWindowWidth, int(WindowHeight("small"))+200)
	tt.Frame()
	if !tt.HasText("Recently launched") {
		t.Errorf("the recents' heading is missing: %q", tt.Texts())
	}
	results := a.Results()
	headingAt := -1
	for i, item := range results {
		if item.heading {
			headingAt = i
		}
	}
	if headingAt < 0 {
		t.Fatal("no heading row in the list")
	}
	// The keyboard never lands on a heading.
	a.Selected = 0
	for i := 0; i < len(results)+2; i++ {
		a.Selected = a.nextSelectable(a.Selected, 1)
		if results[a.Selected].heading {
			t.Fatalf("the arrows landed on the heading at %d", a.Selected)
		}
	}
	// A heading has no action: Enter on it does nothing.
	if results[headingAt].Run != nil {
		t.Error("a heading is runnable")
	}
	// The dropped-file block prints its own heading.
	a.SetDropped([]drops.File{{Name: "a.txt", Path: "/tmp/a.txt", Directory: "/tmp"}})
	a.files = true
	files := a.filesItems()
	if len(files) == 0 || !files[0].heading || files[0].Title != a.copy().FilesSection {
		t.Errorf("the dropped block's heading is missing: %+v", files)
	}
}

// The docked status row: a message shows under the list with the warning tone
// when it reports a failure, it is charged to the window's height, and it
// expires on its own clock.
func TestFeedbackRow(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	if a.FeedbackVisible() {
		t.Fatal("the row shows before any message")
	}
	base := Geometry{Font: a.Font, Spacing: a.Spacing, RowLines: a.RowLines, Held: a.HeldRows}.Height()
	withRow := Geometry{Font: a.Font, Spacing: a.Spacing, RowLines: a.RowLines, Held: a.HeldRows, Feedback: true}.Height()
	if withRow <= base {
		t.Errorf("the feedback band is not charged: %d vs %d", withRow, base)
	}
	a.Feedback("copied")
	if !a.FeedbackVisible() {
		t.Fatal("the message did not show")
	}
	tt.Frame()
	if !tt.HasText("copied") {
		t.Errorf("the row did not draw: %q", tt.Texts())
	}
	if a.toastWarning {
		t.Error("a plain message took the warning tone")
	}
	// A failure message takes the warning tone.
	a.WarnFeedback("it failed")
	tt.Frame()
	if !tt.HasText("it failed") || !a.toastWarning {
		t.Errorf("the warning row: %q warning=%v", tt.Texts(), a.toastWarning)
	}
}

// A command word two integrations share is marked on the rows: the launcher
// enters the first, so the shadowed one says the word is shared rather than
// looking like the only answer.
func TestSharedCommandWordIsMarked(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{
		{IntegrationID: "one", Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}},
		{IntegrationID: "two", Command: extensions.Command{ID: "jv", Name: "JSON Viewer (other)"}},
		{IntegrationID: "three", Command: extensions.Command{ID: "diff", Name: "Diff"}},
	})
	if !a.commandConflicts["jv"] || a.commandConflicts["diff"] {
		t.Errorf("conflicts = %v", a.commandConflicts)
	}
	warned := map[string]bool{}
	for _, item := range a.commandItems() {
		warned[item.Title] = item.warning
	}
	if !warned["JSON Viewer"] || !warned["JSON Viewer (other)"] {
		t.Errorf("the shared word's rows are not marked: %v", warned)
	}
	if warned["Diff"] {
		t.Error("an unshared word was marked")
	}
}

// The invoke row: a tool the catalog knows, that *is* installed, and that
// carries a launch action of its own offers the argv; a pure CLI filter and a
// missing tool offer nothing; and the two rows are mutually exclusive.
func TestToolInvokeRows(t *testing.T) {
	a := testApp()
	spawned := [][]string{}
	terminal := [][]string{}
	a.Actions.SpawnDetached = func(argv []string) { spawned = append(spawned, argv) }
	a.Actions.RunInTerminal = func(argv []string) { terminal = append(terminal, argv) }
	a.SetToolCatalog([]tools.State{
		{Entry: tools.Entry{ID: "flameshot", Name: "Flameshot", Probes: []string{"flameshot"},
			Keywords: []string{"screenshot"}, Launch: []string{"flameshot", "gui"}}, Installed: true},
		{Entry: tools.Entry{ID: "lazygit", Name: "lazygit", Probes: []string{"lazygit"},
			Keywords: []string{"git"}, Launch: []string{"lazygit"}, NeedsTerminal: true}, Installed: true},
		{Entry: tools.Entry{ID: "jq", Name: "jq", Probes: []string{"jq"}, Keywords: []string{"json"}}, Installed: true},
		{Entry: tools.Entry{ID: "fd", Name: "fd", Probes: []string{"fd"}, Keywords: []string{"find"},
			Recipes: map[string][]tools.Recipe{"linux": {{Manager: "pacman", Package: "fd"}}}},
			Command: "pacman -S fd", Installed: false},
	})
	// A detected GUI tool offers its argv, and the row starts it detached.
	a.Query = "flameshot"
	rows := a.toolInvokeItems()
	if len(rows) != 1 || rows[0].Title != "Flameshot" || rows[0].Detail != "flameshot gui" {
		t.Fatalf("invoke rows = %+v", rows)
	}
	rows[0].Run()
	if len(spawned) != 1 || spawned[0][0] != "flameshot" {
		t.Errorf("spawned = %v", spawned)
	}
	// A full-screen TUI goes to the terminal instead.
	a.Query = "lazygit"
	rows = a.toolInvokeItems()
	if len(rows) != 1 {
		t.Fatalf("lazygit rows = %+v", rows)
	}
	rows[0].Run()
	if len(terminal) != 1 || terminal[0][0] != "lazygit" {
		t.Errorf("terminal = %v", terminal)
	}
	// A pure CLI filter carries no launch action: no row.
	a.Query = "jq"
	if rows := a.toolInvokeItems(); len(rows) != 0 {
		t.Errorf("jq produced an invoke row: %+v", rows)
	}
	// A missing tool is the install row's business, not this one.
	a.Query = "fd"
	if rows := a.toolInvokeItems(); len(rows) != 0 {
		t.Errorf("an uninstalled tool produced an invoke row: %+v", rows)
	}
	if rows := a.toolInstallItems(); len(rows) != 1 {
		t.Errorf("install rows = %+v", rows)
	}
}

// The plugin modes name their own empty state rather than claiming a search
// found nothing: nothing copied yet, no favorites, no matches.
func TestModeEmptyMessages(t *testing.T) {
	a, _ := clipboardApp(t)
	copy := a.copy()
	// The clipboard, with nothing copied.
	a.clipboard = true
	if got := a.emptyMessage(copy); got != copy.ClipboardEmpty {
		t.Errorf("clipboard empty = %q", got)
	}
	// A needle that matched nothing.
	a.Query = "clipboard zzz"
	if got := a.emptyMessage(copy); got != copy.ClipboardEmptyFilter {
		t.Errorf("clipboard filter = %q", got)
	}
	// The favorites chip.
	a.Query = "clipboard "
	a.clipboardFilter = filterFavorites
	if got := a.emptyMessage(copy); got != copy.ClipboardEmptyFavorites {
		t.Errorf("clipboard favorites = %q", got)
	}
	// The calculator's two.
	a.clipboard, a.calculatorMode = false, true
	a.calculatorFilter = filterAll
	if got := a.emptyMessage(copy); got != copy.CalculatorEmpty {
		t.Errorf("calculator empty = %q", got)
	}
	a.calculatorFilter = filterFavorites
	if got := a.emptyMessage(copy); got != copy.CalculatorEmptyFavorites {
		t.Errorf("calculator favorites = %q", got)
	}
	// The ordinary page says plainly that a search found nothing.
	a.calculatorMode = false
	if got := a.emptyMessage(copy); got != copy.NoResults {
		t.Errorf("ordinary = %q", got)
	}
}
