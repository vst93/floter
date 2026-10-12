package extensions

import (
	"errors"
	"time"
)

// The freshness of a generated integration's command list: when it was last
// derived, how that derivation went, and how the list moved. The rule this
// exists to keep is that "we do not know" is never painted as a number — a
// missing sidecar, a first probe with nothing to compare against, and a
// genuine zero all look different, because a fabricated "0" or "no change" is
// a lie the user cannot see through.
//
// Nothing here is a version or upgrade concept: the command count moves
// because the *tool's own* `--help` output changed under floter's feet. The
// report observes that; it never offers to fetch anything.

// FreshnessSource says where the displayed time came from.
type FreshnessSource string

// The sources, best first.
const (
	FreshnessProbe  FreshnessSource = "probe"
	FreshnessHealth FreshnessSource = "health"
	FreshnessNone   FreshnessSource = "none"
)

// FreshnessResult is what the last probe did. Degraded is the health report's
// "only optional probes failed" — its own word rather than a rounded success
// or failure, both of which would be wrong.
type FreshnessResult string

// The results.
const (
	FreshnessRunning  FreshnessResult = "running"
	FreshnessSuccess  FreshnessResult = "success"
	FreshnessDegraded FreshnessResult = "degraded"
	FreshnessFailed   FreshnessResult = "failed"
	FreshnessUnknown  FreshnessResult = "unknown"
)

// CommandDeltaKind is how a command list moved against the probe before it.
type CommandDeltaKind string

// The deltas.
const (
	DeltaIncrease  CommandDeltaKind = "increase"
	DeltaDecrease  CommandDeltaKind = "decrease"
	DeltaUnchanged CommandDeltaKind = "unchanged"
	DeltaUnknown   CommandDeltaKind = "unknown"
)

// Freshness is what the settings drawer reports about one integration.
type Freshness struct {
	// AtSeconds is when the last probe ran, or 0 when nothing ever probed it.
	AtSeconds int64
	Source    FreshnessSource
	Result    FreshnessResult
	// CommandCount is the list's size at that probe, and
	// PreviousCommandCount the one before it. Either may be nil, which is
	// "unknown" and never zero.
	CommandCount         *int
	PreviousCommandCount *int
	Delta                CommandDeltaKind
	// Running marks a probe in flight right now.
	Running bool
}

// Known reports whether anything has ever probed this integration: a missing
// sidecar and no health report is "never probed", not a fresh state.
func (f Freshness) Known() bool { return f.Source != FreshnessNone }

// DeltaCount is how many commands the list moved by; only meaningful for an
// increase or a decrease.
func (f Freshness) DeltaCount() int {
	if f.CommandCount == nil || f.PreviousCommandCount == nil {
		return 0
	}
	delta := *f.CommandCount - *f.PreviousCommandCount
	if delta < 0 {
		return -delta
	}
	return delta
}

// FreshnessDot is the row's one-word freshness: a single mark answering "did
// this command list move since it was last compared".
type FreshnessDot string

// The three states, and only three — those are the three things a row can
// honestly say:
//
//   - FreshnessDotChanged: a probe compared this list with the one before it
//     and the count moved. The only state that requires positive evidence.
//   - FreshnessDotSynced: a probe compared the two and found them equal.
//   - FreshnessDotUnknown: there is nothing to compare — never probed, a first
//     probe with no earlier one, a probe in flight, or a failed/degraded probe.
const (
	FreshnessDotSynced  FreshnessDot = "synced"
	FreshnessDotChanged FreshnessDot = "changed"
	FreshnessDotUnknown FreshnessDot = "unknown"
)

// DotState projects what is known about an integration's command list onto the
// row's one-word mark. It reads Delta, not Result alone: a *first* probe is a
// success with nothing to compare against, and painting it "in sync" would be
// the fabricated confidence the delta already refuses to produce. Running,
// failed and degraded all land in unknown, as does a never-probed integration.
func (f Freshness) DotState() FreshnessDot {
	if f.Delta == DeltaIncrease || f.Delta == DeltaDecrease {
		return FreshnessDotChanged
	}
	if f.Result == FreshnessSuccess && f.Delta == DeltaUnchanged {
		return FreshnessDotSynced
	}
	return FreshnessDotUnknown
}

// FreshnessOf reads what is known about an integration's command list: the
// help-probe sidecar beside its descriptor (when the host generated it) and
// the health report the repository keeps. Best-effort by contract: an
// unreadable sidecar or a missing report is simply not a source.
func FreshnessOf(paths Paths, integration Integration) Freshness {
	freshness := Freshness{Source: FreshnessNone, Result: FreshnessUnknown, Delta: DeltaUnknown}
	if root := integration.PackageDir(); root != "" {
		if record := readHelpProbeRecord(root); record != nil {
			freshness.AtSeconds = record.ProbedAt
			freshness.Source = FreshnessProbe
			freshness.CommandCount = record.CommandCount
			freshness.PreviousCommandCount = record.PreviousCommandCount
			freshness.Delta = deltaOf(record.CommandCount, record.PreviousCommandCount)
		}
	}
	// The health report is the fallback source of a time when no sidecar
	// exists, and its own words win for the result when it has them.
	status := ""
	if report := integration.Entry.ProbeReport; report != nil && report.Status != "" {
		status = report.Status
		if freshness.Source == FreshnessNone {
			if at, err := time.Parse(time.RFC3339, report.CheckedAt); err == nil {
				freshness.AtSeconds = at.Unix()
				freshness.Source = FreshnessHealth
			}
		}
	}
	// A failed or degraded health report is the result outright; otherwise a
	// probe that actually ran is a success, and a build that has never probed
	// this integration stays unknown. A *first* probe with no earlier count is
	// a success here — it is the dot, not this, that refuses to call it "in
	// sync" (the dot reads the delta).
	switch result := resultOf(status); {
	case result == FreshnessFailed || result == FreshnessDegraded:
		freshness.Result = result
	case freshness.AtSeconds > 0:
		freshness.Result = FreshnessSuccess
	}
	return freshness
}

// deltaOf compares a probe's command count with the one before it. A missing
// side of the comparison is unknown, never unchanged: a first probe has
// nothing to compare against.
func deltaOf(current, previous *int) CommandDeltaKind {
	if current == nil || previous == nil {
		return DeltaUnknown
	}
	switch {
	case *current > *previous:
		return DeltaIncrease
	case *current < *previous:
		return DeltaDecrease
	default:
		return DeltaUnchanged
	}
}

// resultOf maps a health report's status onto the freshness vocabulary.
func resultOf(status string) FreshnessResult {
	switch status {
	case HealthHealthy:
		return FreshnessSuccess
	case HealthDegraded:
		return FreshnessDegraded
	case HealthUnhealthy:
		return FreshnessFailed
	default:
		return FreshnessUnknown
	}
}

// ErrNoFreshness is what a caller reports when there is nothing to show: an
// integration the host did not generate and whose health was never checked.
var ErrNoFreshness = errors.New("extensions: nothing has probed this integration")
