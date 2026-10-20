package shell

import (
	"strconv"
	"sync"
	"time"

	"floter/internal/extensions"
)

// The Integrations page's per-frame syscalls.
//
// `integrationList` (shell.go) rebuilds the whole list while the settings page
// draws it, i.e. sixty times a second, and two of its facts cost a system call
// per integration each:
//
//   - `extensions.ResolveRuntime` asks whether the entry's program is still
//     there. A `system` runtime with no recorded executable path is looked up by
//     name in every search directory — a stat per name per directory;
//   - `extensions.ManifestDigest` reads and hashes the manifest so the card can
//     say whether what the user approved is still what is on disk.
//
// A third one joined them when the freshness block landed: `integrationRow`
// asks `Actions.Freshness` per row per frame, which resolves the integration in
// the inventory (a lock and a linear scan) and then calls
// `extensions.FreshnessOf` — an `os.Stat` of the package directory and an
// `os.ReadFile` of `help-probe.json`. Rows multiply that: twenty integrations
// is twenty stat+read pairs a frame, on the same thread, on the same disks.
//
// None of these answers changes between two frames a sixtieth of a second apart, and on
// a machine whose PATH runs over a network share (or a sleeping disk, or a
// Windows drive letter that has been unmapped) each stat is milliseconds — the
// report 「很多界面经常卡死」 with the settings page on screen. This is the same
// shape as the Detected list (`detectedtools.go`) and the app icons
// (`apps/icons.go`): the drawing thread reads a memo, and the expensive half
// runs on a goroutine that asks for a frame when it lands.
//
// It differs in one deliberate way: the *first* question of a visit is answered
// in place. `RuntimeAvailable` decides whether a card offers its run verb or the
// repair one, so a memo that answered "not known yet" on the first frame would
// flash a healthy tool as broken. The first frame pays what the page always paid
// once; every frame after it is a map lookup, and a stale memo is refreshed
// behind the list.

// integrationFactsTTL is how long a computed fact stands before the list asks
// for a fresh one behind the next frame. Two seconds is short enough that a tool
// installed in a terminal shows up while the page is still open, and long enough
// that a page left open pays one pass every two seconds rather than sixty a
// second.
const integrationFactsTTL = 2 * time.Second

// integrationFact is what the page needs to know about one integration, plus the
// identity it was computed from: a reinstall changes the executable path or the
// manifest path, and that change is answered on the spot rather than waiting out
// the TTL.
type integrationFact struct {
	// available is whether the entry's runtime still resolves.
	available bool
	// digest is the manifest's on-disk digest, and hasDigest whether it could be
	// read at all. It is only asked for an entry that carries an approval.
	digest    string
	hasDigest bool
	// freshness is what the card's freshness block reports (the help-probe
	// sidecar beside the descriptor, else the repository's health report), and
	// hasFreshness whether anything has ever probed this integration — the
	// reference draws no block at all for one nothing has probed.
	freshness    extensions.Freshness
	hasFreshness bool
	// partial marks a fact that answered only the freshness question (it was
	// remembered from a single row's own ask — see `App.integrationFreshness`).
	// The list's pass is what answers the runtime probe and the digest, so a
	// partial fact is treated as unanswered there rather than served as a row
	// that offers the repair verb for a program that is right there.
	partial bool
	// key is the identity the fact was computed from (see integrationFactKey).
	key string
}

// integrationFactsCache is the process's one memo (one App per process, exactly
// as there is one inventory).
type integrationFactsCache struct {
	mu       sync.Mutex
	facts    map[string]integrationFact
	computed time.Time
	pending  bool
}

// integrationFactsMemo is the memo integrationList reads.
var integrationFactsMemo integrationFactsCache

// integrationFactKey is the identity of the thing the facts describe: where the
// program is, which manifest is on disk, and what the repository recorded for
// the approval. A reinstall, a repair, or a manifest that moved changes it, and
// that entry's fact is recomputed on the frame that sees it.
func integrationFactKey(integration extensions.Integration) string {
	key := integration.Entry.ExecutablePath + "\x00" + integration.ManifestPath()
	if integration.Entry.ApprovedAt != 0 {
		key += "\x00" + strconv.FormatUint(integration.Entry.ApprovedAt, 10)
	}
	return key
}

