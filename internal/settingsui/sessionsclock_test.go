package settingsui

import (
	"testing"
	"time"
)

// A session's close question carries its own three-second clock, as the
// reference's `KILL_CONFIRM_TIMEOUT` did (`src/settings/SessionsPage.tsx:10`),
// and it does not survive a visit to another page: the reference's arm lived in
// the page component, so navigating away unmounted the question.
func TestTheSessionCloseQuestionExpiresAndDoesNotCrossPages(t *testing.T) {
	closed := 0
	a := New(newStore(t), Actions{CloseSessionAt: func(index int) { closed = index + 1 }})
	a.Sessions = func() []Session {
		return []Session{{Title: "zsh — floter", Running: true, Active: true, Index: 0}}
	}
	now := time.Unix(1_700_000_000, 0)
	a.Now = func() time.Time { return now }
	tt := render(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Close session"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Close it?") {
		t.Fatalf("the close did not arm: %v", tt.Texts())
	}

	// Inside the window the question is still up.
	now = now.Add(2 * time.Second)
	tt.Frame()
	if !tt.HasText("Close it?") {
		t.Fatalf("the question left inside its window: %v", tt.Texts())
	}

	// Past it the button is the plain close control again, unanswered.
	now = now.Add(4 * time.Second)
	tt.Frame()
	if tt.HasText("Close it?") {
		t.Errorf("the question outlived its window: %v", tt.Texts())
	}
	if closed != 0 {
		t.Errorf("the lapsed question closed a session: %d", closed)
	}

	// Arm again, then leave the page: the question does not come back with it.
	if err := tt.Click("Close session"); err != nil {
		t.Fatalf("close: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Close it?") {
		t.Fatalf("the close did not arm: %v", tt.Texts())
	}
	if err := tt.Click("General"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("Close it?") {
		t.Errorf("the question survived a page change: %v", tt.Texts())
	}
	if closed != 0 {
		t.Errorf("a page change closed a session: %d", closed)
	}
}
