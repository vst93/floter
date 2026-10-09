// Package calculator keeps the built-in calculator's history.
//
// The history lives under the config directory as
// `calculator-history/index.json`, written atomically (a temporary file in the
// same directory, flushed, then renamed over the target). A missing or corrupt
// index recovers as an empty history rather than taking the app down with it,
// and the keys this build does not model are carried through so a rewrite
// cannot drop what an earlier build recorded.
//
// Two retention axes apply, both configurable: a capacity on the non-favourite
// entries and an age window. A **favourite is exempt from both**, so a starred
// row outlives a full history and an expired one alike. Pruning happens on
// write — a new entry, a favourite toggle, a delete and a settings change each
// trim the index in the same critical section that persists it — so the read
// path stays a pure lookup with no surprise writes.
//
// Re-evaluating an expression promotes it to the top rather than stacking a
// second copy: a stored entry with the same expression text is folded into the
// new one, carrying the favourite flag forward.
package calculator

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// The directory and file names under the config root.
const (
	dirName       = "calculator-history"
	indexFileName = "index.json"
)

// The shipped retention, and the band the capacity is clamped to.
const (
	DefaultMaxItems     = 100
	MinMaxItems         = 10
	MaxMaxItems         = 500
	DefaultRetentionDay = 30
)

// The retention windows the settings page offers; 0 means "never expire".
var RetentionDays = []int{0, 1, 7, 30}

// The two copy modes: what Enter copies from a history row.
const (
	CopyFull   = "full"
	CopyResult = "result"
)

// Paths is where the history lives.
type Paths struct {
	// Root is the history directory
	// (<config dir>/floter/calculator-history).
	Root string
}

// FromConfigRoot builds the calculator paths under the app's config root.
func FromConfigRoot(configRoot string) Paths {
	return Paths{Root: filepath.Join(configRoot, dirName)}
}

// Index is the index file.
func (p Paths) Index() string { return filepath.Join(p.Root, indexFileName) }

// Entry is one recorded calculation. The keys this struct does not model are
// carried through, so a rewrite cannot drop what an older build recorded.
type Entry struct {
	ID string `json:"id"`
	// Expression is the expression exactly as the user typed it.
	Expression string `json:"expression"`
	// Result is the printed result: what Enter copies in result mode.
	Result string `json:"result"`
	// CreatedAt is Unix milliseconds.
	CreatedAt int64 `json:"created_at"`
	// Favorite entries are exempt from the capacity and the age window.
	Favorite bool `json:"favorite"`

	extra map[string]any
}

// UnmarshalJSON keeps the keys this struct does not model.
func (e *Entry) UnmarshalJSON(data []byte) error {
	type plain Entry
	var typed plain
	if err := json.Unmarshal(data, &typed); err != nil {
		return err
	}
	var all map[string]any
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	*e = Entry(typed)
	e.extra = map[string]any{}
	for _, key := range knownEntryKeys {
		delete(all, key)
	}
	for key, value := range all {
		e.extra[key] = value
	}
	return nil
}

