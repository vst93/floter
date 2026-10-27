package clipboard

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// These pins hold the store to the reference's write contract: a change is
// published to memory only after the index write succeeds
// (`clipboard_history/store.rs:233-241` `update_entries`, and its test
// `failed_persistence_does_not_publish_a_mutated_cache`, `store.rs:403-420`).
// Before this round the store mutated the live slice first and saved second, so
// a failed write left a row the panel could see and the disk had never heard
// of — and the watcher, which only remembers a hash after a successful write
// (P11-b), retried a capture that then looked like a duplicate of the phantom.

// blockIndex makes every index write fail while leaving the rest of the history
// directory writable: a rename over a non-empty directory is refused, so
// saveLocked fails at its last step — after the temporary file was written —
// exactly like a full or read-only disk would. Permission bits would not do
// here: the suite may run as root, which ignores them.
func blockIndex(t *testing.T, paths Paths) {
	t.Helper()
	if err := os.MkdirAll(paths.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(paths.Index()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(paths.Index(), "keep"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// unblockIndex puts the index path back so a write can land again.
func unblockIndex(t *testing.T, paths Paths) {
	t.Helper()
	if err := os.RemoveAll(paths.Index()); err != nil {
		t.Fatal(err)
	}
}

// A capture whose index write failed is not in the history at all, and the very
// next attempt (the watcher's next poll) can still record it.
func TestFailedWritePublishesNothing(t *testing.T) {
	paths := FromConfigRoot(t.TempDir())
	store := NewStore(paths, 0)
	blockIndex(t, paths)

	if _, _, err := store.AddText("first"); err == nil {
		t.Fatal("AddText succeeded although the index could not be written")
	}
	if entries := store.Entries(); len(entries) != 0 {
		t.Fatalf("a failed write published %d entries: %+v", len(entries), entries)
	}

	unblockIndex(t, paths)
	if _, added, err := store.AddText("first"); err != nil || !added {
		t.Fatalf("the retry after unblocking = %v, %v", added, err)
	}
	reopened := NewStore(paths, 0)
	if got := reopened.Entries(); len(got) != 1 || got[0].Text != "first" {
		t.Fatalf("the retried entry is not on disk: %+v", got)
	}
}

// A write that fails leaves the history that was already loaded untouched —
// same entries, same order, same flags — for every kind of capture.
func TestFailedWriteKeepsTheLoadedHistory(t *testing.T) {
	paths := FromConfigRoot(t.TempDir())
	store := NewStore(paths, 0)
	if _, _, err := store.AddImage(testPNG(t, 3, 3), 3, 3); err != nil {
		t.Fatal(err)
	}
	text, _, err := store.AddText("kept")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetFavorite(text.ID, true); err != nil {
		t.Fatal(err)
	}
	before := store.Entries()
	blockIndex(t, paths)

	if _, _, err := store.AddText("never lands"); err == nil {
		t.Error("AddText succeeded although the index could not be written")
	}
	if _, _, err := store.AddImageCaption(testPNG(t, 4, 4), 4, 4, "shot"); err == nil {
		t.Error("AddImageCaption succeeded although the index could not be written")
	}
	if _, _, err := store.AddFiles([]string{"/tmp/a.txt"}); err == nil {
		t.Error("AddFiles succeeded although the index could not be written")
	}
	if after := store.Entries(); !reflect.DeepEqual(before, after) {
		t.Fatalf("a failed write changed the history:\nbefore %+v\nafter  %+v", before, after)
	}
}

// A PNG written for a capture that never became a row is taken back: the
// reference deletes the freshly written file when its save fails
// (`clipboard_history/monitor.rs:294-299`).
func TestFailedImageWriteLeavesNoFileBehind(t *testing.T) {
	paths := FromConfigRoot(t.TempDir())
	store := NewStore(paths, 0)
	blockIndex(t, paths)

	if _, _, err := store.AddImageCaption(testPNG(t, 2, 2), 2, 2, "shot"); err == nil {
		t.Fatal("AddImageCaption succeeded although the index could not be written")
	}
	names, err := os.ReadDir(paths.Images())
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("a failed image write left %d file(s) behind: %v", len(names), names)
	}
	if entries := store.Entries(); len(entries) != 0 {
		t.Fatalf("a failed image write published %d entries", len(entries))
	}
}

// Pruning deletes an image only after the pruned index has landed. A failed
// write therefore keeps the file of the entry it was about to drop — the
// surviving history still points at it — and never keeps the file it just
// wrote.
func TestFailedPruneKeepsTheDroppedImages(t *testing.T) {
	paths := FromConfigRoot(t.TempDir())
	store := NewStore(paths, MinMaxItems)

	filled := make([]Entry, 0, MinMaxItems)
	for i := 0; i < MinMaxItems; i++ {
		entry, _, err := store.AddImageCaption(testPNG(t, 2, i+1), 2, i+1, "")
		if err != nil {
			t.Fatal(err)
		}
		filled = append(filled, entry)
	}
	oldest := filled[0]
	oldestPath := store.ImagePath(oldest)
	if _, err := os.Stat(oldestPath); err != nil {
		t.Fatalf("the oldest image is missing before the failed write: %v", err)
	}

	blockIndex(t, paths)
	fresh := testPNG(t, 2, MinMaxItems+1)
	if _, _, err := store.AddImageCaption(fresh, 2, MinMaxItems+1, ""); err == nil {
		t.Fatal("the overflow capture succeeded although the index could not be written")
	}
	if _, err := os.Stat(oldestPath); err != nil {
		t.Errorf("a failed write deleted the image of an entry it did not drop: %v", err)
	}
	if entries := store.Entries(); len(entries) != MinMaxItems || entries[len(entries)-1].ID != oldest.ID {
		t.Fatalf("a failed prune changed the history: %d entries", len(entries))
	}
	if names, err := os.ReadDir(paths.Images()); err == nil && len(names) != MinMaxItems {
		t.Errorf("images on disk = %d, want %d (the failed capture's own PNG must be gone)", len(names), MinMaxItems)
	}

	// Once the index can be written again, the same capture lands and the
	// prune's deletion happens for real.
	unblockIndex(t, paths)
	if _, added, err := store.AddImageCaption(fresh, 2, MinMaxItems+1, ""); err != nil || !added {
		t.Fatalf("the retry after unblocking = %v, %v", added, err)
	}
	if _, err := os.Stat(oldestPath); !os.IsNotExist(err) {
		t.Errorf("the committed prune kept the dropped image: %v", err)
	}
	if entries := store.Entries(); len(entries) != MinMaxItems {
		t.Errorf("history holds %d entries after the committed prune, want %d", len(entries), MinMaxItems)
	}
}

// Favourite, remove and clear are the same read-modify-write: when the index
// cannot be written they change nothing, and no image file is deleted for a
// change that did not land.
func TestFailedMutationsKeepTheOldState(t *testing.T) {
	paths := FromConfigRoot(t.TempDir())
	store := NewStore(paths, 0)
	image, _, err := store.AddImage(testPNG(t, 3, 3), 3, 3)
	if err != nil {
		t.Fatal(err)
	}
	text, _, err := store.AddText("kept")
	if err != nil {
		t.Fatal(err)
	}
	before := store.Entries()
	blockIndex(t, paths)

	if err := store.SetFavorite(image.ID, true); err == nil {
		t.Error("SetFavorite succeeded although the index could not be written")
	}
	if err := store.Remove(text.ID); err == nil {
		t.Error("Remove succeeded although the index could not be written")
	}
	if err := store.Clear(); err == nil {
		t.Error("Clear succeeded although the index could not be written")
	}
	if after := store.Entries(); !reflect.DeepEqual(before, after) {
		t.Fatalf("a failed mutation changed the history:\nbefore %+v\nafter  %+v", before, after)
	}
	if _, err := os.Stat(store.ImagePath(image)); err != nil {
		t.Errorf("a failed mutation deleted the image file: %v", err)
	}

	// And the same mutations still do their work once the index can be written.
	unblockIndex(t, paths)
	if err := store.SetFavorite(image.ID, true); err != nil {
		t.Fatalf("SetFavorite after unblocking: %v", err)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear after unblocking: %v", err)
	}
	if got := store.Entries(); len(got) != 1 || got[0].ID != image.ID || !got[0].Favorite {
		t.Fatalf("the committed clear kept %+v, want only the favourite", got)
	}
	if _, err := os.Stat(store.ImagePath(image)); err != nil {
		t.Errorf("the committed clear dropped the favourite's image: %v", err)
	}
}
