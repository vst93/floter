package clipboard

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const legacyIndex = `[
  {
    "id": "86eca049-4df0-4c2c-8167-2720bb9da876",
    "kind": "text",
    "text": "git checkout mygo-rewrite",
    "hash": "c6890851b525334bb80d67822811bc4fb6803eee4f887e1b4c4696518c14950c",
    "created_at": 1791526157324,
    "favorite": false,
    "source_app": "Terminal"
  },
  {
    "id": "bac91cfd-c0cd-4b22-9028-459fc7581c04",
    "kind": "image",
    "image_file": "bac91cfd-c0cd-4b22-9028-459fc7581c04.png",
    "width": 816,
    "height": 848,
    "hash": "064d3cf27ffe699086aaf7dea0bc161ffa6567e73882ede0f5d7276f9009c83b",
    "created_at": 1791520926857,
    "favorite": true
  },
  {
    "id": "1df7793f-651e-452a-af86-8695a34eaea2",
    "kind": "files",
    "paths": ["/tmp/a.png"],
    "hash": "d93a2b9894ce279c25160b1710d1531490d98e5782bdbc14ce3884abf162b15b",
    "created_at": 1791520019040,
    "favorite": false
  }
]`

func writeIndex(t *testing.T, paths Paths, content string) {
	t.Helper()
	if err := os.MkdirAll(paths.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Index(), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReadsTheOldIndex(t *testing.T) {
	dir := t.TempDir()
	paths := FromConfigRoot(dir)
	if paths.Root != filepath.Join(dir, "clipboard-history") {
		t.Fatalf("root = %q", paths.Root)
	}
	writeIndex(t, paths, legacyIndex)

	store := NewStore(paths, 0)
	entries := store.Entries()
	if len(entries) != 3 {
		t.Fatalf("entries = %d", len(entries))
	}
	// Newest first.
	if entries[0].Kind != KindText || entries[1].Kind != KindImage || entries[2].Kind != KindFiles {
		t.Errorf("order = %v", []string{entries[0].Kind, entries[1].Kind, entries[2].Kind})
	}
	text := entries[0]
	if text.Text != "git checkout mygo-rewrite" || text.Favorite {
		t.Errorf("text entry = %+v", text)
	}
	if text.Time().UnixMilli() != text.CreatedAt {
		t.Errorf("time = %v", text.Time())
	}
	// An unknown key survives a round trip.
	if got := text.Extra()["source_app"]; got != "Terminal" {
		t.Errorf("unknown key = %v", text.Extra())
	}
	image := entries[1]
	if image.Width != 816 || image.Height != 848 || !image.Favorite {
		t.Errorf("image entry = %+v", image)
	}
	if image.Label() != "[image]" || entries[2].Label() != "a.png" {
		t.Errorf("labels = %q / %q", image.Label(), entries[2].Label())
	}

	// A rewrite keeps unknown keys.
	store.SetFavorite(text.ID, true)
	data, err := os.ReadFile(paths.Index())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "source_app") {
		t.Errorf("the unknown key was dropped:\n%s", data)
	}
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw[0]["favorite"] != true {
		t.Errorf("the text entry is not the favourite: %v", raw[0]["favorite"])
	}
}

func TestAddTextDedupesAndPrunes(t *testing.T) {
	dir := t.TempDir()
	paths := FromConfigRoot(dir)
	store := NewStore(paths, 0)

	// The clock is fixed so timestamps and pruning are deterministic.
	now := time.UnixMilli(1_000_000_000)
	store.now = func() time.Time { return now }

	first, added, err := store.AddText("hello")
	if err != nil || !added {
		t.Fatalf("AddText = %v, %v", added, err)
	}
	now = now.Add(time.Second)
	if _, added, err = store.AddText("hello"); err != nil || added {
		t.Errorf("the same text was added twice: %v", added)
	}
	now = now.Add(time.Second)
	if _, _, err = store.AddText("   "); err != nil {
		t.Errorf("blank text: %v", err)
	}
	now = now.Add(time.Second)
	if _, _, err = store.AddText("world"); err != nil {
		t.Fatal(err)
	}

	entries := store.Entries()
	if len(entries) != 2 || entries[0].Text != "world" || entries[1].Text != "hello" {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].ID == first.ID {
		t.Error("ids are not distinct")
	}

	// Re-adding an old clip moves it to the front instead of duplicating.
	now = now.Add(time.Second)
	if _, added, err = store.AddText("hello"); err != nil || !added {
		t.Fatalf("re-adding = %v, %v", added, err)
	}
	entries = store.Entries()
	if len(entries) != 2 || entries[0].Text != "hello" {
		t.Errorf("entries = %+v", entries)
	}

	// The capacity keeps the newest and never drops a favourite.
	if err := store.SetMaxItems(MinMaxItems); err != nil {
		t.Fatal(err)
	}
	favourite := ""
	for _, entry := range store.Entries() {
		if entry.Text == "world" {
			favourite = entry.ID
		}
	}
	if favourite == "" {
		t.Fatalf("the world entry is gone: %+v", store.Entries())
	}
	if err := store.SetFavorite(favourite, true); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		now = now.Add(time.Second)
		if _, _, err := store.AddText(text); err != nil {
			t.Fatalf("add %q: %v", text, err)
		}
	}
	entries = store.Entries()
	favorites := 0
	for _, entry := range entries {
		if entry.Favorite {
			favorites++
		}
	}
	if favorites != 1 {
		t.Errorf("favourites = %d, want 1 kept", favorites)
	}
	if len(entries) > MinMaxItems+1 {
		t.Errorf("kept %d entries, want at most %d plus the favourite", len(entries), MinMaxItems)
	}
	if len(entries) != MinMaxItems+1 {
		t.Errorf("kept %d entries, want the cap plus the favourite", len(entries))
	}
	// The favourite survived the cap even though it is the oldest.
	found := false
	for _, entry := range entries {
		if entry.Favorite && entry.Text == "world" {
			found = true
		}
	}
	if !found {
		t.Errorf("the favourite was pruned: %+v", entries)
	}
}

