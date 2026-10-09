// Package clipboard reads and writes floter's clipboard history.
//
// The disk format is the one every earlier build wrote:
// `clipboard-history/index.json` holds the entries and
// `clipboard-history/images/` one PNG per image entry, so an existing history
// is found as it is and a new entry is one an older build can still read.
//
// Retention is the old build's too: favourites are never dropped, the newest
// `maxItems` (the `clipboard_history_max_items` setting, 300 by default)
// non-favourites are kept, and anything older than thirty days goes.
package clipboard

import (
	"crypto/rand"
	"crypto/sha256"
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
	dirName       = "clipboard-history"
	indexFileName = "index.json"
	imagesDirName = "images"
)

// The shipped retention.
const (
	DefaultMaxItems = 300
	MinMaxItems     = 10
	MaxMaxItems     = 500
	Retention       = 30 * 24 * time.Hour
)

// The entry kinds the old build recorded.
const (
	KindText  = "text"
	KindImage = "image"
	KindFiles = "files"
)

// Paths is where the history lives.
type Paths struct {
	// Root is the history directory (<config dir>/floter/clipboard-history).
	Root string
}

// FromConfigRoot builds the clipboard paths under the app's config root.
func FromConfigRoot(configRoot string) Paths {
	return Paths{Root: filepath.Join(configRoot, dirName)}
}

// Index is the index file.
func (p Paths) Index() string { return filepath.Join(p.Root, indexFileName) }

// Images is the directory of image entries.
func (p Paths) Images() string { return filepath.Join(p.Root, imagesDirName) }

// Entry is one clipboard entry. The fields a kind does not use stay zero; the
// keys this struct does not model are carried through, so a rewrite cannot
// drop what an older build recorded.
type Entry struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`

	Text string `json:"text,omitempty"`

	ImageFile string `json:"image_file,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`

	Paths []string `json:"paths,omitempty"`

	Hash      string `json:"hash"`
	CreatedAt int64  `json:"created_at"`
	Favorite  bool   `json:"favorite"`

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

var knownEntryKeys = []string{
	"id", "kind", "text", "image_file", "width", "height", "paths",
	"hash", "created_at", "favorite",
}

// Extra returns a copy of the keys this build does not model.
func (e Entry) Extra() map[string]any {
	out := make(map[string]any, len(e.extra))
	for key, value := range e.extra {
		out[key] = value
	}
	return out
}

// Time is when the entry was captured.
func (e Entry) Time() time.Time { return time.UnixMilli(e.CreatedAt) }

// Label is the one-line preview a list shows.
func (e Entry) Label() string {
	switch e.Kind {
	case KindImage:
		return "[image]"
	case KindFiles:
		if len(e.Paths) > 0 {
			return filepath.Base(e.Paths[0])
		}
		return "[files]"
	default:
		return strings.Join(strings.Fields(e.Text), " ")
	}
}

// HashText is the digest the index records for a text entry.
func HashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// hashBytes is the digest the index records for an image entry.
func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ErrNotFound is what an operation on an unknown id reports.
var ErrNotFound = errors.New("clipboard: no such entry")

// Store is the history, loaded once and written on every change. It is safe
// from any goroutine.
type Store struct {
	paths Paths

	mu       sync.Mutex
	entries  []Entry
	loaded   bool
	maxItems int
	now      func() time.Time
}

// NewStore builds a store over the history paths. maxItems caps the
// non-favourite entries; zero means the shipped default.
func NewStore(paths Paths, maxItems int) *Store {
	return &Store{paths: paths, maxItems: clampMaxItems(maxItems), now: time.Now}
}

// Paths is where the store reads and writes.
func (s *Store) Paths() Paths { return s.paths }

// SetMaxItems changes the capacity and prunes to it, writing what is left.
func (s *Store) SetMaxItems(maxItems int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxItems = clampMaxItems(maxItems)
	s.loadLocked()
	s.pruneLocked()
	return s.saveLocked()
}

