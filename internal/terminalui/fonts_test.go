package terminalui

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The family key folds the spellings a machine uses: the display name, the
// file name and a hand-typed one compare equal.
func TestFontKey(t *testing.T) {
	for _, pair := range [][2]string{
		{"JetBrains Mono", "jetbrainsmono"},
		{"JetBrainsMono", "jetbrainsmono"},
		{"jetbrains mono", "jetbrainsmono"},
		{"DejaVu Sans Mono", "dejavusansmono"},
	} {
		if got := fontKey(pair[0]); got != pair[1] {
			t.Errorf("fontKey(%q) = %q, want %q", pair[0], got, pair[1])
		}
	}
}

// The walk reads a directory tree's font files and matches them to the
// candidates by name, style suffixes and all.
func TestCollectFontNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"JetBrainsMono-Regular.ttf",
		"DejaVuSansMono.ttf",
		"FiraCode-Bold.otf",
		"not-a-font.txt",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "Menlo.ttc"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	collectFontNames(dir, names, 0)
	for _, want := range []string{"jetbrainsmono", "dejavusansmono", "firacode", "menlo"} {
		if !names[want] {
			t.Errorf("%q was not collected: %v", want, names)
		}
	}
	if names["notafont"] || names["not-a-font"] {
		t.Error("a non-font file was collected")
	}
}

// The picker's list always carries the current value, so a hand-set family is
// never lost.
func TestFontFamilyOptionsKeepsTheCurrent(t *testing.T) {
	options := FontFamilyOptions("My Hand Picked Face")
	found := false
	for _, option := range options {
		if option == "My Hand Picked Face" {
			found = true
		}
	}
	if !found {
		t.Errorf("the current family is missing from %v", options)
	}
	// A detected list may be shorter than the shipped fallback — that is the
	// point of probing — but it is never empty.
	if len(options) == 0 {
		t.Error("the picker has no options at all")
	}
}

// The picker's list never walks the disk on the drawing thread.
//
// `FontFamilyOptions` is asked while the settings page's Terminal card and the
// terminal's appearance drawer are *built*, i.e. once per painted frame, and the
// walk behind it measures ~504µs per call (`BenchmarkFontWalk`) — tens of
// milliseconds a second of directory walking whenever either surface is up, and
// far more on a cold page cache or a network home. It is the same shape the rest
// of this campaign closed at every other call site: a memo read on the drawing
// thread, the expensive half on a goroutine.
//
// The seam is the memo itself: `fontWalkInvalidate` is called by the background
// pass, so a frame that has not run one cannot have called it. This holds the
// lock's invariant rather than a stopwatch — the first frame answers at once
// (with the shipped fallback list) and the pass lands afterwards.
func TestTheFontWalkStaysOffTheDrawingThread(t *testing.T) {
	// Reset the memo to the cold state, and put it back afterwards so the other
	// tests keep the process's one answer.
	fontWalk.mu.Lock()
	savedNames, savedComputed, savedPending := fontWalk.names, fontWalk.computed, fontWalk.pending
	fontWalk.names, fontWalk.computed, fontWalk.pending = nil, time.Time{}, false
	fontWalk.mu.Unlock()
	defer func() {
		fontWalk.mu.Lock()
		fontWalk.names, fontWalk.computed, fontWalk.pending = savedNames, savedComputed, savedPending
		fontWalk.mu.Unlock()
	}()

	landed := make(chan struct{}, 1)
	SetFontWalkInvalidate(func() {
		select {
		case landed <- struct{}{}:
		default:
		}
	})
	defer SetFontWalkInvalidate(nil)

	// The cold frame: an answer is still there (the shipped list), and asking for
	// it did not wait for a walk.
	cold := FontFamilyOptions("")
	if len(cold) == 0 {
		t.Fatal("the cold frame drew an empty picker")
	}
	if fontWalk.computed.IsZero() == false {
		t.Fatal("the cold frame filled the memo itself: the walk ran inline")
	}

	// The pass lands off-thread and asks for a frame — that request is the proof
	// the walk happened somewhere other than where the frame asked. Polling the
	// memo's own timestamp rather than the callback keeps the test free of the
	// previous test's in-flight pass (a walk started before this one reset the
	// memo still lands, and its callback is not this pass's evidence).
	deadline := time.Now().Add(10 * time.Second)
	var computed time.Time
	for {
		fontWalk.mu.Lock()
		computed = fontWalk.computed
		fontWalk.mu.Unlock()
		if !computed.IsZero() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the background font walk never landed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	fontWalk.mu.Lock()
	cached := fontWalk.names
	fontWalk.mu.Unlock()
	// A machine whose font directories cannot be read records an empty (not nil)
	// answer, so the walk is not retried on every frame.
	if cached == nil {
		t.Error("a failed walk left the memo nil: it would re-walk every frame")
	}

	// A warm memo is served from memory: a second ask starts no second walk. The
	// evidence is the memo's own timestamp, which a second pass would move
	// forward — the landing callback cannot be used here, because the pass above
	// may have left its own signal in the channel.
	warm := FontFamilyOptions("")
	if len(warm) == 0 {
		t.Error("the warm frame drew an empty picker")
	}
	fontWalk.mu.Lock()
	after := fontWalk.computed
	fontWalk.mu.Unlock()
	if !after.Equal(computed) {
		t.Error("the warm memo started a second walk instead of answering from memory")
	}
}
