// Package usage remembers what the user launches, so the launcher's empty
// query can offer what they use most.
//
// It is a small JSON file beside the settings, keyed by the thing launched
// (an application's path, an integration command's id). Nothing here is
// authoritative: a missing or unreadable file is an empty history, and a
// write failure never stops a launch.
package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"floter/internal/statefile"
)

// fileName is the file under the app's config root.
const fileName = "usage.json"

// Record is one launched thing.
type Record struct {
	// Count is how many times it was launched.
	Count int `json:"count"`
	// Last is when it was last launched, in milliseconds since the epoch.
	Last int64 `json:"last"`
}

// Store is the launch history. It is safe from any goroutine.
type Store struct {
	path string

	mu      sync.Mutex
	records map[string]Record
	loaded  bool
	now     func() time.Time
}

// Open reads the usage file under an app config root. A missing or
// unreadable file is an empty history, not an error: the file is a
// convenience, never a source of truth.
func Open(root string) *Store {
	return &Store{path: filepath.Join(root, fileName), now: time.Now}
}

// FilePath is where the store reads and writes.
func (s *Store) FilePath() string { return s.path }

// Record counts one launch and writes the file.
func (s *Store) Record(id string) error {
	if id == "" {
		return nil
	}
	s.mu.Lock()
	s.loadLocked()
	record := s.records[id]
	record.Count++
	record.Last = s.now().UnixMilli()
	s.records[id] = record
	s.mu.Unlock()
	return s.Save()
}

// Seed fills in the counts an earlier build recorded for ids this history has
// never seen, and leaves the ones it has alone: the settings file's
// `launch_counts` is where those builds kept the same information, so a user
// moving to this build keeps the "most used" list they had. It reports whether
// anything changed, and does not write: the caller decides when to save.
func (s *Store) Seed(counts map[string]int) bool {
	if len(counts) == 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	changed := false
	for id, count := range counts {
		if id == "" || count <= 0 {
			continue
		}
		if _, seen := s.records[id]; seen {
			continue
		}
		// No timestamp is known for these: the count is what carries them,
		// and Top breaks a tie by recency.
		s.records[id] = Record{Count: count}
		changed = true
	}
	return changed
}

// Count is how many times an id was launched.
func (s *Store) Count(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	return s.records[id].Count
}

// Top returns the ids used most, most-used first and, between equals, most
// recent first. Only ids the caller knows (in keep) are returned, so a
// deleted application does not linger in the list.
//
// It loads the file first, like `Count`: this is the *first* thing a cold
// start asks (the shell's `refreshRecents`), and the store is lazy — `Open`
// only remembers the path. Reading before loading answered 「no history」 for
// the whole first session, so the empty query had no most-used block until the
// user launched something (the report 「常用应用」不见).
func (s *Store) Top(limit int, keep func(id string) bool) []string {
	s.mu.Lock()
	s.loadLocked()
	ids := make([]string, 0, len(s.records))
	records := make(map[string]Record, len(s.records))
	for id, record := range s.records {
		records[id] = record
		ids = append(ids, id)
	}
	s.mu.Unlock()

	known := ids[:0]
	for _, id := range ids {
		if keep == nil || keep(id) {
			known = append(known, id)
		}
	}
	sort.SliceStable(known, func(i, j int) bool {
		left, right := records[known[i]], records[known[j]]
		if left.Count != right.Count {
			return left.Count > right.Count
		}
		return left.Last > right.Last
	})
	if limit > 0 && len(known) > limit {
		known = known[:limit]
	}
	return known
}

// Save writes the file, atomically. It loads first: this store has no "write
// this whole map" entry point, so a save from a store that never read the file
// can only be a mistake — and it used to be a silent one, an empty map over
// the history.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil
	}
	s.loadLocked()
	data, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return err
	}
	// P11-k: one writer for every state file — this is the copy that used to
	// skip the flush, so the rename could land before the bytes did.
	return statefile.Write(s.path, append(data, '\n'), 0o600)
}

// loadLocked reads the file once.
func (s *Store) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	s.records = map[string]Record{}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var records map[string]Record
	if err := json.Unmarshal(data, &records); err != nil {
		return
	}
	if records != nil {
		s.records = records
	}
}