// Entries is the history, newest first.
func (s *Store) Entries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	return s.snapshotLocked()
}

// Search finds the entries whose text (or path) contains every word of the
// query, newest first. An empty query returns the newest entries.
func (s *Store) Search(query string, limit int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()

	terms := strings.Fields(strings.ToLower(query))
	var out []Entry
	for _, entry := range s.entries {
		if matches(entry, terms) {
			out = append(out, entry)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}

func matches(entry Entry, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(entry.Text + " " + strings.Join(entry.Paths, " "))
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

// AddText records a text clip: an entry with the same digest moves to the
// front instead of being duplicated, and the store is pruned and written.
// It reports false for text already at the front, so a watcher does not
// rewrite the file for every poll.
func (s *Store) AddText(text string) (Entry, bool, error) {
	if strings.TrimSpace(text) == "" {
		return Entry{}, false, nil
	}
	hash := HashText(text)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()

	if len(s.entries) > 0 && s.entries[0].Kind == KindText && s.entries[0].Hash == hash {
		return s.entries[0], false, nil
	}
	entry := Entry{
		ID:        newID(),
		Kind:      KindText,
		Text:      text,
		Hash:      hash,
		CreatedAt: s.now().UnixMilli(),
		extra:     map[string]any{},
	}
	s.insertLocked(entry)
	s.pruneLocked()
	return entry, true, s.saveLocked()
}

// AddImage records an image clip: the PNG is written under images/ with the
// entry's id as its name, and an entry with the same digest moves to the
// front.
func (s *Store) AddImage(png []byte, width, height int) (Entry, bool, error) {
	if len(png) == 0 {
		return Entry{}, false, nil
	}
	hash := hashBytes(png)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	if len(s.entries) > 0 && s.entries[0].Kind == KindImage && s.entries[0].Hash == hash {
		return s.entries[0], false, nil
	}

	id := newID()
	file := id + ".png"
	if err := os.MkdirAll(s.paths.Images(), 0o755); err != nil {
		return Entry{}, false, err
	}
	if err := os.WriteFile(filepath.Join(s.paths.Images(), file), png, 0o600); err != nil {
		return Entry{}, false, err
	}
	entry := Entry{
		ID:        id,
		Kind:      KindImage,
		ImageFile: file,
		Width:     width,
		Height:    height,
		Hash:      hash,
		CreatedAt: s.now().UnixMilli(),
		extra:     map[string]any{},
	}
	s.insertLocked(entry)
	s.pruneLocked()
	return entry, true, s.saveLocked()
}

// AddFiles records a file-list clip.
func (s *Store) AddFiles(paths []string) (Entry, bool, error) {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		if trimmed := strings.TrimSpace(path); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return Entry{}, false, nil
	}
	hash := HashText(strings.Join(cleaned, "\n"))

	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	if len(s.entries) > 0 && s.entries[0].Kind == KindFiles && s.entries[0].Hash == hash {
		return s.entries[0], false, nil
	}
	entry := Entry{
		ID:        newID(),
		Kind:      KindFiles,
		Paths:     cleaned,
		Hash:      hash,
		CreatedAt: s.now().UnixMilli(),
		extra:     map[string]any{},
	}
	s.insertLocked(entry)
	s.pruneLocked()
	return entry, true, s.saveLocked()
}

// insertLocked puts an entry at the front, dropping an earlier copy of the
// same clip.
func (s *Store) insertLocked(entry Entry) {
	kept := make([]Entry, 0, len(s.entries)+1)
	kept = append(kept, entry)
	for _, existing := range s.entries {
		if existing.Kind == entry.Kind && existing.Hash == entry.Hash {
			continue
		}
		kept = append(kept, existing)
	}
	s.entries = kept
}

// ImagePath is where an image entry's PNG lives.
func (s *Store) ImagePath(entry Entry) string {
	if entry.ImageFile == "" {
		return ""
	}
	return filepath.Join(s.paths.Images(), filepath.Base(entry.ImageFile))
}

// SetFavorite pins or unpins an entry. Favourites are never pruned.
func (s *Store) SetFavorite(id string, favorite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	for i := range s.entries {
		if s.entries[i].ID == id {
			s.entries[i].Favorite = favorite
			s.pruneLocked()
			return s.saveLocked()
		}
	}
	return ErrNotFound
}

// Remove drops one entry, and its image file when it has one.
func (s *Store) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	for i := range s.entries {
		if s.entries[i].ID != id {
			continue
		}
		removed := s.entries[i]
		s.entries = append(s.entries[:i], s.entries[i+1:]...)
		if removed.ImageFile != "" {
			_ = os.Remove(filepath.Join(s.paths.Images(), filepath.Base(removed.ImageFile)))
		}
		return s.saveLocked()
	}
	return ErrNotFound
}

// Clear drops the unfavourited entries, as the old build's "clear history"
// did, and their images.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	kept := make([]Entry, 0, len(s.entries))
	for _, entry := range s.entries {
		if entry.Favorite {
			kept = append(kept, entry)
			continue
		}
		if entry.ImageFile != "" {
			_ = os.Remove(filepath.Join(s.paths.Images(), filepath.Base(entry.ImageFile)))
		}
	}
	s.entries = kept
	return s.saveLocked()
}

