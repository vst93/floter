package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
)

// P11-cf · Every text the *app* writes into the field leaves the caret at the
// end of what it wrote.
//
// The field's text is the app's (`a.Query`); the editor inside it keeps its own
// buffer and its own caret, and a text written from outside is taken with
// `ed.caret = min(ed.caret, len)` (mygo `ui/editor_history.go:59`) — the caret
// does not follow an insertion it did not make. So every write that *lengthens*
// the line (a deep link's query, a recalled history line, a completion) has to
// say where the caret goes, or the next keystroke lands inside the word the
// user was handed. P11-ce fixed that for the completion; this is the ruler over
// the rest of the family, and P11-ce's own open item (the completion's
// "starts a new word" branch had no arm of its own).
//
// The reference has no such state at all: its input is a DOM `<input>`, and a
// value set from outside puts the caret at the end.

// TestEveryExternalWriteLeavesTheCaretAtTheEnd is the behaviour table: for each
// path that writes a *longer* line, type one character afterwards and read
// where it landed. The empty write has the opposite answer and is here as the
// control arm: `Esc` clears the field, the caret clamps to 0 with it, and the
// next character starts the line rather than appending to a ghost.
func TestEveryExternalWriteLeavesTheCaretAtTheEnd(t *testing.T) {
	// A deep link names what to search for before the first frame; the field
	// starts empty, so a caret left where it was (0) would put the next key in
	// front of the whole query.
	t.Run("deep link", func(t *testing.T) {
		a := testApp()
		a.SetQuery("git status")
		tt := render(t, a)
		tt.Frame()
		if a.Query != "git status" {
			t.Fatalf("the deep link left %q", a.Query)
		}
		tt.Type("x")
		tt.Frame()
		if a.Query != "git statusx" {
			t.Errorf("typing after a deep link landed at %q, want %q", a.Query, "git statusx")
		}
	})

	// A deep link over a field that is already in use is the same write with a
	// caret the user left in the middle: the line it writes is longer than the
	// caret's position, which is the state that corrupts the line.
	t.Run("deep link over an open field", func(t *testing.T) {
		a := testApp()
		tt := render(t, a)
		tt.Type("git")
		tt.Frame()
		a.SetQuery("git status --short")
		tt.Frame()
		tt.Type("x")
		tt.Frame()
		if a.Query != "git status --shortx" {
			t.Errorf("typing after a deep link landed at %q, want %q", a.Query, "git status --shortx")
		}
	})

	// A recalled history line is the same write: the field was empty, the line
	// is not.
	t.Run("history recall", func(t *testing.T) {
		a := testApp()
		a.History = []string{"git status"}
		a.historyIndex = -1
		tt := render(t, a)
		tt.Frame()
		tt.Key(0, ui.KeyUp)
		tt.Frame()
		if a.Query != "git status" {
			t.Fatalf("the up arrow recalled %q, want \"git status\"", a.Query)
		}
		tt.Type("x")
		tt.Frame()
		if a.Query != "git statusx" {
			t.Errorf("typing after a history recall landed at %q, want %q", a.Query, "git statusx")
		}
	})

	// P11-ck · The output insert is the same family's last untested path.
	//
	// `runOutputRow` closes the output view, puts the row's text into the field
	// (`a.Query = row.Action.Text`), and marks the caret pending — the same three
	// lines every external write has. The source contract (`TestEveryQueryWrite
	// CarriesItsCaret`) caught a missing flag, but until now no behavioural arm
	// walked the real UI: Enter on an insert row, then the next keystroke.
	t.Run("output insert", func(t *testing.T) {
		a := testApp()
		// Open a one-row list output whose only action is insert.
		a.output = &OutputView{
			IsList:   true,
			Selected: 0,
			Rows: []extensions.Row{{
				ID:     "ins1",
				Title:  "Insert this",
				Action: &extensions.RowAction{Type: "insert", Text: "git status --short"},
			}},
		}
		tt := render(t, a)
		// Enter fires `runOutputRow`, which calls `leaveOutput` (clears the field)
		// and then writes the row's text.
		tt.Key(0, ui.KeyEnter)
		tt.Frame()
		if a.InOutputView() {
			t.Fatal("Enter on an insert row must leave the output view")
		}
		if a.Query != "git status --short" {
			t.Fatalf("after insert the field holds %q, want \"git status --short\"", a.Query)
		}
		// The pending caret must have moved the field's cursor to the end; the
		// next keystroke lands after the inserted text, not inside it.
		tt.Type("x")
		tt.Frame()
		if a.Query != "git status --shortx" {
			t.Errorf("typing after an output insert landed at %q, want %q: the caret stayed "+
				"at the beginning of the inserted text", a.Query, "git status --shortx")
		}
	})

	// The control arm: an empty write needs no flag, because clamping to 0 is
	// already the end of an empty line.
	t.Run("escape clears the field", func(t *testing.T) {
		a := testApp()
		tt := render(t, a)
		tt.Type("git")
		tt.Frame()
		tt.Key(0, ui.KeyEscape)
		tt.Frame()
		if a.Query != "" {
			t.Fatalf("Escape left %q in the field", a.Query)
		}
		tt.Type("x")
		tt.Frame()
		if a.Query != "x" {
			t.Errorf("typing after Escape landed at %q, want \"x\"", a.Query)
		}
	})
}