func TestRetentionDropsOldEntries(t *testing.T) {
	dir := t.TempDir()
	paths := FromConfigRoot(dir)

	// An entry older than the retention goes; a favourite older than it
	// stays.
	old := time.Now().Add(-40 * 24 * time.Hour).UnixMilli()
	writeIndex(t, paths, `[
  {"id": "old", "kind": "text", "text": "old", "hash": "x", "created_at": `+strconv.FormatInt(old, 10)+`, "favorite": false},
  {"id": "old-fav", "kind": "text", "text": "old favourite", "hash": "y", "created_at": `+strconv.FormatInt(old, 10)+`, "favorite": true}
]`)
	store := NewStore(paths, 0)
	if _, _, err := store.AddText("fresh"); err != nil {
		t.Fatal(err)
	}
	entries := store.Entries()
	labels := make([]string, 0, len(entries))
	for _, entry := range entries {
		labels = append(labels, entry.Text)
	}
	if len(entries) != 2 || labels[0] != "fresh" || labels[1] != "old favourite" {
		t.Errorf("entries = %v", labels)
	}
}

func TestSearchAndMutations(t *testing.T) {
	dir := t.TempDir()
	paths := FromConfigRoot(dir)
	writeIndex(t, paths, legacyIndex)
	store := NewStore(paths, 0)

	if got := store.Search("", 0); len(got) != 3 {
		t.Errorf("empty query = %d entries", len(got))
	}
	if got := store.Search("MYGO", 0); len(got) != 1 || got[0].Kind != KindText {
		t.Errorf("case-insensitive search = %+v", got)
	}
	if got := store.Search("git checkout", 0); len(got) != 1 {
		t.Errorf("multi-word search = %+v", got)
	}
	if got := store.Search("a.png", 0); len(got) != 1 || got[0].Kind != KindFiles {
		t.Errorf("path search = %+v", got)
	}
	if got := store.Search("nothing", 0); len(got) != 0 {
		t.Errorf("no-match search = %+v", got)
	}
	if got := store.Search("", 2); len(got) != 2 {
		t.Errorf("limited search = %d", len(got))
	}

	// Remove drops the entry and keeps the file consistent.
	if err := store.Remove("bac91cfd-c0cd-4b22-9028-459fc7581c04"); err != nil {
		t.Fatal(err)
	}
	if err := store.Remove("nope"); err != ErrNotFound {
		t.Errorf("removing an unknown id = %v", err)
	}
	if got := store.Entries(); len(got) != 2 {
		t.Errorf("after remove = %d entries", len(got))
	}

	// Clear keeps the favourites, and only those.
	if err := store.SetFavorite("86eca049-4df0-4c2c-8167-2720bb9da876", true); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	got := store.Entries()
	if len(got) != 1 || !got[0].Favorite || got[0].Kind != KindText {
		t.Errorf("after clear = %+v", got)
	}

	// A missing and a corrupt index are empty histories, not failures.
	missing := NewStore(FromConfigRoot(t.TempDir()), 0)
	if entries := missing.Entries(); len(entries) != 0 {
		t.Errorf("a missing index = %+v", entries)
	}
	brokenPaths := FromConfigRoot(t.TempDir())
	writeIndex(t, brokenPaths, "{not json")
	if entries := NewStore(brokenPaths, 0).Entries(); len(entries) != 0 {
		t.Errorf("a corrupt index = %+v", entries)
	}
}

