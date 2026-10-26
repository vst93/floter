package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Two pins on one upgrade, both driving the real key path rather than a
// helper: upstream mygo v0.6.4 made each Han ideograph a word of its own
// (`internal/text/word.go`, `buffer.prevWord` in `ui/textbuffer.go`), so
// word-wise deleting in a text field stops at one character instead of
// swallowing the whole clause.
//
// The search field is where floter's users meet it: a Chinese query typed
// with an input method, then ⌘⌫/Ctrl+⌫ to trim the last character. The
// launcher does not claim that press while something is typed — its own
// handler says so ("claimed only while … nothing is typed in it",
// `fieldkeys.go`) — so the press is the editor's, which is the code the
// upgrade changed.

func TestCtrlBackspaceInTheSearchFieldDeletesOneHanCharacter(t *testing.T) {
	a := testApp()
	tt := render(t, a)

	tt.Type("搜索应用")
	tt.Frame()
	if a.Query != "搜索应用" {
		t.Fatalf("the field did not take the query: %q", a.Query)
	}

	tt.Key(historyDeleteMods, ui.KeyBackspace)
	tt.Frame()
	if got := a.Query; got != "搜索应" {
		t.Errorf("⌘⌫ on %q gave %q, want %q — one character, not the clause",
			"搜索应用", got, "搜索应")
	}
}

// The other half of the same change: the scripts written with spaces between
// words must still delete a whole word. A future upgrade that made every rune
// a word would break this one, and that is what it is here for.
func TestCtrlBackspaceInTheSearchFieldStillDeletesTheWholeLatinWord(t *testing.T) {
	a := testApp()
	tt := render(t, a)

	tt.Type("git status")
	tt.Frame()
	if a.Query != "git status" {
		t.Fatalf("the field did not take the query: %q", a.Query)
	}

	tt.Key(historyDeleteMods, ui.KeyBackspace)
	tt.Frame()
	if got := a.Query; got != "git " {
		t.Errorf("⌘⌫ on %q gave %q, want %q — a spaced-script word, all of it",
			"git status", got, "git ")
	}
}
