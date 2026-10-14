package settingsui

import "testing"

// The close question names a session by identity, not by its position in the
// list: the reference's `killArmedId` held the stable `session.sessionId`
// (`src/settings/SessionsPage.tsx:54`), so a session whose row moved — because
// one ahead of it was removed — was still the one the answer ended.
func TestTheCloseArmFollowsTheSessionWhenTheListShrinks(t *testing.T) {
	var closed []int
	a := New(newStore(t), Actions{CloseSessionAt: func(index int) { closed = append(closed, index) }})
	// A, B, C; B is the one asked about. Removing A shifts B from position 1
	// to position 0.
	sessions := []Session{
		{ID: 1, Title: "A", Running: true, Index: 0},
		{ID: 2, Title: "B", Running: true, Index: 1},
		{ID: 3, Title: "C", Running: true, Index: 2},
	}
	a.Sessions = func() []Session { return sessions }
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	a.armSessionClose(2)
	tt.Frame()
	if !tt.HasText("Close it?") {
		t.Fatalf("the close did not arm: %v", tt.Texts())
	}

	// A goes away. The question is B's, and B is still on a row, so it stays
	// where B is — not on whoever now happens to sit at position 1.
	sessions = []Session{
		{ID: 2, Title: "B", Running: true, Index: 0},
		{ID: 3, Title: "C", Running: true, Index: 1},
	}
	tt.Frame()
	if !tt.HasText("Close it?") {
		t.Fatalf("the question did not survive a neighbouring deletion: %v", tt.Texts())
	}
	if err := tt.Click("Close it?"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	tt.Frame()
	if len(closed) != 1 || closed[0] != 0 {
		t.Fatalf("confirm closed %v, want [0] — B's new position, not the stale 1", closed)
	}
}

// When the session the question names disappears, the question goes with its
// row: it must not be inherited by whoever is left (the reference re-rendered
// its list against the broker each poll, taking a removed row's arm with it).
func TestTheCloseArmIsDroppedWhenItsSessionDisappears(t *testing.T) {
	closed := 0
	a := New(newStore(t), Actions{CloseSessionAt: func(int) { closed++ }})
	sessions := []Session{
		{ID: 1, Title: "A", Running: true, Index: 0},
		{ID: 2, Title: "B", Running: true, Index: 1},
	}
	a.Sessions = func() []Session { return sessions }
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	a.armSessionClose(2)
	tt.Frame()
	if !tt.HasText("Close it?") {
		t.Fatalf("the close did not arm: %v", tt.Texts())
	}

	// B ends; the row is gone, and the question with it — it does not fall on
	// A, and nothing can be confirmed.
	sessions = []Session{{ID: 1, Title: "A", Running: true, Index: 0}}
	tt.Frame()
	if tt.HasText("Close it?") {
		t.Errorf("the question outlived its session: %v", tt.Texts())
	}
	if a.anySessionArmed() {
		t.Errorf("the arm for the departed session was not dropped (the state, not just the label)")
	}
	if err := tt.Click("Close it?"); err == nil {
		t.Errorf("a confirm control was still clickable after its session left")
	}
	if closed != 0 {
		t.Errorf("a departed session's question closed something: %d", closed)
	}
}
