package clipboard

import (
	"testing"
	"time"
)

// The fold contract, as the reference states it
// (`clipboard_history/store.rs:206-217` `fold_capture` + its four tests at
// store.rs:598-676): a fresh capture of content the history already holds lands
// on top with a fresh timestamp and the earlier non-favorite copy is folded
// away — but a copy the user favorited is never folded away, and its flag
// carries onto the fresh entry.
//
// These are one code path (`insertLocked`) with three entry points
// (`AddText` / `AddImageCaption` / `AddFiles`), so each invariant is pinned once
// and the favorite half is pinned per kind.

// foldClock is a store with a clock the test advances by hand.
func foldClock(t *testing.T) (*Store, func() time.Time, func(time.Duration)) {
	t.Helper()
	store := NewStore(FromConfigRoot(t.TempDir()), 0)
	now := time.UnixMilli(1_700_000_000_000)
	store.now = func() time.Time { return now }
	advance := func(d time.Duration) { now = now.Add(d) }
	return store, func() time.Time { return now }, advance
}

func findEntry(entries []Entry, id string) (Entry, bool) {
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return Entry{}, false
}

// A duplicate sitting on top is a no-op: same id, same timestamp, same flag.
// Refreshing the timestamp buys nothing — the copy is already where the user
// expects it.
func TestFoldTopDuplicateIsANoOpRegardlessOfFavorite(t *testing.T) {
	for _, favorite := range []bool{false, true} {
		store, now, advance := foldClock(t)
		top, _, err := store.AddText("hello")
		if err != nil {
			t.Fatal(err)
		}
		if favorite {
			if err := store.SetFavorite(top.ID, true); err != nil {
				t.Fatal(err)
			}
		}
		stamp := now()
		advance(5 * time.Second)

		again, added, err := store.AddText("hello")
		if err != nil {
			t.Fatal(err)
		}
		if added {
			t.Errorf("favorite=%v: the top duplicate was added again", favorite)
		}
		if again.ID != top.ID || again.CreatedAt != stamp.UnixMilli() || again.Favorite != favorite {
			t.Errorf("favorite=%v: entry = %+v, want the untouched top %+v", favorite, again, top)
		}
		if entries := store.Entries(); len(entries) != 1 {
			t.Errorf("favorite=%v: %d entries, want 1", favorite, len(entries))
		}
	}
}

// An older duplicate moves to the top with a fresh timestamp, and the older
// non-favorite copy is gone.
func TestFoldOlderDuplicateMovesToTopWithFreshTimestamp(t *testing.T) {
	store, now, advance := foldClock(t)
	older, _, err := store.AddText("hello")
	if err != nil {
		t.Fatal(err)
	}
	advance(time.Second)
	newer, _, err := store.AddText("world")
	if err != nil {
		t.Fatal(err)
	}
	advance(time.Minute)

	fresh, added, err := store.AddText("hello")
	if err != nil || !added {
		t.Fatalf("re-adding = %v, %v", added, err)
	}
	entries := store.Entries()
	if len(entries) != 2 {
		t.Fatalf("%d entries, want 2: %+v", len(entries), entries)
	}
	if entries[0].ID != fresh.ID || entries[0].Text != "hello" {
		t.Errorf("top = %+v, want the fresh copy", entries[0])
	}
	if entries[0].CreatedAt != now().UnixMilli() {
		t.Errorf("created_at = %d, want the fresh %d", entries[0].CreatedAt, now().UnixMilli())
	}
	if entries[1].ID != newer.ID {
		t.Errorf("second = %+v, want %q", entries[1], newer.ID)
	}
	if _, ok := findEntry(entries, older.ID); ok {
		t.Error("the older copy survived the fold")
	}
}

// The favorited copy stays where it is and the fresh one lands on top carrying
// its flag — re-copying something must never unpin it.
func TestFoldKeepsTheFavoritedDuplicateAndCarriesItsFlag(t *testing.T) {
	store, _, advance := foldClock(t)
	starred, _, err := store.AddText("hello")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetFavorite(starred.ID, true); err != nil {
		t.Fatal(err)
	}
	advance(time.Second)
	if _, _, err := store.AddText("world"); err != nil {
		t.Fatal(err)
	}
	advance(time.Minute)

	fresh, added, err := store.AddText("hello")
	if err != nil || !added {
		t.Fatalf("re-adding = %v, %v", added, err)
	}
	if !fresh.Favorite {
		t.Error("the returned entry did not carry the flag the history has")
	}
	entries := store.Entries()
	if len(entries) != 3 {
		t.Fatalf("%d entries, want 3 (fresh + world + the starred one): %+v", len(entries), entries)
	}
	if entries[0].ID != fresh.ID || !entries[0].Favorite {
		t.Errorf("top = %+v, want the fresh copy carrying the flag", entries[0])
	}
	kept, ok := findEntry(entries, starred.ID)
	if !ok {
		t.Fatal("the favorited entry was folded away")
	}
	if !kept.Favorite {
		t.Error("the favorited entry lost its flag")
	}
}

