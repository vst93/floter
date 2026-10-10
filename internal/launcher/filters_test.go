package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/drops"

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