// TestACompletionThatStartsANewWordLeavesTheCaretAtTheEnd is P11-ce's open
// item: `appendWord` has two shapes — it finishes the word being typed
// (`replaceLastWord`) and it starts a new one (`trimmed + " " + word + " "`) —
// and only the first had an arm. The second is the one that *appends*, so it is
// the one a caret left behind would corrupt the most visibly (`-f x-f `).
func TestACompletionThatStartsANewWordLeavesTheCaretAtTheEnd(t *testing.T) {
	a := commandApp()
	tt := render(t, a)
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	// The line ends in a space, so the next completion cannot "finish" the last
	// word: it starts a new one.
	tt.Type("-f ")
	tt.Frame()
	if a.Query != "-f " {
		t.Fatalf("the field holds %q, want \"-f \"", a.Query)
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	// The word as the row spells it is the row's own (`-f --format`): read it
	// rather than spell it twice, so this arm is about the caret and not about
	// a completion string somebody may retitle.
	completed := a.Query
	if !strings.HasPrefix(completed, "-f ") || len(completed) <= len("-f ") ||
		!strings.HasSuffix(completed, " ") {
		t.Fatalf("the completion left %q, want a new word appended to \"-f \" and a trailing space", completed)
	}
	tt.Type("x")
	tt.Frame()
	if a.Query != completed+"x" {
		t.Errorf("typing after a completion that started a new word landed at %q, want %q: "+
			"the caret stayed where the appended word began", a.Query, completed+"x")
	}
}

// TestEveryQueryWriteCarriesItsCaret is the source contract, so a *new* path
// that writes the field cannot quietly skip the step: every write of a
// non-empty line in this package must set `pendingCaret` a few lines below it,
// where the field's next build reads it. The empty writes are exempt by
// construction — clamping the caret to a zero-length buffer already puts it at
// 0 (the control arm above).
func TestEveryQueryWriteCarriesItsCaret(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package: %v", err)
	}
	const write = "a.Query = "
	const caret = "a.pendingCaret = true"
	// The field reads the flag on the frame after the write; a write and its
	// flag sit inside one function body, a handful of lines apart. Six is the
	// widest gap the package has today (`SetQuery`, which carries four lines of
	// comment between them).
	const window = 6
	wrote, empty := 0, 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, write) {
				continue
			}
			rhs := strings.TrimSpace(strings.TrimPrefix(trimmed, write))
			if rhs == `""` {
				empty++
				continue
			}
			wrote++
			found := false
			for j := i + 1; j <= i+window && j < len(lines); j++ {
				if strings.Contains(lines[j], caret) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s:%d writes %q without setting %s within %d lines: the caret would stay "+
					"where it was and the next keystroke would land inside the line",
					name, i+1, rhs, caret, window)
			}
		}
	}
	// A scan that matches nothing is not a contract: the family has seven
	// non-empty writes and sixteen empty ones today (P11-cf measured), and a
	// rewrite that renamed the field would otherwise pass silently.
	if wrote < 7 || empty < 10 {
		t.Fatalf("the scan found %d non-empty and %d empty writes; the family is 7 and 16, "+
			"so the scan is no longer reading the writes it is meant to guard", wrote, empty)
	}
}
