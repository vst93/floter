package terminalui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// The panel's own notice (the old build's `.terminal-feedback`): an action the
// panel owns that could not run says so in the panel, and the line clears
// itself on the old build's 4500ms clock rather than staying up forever.
func TestTerminalFeedbackShowsAndExpires(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	const message = "Could not continue in the system terminal"
	a.ShowFeedback(message)
	if !a.FeedbackVisible() {
		t.Fatal("ShowFeedback did not raise the notice")
	}
	tt := render(t, a, 860, 600)
	if !tt.HasText(message) {
		t.Fatalf("the notice did not show: %q", tt.Texts())
	}
	if a.feedbackUntil.IsZero() {
		t.Fatal("drawing the notice did not start its clock")
	}
	if got := a.feedbackUntil.Sub(time.Now()); got <= 0 || got > terminalFeedbackDuration {
		t.Errorf("the clock runs %v, want at most %v", got, terminalFeedbackDuration)
	}

	// Frames in a test all land inside the window; push the clock past it to
	// prove the notice expires on its own and is cleared.
	a.feedbackUntil = time.Now().Add(-time.Second)
	tt.Frame()
	if tt.HasText(message) {
		t.Errorf("the notice outlived its window: %q", tt.Texts())
	}
	if a.FeedbackVisible() {
		t.Errorf("the expired notice was not cleared: %q", a.feedback)
	}
}

// A newer message takes the notice and a fresh clock: an already-expired line
// must not swallow the message that arrives after it (the old build restarted
// the timer rather than stacking a second one).
func TestTerminalFeedbackRestartsOnANewerMessage(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	a.ShowFeedback("first")
	tt := render(t, a, 860, 600)

	// The first message's window runs out, but nothing has drawn the expiry
	// yet; the second message lands in that gap.
	a.feedbackUntil = time.Now().Add(-time.Second)
	a.ShowFeedback("second")
	tt.Frame()
	if !tt.HasText("second") {
		t.Fatalf("the newer message did not show: %q", tt.Texts())
	}
	if tt.HasText("first") {
		t.Errorf("the older message stayed under the newer one: %q", tt.Texts())
	}
	if !a.feedbackUntil.After(time.Now()) {
		t.Errorf("the newer message did not restart the clock: %v", a.feedbackUntil)
	}
}

// The notice is drawn over the empty state too: the header's control is
// offered with no session, so its failure has somewhere to land.
func TestTerminalFeedbackShowsOverTheEmptyState(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	a.ShowFeedback("no emulator")
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 860, 600)
	tt.Frame()
	if !tt.HasText("no emulator") {
		t.Errorf("the empty state swallowed the notice: %q", tt.Texts())
	}
}

// And over a live session, which is where the header's control is used: the
// canvas is the real panel, not the empty state.
func TestTerminalFeedbackShowsOverASession(t *testing.T) {
	a := New(newStore(), Actions{Settings: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	a.ShowFeedback("no emulator")
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 860, 600)
	tt.Frame()
	if !tt.HasText("no emulator") {
		t.Errorf("the session swallowed the notice: %q", tt.Texts())
	}
}