// MarshalJSON writes the entry with every key it carries.
func (e Entry) MarshalJSON() ([]byte, error) {
	type plain Entry
	data, err := json.Marshal(plain(e))
	if err != nil {
		return nil, err
	}
	if len(e.extra) == 0 {
		return data, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	for key, value := range e.extra {
		if _, owned := out[key]; !owned {
			out[key] = value
		}
	}
	return json.Marshal(out)
}

var knownEntryKeys = []string{"id", "expression", "result", "created_at", "favorite"}

// Extra returns a copy of the keys this build does not model.
func (e Entry) Extra() map[string]any {
	out := make(map[string]any, len(e.extra))
	for key, value := range e.extra {
		out[key] = value
	}
	return out
}

// Time is the entry's timestamp.
func (e Entry) Time() time.Time { return time.UnixMilli(e.CreatedAt) }

// Label is the one-line text a list shows.
func (e Entry) Label() string {
	if strings.TrimSpace(e.Result) == "" {
		return e.Expression
	}
	return e.Expression + " = " + e.Result
}

// Text is what Enter copies for a copy mode.
func (e Entry) Text(mode string) string {
	if mode == CopyResult {
		return e.Result
	}
	return e.Label()
}

// Store is the history, kept in memory and persisted on every change. It is
// safe from any goroutine.
type Store struct {
	mu sync.Mutex

	paths Paths
	// maxItems and retentionDays are the retention the settings ask for; the
	// caller updates them when they change.
	maxItems      int
	retentionDays int
	// now is the clock, so a test can age entries.
	now func() time.Time

	entries []Entry
}

// NewStore opens the history at paths, pruning it to the retention the caller
// asks for. A missing or corrupt index is an empty history, not an error.
func NewStore(paths Paths, maxItems, retentionDays int) *Store {
	s := &Store{
		paths:         paths,
		maxItems:      clampMaxItems(maxItems),
		retentionDays: NormalizeRetentionDays(retentionDays),
		now:           time.Now,
	}
	s.entries = loadIndex(paths.Index())
	s.mu.Lock()
	s.pruneLocked()
	s.mu.Unlock()
	return s
}

// Paths is where the store reads and writes.
func (s *Store) Paths() Paths { return s.paths }

// Entries returns a copy of the history, newest first.
func (s *Store) Entries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Entry{}, s.entries...)
}

// Search returns the entries matching a query, newest first. Every
// whitespace-separated term must appear in the expression or the result;
// favouritesOnly narrows the list to the starred rows.
func (s *Store) Search(query string, favoritesOnly bool, limit int) []Entry {
	terms := strings.Fields(strings.ToLower(query))
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if favoritesOnly && !entry.Favorite {
			continue
		}
		if !matches(entry, terms) {
			continue
		}
		out = append(out, entry)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Add records a calculation, folding a previous entry with the same
// expression into the new top row (carrying its favourite flag forward).
func (s *Store) Add(expression, result string) (Entry, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return Entry{}, errors.New("calculator: no expression to record")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := Entry{
		ID:         newID(),
		Expression: expression,
		Result:     result,
		CreatedAt:  s.now().UnixMilli(),
	}
	for _, existing := range s.entries {
		if existing.Expression == expression && existing.Favorite {
			entry.Favorite = true
			break
		}
	}
	kept := make([]Entry, 0, len(s.entries)+1)
	for _, existing := range s.entries {
		if existing.Expression == expression {
			continue
		}
		kept = append(kept, existing)
	}
	s.entries = append([]Entry{entry}, kept...)
	s.pruneLocked()
	return entry, s.saveLocked()
}

// SetFavorite stars or unstars one entry, and prunes: an unstared row is no
// longer exempt from the retention.
func (s *Store) SetFavorite(id string, favorite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for i := range s.entries {
		if s.entries[i].ID == id {
			s.entries[i].Favorite = favorite
			found = true
			break
		}
	}
	if !found {
		return ErrNoEntry
	}
	s.pruneLocked()
	return s.saveLocked()
}

// ToggleFavorite flips one entry's star.
func (s *Store) ToggleFavorite(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.entries {
		if s.entries[i].ID != id {
			continue
		}
		s.entries[i].Favorite = !s.entries[i].Favorite
		favorite := s.entries[i].Favorite
		s.pruneLocked()
		return favorite, s.saveLocked()
	}
	return false, ErrNoEntry
}

// Delete drops one entry.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]Entry, 0, len(s.entries))
	found := false
	for _, entry := range s.entries {
		if entry.ID == id {
			found = true
			continue
		}
		kept = append(kept, entry)
	}
	if !found {
		return ErrNoEntry
	}
	s.entries = kept
	return s.saveLocked()
}

// Clear drops every entry that is not a favourite.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.Favorite {
			kept = append(kept, entry)
		}
	}
	s.entries = kept
	return s.saveLocked()
}

