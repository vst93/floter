package extensions

import (
	"strconv"
	"strings"
)

// The visible half of the silent drift re-probe: when the background probe
// that keeps a generated integration's command list current finds the list
// moved, exactly one notice is raised per actual change in a session. This
// file is the *pure* half of that — the frame's two outcomes (what the
// drawer shows, and whether a toast is owed) decided without a clock, a
// store, or a notification sink, so the ordering that keeps a no-op frame
// from eating the once-only slot is testable on its own.
//
// The backend is the one that decides to re-probe; none of this schedules
// anything. It is deliberately session-scoped: a relaunch re-arms, because
// "your commands just changed" is news, not a read/unread flag.

// ReprobeNoticeFrame is what a completed drift re-probe reports, as it
// arrives from the backend: which integration was probed, the tool version
// it was probed at, and the command counts before and after. A nil count is
// "the frame did not say", never zero.
type ReprobeNoticeFrame struct {
	ExtensionID string
	ToolVersion string
	// PreviousCommandCount and CommandCount are the list's size before and
	// after this probe; either may be nil.
	PreviousCommandCount *int
	CommandCount         *int
}

// DriftDisplay is the drawer's facts for one completed re-probe. It is
// produced for *every* frame — announced or not — because the freshness line
// reports what happened while the toast only reports what is worth
// interrupting for. Missing counts stay nil rather than being fabricated.
type DriftDisplay struct {
	ExtensionID          string
	ToolVersion          string
	CommandCount         *int
	PreviousCommandCount *int
}

// DriftDelta is one direction and magnitude of command-list drift.
type DriftDelta struct {
	Kind  CommandDeltaKind
	Count int
}

// DriftAnnouncement is the single toast one frame may raise.
type DriftAnnouncement struct {
	Delta DriftDelta
	Key   string
}

// DriftNoticeDecision is the two independent outcomes a notice frame
// produces: the display facts, always, and the announcement, only when this
// frame is news.
type DriftNoticeDecision struct {
	Display  DriftDisplay
	Announce *DriftAnnouncement
}

// DriftGate is what DecideDriftNotice consults to keep one change from being
// announced twice. It is an interface so the decision can be tested without
// a session.
type DriftGate interface {
	// Allow reports whether this key may be announced now; the gate
	// remembers a key it has allowed.
	Allow(key string) bool
}

// DriftNoticeGate is the session-scoped "tell the user once" gate: the first
// time a key is offered it allows it and remembers it, and every offer after
// that is refused. The zero value is not usable; call NewDriftNoticeGate.
type DriftNoticeGate struct {
	seen map[string]struct{}
}

// NewDriftNoticeGate returns an empty, armed gate.
func NewDriftNoticeGate() *DriftNoticeGate {
	return &DriftNoticeGate{seen: map[string]struct{}{}}
}

// Allow reports true the first time a key is offered, false every time after.
func (g *DriftNoticeGate) Allow(key string) bool {
	if g == nil {
		return true
	}
	if g.seen == nil {
		g.seen = map[string]struct{}{}
	}
	if _, ok := g.seen[key]; ok {
		return false
	}
	g.seen[key] = struct{}{}
	return true
}

// Size is how many distinct changes this session has announced.
func (g *DriftNoticeGate) Size() int {
	if g == nil {
		return 0
	}
	return len(g.seen)
}

// ReprobeNoticeKey is the stable identity of one drift notice, so a frame
// re-offered unchanged stays silent while a genuinely different change still
// speaks. The separator is NUL, which neither an id nor a version can carry.
func ReprobeNoticeKey(extensionID, toolVersion string, delta DriftDelta) string {
	var b strings.Builder
	b.WriteString(extensionID)
	b.WriteByte(0)
	b.WriteString(toolVersion)
	b.WriteByte(0)
	b.WriteString(string(delta.Kind))
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(delta.Count))
	return b.String()
}

// DecideDriftNotice decides what one drift notice frame means.
//
// Three rules, in this order, and the order is the whole point:
//
//  1. No change, no toast. A re-probe that left the command count alone has
//     nothing a user could act on. This is checked *before* the gate is
//     consulted, so a no-op frame can never burn the once-only slot and
//     silence the real change that follows. A missing count on either side is
//     "no change we can name" — nothing is fabricated.
//  2. No name, no toast. Without the row's name the message would have to say
//     "an integration", which is worse than waiting; the gate is therefore
//     consulted only once the caller has supplied a name.
//  3. Once per change, per session. The gate keys on (integration, version,
//     direction, magnitude), so a re-offered identical frame stays silent
//     while a genuinely different change still speaks.
//
// name is "" when the row is not in the list yet; the caller passes it in
// because only it can resolve ids to names.
func DecideDriftNotice(frame ReprobeNoticeFrame, name string, gate DriftGate) DriftNoticeDecision {
	previous := frame.PreviousCommandCount
	current := frame.CommandCount
	display := DriftDisplay{
		ExtensionID:          frame.ExtensionID,
		ToolVersion:          frame.ToolVersion,
		CommandCount:         current,
		PreviousCommandCount: previous,
	}
	if previous == nil || current == nil || *previous == *current {
		return DriftNoticeDecision{Display: display}
	}
	if name == "" {
		return DriftNoticeDecision{Display: display}
	}
	kind := DeltaIncrease
	if *current < *previous {
		kind = DeltaDecrease
	}
	count := *current - *previous
	if count < 0 {
		count = -count
	}
	delta := DriftDelta{Kind: kind, Count: count}
	key := ReprobeNoticeKey(frame.ExtensionID, frame.ToolVersion, delta)
	if gate != nil && !gate.Allow(key) {
		return DriftNoticeDecision{Display: display}
	}
	return DriftNoticeDecision{
		Display:  display,
		Announce: &DriftAnnouncement{Delta: delta, Key: key},
	}
}
