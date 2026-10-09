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
func (s *Store) Top(limit int, keep func(id string) bool) []string {
	s.mu.Lock()
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

// Save writes the file, atomically.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil
	}
	if s.records == nil {
		s.records = map[string]Record{}
	}
	data, err := json.MarshalIndent(s.records, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".usage-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	return os.Rename(name, s.path)
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