// computeIntegrationFacts is the expensive half: one stat walk per integration
// for the runtime, one read per approved manifest for the digest. It runs on the
// drawing thread only for the first frame of a visit (and for a single entry
// whose identity changed), and on the background pass otherwise.
func computeIntegrationFacts(items []extensions.Integration) map[string]integrationFact {
	out := make(map[string]integrationFact, len(items))
	for _, integration := range items {
		fact := integrationFact{key: integrationFactKey(integration)}
		if _, err := extensions.ResolveRuntime(integration); err == nil {
			fact.available = true
		}
		// The digest is only ever shown beside an approval record, so an entry
		// without one is not read at all.
		if integration.Entry.ApprovedAt != 0 {
			if digest, err := extensions.ManifestDigest(integration.ManifestPath()); err == nil {
				fact.digest, fact.hasDigest = digest, true
			}
		}
		// The freshness block's own read: the sidecar beside the descriptor and
		// the repository's health report, via `PackageDir` — a stat plus a file
		// read per integration, which is why it is here and not on the row.
		if freshness := extensions.FreshnessOf(integration.Paths, integration); freshness.Known() {
			fact.freshness, fact.hasFreshness = freshness, true
		}
		out[integration.Entry.ID] = fact
	}
	return out
}

// snapshot reads the memo's three fields under one lock.
func (m *integrationFactsCache) snapshot() (map[string]integrationFact, time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.facts, m.computed, m.pending
}

// publish installs a computed map and its clock.
func (m *integrationFactsCache) publish(facts map[string]integrationFact, at time.Time) {
	m.mu.Lock()
	m.facts, m.computed, m.pending = facts, at, false
	m.mu.Unlock()
}

// rememberIntegrationFact adds one computed fact to the memo without moving its
// clock, and copy-on-write like `publish`: the map handed to a reader stays
// immutable, so a frame already holding the old one keeps a consistent answer.
//
// It exists for the surface that asks about a single row without building the
// list first (see `App.integrationFreshness`): the question is answered once and
// remembered, so the frames after it are a map lookup rather than a stat.
func rememberIntegrationFact(m *integrationFactsCache, id string, fact integrationFact) {
	m.mu.Lock()
	merged := make(map[string]integrationFact, len(m.facts)+1)
	for key, value := range m.facts {
		merged[key] = value
	}
	merged[id] = fact
	m.facts = merged
	m.mu.Unlock()
}

// invalidate drops the memo, so the next frame recomputes in place. The
// integration list is reloaded after an install, a repair or a rescan, and a
// memo that outlived that would show the pre-change answer for up to a TTL.
func invalidateIntegrationFacts() {
	integrationFactsMemo.mu.Lock()
	integrationFactsMemo.facts = nil
	integrationFactsMemo.computed = time.Time{}
	integrationFactsMemo.mu.Unlock()
}

// refreshIntegrationFacts is the memo's whole policy, with the clock and the
// compute-and-invalidate pair as parameters so the rule is a unit test rather
// than a stopwatch:
//
//   - an entry whose identity changed is recomputed now (one stat, and only on
//     the frame that sees the change);
//   - an empty memo is computed now, so the first frame of a visit is right;
//   - a stale memo hands back what it has and asks for one background pass,
//     which asks for a frame when it lands.
func refreshIntegrationFacts(
	m *integrationFactsCache,
	items []extensions.Integration,
	compute func([]extensions.Integration) map[string]integrationFact,
	invalidate func(),
	now time.Time,
) map[string]integrationFact {
	facts, computed, _ := m.snapshot()

	// A changed identity is answered on the spot: one integration is one stat,
	// and the alternative — serving the old answer until the TTL runs out — is a
	// card showing the repair verb for a tool the user just installed.
	var changed []extensions.Integration
	for _, integration := range items {
		fact, known := facts[integration.Entry.ID]
		if !known || fact.partial || fact.key != integrationFactKey(integration) {
			changed = append(changed, integration)
		}
	}
	if len(changed) > 0 && !computed.IsZero() {
		fresh := compute(changed)
		merged := make(map[string]integrationFact, len(facts)+len(fresh))
		for id, fact := range facts {
			merged[id] = fact
		}
		for id, fact := range fresh {
			merged[id] = fact
		}
		m.publish(merged, computed)
		facts = merged
	}

	if computed.IsZero() || now.Sub(computed) >= integrationFactsTTL {
		if computed.IsZero() {
			// The visit's first question: answered in place (see the header).
			fresh := compute(items)
			m.publish(fresh, now)
			return fresh
		}
		// One pass at a time: a frame that arrives while the previous one is
		// still walking the PATH must not start a second.
		m.mu.Lock()
		start := !m.pending
		m.pending = true
		m.mu.Unlock()
		if start {
			go func() {
				fresh := compute(items)
				m.publish(fresh, time.Now())
				if invalidate != nil {
					invalidate()
				}
			}()
		}
	}
	return facts
}

// integrationFacts is the drawing thread's view of the memo (see
// refreshIntegrationFacts).
func (a *App) integrationFacts(items []extensions.Integration) map[string]integrationFact {
	return refreshIntegrationFacts(&integrationFactsMemo, items, computeIntegrationFacts, func() {
		if a.Win != nil {
			a.Win.Invalidate()
		}
	}, a.now())
}
