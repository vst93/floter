package calculator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// These pins hold the store to the reference's write contract: a change is
// published to memory only after the index write succeeds
// (`main:src-tauri/src/calculator_history.rs:169-186` `update` — lock, load,
// apply, prune, `save_index(&kept)?`; a failed save returns `Err` and the
// caller keeps the list it already had).
//
// Before this round every write path mutated the live slice first and saved
// second, so a failed write left a row the panel could see and the disk had
// never heard of. The sharpest case was `SetRetention`: the prune ran against
// the live slice, so a failed save left the rows gone from memory while the
// index still held them — and the next successful write made that prune
// permanent, deleting rows nobody ever asked to delete.
//
// The clipboard store got the same treatment in P11-c
// (`internal/clipboard/writefail_test.go`); the two stores share the contract,
// not the code.

// blockIndex makes every index write fail while leaving the rest of the history
// directory writable: a rename over a non-empty directory is refused, so
// saveLocked fails at its last step — after the temporary file was written —
// exactly like a full or read-only disk would. Permission bits would not do
// here: the suite may run as root, which ignores them.
// It returns the index bytes as they stood, so unblockIndex can put the very
// same file back and a pin can then read what the failed write left behind.
func blockIndex(t *testing.T, paths Paths) []byte {
	t.Helper()
	before, err := os.ReadFile(paths.Index())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(paths.Index()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(paths.Index(), "keep"), 0o755); err != nil {
		t.Fatal(err)
	}
	return before
}

// unblockIndex puts the index path back — with the bytes it held — so a write
// can land again and the disk can be read.
func unblockIndex(t *testing.T, paths Paths, before []byte) {
	t.Helper()
	if err := os.RemoveAll(paths.Index()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Index(), before, 0o600); err != nil {
		t.Fatal(err)
	}
}

