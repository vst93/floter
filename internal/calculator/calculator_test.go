package calculator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T, maxItems, retentionDays int) *Store {
	t.Helper()
	return NewStore(FromConfigRoot(t.TempDir()), maxItems, retentionDays)
}

func TestAddFoldsAndKeepsFavorites(t *testing.T) {
	store := newTestStore(t, DefaultMaxItems, 30)

	first, err := store.Add("1+1", "2")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.Expression != "1+1" || first.Result != "2" || first.Favorite {
		t.Fatalf("entry = %+v", first)
	}
	if _, err := store.Add("2+2", "4"); err != nil {
		t.Fatal(err)
	}
	if got := store.Entries(); len(got) != 2 || got[0].Expression != "2+2" {
		t.Fatalf("entries = %+v", got)
	}

	// Starring the older row, then re-running its expression, promotes it to
	// the top and carries the star forward instead of stacking a second copy.
	if err := store.SetFavorite(first.ID, true); err != nil {
		t.Fatal(err)
	}
	promoted, err := store.Add("1+1", "2")
	if err != nil {
		t.Fatal(err)
	}
	entries := store.Entries()
	if len(entries) != 2 {
		t.Fatalf("a re-run stacked a copy: %+v", entries)
	}
	if entries[0].Expression != "1+1" || !entries[0].Favorite || entries[0].ID != promoted.ID {
		t.Errorf("the promoted row = %+v", entries[0])
	}
	if entries[1].Expression != "2+2" {
		t.Errorf("the other row = %+v", entries[1])
	}

	// An empty expression is refused rather than recorded.
	if _, err := store.Add("   ", "0"); err == nil {
		t.Error("an empty expression was recorded")
	}
	if err := store.SetFavorite("nope", true); err != ErrNoEntry {
		t.Errorf("SetFavorite on an unknown id = %v", err)
	}
}