func TestClampMaxItems(t *testing.T) {
	cases := map[int]int{0: DefaultMaxItems, -5: DefaultMaxItems, 1: MinMaxItems, 300: 300, 10_000: MaxMaxItems}
	for in, want := range cases {
		if got := clampMaxItems(in); got != want {
			t.Errorf("clampMaxItems(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestAddImage(t *testing.T) {
	dir := t.TempDir()
	paths := FromConfigRoot(dir)
	store := NewStore(paths, 0)
	now := time.UnixMilli(1_700_000_000_000)
	store.now = func() time.Time { return now }

	png := testPNG(t, 8, 4)
	entry, added, err := store.AddImage(png, 8, 4)
	if err != nil || !added {
		t.Fatalf("AddImage = %v, %v", added, err)
	}
	if entry.Kind != KindImage || entry.Width != 8 || entry.Height != 4 {
		t.Errorf("entry = %+v", entry)
	}
	if entry.ImageFile != entry.ID+".png" {
		t.Errorf("image file = %q", entry.ImageFile)
	}
	// The PNG is on disk, at the path the store reports.
	path := store.ImagePath(entry)
	if data, err := os.ReadFile(path); err != nil || len(data) != len(png) {
		t.Fatalf("image file = %d bytes, %v", len(data), err)
	}
	if !strings.HasSuffix(path, filepath.Join("images", entry.ImageFile)) {
		t.Errorf("path = %q", path)
	}

	// The same image twice moves to the front instead of duplicating.
	now = now.Add(time.Second)
	if _, added, err := store.AddImage(png, 8, 4); err != nil || added {
		t.Errorf("the same image was added twice: %v", added)
	}
	if _, _, err := store.AddImage(nil, 0, 0); err != nil {
		t.Errorf("an empty image: %v", err)
	}
	entries := store.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}

	// Pruning a capacity eviction takes the file with it.
	now = now.Add(time.Second)
	if _, _, err := store.AddImage(testPNG(t, 9, 9), 9, 9); err != nil {
		t.Fatal(err)
	}
	first := store.Entries()[1]
	if err := store.SetMaxItems(MinMaxItems); err != nil {
		t.Fatal(err)
	}
	// Fill past the capacity so the oldest entries are evicted.
	for i := 0; i < MinMaxItems+2; i++ {
		now = now.Add(time.Second)
		if _, _, err := store.AddText(string(rune('a'+i%26)) + string(rune('0'+i%10))); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(store.ImagePath(first)); !os.IsNotExist(err) {
		t.Errorf("an evicted image's file stayed: %v", err)
	}
}

func TestAddFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(FromConfigRoot(dir), 0)
	now := time.UnixMilli(1_700_000_000_000)
	store.now = func() time.Time { return now }

	entry, added, err := store.AddFiles([]string{"  /tmp/a.png ", "/tmp/b.png", ""})
	if err != nil || !added {
		t.Fatalf("AddFiles = %v, %v", added, err)
	}
	if entry.Kind != KindFiles || len(entry.Paths) != 2 || entry.Paths[0] != "/tmp/a.png" {
		t.Errorf("entry = %+v", entry)
	}
	if entry.Label() != "a.png" {
		t.Errorf("label = %q", entry.Label())
	}
	now = now.Add(time.Second)
	if _, added, err := store.AddFiles([]string{"/tmp/a.png", "/tmp/b.png"}); err != nil || added {
		t.Errorf("the same file list was added twice: %v", added)
	}
	if _, added, err := store.AddFiles(nil); err != nil || added {
		t.Errorf("an empty file list added: %v", added)
	}
	if got := store.Search("b.png", 0); len(got) != 1 {
		t.Errorf("search by path = %+v", got)
	}
}

// The row preview: the first line of a capture, elided, with an image falling
// back to its recorded size — the old build's `clipboardPreview`.
func TestPreviewTakesTheFirstLineAndFallsBackToTheImageSize(t *testing.T) {
	cases := []struct {
		name  string
		entry Entry
		max   int
		want  string
	}{
		{
			name:  "text keeps only its first line",
			entry: Entry{Kind: KindText, Text: "git status\ngit diff\ngit log"},
			max:   96,
			want:  "git status",
		},
		{
			name:  "text is elided at the cap",
			entry: Entry{Kind: KindText, Text: strings.Repeat("x", 200)},
			max:   96,
			want:  strings.Repeat("x", 96) + "\u2026",
		},
		{
			name:  "the cap never exceeds twice the image preview",
			entry: Entry{Kind: KindText, Text: strings.Repeat("y", 400)},
			max:   1_000,
			want:  strings.Repeat("y", 120) + "\u2026",
		},
		{
			name:  "a non-positive cap is one rune, not none",
			entry: Entry{Kind: KindText, Text: "abcdef"},
			max:   0,
			want:  "a\u2026",
		},
		{
			name:  "an image with a caption shows the caption",
			entry: Entry{Kind: KindImage, Text: "\n  shot.png  \nsecond line", Width: 10, Height: 20},
			max:   96,
			want:  "shot.png",
		},
		{
			name:  "an image caption is elided at 60",
			entry: Entry{Kind: KindImage, Text: strings.Repeat("z", 100)},
			max:   96,
			want:  strings.Repeat("z", 60) + "\u2026",
		},
		{
			name:  "a bare image shows its size",
			entry: Entry{Kind: KindImage, Width: 816, Height: 848},
			max:   96,
			want:  "[image 816x848]",
		},
		{
			name:  "an unrecorded size is a question mark, not a zero",
			entry: Entry{Kind: KindImage},
			max:   96,
			want:  "[image ?x?]",
		},
		{
			name:  "an empty text entry previews as nothing",
			entry: Entry{Kind: KindText},
			max:   96,
			want:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.Preview(tc.max); got != tc.want {
				t.Errorf("Preview(%d) = %q, want %q", tc.max, got, tc.want)
			}
		})
	}
}

// A file list's row preview is its first name plus how many more it holds, and
// the split understands both separators.
func TestFilesSummaryCountsTheRest(t *testing.T) {
	cases := []struct {
		paths    []string
		basename string
		extra    int
	}{
		{nil, "", 0},
		{[]string{"/tmp/a.png"}, "a.png", 0},
		{[]string{`C:\Users\me\a.png`, `C:\Users\me\b.png`}, "a.png", 1},
		{[]string{"/tmp/dir/"}, "dir", 0},
		{[]string{"/tmp/a.txt", "/tmp/b.txt", "/tmp/c.txt"}, "a.txt", 2},
		{[]string{"bare.png"}, "bare.png", 0},
	}
	for _, tc := range cases {
		base, extra := FilesSummary(tc.paths)
		if base != tc.basename || extra != tc.extra {
			t.Errorf("FilesSummary(%v) = %q, %d; want %q, %d", tc.paths, base, extra, tc.basename, tc.extra)
		}
	}
}

// The compact age: minutes, then hours, then a concrete local time. Seconds
// never appear.
func TestAgeIsCompactUntilItNeedsADate(t *testing.T) {
	now := time.Date(2026, 10, 14, 15, 30, 0, 0, time.Local)
	cases := []struct {
		name string
		at   time.Time
		want string
	}{
		{"no timestamp", time.Time{}, ""},
		{"just now", now, "0m"},
		{"under an hour", now.Add(-42 * time.Minute), "42m"},
		{"exactly an hour", now.Add(-time.Hour), "1h"},
		{"under a day", now.Add(-5 * time.Hour), "5h"},
		{"a day or more", now.Add(-30 * time.Hour), now.Add(-30 * time.Hour).Format("2006-01-02 15:04")},
		{"the future never goes negative", now.Add(time.Hour), "0m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Age(tc.at, now); got != tc.want {
				t.Errorf("Age = %q, want %q", got, tc.want)
			}
		})
	}
}

