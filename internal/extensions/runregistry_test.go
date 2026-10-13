package extensions

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
)

// The registry stops exactly the runs the named integration has in flight, and
// reports how many it stopped.
func TestRunRegistryKillsEveryRunForTheID(t *testing.T) {
	var first, second, other int64
	registry := NewRunRegistry()
	forgetFirst := registry.Add("a.b", func() { atomic.AddInt64(&first, 1) })
	forgetSecond := registry.Add("a.b", func() { atomic.AddInt64(&second, 1) })
	registry.Add("other", func() { atomic.AddInt64(&other, 1) })

	if got := registry.InFlight("a.b"); got != 2 {
		t.Fatalf("in flight = %d, want 2", got)
	}
	if got := registry.Kill("a.b"); got != 2 {
		t.Fatalf("killed = %d, want 2", got)
	}
	if atomic.LoadInt64(&first) != 1 || atomic.LoadInt64(&second) != 1 {
		t.Errorf("kills = %d/%d, want 1/1", first, second)
	}
	if atomic.LoadInt64(&other) != 0 {
		t.Error("killing one integration cancelled another's run")
	}
	// Killing again has nothing left to stop.
	if got := registry.Kill("a.b"); got != 0 {
		t.Errorf("second kill = %d, want 0", got)
	}
	// The stale forget function for an already-killed run is a no-op.
	forgetFirst()
	forgetSecond()
	if got := registry.InFlight("a.b"); got != 0 {
		t.Errorf("in flight after forget = %d, want 0", got)
	}
	if got := registry.InFlight("other"); got != 1 {
		t.Errorf("the other run was forgotten: %d", got)
	}
}

// A run that finishes on its own leaves the registry, and the forget function
// is idempotent: the run's goroutine and a caller's cleanup cannot double
// delete the handle.
func TestRunRegistryForgetIsIdempotent(t *testing.T) {
	cancelled := 0
	registry := NewRunRegistry()
	forget := registry.Add("a.b", func() { cancelled++ })
	forget()
	forget()

	if got := registry.InFlight("a.b"); got != 0 {
		t.Fatalf("in flight = %d, want 0", got)
	}
	if got := registry.Kill("a.b"); got != 0 {
		t.Errorf("killed = %d, want 0", got)
	}
	if cancelled != 0 {
		t.Errorf("a forgotten run was cancelled: %d", cancelled)
	}
}

// A nil registry is usable: an App built without one (a bare test) must not
// panic on a disable.
func TestRunRegistryNilIsANoOp(t *testing.T) {
	var registry *RunRegistry
	forget := registry.Add("a.b", func() { t.Error("a nil registry cancelled a run") })
	forget()
	if got := registry.Kill("a.b"); got != 0 {
		t.Errorf("killed = %d, want 0", got)
	}
	if got := registry.InFlight("a.b"); got != 0 {
		t.Errorf("in flight = %d, want 0", got)
	}
}

// A cancelled parent context turns into the run_killed failure, naming the
// integration whose registry handle was cancelled.
func TestRunCapturedReportsAKillWithTheIntegrationID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture runs through sh")
	}
	ctx, cancel := context.WithCancel(context.Background())
	registry := NewRunRegistry()
	forget := registry.Add("test.ext", cancel)
	defer forget()

	// Cancel once the run has certainly started.
	go func() {
		registry.Kill("test.ext")
	}()

	run, err := RunCaptured(ctx, runEntry("sleep 5"), nil)
	if err == nil {
		t.Fatal("a cancelled run reported success")
	}
	if run.TimedOut {
		t.Errorf("a killed run is not a timeout: %+v", run)
	}
	failure, ok := err.(*RunFailure)
	if !ok {
		t.Fatalf("error = %T %v", err, err)
	}
	if failure.Key != RunErrorKilled || failure.String("extension_id") != "test.ext" {
		t.Errorf("failure = %s %v", failure.Key, failure.Payload)
	}
}
