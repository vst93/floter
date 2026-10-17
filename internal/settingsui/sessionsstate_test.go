package settingsui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// The sessions meta line keeps the state word as a segment of its own, so
// the old page's two state colours can come back: `.session-state` is the
// accent while the session runs, `.session-state--exited` the quiet grey
// after it has exited, and the size/start time stay on that quiet grey.
func TestSessionLinesMarkTheStateWordWithItsOwnColourRole(t *testing.T) {
	created := time.Date(2024, 1, 2, 15, 4, 0, 0, time.Local)
	running := Session{
		Title: "zsh", Running: true, Note: "Running · attached",
		Size: "80×24", Created: created,
	}
	exited := Session{
		Title: "build", Running: false, Note: "Exited",
		Size: "100×30", Created: created,
	}
	for name, session := range map[string]Session{
		"running": running,
		"exited":  exited,
	} {
		lines := sessionLines(session)
		if len(lines) != 1 || len(lines[0].Segments) != 3 {
			t.Fatalf("%s lines = %+v, want one meta line of three segments", name, lines)
		}
		meta := lines[0].Segments
		if meta[0].Text != session.Note {
			t.Errorf("%s state segment = %q, want %q", name, meta[0].Text, session.Note)
		}
		if meta[1].Text != session.Size {
			t.Errorf("%s size segment = %q, want %q", name, meta[1].Text, session.Size)
		}
		if meta[2].Role != rowSegmentTertiary {
			t.Errorf("%s created role = %v, want the quiet grey", name, meta[2].Role)
		}
		if got := meta[0].Role; got != rowSegmentAccent && name == "running" {
			t.Errorf("running state role = %v, want the accent", got)
		}
		if got := meta[0].Role; got != rowSegmentTertiary && name == "exited" {
			t.Errorf("exited state role = %v, want the quiet grey", got)
		}
	}
}

// The old page painted the state word in two different colours, not one: a
// running session's state is the accent while the size and start time stay
// muted, and an exited session's state joins them. This pins the rendered
// result, not just the role plumbing.
func TestSessionStateWordsCarryTheOldStateColours(t *testing.T) {
	a := New(newStore(t), Actions{})
	a.Sessions = func() []Session {
		created := time.Date(2024, 1, 2, 15, 4, 0, 0, time.Local)
		return []Session{
			{ID: 1, Title: "zsh", Running: true, Active: true, Index: 0,
				Note: "Running · attached", Size: "80×24", Created: created},
			{ID: 2, Title: "build", Running: false, Index: 1,
				Note: "Exited", Size: "100×30", Created: created},
		}
	}
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	rects := map[string]ui.Rect{}
	for _, label := range []string{"Running · attached", "Exited", "80×24", "100×30"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %q in %q", label, tt.Texts())
		}
		rects[label] = r
	}
	runningState := inkColor(tt, rects["Running · attached"])
	exitedState := inkColor(tt, rects["Exited"])
	runningSize := inkColor(tt, rects["80×24"])
	exitedSize := inkColor(tt, rects["100×30"])

	if colorClose(runningState, exitedState, 12) {
		t.Errorf("running and exited state words render the same colour: %v vs %v",
			runningState, exitedState)
	}
	if colorClose(runningState, runningSize, 12) {
		t.Errorf("a running state word renders like the muted size text: %v vs %v",
			runningState, runningSize)
	}
	if !colorClose(exitedState, exitedSize, 12) {
		t.Errorf("an exited state word does not join the muted meta text: %v vs %v",
			exitedState, exitedSize)
	}
	if !colorClose(runningSize, exitedSize, 12) {
		t.Errorf("the size text differs between rows: %v vs %v", runningSize, exitedSize)
	}
}

// inkColor is the colour a text rect is actually drawn in: the pixel of the
// rect farthest from its background. Antialiased glyph edges sit between the
// two, so the farthest pixel is the glyph's own colour.
func inkColor(tt *ui.Tester, r ui.Rect) ui.Color {
	img := tt.Image()
	background, _ := modalColor(tt, r)
	var ink ui.Color
	best := -1
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			if d := colorDistance(c, background); d > best {
				best = d
				ink = c
			}
		}
	}
	return ink
}

// modalColor returns the most common colour in a rect, which for a text box
// is the surface the glyphs sit on.
func modalColor(tt *ui.Tester, r ui.Rect) (ui.Color, int) {
	img := tt.Image()
	counts := map[ui.Color]int{}
	var mode ui.Color
	best := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := y*img.Stride + x*4
			c := ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
			counts[c]++
			if counts[c] > best {
				best = counts[c]
				mode = c
			}
		}
	}
	return mode, best
}

func colorDistance(a, b ui.Color) int {
	dr := int(a.R) - int(b.R)
	dg := int(a.G) - int(b.G)
	db := int(a.B) - int(b.B)
	return dr*dr + dg*dg + db*db
}

func colorClose(a, b ui.Color, tol int) bool {
	dist := colorDistance(a, b)
	return dist <= tol*tol*3
}

// The old page's row was itself the resume control: its `onActivate` was
// wired only for a resumable session — `!session.exited && !session.attached`
// (`SessionsPage.tsx:170,197`) — while the Play button beside it is the same
// action's explicit form. So pressing anywhere on a running-but-not-on-screen
// row brings it back; an exited row and the row already on screen answer
// nothing, and the explicit Show button still works.
func TestClickingAResumableSessionRowResumesIt(t *testing.T) {
	a := New(newStore(t), Actions{})
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	var shown []int
	a.Actions.SelectSession = func(index int) { shown = append(shown, index) }
	a.Sessions = func() []Session {
		return []Session{
			{ID: 1, Title: "zsh", Running: true, Active: true, Index: 0},
			{ID: 2, Title: "vim", Running: true, Index: 1},
			{ID: 3, Title: "build", Running: false, Index: 2},
		}
	}
	tt.Frame()

	en := i18n.For("en").Settings
	if err := tt.Click("vim"); err != nil {
		t.Fatalf("click the resumable row: %v", err)
	}
	if len(shown) != 1 || shown[0] != 1 {
		t.Fatalf("clicking a resumable row resumed %v, want [1]", shown)
	}
	// The session on screen is already there, and an exited one is not
	// resumable: neither row may answer.
	if err := tt.Click("zsh"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("build"); err != nil {
		t.Fatal(err)
	}
	if len(shown) != 1 {
		t.Fatalf("an active/exited row answered a click: %v", shown)
	}
	// The explicit control is the same action, and still there.
	if err := tt.Click(en.SessionsResume); err != nil {
		t.Fatalf("click Show: %v", err)
	}
	if len(shown) != 2 || shown[1] != 0 {
		t.Fatalf("the on-screen session's Play control resumed %v, want to re-select 0", shown)
	}
}
