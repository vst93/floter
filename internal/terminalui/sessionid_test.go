package terminalui

import "testing"

// A session's identity is handed out once and never reused: the settings
// list's close question names a session by it, so a session that moved down
// after one ahead of it was deleted keeps the name the question holds (the
// reference's `sessionId`, `src/settings/SessionsPage.tsx:54`).
func TestSessionIDsAreStableAcrossDeletion(t *testing.T) {
	newTerminal, _ := countingTerminal(t)
	a := New(newStore(), Actions{}, newTerminal)
	a.EnsureSession()
	a.NewSession()
	a.NewSession()
	if len(a.Sessions) != 3 {
		t.Fatalf("sessions = %d", len(a.Sessions))
	}
	ids := [3]uint64{a.Sessions[0].ID, a.Sessions[1].ID, a.Sessions[2].ID}
	if ids[0] == 0 || ids[1] == 0 || ids[2] == 0 {
		t.Fatalf("a session was created without an identity: %v", ids)
	}
	if ids[0] == ids[1] || ids[1] == ids[2] || ids[0] == ids[2] {
		t.Fatalf("two sessions share an identity: %v", ids)
	}

	// Delete the first: the ones behind it shift down a position but keep
	// their names, so an arm taken before the deletion still names the same
	// session it did. (Its terminal is dropped first: a terminal the test's
	// fake never really started is not the subject here.)
	a.Sessions[0].Term = nil
	a.CloseSession(0)
	if len(a.Sessions) != 2 {
		t.Fatalf("sessions = %d", len(a.Sessions))
	}
	if a.Sessions[0].ID != ids[1] || a.Sessions[1].ID != ids[2] {
		t.Fatalf("an identity followed its position: got [%d %d], want [%d %d]",
			a.Sessions[0].ID, a.Sessions[1].ID, ids[1], ids[2])
	}
	// A later session still gets a fresh name, never a recycled one.
	a.NewSession()
	last := a.Sessions[len(a.Sessions)-1].ID
	if last == ids[0] || last == ids[1] || last == ids[2] {
		t.Fatalf("a new session reused an old identity: %d", last)
	}
}
