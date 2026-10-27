package shell

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/transfer"

	"floter/internal/clipboard"
	"floter/internal/settings"
)

// The watcher's loop is the one part of the clipboard history that only exists
// while the app runs, so these pins stand in for the system clipboard and drive
// the loop through the settings the product itself uses to turn it on and off.
//
// The memory rule they pin is the reference's
// (`clipboard_history/monitor.rs:178-186` `capture_result_hash` and `:443`
// `let mut last_hash` inside the spawned task): remember what was written, and
// only for as long as this session lasts.

// watchStage is the system clipboard as the watcher reads it, with a poll
// counter the test goroutine may read while the watcher runs.
type watchStage struct {
	mu     sync.Mutex
	sample clipSample
	reads  int
}

func (s *watchStage) set(text string) { s.setSample(clipSample{Text: text}) }

func (s *watchStage) setSample(sample clipSample) {
	s.mu.Lock()
	s.sample = sample
	s.mu.Unlock()
}

func (s *watchStage) polls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads
}

// stageWatchClipboard installs the stand-in and restores the real readers when
// the test ends.
func stageWatchClipboard(t *testing.T) *watchStage {
	t.Helper()
	stage := &watchStage{}
	originalSample, originalImage := clipboardSample, clipboardImageReader
	clipboardSample = func() clipSample {
		stage.mu.Lock()
		defer stage.mu.Unlock()
		stage.reads++
		return stage.sample
	}
	clipboardImageReader = func([]transfer.Format) ([]byte, int, int) { return nil, 0, 0 }
	t.Cleanup(func() { clipboardSample, clipboardImageReader = originalSample, originalImage })
	return stage
}

// watchApp is an App with a settings store, a clipboard store in a temp dir,
// and a watcher that has not started yet.
func watchApp(t *testing.T, interval time.Duration) *App {
	t.Helper()
	a := New(Options{Store: settings.NewStore(settings.Default())})
	a.Clipboard = clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), 50)
	a.clipboardInterval = interval
	return a
}