// A bare image has no caption to search, so it answers to its own words — the
// same word its chip prints.
func TestImageEntriesAnswerToTheirOwnWords(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(FromConfigRoot(dir), 0)
	if _, _, err := store.AddImage(testPNG(t, 4, 4), 4, 4); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.AddText("nothing to do with pictures"); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"image", "IMG", "图片"} {
		got := store.Search(query, 0)
		if len(got) != 1 || got[0].Kind != KindImage {
			t.Errorf("Search(%q) = %+v", query, got)
		}
	}
}

// A caption is searchable through its own text, and the terms are ANDed over
// the entry's fields.
func TestImageCaptionIsSearchable(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(FromConfigRoot(dir), 0)
	entry, _, err := store.AddImage(testPNG(t, 2, 2), 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.loadLocked()
	for i := range store.entries {
		if store.entries[i].ID == entry.ID {
			store.entries[i].Text = "quarterly report screenshot"
		}
	}
	if err := store.saveLocked(); err != nil {
		t.Fatal(err)
	}
	store.mu.Unlock()

	if got := store.Search("report screenshot", 0); len(got) != 1 || got[0].ID != entry.ID {
		t.Errorf("caption search = %+v", got)
	}
	if got := store.Search("report missing", 0); len(got) != 0 {
		t.Errorf("a partial AND match = %+v", got)
	}
}

// testPNG builds a real PNG of the given size.
func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 128, A: 255})
		}
	}
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// The same batch of files copied again in another order is the same clip, not a
// second history row: the identity is the set of paths (P10-w).
func TestAddFilesIsOrderInsensitive(t *testing.T) {
	store := NewStore(FromConfigRoot(t.TempDir()), 50)
	if _, added, err := store.AddFiles([]string{"/tmp/a.txt", "/tmp/b.txt"}); err != nil || !added {
		t.Fatalf("AddFiles = %v, %v", added, err)
	}
	if _, added, err := store.AddFiles([]string{"/tmp/b.txt", "/tmp/a.txt"}); err != nil || added {
		t.Errorf("a reordered list was added twice: %v, %v", added, err)
	}
	if entries := store.Entries(); len(entries) != 1 {
		t.Fatalf("history holds %d entries, want 1", len(entries))
	}
	// Another list is another clip.
	if _, added, err := store.AddFiles([]string{"/tmp/a.txt", "/tmp/c.txt"}); err != nil || !added {
		t.Errorf("a different list was dropped: %v, %v", added, err)
	}
}

