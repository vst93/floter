package shell

import (
	"sync"
	"time"

	"floter/internal/extensions"
)

// The Integrations page's second per-frame syscall: its configuration reads.
//
// `integrationList` (shell.go) calls `applyIntegrationConfig` per row, and that
// asks `extensions.LoadStoredConfiguration` for the values the card's form
// starts from. That is a read of `configuration.json` **plus** a read of the
// secrets generation file the document names — two blocking reads per
// configurable integration, per frame, on the drawing thread, for a page that
// redraws sixty times a second. The runtime/digest memo beside this one
// (integrationfacts.go) was written for exactly the same shape of cost; this is
// the half it did not cover, and the pending list had it as the last
// unexamined per-frame read on the drawing thread.
//
// On a spinning disk, an external drive that has gone to sleep, or a home
// directory over a network share, each read is milliseconds: twenty
// integrations is then well over a second of blocking work per frame — the
// report 「很多界面经常卡死」 with the settings page on screen.
//
// The values only change when this app writes them (a save) or, rarely, when
// another process does, so the memo is dropped by the save itself
// (saveIntegrationConfigurationNow) and by a refresh, and is re-probed behind
// the list on a TTL. The visit's first read is answered in place: the form's own
// starting values have to be right on the frame that draws it.

// storedConfigTTL is how long a read stands before the list asks for a fresh one
// behind the next frame. Two seconds matches the facts memo: short enough that a
// value changed from another floter window (or a settings.json edit) shows while
// the page is still open, long enough that a page left open costs one pass every
// two seconds instead of sixty a second.
const storedConfigTTL = 2 * time.Second

// storedConfig is one integration's configuration as the page last read it.
type storedConfig struct {
	// values are the stored values with the secrets generation folded in,
	// exactly as LoadStoredConfiguration returns them.
	values map[string]any
	// version is the generation the values were written against.
	version int
	// ok is false for a read that failed: the row then carries no values rather
	// than paying the same failing read on every frame after it.
	ok bool
	// key is the identity the entry was read for (see storedConfigKey).
	key string
}

// storedConfigKey is the identity of a stored configuration: the data root and
// the integration's id. A reinstall does not change it (the values outlive the
// package), a save does — and a save invalidates the memo directly.
func storedConfigKey(paths extensions.Paths, id string) string {
	return paths.Data + "\x00" + id
}

// storedConfigMemoCache is the process's one memo, like the facts memo: one App
// per process, so one cache.
type storedConfigMemoCache struct {
	mu       sync.Mutex
	entries  map[string]storedConfig
	computed time.Time
	pending  bool
}

// storedConfigMemoC is the memo integrationList reads.
var storedConfigMemoC storedConfigMemoCache

// readStoredConfig is the expensive half for one integration: the document and
// the secrets generation it names.
func readStoredConfig(integration extensions.Integration) storedConfig {
	paths, id := integration.Paths, integration.Entry.ID
	fact := storedConfig{key: storedConfigKey(paths, id)}
	stored, err := extensions.LoadStoredConfiguration(paths.Data, id)
	if err != nil {
		return fact
	}
	fact.values, fact.version, fact.ok = stored.Values, stored.ConfigVersion, true
	return fact
}

// computeStoredConfigs reads every configuration in the given set. It runs on
// the drawing thread only for entries the memo has never answered (the first
// frame of a visit, and a newly installed integration) and on the background
// pass otherwise.
func computeStoredConfigs(items []extensions.Integration) map[string]storedConfig {
	out := make(map[string]storedConfig, len(items))
	for _, integration := range items {
		out[integration.Entry.ID] = readStoredConfig(integration)
	}
	return out
}

// snapshot reads the memo's three fields under one lock.
func (m *storedConfigMemoCache) snapshot() (map[string]storedConfig, time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries, m.computed, m.pending
}

// publish installs a computed map and its clock.
func (m *storedConfigMemoCache) publish(entries map[string]storedConfig, at time.Time) {
	m.mu.Lock()
	m.entries, m.computed, m.pending = entries, at, false
	m.mu.Unlock()
}

// invalidateStoredConfigs drops the memo, so the next frame reads in place. The
// save path calls it: the values the user just committed are what the card has
// to draw, not the ones it read a moment ago.
func invalidateStoredConfigs() {
	storedConfigMemoC.mu.Lock()
	storedConfigMemoC.entries = nil
	storedConfigMemoC.computed = time.Time{}
	storedConfigMemoC.mu.Unlock()
}

// refreshStoredConfigs is the memo's whole policy, with the clock, the read and
// the frame request as parameters so the rule is a unit test rather than a
// stopwatch:
//
//   - an integration the memo has never answered is read now (one read, on the
//     frame that first carries it), so a newly installed row shows its values
//     on its first frame;
//   - an empty memo is therefore answered in place for the whole set, which is
//     the visit's first frame;
//   - a stale memo hands back what it has and asks for one background pass,
//     which requests a frame when it lands.
func refreshStoredConfigs(
	m *storedConfigMemoCache,
	items []extensions.Integration,
	compute func([]extensions.Integration) map[string]storedConfig,
	invalidate func(),
	now time.Time,
) map[string]storedConfig {
	entries, computed, _ := m.snapshot()

	var missing []extensions.Integration
	for _, integration := range items {
		fact, known := entries[integration.Entry.ID]
		if !known || fact.key != storedConfigKey(integration.Paths, integration.Entry.ID) {
			missing = append(missing, integration)
		}
	}
	if len(missing) > 0 && !computed.IsZero() {
		fresh := compute(missing)
		merged := make(map[string]storedConfig, len(entries)+len(fresh))
		for id, fact := range entries {
			merged[id] = fact
		}
		for id, fact := range fresh {
			merged[id] = fact
		}
		m.publish(merged, computed)
		entries = merged
	}

	if computed.IsZero() {
		// The visit's first question: everything is missing, so the pass above
		// (which had nothing to merge into) is the whole answer, read in place.
		fresh := compute(missing)
		m.publish(fresh, now)
		return fresh
	}

	if now.Sub(computed) >= storedConfigTTL {
		// One pass at a time: a frame that arrives while the previous one is
		// still reading must not start a second.
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
	return entries
}

// storedConfigs is the drawing thread's view of the memo (see
// refreshStoredConfigs).
func (a *App) storedConfigs(items []extensions.Integration) map[string]storedConfig {
	return refreshStoredConfigs(&storedConfigMemoC, items, computeStoredConfigs, func() {
		if a.Win != nil {
			a.Win.Invalidate()
		}
	}, a.now())
}