// A hash match only touches entries of the same kind: text, image and file
// hashes do not collide, and nothing else may be folded.
func TestFoldOnlyTouchesTheSameContent(t *testing.T) {
	store, _, advance := foldClock(t)
	text, _, err := store.AddText("hello")
	if err != nil {
		t.Fatal(err)
	}
	advance(time.Second)
	image, _, err := store.AddImage(testPNG(t, 8, 4), 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	advance(time.Second)
	files, _, err := store.AddFiles([]string{"/tmp/one.txt"})
	if err != nil {
		t.Fatal(err)
	}
	advance(time.Second)

	fresh, added, err := store.AddImage(testPNG(t, 8, 4), 8, 4)
	if err != nil || !added {
		t.Fatalf("re-adding the image = %v, %v", added, err)
	}
	entries := store.Entries()
	if len(entries) != 3 {
		t.Fatalf("%d entries, want 3: %+v", len(entries), entries)
	}
	if entries[0].ID != fresh.ID {
		t.Errorf("top = %+v, want the fresh image", entries[0])
	}
	if _, ok := findEntry(entries, image.ID); ok {
		t.Error("the older image survived the fold")
	}
	if _, ok := findEntry(entries, text.ID); !ok {
		t.Error("the text entry was folded away")
	}
	if _, ok := findEntry(entries, files.ID); !ok {
		t.Error("the file entry was folded away")
	}
}

// The favorite rule is the same for all three kinds: they share `insertLocked`
// but not their hash sources.
func TestFoldKeepsFavoritesForEveryKind(t *testing.T) {
	png := testPNG(t, 8, 4)
	paths := []string{"/tmp/one.txt", "/tmp/two.txt"}
	cases := []struct {
		kind  string
		add   func(*Store) (Entry, error)
		other func(*Store) (Entry, error)
	}{
		{"text", func(s *Store) (Entry, error) {
			entry, _, err := s.AddText("hello")
			return entry, err
		}, func(s *Store) (Entry, error) {
			entry, _, err := s.AddText("world")
			return entry, err
		}},
		{"image", func(s *Store) (Entry, error) {
			entry, _, err := s.AddImage(png, 8, 4)
			return entry, err
		}, func(s *Store) (Entry, error) {
			entry, _, err := s.AddImage(testPNG(t, 9, 9), 9, 9)
			return entry, err
		}},
		{"files", func(s *Store) (Entry, error) {
			entry, _, err := s.AddFiles(paths)
			return entry, err
		}, func(s *Store) (Entry, error) {
			entry, _, err := s.AddFiles([]string{"/tmp/other.txt"})
			return entry, err
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.kind, func(t *testing.T) {
			store, _, advance := foldClock(t)
			starred, err := testCase.add(store)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SetFavorite(starred.ID, true); err != nil {
				t.Fatal(err)
			}
			advance(time.Second)
			// A different clip on top, so the starred one is no longer the
			// top entry and the fold actually has something to decide.
			if _, err := testCase.other(store); err != nil {
				t.Fatal(err)
			}
			advance(time.Second)
			fresh, err := testCase.add(store)
			if err != nil {
				t.Fatal(err)
			}
			if !fresh.Favorite {
				t.Errorf("the fresh %s entry did not inherit the flag", testCase.kind)
			}
			entries := store.Entries()
			kept, ok := findEntry(entries, starred.ID)
			if !ok {
				t.Fatalf("the favorited %s entry was folded away: %+v", testCase.kind, entries)
			}
			if !kept.Favorite {
				t.Errorf("the favorited %s entry lost its flag", testCase.kind)
			}
			if len(entries) != 3 {
				t.Errorf("%d %s entries, want 3", len(entries), testCase.kind)
			}
		})
	}
}
