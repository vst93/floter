package shell

import (
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"

	"floter/internal/settings"
)

// The sampler's lifetime is a session boundary, not a flag the loop re-reads:
// the reference reconciles the monitor on every settings change
// (`clipboard_history/mod.rs:436-442` `sync_runtime` → `start`/`stop`), `stop`
// aborts the task outright (`monitor.rs:463-476`), and `start` spawns a fresh
// task with a fresh `last_hash` (`monitor.rs:443`). These pins hold the same
// three properties here: turning the history off ends the sampler now, turning
// it back on is a new session with no memory, and one enable is one session.
//
// The stand-ins (`stageWatchClipboard`, `watchApp`, `setHistory`, `waitFor`)
// live in watch_test.go.

// Turning the history off ends the sampler at once, not at its next tick: the
// interval here is long enough that no tick can arrive during the test.
func TestTurningTheHistoryOffEndsTheSamplerNow(t *testing.T) {
	stage := stageWatchClipboard(t)
	stage.set("a copy")
	const interval = 5 * time.Second
	a := watchApp(t, interval)
	a.watchClipboard()

	session := a.clipboardSession
	if session == nil {
		t.Fatal("turning the history on started no sampler")
	}
	setHistory(t, a, false)
	if a.clipboardSession != nil {
		t.Fatal("the sampler is still registered after the history was turned off")
	}
	select {
	case <-session.done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("the sampler was still running 500ms after the history was turned off")
	}
	// A second stop is a no-op, as the reference's `stop` is.
	a.stopClipboard()
	if got := stage.polls(); got != 0 {
		t.Fatalf("the stopped sampler read the clipboard %d times", got)
	}
}

// Off and on again before the next tick is still two sessions: the copy on the
// clipboard must come back after the history was cleared. With the loop
// re-reading the setting instead of being stopped, the old session survives the
// quick toggle with its memory intact, and the cleared copy never returns.
func TestTogglingOffAndOnWithinOneIntervalIsANewSession(t *testing.T) {
	stage := stageWatchClipboard(t)
	stage.set("still on the clipboard")
	const interval = 300 * time.Millisecond
	a := watchApp(t, interval)
	a.watchClipboard()

	waitFor(t, "the first copy to be recorded", func() bool { return len(a.Clipboard.Entries()) == 1 })
	if err := a.Clipboard.Clear(); err != nil {
		t.Fatalf("clearing the history: %v", err)
	}

	setHistory(t, a, false)
	setHistory(t, a, true)

	waitFor(t, "the new session to record the clipboard again", func() bool {
		return len(a.Clipboard.Entries()) == 1
	})
	if got := a.Clipboard.Entries()[0].Text; got != "still on the clipboard" {
		t.Fatalf("recorded %q, want the text that was on the clipboard", got)
	}
	stopWatcher(t, a, stage, interval)
}

// One enable is one session, however many settings changes land while it runs.
func TestOneEnableIsOneSession(t *testing.T) {
	stage := stageWatchClipboard(t)
	stage.set("a copy")
	const interval = 20 * time.Millisecond
	a := watchApp(t, interval)
	a.watchClipboard()

	first := a.clipboardSession
	if first == nil {
		t.Fatal("turning the history on started no sampler")
	}
	if got := a.clipboardSessions; got != 1 {
		t.Fatalf("%d sessions after the first enable, want 1", got)
	}

	// A settings change that does not touch the switch: still the same session.
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetClipboard(settings.ClipboardSettings{Enabled: true, MaxItems: 7})
	}); err != nil {
		t.Fatalf("changing the capacity: %v", err)
	}
	if a.clipboardSession != first {
		t.Fatal("a settings change while the history was on started a second sampler")
	}
	if got := a.clipboardSessions; got != 1 {
		t.Fatalf("%d sessions after a settings change that left the switch on, want 1", got)
	}

	setHistory(t, a, false)
	setHistory(t, a, true)
	if got := a.clipboardSessions; got != 2 {
		t.Fatalf("%d sessions after off and on again, want 2", got)
	}
	setHistory(t, a, false)
	a.stopClipboard()
	if a.clipboardSession != nil {
		t.Fatal("the sampler is still registered after being stopped twice")
	}
	stopWatcher(t, a, stage, interval)
}

// A sampler that was stopped while it was inside a capture still owns the
// memory until it is gone: the next session waits for it before it clears the
// field. Without that hand-off the old capture's hash lands after the clear and
// the new session starts out believing it already recorded what is on the
// clipboard.
func TestTheNewSessionTakesOverOnlyAfterTheOldOneIsGone(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	originalSample, originalImage := clipboardSample, clipboardImageReader
	clipboardImageReader = func([]transfer.Format) ([]byte, int, int) { return nil, 0, 0 }
	clipboardSample = func() clipSample {
		// The first poll is held inside its capture, the way a slow clipboard
		// read or a slow write holds it in the product.
		once.Do(func() {
			close(entered)
			<-release
		})
		return clipSample{Text: "held in the middle of a capture"}
	}
	t.Cleanup(func() { clipboardSample, clipboardImageReader = originalSample, originalImage })

	const interval = 2 * time.Second
	a := watchApp(t, interval)
	a.watchClipboard()
	held := a.clipboardSession
	if held == nil {
		t.Fatal("turning the history on started no sampler")
	}
	waitFor(t, "the sampler to enter its first capture", func() bool {
		select {
		case <-entered:
			return true
		default:
			return false
		}
	})

	// Off and on again while that capture is still in flight.
	setHistory(t, a, false)
	setHistory(t, a, true)
	close(release)

	waitFor(t, "the held capture to be written", func() bool { return len(a.Clipboard.Entries()) == 1 })
	select {
	case <-held.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the stopped sampler never ended after its capture returned")
	}
	// The old sampler wrote its hash on the way out; the new one clears it only
	// now, and it stays cleared until its own first tick.
	waitFor(t, "the new session to clear the memory", func() bool { return a.lastClipboardHash == "" })
	time.Sleep(100 * time.Millisecond)
	if got := a.lastClipboardHash; got != "" {
		t.Fatalf("the stopped sampler's hash came back into the new session's memory: %q", got)
	}
	setHistory(t, a, false)
	if a.clipboardSession != nil {
		t.Fatal("the sampler is still registered after the history was turned off")
	}
	time.Sleep(100 * time.Millisecond)
	if got := len(a.Clipboard.Entries()); got != 1 {
		t.Fatalf("history holds %d entries after the switch was turned off, want 1", got)
	}
}
