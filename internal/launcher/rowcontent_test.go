package launcher

import (
	"testing"

	"floter/internal/extensions"
)

// The transcription fold: everything that cannot change *which name* is being
// said comes off — Unicode width, case, whitespace and punctuation/symbols —
// so a subtitle that is the title written another way collapses onto it.
func TestTranscriptionKeyFoldsWidthCaseAndPunctuation(t *testing.T) {
	same := [][2]string{
		{"QQ 音乐", "qq音乐"},
		{"WeCom", "wecom"},
		{"Ｓａｆａｒｉ", "Safari"},        // full-width → ASCII
		{"Safari 浏览器", "safari浏览器"}, // whitespace
		{"A-B_C", "a b c"},          // punctuation
		{"Café", "café"},            // case only
		{"ＱＱ　音乐", "qq 音乐"},          // full-width letters and ideographic space
	}
	for _, pair := range same {
		if !isTranscription(pair[0], pair[1]) {
			t.Errorf("%q and %q are one name, but read as two", pair[0], pair[1])
		}
	}

	different := [][2]string{
		{"QQ音乐", "WeCom"},
		{"Safari", "Firefox"},
		{"Open settings", "Open terminal"},
	}
	for _, pair := range different {
		if isTranscription(pair[0], pair[1]) {
			t.Errorf("%q and %q are different names, but read as one", pair[0], pair[1])
		}
	}
}

// A row whose second line is only its own title written again prints one line:
// the old build's R12/R14 rule, applied by rowSubtitle so the renderer and the
// height accounting cannot disagree.
func TestARowDropsASubtitleThatOnlyRepeatsItsTitle(t *testing.T) {
	transcribed := Item{Title: "企业微信", Detail: "WeCom"}
	if got := rowSubtitle(transcribed); got != "WeCom" {
		t.Errorf("the platform's other name for an app is not a transcription, but it was dropped: %q", got)
	}
	repeated := Item{Title: "Zebra", Detail: "zebra"}
	if got := rowSubtitle(repeated); got != "" {
		t.Errorf("a subtitle that repeats the title survived: %q", got)
	}
	real := Item{Title: "Zebra", Detail: "A striped animal"}
	if got := rowSubtitle(real); got != "A striped animal" {
		t.Errorf("a real subtitle was dropped: %q", got)
	}
	if got := rowSubtitle(Item{Title: "Zebra"}); got != "" {
		t.Errorf("a row with no subtitle invented one: %q", got)
	}
}

// The drawn row and the charged row follow one rule: a command whose
// description is the name again loses its second line *and* is charged one
// line, so the window is never a row taller than its list.
func TestADroppedSubtitleIsChargedOneLine(t *testing.T) {
	cases := []struct {
		name         string
		description  string
		wantSubtitle bool
	}{
		{"repeated", "zebra", false},
		{"real", "A striped animal", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			a := testApp()
			a.SetCommands([]extensions.CommandEntry{{
				IntegrationID:   "io.github.vst93.zoo",
				IntegrationName: "Zoo Tools",
				Command: extensions.Command{
					ID:          "zebra",
					Name:        "Zebra",
					Description: tt.description,
				},
				Program: "/bin/echo",
				Mode:    "pty",
			}})
			view := render(t, a)
			a.Query = "Zebra"
			view.Frame()

			if !view.HasText("Zebra") {
				t.Fatalf("the command row is missing: %q", view.Texts())
			}
			if got := view.HasText(tt.description); got != tt.wantSubtitle {
				t.Errorf("subtitle %q drawn = %v, want %v: %q", tt.description, got, tt.wantSubtitle, view.Texts())
			}

			index := -1
			for i, item := range a.Results() {
				if item.ID == "cmd:io.github.vst93.zoo:zebra" {
					index = i
				}
			}
			if index < 0 {
				t.Fatalf("the command row is not in the results: %v", ids(a.Results()))
			}
			want := 1.0
			if tt.wantSubtitle {
				want = 2
			}
			if got := a.Rows[index].Lines; got != want {
				t.Errorf("the row is charged %v lines, want %v", got, want)
			}
		})
	}
}

// A plugin list whose *opening* row names a group prints that group's heading:
// there is no row above it to continue, so it starts the block (the old
// build's `list-sections.ts` rule). A later block still prints its heading too.
func TestAnOutputListsOpeningGroupPrintsItsHeading(t *testing.T) {
	a := testApp()
	stdout := `[` +
		`{"id":"a","title":"Alpha","group":"First"},` +
		`{"id":"b","title":"Beta"},` +
		`{"id":"c","title":"Gamma","group":"Second"}` +
		`]`
	view := runListOutput(t, a, stdout)
	if !view.HasText("First") {
		t.Errorf("the opening group's heading is missing: %q", view.Texts())
	}
	if !view.HasText("Second") {
		t.Errorf("the second group's heading is missing: %q", view.Texts())
	}
}
