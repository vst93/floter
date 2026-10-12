package extensions

import (
	"context"
	"testing"
)

// Freshness reads the sidecar the re-probe writes and the health report the
// repository keeps: the probe's own time and counts first, the health report's
// status and time as the fallback source.
func TestFreshnessOfGeneratedIntegration(t *testing.T) {
	paths, _ := reprobeFixture(t, `#!/bin/sh
if [ "$1" = "--help" ]; then
  echo "tool - Gadgets"
  echo "Available Plugins"
  echo "📦 one 1.0"
  echo "📦 two 1.0"
elif [ "$2" = "--help" ]; then
  echo "Options:"
  echo "  -x    do x"
else
  exit 3
fi
`)
	// Nothing probed yet: "never probed" is not a fresh state.
	integration, _ := LoadInventory(paths).WithID("dev.floter.tool")
	before := FreshnessOf(paths, integration)
	if before.Known() {
		t.Errorf("a never-probed integration reads as known: %+v", before)
	}
	if before.Delta != DeltaUnknown {
		t.Errorf("delta = %q, want unknown", before.Delta)
	}
	// A first probe: known, with nothing to compare against.
	if _, err := ReprobeCommands(context.Background(), paths, "dev.floter.tool"); err != nil {
		t.Fatalf("reprobe: %v", err)
	}
	integration, _ = LoadInventory(paths).WithID("dev.floter.tool")
	first := FreshnessOf(paths, integration)
	if !first.Known() || first.Source != FreshnessProbe {
		t.Fatalf("first probe = %+v", first)
	}
	// A probe that ran is a success even with no health report beside it:
	// "we probed and it worked" is what the timestamp already says.
	if first.Result != FreshnessSuccess {
		t.Errorf("a recorded probe = %q, want success", first.Result)
	}
	if first.CommandCount == nil || *first.CommandCount != 3 {
		t.Errorf("command count = %v, want 3", first.CommandCount)
	}
	if first.Delta != DeltaUnknown || first.PreviousCommandCount != nil {
		t.Errorf("a first probe has nothing to compare against: %+v", first)
	}
	// A second probe compares against the first.
	if _, err := ReprobeCommands(context.Background(), paths, "dev.floter.tool"); err != nil {
		t.Fatalf("second reprobe: %v", err)
	}
	integration, _ = LoadInventory(paths).WithID("dev.floter.tool")
	second := FreshnessOf(paths, integration)
	if second.PreviousCommandCount == nil || *second.PreviousCommandCount != 3 {
		t.Errorf("previous = %v, want 3", second.PreviousCommandCount)
	}
	if second.Delta != DeltaUnchanged || second.DeltaCount() != 0 {
		t.Errorf("delta = %q/%d, want unchanged", second.Delta, second.DeltaCount())
	}
}

// The delta's four cases, including the two "unknowns" that must never be
// painted as a number.
func TestFreshnessDelta(t *testing.T) {
	count := func(n int) *int { return &n }
	cases := []struct {
		current, previous *int
		want              CommandDeltaKind
	}{
		{count(5), count(3), DeltaIncrease},
		{count(3), count(5), DeltaDecrease},
		{count(4), count(4), DeltaUnchanged},
		{count(4), nil, DeltaUnknown},
		{nil, count(4), DeltaUnknown},
		{nil, nil, DeltaUnknown},
	}
	for _, c := range cases {
		if got := deltaOf(c.current, c.previous); got != c.want {
			t.Errorf("deltaOf(%v, %v) = %q, want %q", c.current, c.previous, got, c.want)
		}
	}
	// The count of a decrease is its size, not its sign.
	f := Freshness{CommandCount: count(3), PreviousCommandCount: count(7)}
	if got := f.DeltaCount(); got != 4 {
		t.Errorf("DeltaCount = %d, want 4", got)
	}
}

// A health report with no sidecar is a source of its own, and its status maps
// onto the freshness vocabulary.
func TestFreshnessFromHealthReport(t *testing.T) {
	for _, c := range []struct {
		status string
		want   FreshnessResult
	}{
		{HealthHealthy, FreshnessSuccess},
		{HealthDegraded, FreshnessDegraded},
		{HealthUnhealthy, FreshnessFailed},
		{"", FreshnessUnknown},
		{"something-else", FreshnessUnknown},
	} {
		if got := resultOf(c.status); got != c.want {
			t.Errorf("resultOf(%q) = %q, want %q", c.status, got, c.want)
		}
	}
}

// The row's one-word mark has exactly three states, and a state that is not a
// completed comparison can never read as synced: a first probe, a failed or
// degraded one, one still running, and a never-probed integration all land in
// unknown.
func TestFreshnessDotState(t *testing.T) {
	count := func(n int) *int { return &n }
	cases := []struct {
		name      string
		freshness Freshness
		want      FreshnessDot
	}{
		{"a growth", Freshness{Result: FreshnessSuccess, Delta: DeltaIncrease,
			CommandCount: count(6), PreviousCommandCount: count(4)}, FreshnessDotChanged},
		{"a shrink", Freshness{Result: FreshnessSuccess, Delta: DeltaDecrease,
			CommandCount: count(4), PreviousCommandCount: count(6)}, FreshnessDotChanged},
		{"a real comparison", Freshness{Result: FreshnessSuccess, Delta: DeltaUnchanged,
			CommandCount: count(4), PreviousCommandCount: count(4)}, FreshnessDotSynced},
		{"zero compared with zero", Freshness{Result: FreshnessSuccess, Delta: DeltaUnchanged,
			CommandCount: count(0), PreviousCommandCount: count(0)}, FreshnessDotSynced},
		// A first probe: a real success, but nothing was compared. Calling it
		// "in sync" would be the same fabricated confidence the delta refuses.
		{"a first probe", Freshness{Result: FreshnessSuccess, Delta: DeltaUnknown,
			CommandCount: count(3)}, FreshnessDotUnknown},
		{"a failed probe", Freshness{Result: FreshnessFailed, Delta: DeltaUnchanged,
			CommandCount: count(4), PreviousCommandCount: count(4)}, FreshnessDotUnknown},
		{"a degraded probe", Freshness{Result: FreshnessDegraded, Delta: DeltaUnchanged,
			CommandCount: count(4), PreviousCommandCount: count(4)}, FreshnessDotUnknown},
		{"a probe in flight", Freshness{Result: FreshnessRunning, Delta: DeltaUnchanged,
			CommandCount: count(4), PreviousCommandCount: count(4)}, FreshnessDotUnknown},
		{"never probed", Freshness{Result: FreshnessUnknown, Delta: DeltaUnknown}, FreshnessDotUnknown},
	}
	for _, c := range cases {
		if got := c.freshness.DotState(); got != c.want {
			t.Errorf("%s: DotState = %q, want %q", c.name, got, c.want)
		}
	}
}
