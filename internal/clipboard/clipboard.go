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
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
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

// Label is the short name a caller that has no room for a preview uses: the
// first stored path of a file list, `[image]`/`[files]` for the kinds that
// carry no text, and the entry's own text. A list row shows Preview instead.
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

// imagePreviewMax is how long an image entry's caption preview may be before it
// is elided, and twice that is the ceiling for a text entry whatever the caller
// asks for — the old build's `IMAGE_PREVIEW_MAX`.
const imagePreviewMax = 60

// Preview is the one-line preview a list row shows for an entry: the first line
// of a text entry (elided at maxLength), and for an image the first line of the
// caption stored beside it, falling back to `[image WxH]` when there is none.
// A dimension the entry did not record prints as `?` rather than as a
// confident zero.
//
// The preview is never the whole text: a copied paragraph would otherwise push
// every other row off the list. Only the first line survives, because a row is
// one line and the second line of a capture is never the part that identifies
// it.
func (e Entry) Preview(maxLength int) string {
	source := e.Text
	if e.Kind != KindText {
		// A caption is stored trimmed; an older one may not be.
		source = strings.TrimLeftFunc(source, unicode.IsSpace)
	}
	firstLine := source
	if end := strings.IndexByte(source, '\n'); end >= 0 {
		firstLine = source[:end]
	}
	firstLine = strings.TrimSpace(firstLine)
	if e.Kind != KindText {
		if firstLine == "" {
			return "[image " + dimension(e.Width) + "x" + dimension(e.Height) + "]"
		}
		return elide(firstLine, imagePreviewMax)
	}
	cap := maxLength
	if cap < 1 {
		cap = 1
	}
	if cap > imagePreviewMax*2 {
		cap = imagePreviewMax * 2
	}
	return elide(firstLine, cap)
}

// dimension is one side of an image entry's size, `?` when it was not
// recorded. The stored zero is an absence, not a measurement.
func dimension(value int) string {
	if value <= 0 {
		return "?"
	}
	return strconv.Itoa(value)
}

// elide cuts a string to max runes and marks the cut. Cutting on runes rather
// than bytes is the one place this build is deliberately stricter than the old
// one: slicing UTF-16 code units could split a surrogate pair.
func elide(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "\u2026"
}

// FilesSummary is a file list's row preview: the last segment of the first
// stored path, and how many more paths the entry holds. The count is what tells
// a three-file copy from a one-file copy at a glance — the old build's
// `formatFilesPreview` reduced to the two facts the row prints.
func FilesSummary(paths []string) (basename string, extra int) {
	if len(paths) == 0 {
		return "", 0
	}
	_, basename = splitPath(paths[0])
	return basename, len(paths) - 1
}