func TestRetentionExemptsFavorites(t *testing.T) {
	store := newTestStore(t, 10, 1)
	base := time.Unix(1_700_000_000, 0)
	store.now = func() time.Time { return base }

	starred, err := store.Add("keep me", "1")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetFavorite(starred.ID, true); err != nil {
		t.Fatal(err)
	}
	// Ten more entries fill the capacity; the star must survive them.
	for i := 0; i < 12; i++ {
		if _, err := store.Add("x"+string(rune('a'+i)), "0"); err != nil {
			t.Fatal(err)
		}
	}
	entries := store.Entries()
	ordinary := 0
	favorites := 0
	for _, entry := range entries {
		if entry.Favorite {
			favorites++
		} else {
			ordinary++
		}
	}
	if favorites != 1 || ordinary != 10 {
		t.Fatalf("capacity: %d ordinary, %d favorites", ordinary, favorites)
	}

	// The age window drops the ordinary rows once they are old enough, and
	// still leaves the favorite alone.
	store.now = func() time.Time { return base.Add(48 * time.Hour) }
	if err := store.SetRetention(10, 1); err != nil {
		t.Fatal(err)
	}
	entries = store.Entries()
	if len(entries) != 1 || !entries[0].Favorite {
		t.Fatalf("after the window = %+v", entries)
	}

	// Un-starring it puts it back under the retention.
	if err := store.SetFavorite(entries[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if got := store.Entries(); len(got) != 0 {
		t.Errorf("an unstared expired row survived: %+v", got)
	}

	// A window of 0 never expires anything.
	store.now = func() time.Time { return base }
	if _, err := store.Add("forever", "1"); err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return base.Add(10 * 365 * 24 * time.Hour) }
	if err := store.SetRetention(10, 0); err != nil {
		t.Fatal(err)
	}
	if got := store.Entries(); len(got) != 1 {
		t.Errorf("the never-expire window dropped a row: %+v", got)
	}
}

func TestSearchDeleteAndClear(t *testing.T) {
	store := newTestStore(t, DefaultMaxItems, 30)
	for _, pair := range [][2]string{{"1+1", "2"}, {"10*3", "30"}, {"sqrt(16)", "4"}} {
		if _, err := store.Add(pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}

	// Every term must match, in the expression or the result.
	if got := store.Search("sqrt", false, 0); len(got) != 1 || got[0].Expression != "sqrt(16)" {
		t.Errorf("search sqrt = %+v", got)
	}
	if got := store.Search("30", false, 0); len(got) != 1 || got[0].Expression != "10*3" {
		t.Errorf("search by result = %+v", got)
	}
	if got := store.Search("", false, 0); len(got) != 3 {
		t.Errorf("empty query = %+v", got)
	}
	if got := store.Search("", false, 2); len(got) != 2 {
		t.Errorf("limit = %+v", got)
	}

	// The favourite filter keeps only the starred rows.
	entries := store.Entries()
	if _, err := store.ToggleFavorite(entries[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := store.Search("", true, 0); len(got) != 1 || !got[0].Favorite {
		t.Errorf("favourites = %+v", got)
	}
	if favorite, err := store.ToggleFavorite(entries[0].ID); err != nil || favorite {
		t.Errorf("the second toggle = %v, %v", favorite, err)
	}

	// Delete drops one row, and Clear keeps the favourites.
	if err := store.Delete(entries[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := store.Entries(); len(got) != 2 {
		t.Fatalf("after a delete = %+v", got)
	}
	if err := store.Delete("nope"); err != ErrNoEntry {
		t.Errorf("Delete on an unknown id = %v", err)
	}
	remaining := store.Entries()
	if _, err := store.ToggleFavorite(remaining[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	got := store.Entries()
	if len(got) != 1 || !got[0].Favorite {
		t.Errorf("Clear kept %+v", got)
	}
}

func TestPersistenceAndRecovery(t *testing.T) {
	root := t.TempDir()
	store := NewStore(FromConfigRoot(root), DefaultMaxItems, 30)
	entry, err := store.Add("6*7", "42")
	if err != nil {
		t.Fatal(err)
	}

	// The file is where the old build kept it, and a second store reads it
	// back.
	path := filepath.Join(root, dirName, indexFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the index was not written: %v", err)
	}
	reopened := NewStore(FromConfigRoot(root), DefaultMaxItems, 30)
	entries := reopened.Entries()
	if len(entries) != 1 || entries[0].ID != entry.ID || entries[0].Result != "42" {
		t.Fatalf("reopened = %+v", entries)
	}

	// A key this build does not model survives a rewrite.
	raw := []map[string]any{{
		"id": "legacy", "expression": "1/3", "result": "0.3333",
		"created_at": time.Now().UnixMilli(), "favorite": false, "futureKey": map[string]any{"a": 1},
	}}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	withExtra := NewStore(FromConfigRoot(root), DefaultMaxItems, 30)
	if _, err := withExtra.Add("2", "2"); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(written, &decoded); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range decoded {
		if row["id"] == "legacy" {
			found = true
			if row["futureKey"] == nil {
				t.Errorf("an unknown key was dropped: %v", row)
			}
		}
	}
	if !found {
		t.Errorf("the legacy row is gone: %v", decoded)
	}

	// A missing or corrupt index is an empty history, not a failure.
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := NewStore(FromConfigRoot(root), DefaultMaxItems, 30).Entries(); len(got) != 0 {
		t.Errorf("a corrupt index = %+v", got)
	}
	empty := NewStore(FromConfigRoot(t.TempDir()), DefaultMaxItems, 30)
	if got := empty.Entries(); len(got) != 0 {
		t.Errorf("a missing index = %+v", got)
	}
	// A row without an identity or an expression cannot be shown or acted on.
	if err := os.WriteFile(path, []byte(`[{"id":"","expression":"x"},{"id":"a","expression":"  "}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := NewStore(FromConfigRoot(root), DefaultMaxItems, 30).Entries(); len(got) != 0 {
		t.Errorf("rows without identity = %+v", got)
	}
}

func TestNormalizers(t *testing.T) {
	if got := clampMaxItems(0); got != DefaultMaxItems {
		t.Errorf("clampMaxItems(0) = %d", got)
	}
	if got := clampMaxItems(1); got != MinMaxItems {
		t.Errorf("clampMaxItems(1) = %d", got)
	}
	if got := clampMaxItems(10_000); got != MaxMaxItems {
		t.Errorf("clampMaxItems(10000) = %d", got)
	}
	if got := NormalizeRetentionDays(7); got != 7 {
		t.Errorf("NormalizeRetentionDays(7) = %d", got)
	}
	if got := NormalizeRetentionDays(5); got != DefaultRetentionDay {
		t.Errorf("NormalizeRetentionDays(5) = %d", got)
	}
	if got := NormalizeCopyMode(CopyResult); got != CopyResult {
		t.Errorf("NormalizeCopyMode(result) = %q", got)
	}
	if got := NormalizeCopyMode("nonsense"); got != CopyFull {
		t.Errorf("NormalizeCopyMode(nonsense) = %q", got)
	}
}

func TestEntryText(t *testing.T) {
	entry := Entry{Expression: "1+1", Result: "2"}
	if got := entry.Label(); got != "1+1 = 2" {
		t.Errorf("Label = %q", got)
	}
	if got := entry.Text(CopyFull); got != "1+1 = 2" {
		t.Errorf("Text(full) = %q", got)
	}
	if got := entry.Text(CopyResult); got != "2" {
		t.Errorf("Text(result) = %q", got)
	}
	if got := (Entry{Expression: "1+1"}).Label(); got != "1+1" {
		t.Errorf("a row with no result = %q", got)
	}
}