// waitFor polls a condition until it holds or the test gives up.
func waitFor(t *testing.T, what string, holds func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if holds() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// setHistory turns the clipboard history off or on through the store, which is
// the path the settings page takes.
func setHistory(t *testing.T, a *App, enabled bool) {
	t.Helper()
	err := a.Store.Update(func(s *settings.Settings) {
		s.SetClipboard(settings.ClipboardSettings{Enabled: enabled, MaxItems: 50})
	})
	if err != nil {
		t.Fatalf("turning the clipboard history to %v: %v", enabled, err)
	}
}

// stopWatcher turns the history off and proves the loop actually ended: it is
// the only way out of the goroutine, so a test that leaves without it would
// leave a sampler running into the next test's stand-in.
func stopWatcher(t *testing.T, a *App, stage *watchStage, interval time.Duration) {
	t.Helper()
	setHistory(t, a, false)
	time.Sleep(2*interval + 50*time.Millisecond)
	frozen := stage.polls()
	time.Sleep(2*interval + 50*time.Millisecond)
	if got := stage.polls(); got != frozen {
		t.Fatalf("the watcher kept reading the clipboard %d times after the setting was turned off", got-frozen)
	}
}

// The reference skips the ticker's immediate first tick so the app does not
// read the clipboard during startup, before the window server has settled
// (`monitor.rs:440-442`). A Go ticker's first tick is already one interval out.
func TestTheWatcherWaitsAnIntervalBeforeItsFirstRead(t *testing.T) {
	stage := stageWatchClipboard(t)
	stage.set("the first copy")
	const interval = 200 * time.Millisecond
	a := watchApp(t, interval)
	a.watchClipboard()

	time.Sleep(interval / 2)
	if got := stage.polls(); got != 0 {
		t.Fatalf("the watcher read the clipboard %d times before its first interval", got)
	}
	waitFor(t, "the first poll", func() bool { return stage.polls() > 0 })
	stopWatcher(t, a, stage, interval)
}

// One copy is one entry, however often the watcher polls it.
func TestTheWatcherRecordsACopyOnce(t *testing.T) {
	stage := stageWatchClipboard(t)
	stage.set("copied once")
	const interval = 20 * time.Millisecond
	a := watchApp(t, interval)
	a.watchClipboard()

	waitFor(t, "the copy to be recorded", func() bool { return len(a.Clipboard.Entries()) == 1 })
	time.Sleep(10 * interval)
	if got := stage.polls(); got < 3 {
		t.Fatalf("the watcher polled %d times, want at least 3", got)
	}
	if got := len(a.Clipboard.Entries()); got != 1 {
		t.Fatalf("history holds %d entries after repeated polls of one copy, want 1", got)
	}
	stopWatcher(t, a, stage, interval)
}

// Turning the history off ends the loop; nothing samples the clipboard behind
// a switched-off setting.
func TestTheWatcherStopsWhenTheSettingTurnsOff(t *testing.T) {
	stage := stageWatchClipboard(t)
	stage.set("a copy")
	const interval = 20 * time.Millisecond
	a := watchApp(t, interval)
	a.watchClipboard()

	waitFor(t, "three polls", func() bool { return stage.polls() >= 3 })
	stopWatcher(t, a, stage, interval)
}

// A session's memory belongs to that session: the reference builds `last_hash`
// inside the task (`monitor.rs:443`), so turning the history off and on again
// re-reads the clipboard instead of trusting what the previous session saw.
// Without this, a cleared history could not get the current clipboard back.
func TestANewWatcherSessionForgetsWhatTheLastOneRecorded(t *testing.T) {
	stage := stageWatchClipboard(t)
	stage.set("still on the clipboard")
	const interval = 20 * time.Millisecond
	a := watchApp(t, interval)

	// A first session records the copy and remembers it.
	if !a.captureClipboard() {
		t.Fatal("the first capture was not recorded")
	}
	if a.lastClipboardHash == "" {
		t.Fatal("the recorded copy was not remembered")
	}
	if err := a.Clipboard.Clear(); err != nil {
		t.Fatalf("clearing the history: %v", err)
	}
	if got := len(a.Clipboard.Entries()); got != 0 {
		t.Fatalf("history holds %d entries after clearing, want 0", got)
	}

	setHistory(t, a, true)
	waitFor(t, "the new session to record the clipboard again", func() bool {
		return len(a.Clipboard.Entries()) == 1
	})
	if got := a.Clipboard.Entries()[0].Text; got != "still on the clipboard" {
		t.Fatalf("recorded %q, want the text that was on the clipboard", got)
	}
	stopWatcher(t, a, stage, interval)
}

// A capture that could not be written is not remembered, so the next poll tries
// again — the reference's `capture_result_hash` returns None on error for
// exactly this reason (`monitor.rs:178-186`, "will retry"). Both renditions the
// watcher can pick have their own write, so both are pinned.
func TestAFailedCaptureIsRetriedOnTheNextPoll(t *testing.T) {
	copied := filepath.Join(t.TempDir(), "one.txt")
	if err := os.WriteFile(copied, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalExists := clipboardFileExists
	clipboardFileExists = func(path string) bool { return path == copied }
	t.Cleanup(func() { clipboardFileExists = originalExists })

	for _, testCase := range []struct {
		name   string
		sample clipSample
	}{
		{name: "text", sample: clipSample{Text: "write me"}},
		{name: "files", sample: clipSample{Paths: []string{copied}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stage := stageWatchClipboard(t)
			stage.setSample(testCase.sample)

			root := t.TempDir()
			// A regular file where the history's directory has to go: every
			// write fails, as a full disk or a read-only data directory would.
			blocked := filepath.Join(root, "blocked")
			if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
				t.Fatal(err)
			}
			paths := clipboard.FromConfigRoot(filepath.Join(blocked, "clipboard"))
			a := &App{Clipboard: clipboard.NewStore(paths, 50)}

			if a.captureClipboard() {
				t.Fatal("a capture that could not be written was reported as recorded")
			}
			if got := a.lastClipboardHash; got != "" {
				t.Fatalf("the failed capture was remembered as %q, so it would never be retried", got)
			}

			// Unblock the store. A failed write leaves nothing on disk, so the
			// history is read fresh — what the next launch would see.
			if err := os.Remove(blocked); err != nil {
				t.Fatal(err)
			}
			a.Clipboard = clipboard.NewStore(paths, 50)
			if !a.captureClipboard() {
				t.Fatal("the failed capture was not retried on the next poll")
			}
			if _, err := os.Stat(paths.Index()); err != nil {
				t.Fatalf("the retried capture never reached the index: %v", err)
			}
			if got := len(a.Clipboard.Entries()); got != 1 {
				t.Fatalf("history holds %d entries after the retry, want 1", got)
			}
		})
	}
}
