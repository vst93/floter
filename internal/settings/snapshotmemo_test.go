package settings

import (
	"path/filepath"
	"testing"
)

// P11-co · **一帧只拷一份设置。**
//
// `Store.Snapshot` is a deep copy (`clone` rebuilds the carried-through map),
// and a surface reads the settings from every row it draws — measured at 15
// copies per General frame, 53 per Integrations frame, 5 per terminal frame
// before this memo existed. `Memo` answers one copy per version; the arms
// below are the properties that make the cache sound:
//
//   - a hit is not a stale value: an accepted change bumps the version and the
//     next read takes a fresh copy (a preview slider depends on it);
//   - a *refused* write is read back as the rolled-back value, so a frame never
//     draws a value the file rejected;
//   - two stores are two values even though both start at version zero;
//   - a nil store draws with the zero value rather than panicking.
func TestAMemoServesOneSnapshotPerVersion(t *testing.T) {
	store := NewStore(Default())
	var memo Memo

	base := store.Snapshots()
	for i := 0; i < 200; i++ {
		_ = memo.Get(store)
	}
	if got := store.Snapshots() - base; got != 1 {
		t.Fatalf("200 reads took %d snapshots, want 1", got)
	}
	if got := memo.Builds(); got != 1 {
		t.Fatalf("the memo built %d times, want 1", got)
	}

	// An accepted change is a new version, and the next read sees it.
	if err := store.Update(func(s *Settings) { s.Language = "zh" }); err != nil {
		t.Fatalf("the update failed: %v", err)
	}
	if got := memo.Get(store).Language; got != "zh" {
		t.Fatalf("after a change the memo served language %q, want zh", got)
	}
	if got := memo.Builds(); got != 2 {
		t.Fatalf("a change left the memo at %d builds, want 2", got)
	}

	// A refused write rolls the store back; the frame must read what the file
	// accepted, not the value it tried to write. A store needs a path to write
	// at all, which is why this one is opened over a temp file.
	refusing, err := OpenStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatalf("the store did not open: %v", err)
	}
	refusing.save = func(string, Settings) error { return errRefused }
	var refused Memo
	before := refused.Get(refusing).Language
	if err := refusing.Update(func(s *Settings) { s.Language = "zh" }); err == nil {
		t.Fatal("the refused write reported success")
	}
	if got := refused.Get(refusing).Language; got != before {
		t.Fatalf("after a refused write the memo served language %q, want %q", got, before)
	}

	// Two stores both begin at version zero: the identity is part of the key.
	first, second := NewStore(Default()), NewStore(Default())
	if err := second.Update(func(s *Settings) { s.Language = "zh" }); err != nil {
		t.Fatalf("the update failed: %v", err)
	}
	var swapped Memo
	_ = swapped.Get(first)
	if got := swapped.Get(second).Language; got != "zh" {
		t.Fatalf("a swapped store served language %q, want zh", got)
	}

	// A nil store is the zero value, which is what a surface with no settings
	// to read draws with.
	var empty Memo
	if got := empty.Get(nil); got.Language != "" {
		t.Fatalf("a nil store served %v, want the zero value", got)
	}
}

// errRefused is a write the file refused, for the rollback arm above.
var errRefused = refusedError{}

type refusedError struct{}

func (refusedError) Error() string { return "the file refused the write" }