// diskEntries reads the index as it stands, so a pin can tell "not in memory"
// from "not on disk".
func diskEntries(t *testing.T, paths Paths) []Entry {
	t.Helper()
	data, err := os.ReadFile(paths.Index())
	if err != nil {
		t.Fatal(err)
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	return entries
}

// A calculation whose index write failed is not in the history at all — not in
// memory, not on disk — and the very next attempt can still record it.
func TestFailedAddPublishesNothing(t *testing.T) {
	paths := FromConfigRoot(t.TempDir())
	store := NewStore(paths, DefaultMaxItems, 30)
	if _, err := store.Add("1+1", "2"); err != nil {
		t.Fatal(err)
	}

	before := blockIndex(t, paths)
	entry, err := store.Add("6*7", "42")
	if err == nil {
		t.Fatal("Add succeeded although the index could not be written")
	}
	if entry.ID != "" {
		t.Errorf("a failed Add returned an entry that was never published: %+v", entry)
	}
	if entries := store.Entries(); len(entries) != 1 || entries[0].Expression != "1+1" {
		t.Fatalf("a failed write published %d entries: %+v", len(entries), entries)
	}
	// The retry the reference would make next: same expression, now it lands,
	// and nothing was stacked on top of the phantom.
	unblockIndex(t, paths, before)
	if entries := diskEntries(t, paths); len(entries) != 1 || entries[0].Expression != "1+1" {
		t.Fatalf("the index on disk moved although the write failed: %+v", entries)
	}
	if _, err := store.Add("6*7", "42"); err != nil {
		t.Fatal(err)
	}
	entries := store.Entries()
	if len(entries) != 2 || entries[0].Expression != "6*7" {
		t.Fatalf("the retry = %+v", entries)
	}
	if entries := diskEntries(t, paths); len(entries) != 2 {
		t.Fatalf("the retry did not reach the disk: %+v", entries)
	}
}

// favoriteOf reads one row's flag out of the live history.
func favoriteOf(t *testing.T, store *Store, id string) bool {
	t.Helper()
	for _, entry := range store.Entries() {
		if entry.ID == id {
			return entry.Favorite
		}
	}
	t.Fatalf("entry %q is not in the history", id)
	return false
}

// A failed star flip leaves the old flag in place, and a failed toggle does not
// hand back a value that was never written.
func TestFailedFavoriteKeepsTheOldStar(t *testing.T) {
	paths := FromConfigRoot(t.TempDir())
	store := NewStore(paths, DefaultMaxItems, 30)
	starred, err := store.Add("1+1", "2")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetFavorite(starred.ID, true); err != nil {
		t.Fatal(err)
	}
	// A second, unstarred row: toggling it is the direction that would hand
	// back `true` — the value a failed write must not publish.
	plain, err := store.Add("2+2", "4")
	if err != nil {
		t.Fatal(err)
	}

	before := blockIndex(t, paths)
	if err := store.SetFavorite(starred.ID, false); err == nil {
		t.Fatal("SetFavorite succeeded although the index could not be written")
	}
	if !favoriteOf(t, store, starred.ID) {
		t.Error("a failed un-star published the new flag")
	}

	favorite, err := store.ToggleFavorite(plain.ID)
	if err == nil {
		t.Fatal("ToggleFavorite succeeded although the index could not be written")
	}
	if favorite {
		t.Error("a failed ToggleFavorite returned the value it wanted to publish")
	}
	if favoriteOf(t, store, plain.ID) {
		t.Error("a failed toggle published the flipped flag")
	}
	// Once writes land again the flip is a plain success.
	unblockIndex(t, paths, before)
	for _, entry := range diskEntries(t, paths) {
		if entry.ID == starred.ID && !entry.Favorite {
			t.Errorf("the index on disk lost the star: %+v", entry)
		}
		if entry.ID == plain.ID && entry.Favorite {
			t.Errorf("the index on disk gained a star: %+v", entry)
		}
	}
	if err := store.SetFavorite(starred.ID, false); err != nil {
		t.Fatal(err)
	}
	if favoriteOf(t, store, starred.ID) {
		t.Error("the retry did not un-star the row")
	}
}

// A delete and a clear whose write failed leave the rows where they were.
func TestFailedDeleteAndClearKeepTheRows(t *testing.T) {
	paths := FromConfigRoot(t.TempDir())
	store := NewStore(paths, DefaultMaxItems, 30)
	first, err := store.Add("1+1", "2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add("2+2", "4"); err != nil {
		t.Fatal(err)
	}

	before := blockIndex(t, paths)
	if err := store.Delete(first.ID); err == nil {
		t.Fatal("Delete succeeded although the index could not be written")
	}
	if entries := store.Entries(); len(entries) != 2 {
		t.Fatalf("a failed Delete dropped a row: %+v", entries)
	}
	if err := store.Clear(); err == nil {
		t.Fatal("Clear succeeded although the index could not be written")
	}
	if entries := store.Entries(); len(entries) != 2 {
		t.Fatalf("a failed Clear dropped rows: %+v", entries)
	}
	// The retry lands, and lands once.
	unblockIndex(t, paths, before)
	if entries := diskEntries(t, paths); len(entries) != 2 {
		t.Fatalf("the index on disk was trimmed although the writes failed: %+v", entries)
	}
	if err := store.Delete(first.ID); err != nil {
		t.Fatal(err)
	}
	if entries := store.Entries(); len(entries) != 1 || entries[0].Expression != "2+2" {
		t.Fatalf("the retry = %+v", entries)
	}
}

// A retention change whose write failed must not leave the prune behind: the
// rows the new capacity would drop are still in the index on disk, so they have
// to still be in memory too — otherwise the next successful write makes a
// prune nobody committed permanent.
func TestFailedRetentionDoesNotCommitThePrune(t *testing.T) {
	paths := FromConfigRoot(t.TempDir())
	store := NewStore(paths, DefaultMaxItems, 0)
	// More rows than the smallest capacity the band allows, so the prune the
	// retention asks for actually drops something.
	for _, expression := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		if _, err := store.Add(expression, "0"); err != nil {
			t.Fatal(err)
		}
	}

	before := blockIndex(t, paths)
	if err := store.SetRetention(MinMaxItems, 0); err == nil {
		t.Fatal("SetRetention succeeded although the index could not be written")
	}
	if entries := store.Entries(); len(entries) != 12 {
		t.Fatalf("a failed SetRetention published its prune: %+v", entries)
	}
	// The setting itself is not rolled back: it was persisted by the settings
	// page, and the store is only being told about it. It takes effect on the
	// next write that lands.
	if got := store.MaxItems(); got != MinMaxItems {
		t.Errorf("the retention the settings pushed = %d", got)
	}

	unblockIndex(t, paths, before)
	if entries := diskEntries(t, paths); len(entries) != 12 {
		t.Fatalf("the index on disk was pruned although the write failed: %+v", entries)
	}
	if _, err := store.Add("m", "0"); err != nil {
		t.Fatal(err)
	}
	if entries := store.Entries(); len(entries) != MinMaxItems || entries[0].Expression != "m" {
		t.Fatalf("the next write did not apply the retention: %+v", entries)
	}
}