// loadLocked reads the index once. A missing or corrupt index is an empty
// history, not an error: losing captured entries is bad, taking the panel
// down with it is worse.
func (s *Store) loadLocked() {
	if s.loaded {
		return
	}
	s.loaded = true
	data, err := os.ReadFile(s.paths.Index())
	if err != nil {
		s.entries = nil
		return
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		s.entries = nil
		return
	}
	for i := range entries {
		if entries[i].extra == nil {
			entries[i].extra = map[string]any{}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].CreatedAt > entries[j].CreatedAt })
	s.entries = entries
}

func (s *Store) snapshotLocked() []Entry {
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return out
}

// pruneLocked applies the capacity and the age, keeping favourites.
func (s *Store) pruneLocked() {
	cutoff := s.now().Add(-Retention).UnixMilli()
	kept := make([]Entry, 0, len(s.entries))
	nonFavorites := 0
	for _, entry := range s.entries {
		if entry.Favorite {
			kept = append(kept, entry)
			continue
		}
		if entry.CreatedAt < cutoff || nonFavorites >= s.maxItems {
			// A dropped image takes its file with it.
			if entry.ImageFile != "" {
				_ = os.Remove(filepath.Join(s.paths.Images(), filepath.Base(entry.ImageFile)))
			}
			continue
		}
		nonFavorites++
		kept = append(kept, entry)
	}
	s.entries = kept
}

// saveLocked writes the index atomically: a temporary file in the same
// directory, then a rename over the target.
func (s *Store) saveLocked() error {
	if err := os.MkdirAll(s.paths.Root, 0o755); err != nil {
		return err
	}
	if s.entries == nil {
		s.entries = []Entry{}
	}
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.paths.Root, ".index-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return err
	}
	return os.Rename(name, s.paths.Index())
}

func clampMaxItems(maxItems int) int {
	if maxItems <= 0 {
		return DefaultMaxItems
	}
	if maxItems < MinMaxItems {
		return MinMaxItems
	}
	if maxItems > MaxMaxItems {
		return MaxMaxItems
	}
	return maxItems
}

// newID is a UUID v4, formatted as the old build's ids were.
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// No randomness at all: a clock-derived id is still unique enough
		// for a history entry, and never fails the capture.
		seed := uint64(time.Now().UnixNano())
		for i := range b {
			seed = seed*6364136223846793005 + 1442695040888963407
			b[i] = byte(seed >> 33)
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[0:4]) + "-" + hex.EncodeToString(b[4:6]) + "-" +
		hex.EncodeToString(b[6:8]) + "-" + hex.EncodeToString(b[8:10]) + "-" +
		hex.EncodeToString(b[10:16])
}
