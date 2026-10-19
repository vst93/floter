package shell

import "sync"

// scanFlight is a one-at-a-time latch for a background re-read of the machine.
//
// Every re-read here — the applications, the PATH commands, the install
// catalog, the whole tray rescan — is triggered by something the user can press
// repeatedly: the tray's "re-scan" item, a settings toggle, a summon. A press
// that arrives while the same read is already running is not a second request:
// the state it would read is the state the running scan is reading, so it is
// dropped rather than queued, and a burst of presses collapses into one scan.
// Queueing them instead is what turns a held tray item into a pile of
// concurrent full-disk scans that starve the main thread — the report
// 「很多界面经常卡死」. The old build's `refreshToolCatalog` did the same thing
// with an in-flight promise.
//
// The latch is a plain mutex rather than a channel: the scans it guards finish
// by handing their result to the main thread, and none of them is waited on.
type scanFlight struct {
	mu      sync.Mutex
	running bool
}

// begin claims the latch, reporting whether the caller owns this scan: false
// means one is already in flight and the caller must return without doing
// anything. Every true must be paired with an end, including on the paths that
// only hand work to another goroutine.
func (f *scanFlight) begin() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.running {
		return false
	}
	f.running = true
	return true
}

// end releases the latch.
func (f *scanFlight) end() {
	f.mu.Lock()
	f.running = false
	f.mu.Unlock()
}

// busy reports whether a scan is in flight, which is what a test reads instead
// of the field.
func (f *scanFlight) busy() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.running
}