// SetRetention updates the capacity and the age window, and prunes.
func (s *Store) SetRetention(maxItems, retentionDays int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxItems = clampMaxItems(maxItems)
	s.retentionDays = NormalizeRetentionDays(retentionDays)
	s.pruneLocked()
	return s.saveLocked()
}

// MaxItems and RetentionDays are the retention in force.
func (s *Store) MaxItems() int      { return s.maxItems }
func (s *Store) RetentionDays() int { return s.retentionDays }

// ErrNoEntry is what an operation reports for an id the history does not know.
var ErrNoEntry = errors.New("calculator: no such entry")

// pruneLocked applies the retention: favourites are exempt from both axes, and
// of the rest the expired ones go first, then the oldest beyond the capacity.
func (s *Store) pruneLocked() {
	cutoff := retentionCutoff(s.now().UnixMilli(), s.retentionDays)
	fresh := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if !entry.Favorite && entry.CreatedAt < cutoff {
			continue
		}
		fresh = append(fresh, entry)
	}
	if s.maxItems > 0 {
		kept := make([]Entry, 0, len(fresh))
		ordinary := 0
		for _, entry := range fresh {
			if entry.Favorite {
				kept = append(kept, entry)
				continue
			}
			ordinary++
			if ordinary > s.maxItems {
				continue
			}
			kept = append(kept, entry)
		}
		fresh = kept
	}
	s.entries = fresh
}

// retentionCutoff is the age cutoff for a window of days at now; the oldest
// possible time when the window is off, so nothing is ever older than it.
func retentionCutoff(now int64, days int) int64 {
	if days <= 0 {
		return -1 << 62
	}
	return now - int64(days)*24*60*60*1000
}

// matches reports whether every term appears in the entry.
func matches(entry Entry, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(entry.Expression + " " + entry.Result)
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

// loadIndex reads the index, recovering from a missing or corrupt file with an
// empty history.
func loadIndex(path string) []Entry {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	// A row with no identity or no expression cannot be shown or acted on;
	// dropping it here keeps every later write honest.
	kept := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.ID == "" || strings.TrimSpace(entry.Expression) == "" {
			continue
		}
		kept = append(kept, entry)
	}
	return kept
}

// saveLocked writes the index atomically: a temporary file in the same
// directory, flushed, then renamed over the target.
func (s *Store) saveLocked() error {
	if err := os.MkdirAll(s.paths.Root, 0o755); err != nil {
		return err
	}
	entries := s.entries
	if entries == nil {
		entries = []Entry{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(s.paths.Root, ".index-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	if err := os.Rename(name, s.paths.Index()); err != nil {
		return err
	}
	if dir, err := os.Open(s.paths.Root); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

// clampMaxItems snaps a capacity into the shipped band; a non-positive value
// falls back to the default rather than to a bound.
func clampMaxItems(value int) int {
	if value <= 0 {
		return DefaultMaxItems
	}
	if value < MinMaxItems {
		return MinMaxItems
	}
	if value > MaxMaxItems {
		return MaxMaxItems
	}
	return value
}

// NormalizeRetentionDays accepts one of the offered windows; anything else is
// the shipped one.
func NormalizeRetentionDays(days int) int {
	for _, allowed := range RetentionDays {
		if days == allowed {
			return days
		}
	}
	return DefaultRetentionDay
}

// NormalizeCopyMode accepts one of the two copy modes; anything else is `full`.
func NormalizeCopyMode(mode string) string {
	if mode == CopyResult {
		return CopyResult
	}
	return CopyFull
}

// SortNewestFirst orders entries by their timestamp, newest first.
func SortNewestFirst(entries []Entry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].CreatedAt > entries[j].CreatedAt })
}

// newID builds an entry id: a timestamp and random bytes, so two entries made
// in the same millisecond still differ.
func newID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000000")))
	}
	return hex.EncodeToString(buffer)
}