// An index written by an earlier build carries that build's file identity (a
// newline-joined digest). Loading it must refresh the identity in memory — the
// paths are the user's own strings and stay as they are — so the next copy of
// the same files dedupes instead of leaving a duplicate row (the old build's
// `load_index`, `main:src-tauri/src/clipboard_history/mod.rs:90-101`).
func TestLoadRefreshesFilesIdentity(t *testing.T) {
	dir := t.TempDir()
	paths := FromConfigRoot(dir)
	stored := []string{"/tmp/a.txt", "/tmp/b.txt"}
	legacy := []Entry{{
		ID:        "legacy",
		Kind:      KindFiles,
		Paths:     stored,
		Hash:      HashText(strings.Join(stored, "\n")),
		CreatedAt: time.Now().UnixMilli(),
	}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	writeIndex(t, paths, string(data))

	store := NewStore(paths, 50)
	entries := store.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want the one stored row", len(entries))
	}
	if entries[0].Hash != FilesHash(stored) {
		t.Errorf("hash = %q, want the refreshed %q", entries[0].Hash, FilesHash(stored))
	}
	if len(entries[0].Paths) != 2 || entries[0].Paths[0] != stored[0] || entries[0].Paths[1] != stored[1] {
		t.Errorf("paths = %q, want the stored strings untouched %q", entries[0].Paths, stored)
	}

	// The refreshed identity is the one a fresh capture produces, so the same
	// copy does not add a second row.
	capture, ok := PrepareFiles(stored, func(string) bool { return true })
	if !ok {
		t.Fatal("the capture was rejected")
	}
	if capture.Hash != entries[0].Hash {
		t.Errorf("capture hash = %q, loaded hash = %q", capture.Hash, entries[0].Hash)
	}
	if _, added, err := store.AddFilesCapture(capture); err != nil || added {
		t.Errorf("the same files were added again after the refresh: %v, %v", added, err)
	}
}

