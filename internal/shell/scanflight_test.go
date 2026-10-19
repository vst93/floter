package shell

import (
	"testing"
	"time"

	"floter/internal/tools"
)

// A scan latch is claimed once: the second claim while the first is in flight
// is the same request, and is refused until the first ends.
func TestAScanLatchAdmitsOneScanAtATime(t *testing.T) {
	var flight scanFlight
	if !flight.begin() {
		t.Fatal("the first claim was refused")
	}
	if !flight.busy() {
		t.Error("the latch does not report the scan it admitted")
	}
	if flight.begin() {
		t.Error("a second scan was admitted while one was in flight")
	}
	flight.end()
	if flight.busy() {
		t.Error("the latch is still held after the scan ended")
	}
	if !flight.begin() {
		t.Error("a scan after the first ended was refused")
	}
	flight.end()
}

// The tray's "re-scan" item can be pressed again before the first press is
// done — it is a menu entry, and a held one repeats. The second press is the
// same request: nothing of it may start while the first is still reading the
// machine (「很多界面经常卡死」).
func TestARescanPressWhileOneRunsIsDropped(t *testing.T) {
	a := newApp(t)
	starts := make(chan struct{}, 4)
	a.lookTools = func() []tools.State { starts <- struct{}{}; return nil }

	// A press already in flight: the latch is held, so the next press must
	// return without touching any of the four reads. The install catalog is
	// the one whose start is observable.
	if !a.rescan.begin() {
		t.Fatal("the latch was already held")
	}
	a.Rescan()
	select {
	case <-starts:
		t.Error("a second rescan started while the first was in flight")
	case <-time.After(50 * time.Millisecond):
	}
	a.rescan.end()

	// And once the first is done, the next press is a new rescan.
	a.Rescan()
	awaitCatalogStart(t, starts, "a rescan after the first settled")
}
