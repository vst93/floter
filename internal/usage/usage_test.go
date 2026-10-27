package usage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestRecordAndTop(t *testing.T) {
	dir := t.TempDir()
	store := Open(dir)
	clock := time.UnixMilli(1_700_000_000_000)
	store.now = func() time.Time { return clock }

	for _, id := range []string{"/Applications/A.app", "/Applications/A.app", "/Applications/B.app"} {
		if err := store.Record(id); err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(time.Second)
	}
	if got := store.Count("/Applications/A.app"); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}

	top := store.Top(0, nil)
	if !reflect.DeepEqual(top, []string{"/Applications/A.app", "/Applications/B.app"}) {
		t.Errorf("top = %v", top)
	}
	if top := store.Top(1, nil); len(top) != 1 || top[0] != "/Applications/A.app" {
		t.Errorf("limited top = %v", top)
	}
	// Only ids the caller knows come back.
	top = store.Top(0, func(id string) bool { return id == "/Applications/B.app" })
	if !reflect.DeepEqual(top, []string{"/Applications/B.app"}) {
		t.Errorf("filtered top = %v", top)
	}
	// Between equals, the more recent wins.
	clock = clock.Add(time.Second)
	if err := store.Record("/Applications/B.app"); err != nil {
		t.Fatal(err)
	}
	if top := store.Top(0, nil); top[0] != "/Applications/B.app" {
		t.Errorf("after a tie, top = %v", top)
	}

	// The file is readable again, and a blank id is not recorded.
	if _, err := os.Stat(filepath.Join(dir, fileName)); err != nil {
		t.Errorf("the file was not written: %v", err)
	}
	if err := store.Record(""); err != nil {
		t.Errorf("blank id: %v", err)
	}
	if got := Open(dir).Count("/Applications/A.app"); got != 2 {
		t.Errorf("reopened count = %d", got)
	}
}

func TestMissingOrBrokenFile(t *testing.T) {
	// A missing file is an empty history.
	store := Open(t.TempDir())
	if got := store.Top(5, nil); len(got) != 0 {
		t.Errorf("missing file = %v", got)
	}
	// A malformed file too, and recording from there works.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	store = Open(dir)
	if err := store.Record("x"); err != nil {
		t.Fatal(err)
	}
	if got := store.Top(5, nil); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("after a broken file = %v", got)
	}
	// No path at all: nothing is written and nothing fails.
	empty := &Store{now: time.Now}
	if err := empty.Record("y"); err != nil {
		t.Errorf("a store with no path: %v", err)
	}
}

// The counts an earlier build kept in the settings file seed this history
// once, and never overwrite what this build has recorded.
func TestSeedFromAnEarlierBuild(t *testing.T) {
	root := t.TempDir()
	store := Open(root)

	// A fresh history takes the counts over.
	if !store.Seed(map[string]int{"/Applications/Safari.app": 9, "/Applications/Gone.app": 0, "": 3}) {
		t.Fatal("Seed reported no change")
	}
	if got := store.Count("/Applications/Safari.app"); got != 9 {
		t.Errorf("seeded count = %d", got)
	}
	if got := store.Count("/Applications/Gone.app"); got != 0 {
		t.Errorf("a zero count was seeded: %d", got)
	}
	if err := store.Save(); err != nil {
		t.Fatal(err)
	}

	// This build's own record wins over the seed.
	if err := store.Record("/Applications/Safari.app"); err != nil {
		t.Fatal(err)
	}
	if store.Seed(map[string]int{"/Applications/Safari.app": 100}) {
		t.Error("Seed overwrote a recorded count")
	}
	if got := store.Count("/Applications/Safari.app"); got != 10 {
		t.Errorf("count after a launch = %d", got)
	}

	// A new entry from the settings file still lands.
	if !store.Seed(map[string]int{"/Applications/Editor.app": 4}) {
		t.Error("a new id was not seeded")
	}
	if got := store.Count("/Applications/Editor.app"); got != 4 {
		t.Errorf("later seed = %d", got)
	}
	if store.Seed(nil) {
		t.Error("an empty map changed something")
	}
}

// A read that comes first still sees the file: the store loads on the read
// itself, the way `Count` does, so a cold start's "most used" list is not
// empty until something is launched.
//
// The shape here is the real one: one store writes the file, another one —
// this process's store, opened at startup — only reads. Nothing calls
// `Record`, `Count` or `Seed` in between, because on a real cold start nothing
// does: the only other reader at startup is `Seed`, which returns before
// loading when the settings file has no `launch_counts` to import.
func TestAColdReadSeesTheFileItDidNotLoad(t *testing.T) {
	dir := t.TempDir()
	previous := Open(dir)
	clock := time.UnixMilli(1_700_000_000_000)
	previous.now = func() time.Time { return clock }
	for _, id := range []string{"/Applications/A.app", "/Applications/A.app", "/Applications/B.app"} {
		if err := previous.Record(id); err != nil {
			t.Fatal(err)
		}
		clock = clock.Add(time.Second)
	}

	cold := Open(dir)
	if got := cold.Top(0, nil); !reflect.DeepEqual(got, []string{"/Applications/A.app", "/Applications/B.app"}) {
		t.Errorf("a cold read's top = %v, want the file's two ids, A first", got)
	}
}

// And the caller's filter is asked, which it never was when the map was still
// nil: `keep` is how the launcher drops applications that are no longer
// installed, so a read that answers before loading silently drops everything.
func TestTopLoadsWithoutACountFirst(t *testing.T) {
	dir := t.TempDir()
	previous := Open(dir)
	for _, id := range []string{"/Applications/A.app", "/Applications/B.app"} {
		if err := previous.Record(id); err != nil {
			t.Fatal(err)
		}
	}

	asked := 0
	cold := Open(dir)
	got := cold.Top(0, func(id string) bool {
		asked++
		return id == "/Applications/B.app"
	})
	if !reflect.DeepEqual(got, []string{"/Applications/B.app"}) {
		t.Errorf("filtered cold read = %v, want [B]", got)
	}
	if asked == 0 {
		t.Error("the filter was never asked: the read answered before loading")
	}
}

// A save from a store that has not read the file must not replace it: the
// store has no "write this whole map" API, so an unloaded save can only be a
// mistake — and it used to be a silent one, `{}` over the history.
func TestSaveWithoutAReadKeepsTheFile(t *testing.T) {
	dir := t.TempDir()
	previous := Open(dir)
	for _, id := range []string{"/Applications/A.app", "/Applications/A.app"} {
		if err := previous.Record(id); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}

	if err := Open(dir).Save(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("an unread store's save replaced the file:\nbefore %s\nafter  %s", before, after)
	}
}
