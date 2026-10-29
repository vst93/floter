package launcher

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// P11-ce · A completion leaves the caret at the end of the line it completed.
//
// The field's text is the app's (`a.Query`); the editor inside it keeps its own
// buffer and its own caret, and a text the *app* writes is taken with
// `ed.caret = min(ed.caret, len)` (mygo `ui/editor_history.go:59`) — the caret
// does not follow an insertion it did not make. `appendWord` writes a longer
// line than the one the caret was on (`-m` → `-mode `), so without
// `pendingCaret` the next keystroke landed **inside** the completed word:
// measured before the fix, Tab then `x` left `-mxode ` on the field.
//
// The reference has no such state: its input is a DOM `<input>` whose value the
// app sets, and a value set from outside puts the caret at the end. So the
// completion is a write the user is meant to keep typing after, and the caret
// has to be where the new text ends.
func TestACompletionLeavesTheCaretAtTheEnd(t *testing.T) {
	// The Tab key's own arm: the chord completes the chosen argument into the
	// line, and the next character typed continues it.
	a := commandApp()
	tt := render(t, a)
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	tt.Type("-m")
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.Query != "-mode " {
		t.Fatalf("the completion left %q, want \"-mode \"", a.Query)
	}
	tt.Type("x")
	tt.Frame()
	if a.Query != "-mode x" {
		t.Errorf("typing after a completion landed at %q, want \"-mode x\": "+
			"the caret stayed where the word being completed began", a.Query)
	}

	// The row's own arm: a completion row runs the same `appendWord`, so a
	// completion taken from the list (a click, or Enter on the row) leaves the
	// field in the same state as Tab does.
	b := commandApp()
	st := render(t, b)
	st.Key(0, ui.KeyTab)
	st.Frame()
	st.Type("-m")
	st.Frame()
	var row Item
	for _, item := range b.Results() {
		if item.complete == "-mode" {
			row = item
			break
		}
	}
	if row.Run == nil {
		t.Fatalf("the mode offered no \"-mode\" completion: %v", b.Results())
	}
	row.Run()
	st.Frame()
	if b.Query != "-mode " {
		t.Fatalf("the row's completion left %q, want \"-mode \"", b.Query)
	}
	st.Type("x")
	st.Frame()
	if b.Query != "-mode x" {
		t.Errorf("typing after a completion taken from the list landed at %q, want \"-mode x\"", b.Query)
	}
}