// The store's search rule is the reference's rule, vector for vector: the
// eighteen cases below are byte-for-byte the shared fixture in
// `main:src-tauri/src/clipboard_history/search.rs` (which the old build's node
// suite reads back to keep the two languages from drifting). They are pinned
// here so a later change to `searchFields`/`matches` has to face the same
// table the reference faces.
func TestSearchMatchesTheSharedContractVectors(t *testing.T) {
	cases := []struct {
		name   string
		kind   string
		text   string
		paths  []string
		query  string
		expect bool
	}{
		{"empty query matches everything", KindText, "hello", nil, "", true},
		{"whitespace-only query matches everything", KindText, "hello", nil, "   ", true},
		{"single token hits the text field", KindText, "hello world", nil, "hello", true},
		{"AND: two tokens in one text field", KindText, "hello world", nil, "world hello", true},
		{"AND: one missing token rejects", KindText, "hello world", nil, "hello missing", false},
		{"case-insensitive hit", KindText, "Rust Async", nil, "rUsT aSYNC", true},
		{"CJK token", KindText, "剪贴板历史", nil, "剪贴板", true},
		{"CJK two tokens AND", KindText, "剪贴板历史", nil, "历史 剪贴板", true},
		{"files entry matches a path token", KindFiles, "", []string{"/tmp/report.pdf"}, "report", true},
		{"files entry matches a full path token", KindFiles, "", []string{"/tmp/report.pdf"}, "/tmp/report.pdf", true},
		{"files entry ignores a stray text field", KindFiles, "hidden", []string{"/tmp/a.txt"}, "hidden", false},
		{"files entry misses an absent path token", KindFiles, "", []string{"/tmp/a.txt"}, "zzz", false},
		{"bare image answers to its name", KindImage, "", nil, "图片", true},
		{"bare image answers to img", KindImage, "", nil, "IMG", true},
		{"captioned image matches its caption", KindImage, "diagram", nil, "diagram", true},
		{"captioned image: tokens may hit different fields", KindImage, "diagram", nil, "diagram 图片", true},
		{"text entry does not answer to the image word", KindText, "diagram", nil, "diagram 图片", false},
		{"non-files text entry matches its text", KindText, "hidden", nil, "hidden", true},
	}
	for _, tc := range cases {
		entry := Entry{Kind: tc.kind, Text: tc.text, Paths: tc.paths}
		tokens := strings.Fields(strings.ToLower(tc.query))
		if got := matches(entry, tokens); got != tc.expect {
			t.Errorf("shared vector drifted: %s: matches(%q) = %v, want %v", tc.name, tc.query, got, tc.expect)
		}
	}
}
