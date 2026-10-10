package launcher

import (
	"testing"

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