// splitPath splits a stored path into its directory prefix and last segment,
// aware of both separators and of trailing ones. A path that is nothing but a
// separator is its own last segment, so a row never shows an empty name.
func splitPath(path string) (dirname, basename string) {
	trimmed := strings.TrimRight(path, `/\`)
	index := strings.LastIndexAny(trimmed, `/\`)
	if index < 0 {
		if trimmed == "" {
			return "", path
		}
		return "", trimmed
	}
	if last := trimmed[index+1:]; last != "" {
		return trimmed[:index], last
	}
	return trimmed[:index], trimmed
}

// Age is the compact relative age a row's subtitle prints: `42m`, `5h`, and a
// concrete local `YYYY-MM-DD HH:mm` beyond a day. Seconds never appear — they
// churn too fast to read. An entry with no timestamp has no age at all.
func Age(createdAt, now time.Time) string {
	if createdAt.IsZero() {
		return ""
	}
	seconds := int64(now.Sub(createdAt) / time.Second)
	if seconds < 0 {
		seconds = 0
	}
	minutes := seconds / 60
	if minutes < 60 {
		return strconv.FormatInt(minutes, 10) + "m"
	}
	hours := minutes / 60
	if hours < 24 {
		return strconv.FormatInt(hours, 10) + "h"
	}
	return createdAt.Format("2006-01-02 15:04")
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

// ImageHash is an image entry's identity: its PNG bytes, and — when a copy
// carried text beside the pixels — that caption. Re-copying the same pixels
// with different text is different history; the same pair hashes equal and
// dedupes. An empty caption hashes exactly as the pixels alone, which is what
// keeps AddImage's digest unchanged.
func ImageHash(png []byte, caption string) string {
	if caption == "" {
		return hashBytes(png)
	}
	hashed := make([]byte, 0, len(png)+len(caption))
	hashed = append(hashed, png...)
	hashed = append(hashed, caption...)
	return hashBytes(hashed)
}

// CaptionFor is the text stored beside an image: the clipboard's text trimmed,
// or "" when it is whitespace-only or too large to keep. Whitespace-only text
// does not count — screenshots routinely ride along invisible leftovers — and
// it is stored trimmed so a row preview's first line is the caption's first
// line.
func CaptionFor(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || len(trimmed) > MaxTextBytes {
		return ""
	}
	return trimmed
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

	// pendingRemovals are the image files a committed write will drop. They
	// are collected while the change is still uncommitted and deleted only
	// after the index lands (see mutateLocked): a write that fails must not
	// leave a live entry pointing at a file it just deleted.
	pendingRemovals []string
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
	return s.mutateLocked(func() error {
		s.pruneLocked()
		return nil
	})
}

// mutateLocked is the one write path of the history: it applies a change and
// publishes it only once the index is on disk. A failed write leaves the
// memory, the index and the image files exactly as they were — the reference's
// `update_entries`, "Commit the memory snapshot only after its persistence
// operation succeeds" (`clipboard_history/store.rs:233-241`, and its test
// `failed_persistence_does_not_publish_a_mutated_cache`, `store.rs:403-420`).
//
// The copy is shallow on purpose: every mutation here assigns struct fields or
// rebuilds the slice, and none edits an entry's `extra` map in place.
func (s *Store) mutateLocked(mutate func() error) error {
	s.loadLocked()
	before := slices.Clone(s.entries)
	s.pendingRemovals = nil
	if err := mutate(); err != nil {
		s.entries = before
		s.pendingRemovals = nil
		return err
	}
	if err := s.saveLocked(); err != nil {
		s.entries = before
		s.pendingRemovals = nil
		return err
	}
	s.dropPendingImagesLocked()
	return nil
}

// dropPendingImagesLocked deletes the image files of the entries a committed
// write dropped. Only a committed write gets here: a file is never removed for
// a change that did not land.
func (s *Store) dropPendingImagesLocked() {
	for _, name := range s.pendingRemovals {
		_ = os.Remove(filepath.Join(s.paths.Images(), filepath.Base(name)))
	}
	s.pendingRemovals = nil
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

// searchFields are the strings one entry is searched through: a file list
// matches on any stored path, everything else on its text, and an image entry
// also answers to its own words. A bare screenshot has no caption to search, so
// "image"/"img"/"图片" are the only words that can find it — and the chip that
// reveals it is named 图片, so the word and the row agree (the old build's
// `clipboardEntrySearchFields`).
func searchFields(entry Entry) []string {
	if entry.Kind == KindFiles {
		return entry.Paths
	}
	fields := make([]string, 0, 4)
	if entry.Text != "" {
		fields = append(fields, entry.Text)
	}
	if entry.Kind == KindImage {
		fields = append(fields, "image", "img", "图片")
	}
	return fields
}

// matches reports whether every term is found in one of the entry's own
// fields: case-insensitive, AND over the terms, and each term answered by a
// single field rather than by a concatenation of them (so a term cannot be
// spelled across the seam between a caption and a path).
func matches(entry Entry, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	fields := searchFields(entry)
	for _, term := range terms {
		found := false
		for _, field := range fields {
			if strings.Contains(strings.ToLower(field), term) {
				found = true
				break
			}
		}
		if !found {
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
	if err := s.mutateLocked(func() error {
		entry = s.insertLocked(entry)
		s.pruneLocked()
		return nil
	}); err != nil {
		return Entry{}, false, err
	}
	return entry, true, nil
}

// AddImage records an image clip with no caption beside it.
func (s *Store) AddImage(png []byte, width, height int) (Entry, bool, error) {
	return s.AddImageCaption(png, width, height, "")
}

// AddImageCaption records an image clip: the PNG is written under images/ with
// the entry's id as its name, and an entry with the same digest moves to the
// front. The caption is the text the same copy carried (an editor handing over
// pixels plus their caption), stored as the entry's own text so one copy is one
// entry — never the text alone shadowing the image.
func (s *Store) AddImageCaption(png []byte, width, height int, caption string) (Entry, bool, error) {
	if len(png) == 0 {
		return Entry{}, false, nil
	}
	caption = CaptionFor(caption)
	hash := ImageHash(png, caption)

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
		Text:      caption,
		ImageFile: file,
		Width:     width,
		Height:    height,
		Hash:      hash,
		CreatedAt: s.now().UnixMilli(),
		extra:     map[string]any{},
	}
	if err := s.mutateLocked(func() error {
		entry = s.insertLocked(entry)
		s.pruneLocked()
		return nil
	}); err != nil {
		// The row never landed, so the pixels must not linger either: the
		// reference takes the freshly written PNG back when its save fails
		// (`clipboard_history/monitor.rs:294-299`).
		_ = os.Remove(filepath.Join(s.paths.Images(), file))
		return Entry{}, false, err
	}
	return entry, true, nil
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
	return s.addFilesEntry(cleaned, filesIdentity(cleaned))
}

// AddFilesCapture records a file-list clip that already went through
// PrepareFiles: the paths it kept, under the identity it computed from their
// canonical forms.
func (s *Store) AddFilesCapture(capture FilesCapture) (Entry, bool, error) {
	if len(capture.Paths) == 0 {
		return Entry{}, false, nil
	}
	return s.addFilesEntry(capture.Paths, capture.Hash)
}

// addFilesEntry is the file-list insert both entry points share.
func (s *Store) addFilesEntry(cleaned []string, hash string) (Entry, bool, error) {
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
	if err := s.mutateLocked(func() error {
		entry = s.insertLocked(entry)
		s.pruneLocked()
		return nil
	}); err != nil {
		return Entry{}, false, err
	}
	return entry, true, nil
}

// insertLocked puts an entry at the front, folding an earlier copy of the same
// clip into it — with one exception: a copy the user favorited is never folded
// away. It stays where it is, and its flag carries onto the fresh entry that
// lands on top. Re-copying something must not unpin it, and must not make the
// star disappear along with the row.
//
// This is the reference's rule, verbatim
// (`clipboard_history/store.rs:206-217` `fold_capture`:
// `retain(|entry| !matches(entry) || entry.favorite)` then
// `capture.favorite = carries_favorite`).
//
// It returns the entry that was actually stored: the fresh one, now possibly
// carrying the flag it inherited, so a caller never reports a `Favorite` the
// history does not have.
func (s *Store) insertLocked(entry Entry) Entry {
	for _, existing := range s.entries {
		if existing.Kind == entry.Kind && existing.Hash == entry.Hash && existing.Favorite {
			entry.Favorite = true
			break
		}
	}
	kept := make([]Entry, 0, len(s.entries)+1)
	kept = append(kept, entry)
	for _, existing := range s.entries {
		if existing.Kind == entry.Kind && existing.Hash == entry.Hash && !existing.Favorite {
			continue
		}
		kept = append(kept, existing)
	}
	s.entries = kept
	return entry
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
			return s.mutateLocked(func() error {
				s.entries[i].Favorite = favorite
				s.pruneLocked()
				return nil
			})
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
		return s.mutateLocked(func() error {
			removed := s.entries[i]
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			if removed.ImageFile != "" {
				s.pendingRemovals = append(s.pendingRemovals, removed.ImageFile)
			}
			return nil
		})
	}
	return ErrNotFound
}

// Clear drops the unfavourited entries, as the old build's "clear history"
// did, and their images.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
	return s.mutateLocked(func() error {
		kept := make([]Entry, 0, len(s.entries))
		for _, entry := range s.entries {
			if entry.Favorite {
				kept = append(kept, entry)
				continue
			}
			if entry.ImageFile != "" {
				s.pendingRemovals = append(s.pendingRemovals, entry.ImageFile)
			}
		}
		s.entries = kept
		return nil
	})
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
		// A file list's identity is a set of canonical paths, and an index
		// written by an earlier build carries whatever the identity function
		// of that build produced (a newline-joined digest of the spellings it
		// stored). Refresh it in memory so the next copy of the same files
		// dedupes instead of adding a duplicate row; the stored paths are the
		// user's own strings and are never rewritten here (the old build's
		// `load_index`, `main:src-tauri/src/clipboard_history/mod.rs:90-101`).
		if entries[i].Kind == KindFiles && len(entries[i].Paths) > 0 {
			entries[i].Hash = filesIdentity(entries[i].Paths)
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
			// A dropped image takes its file with it — once the pruned history
			// is committed, never before (see mutateLocked).
			if entry.ImageFile != "" {
				s.pendingRemovals = append(s.pendingRemovals, entry.ImageFile)
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
