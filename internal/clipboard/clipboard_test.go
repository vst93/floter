package clipboard

import (
	"encoding/json"
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
