package browser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/sqlite"
)

// TestReadsRealDatabase writes a Chrome-shaped History database with the
// sqlite plugin and reads it back, so the SQL and the timestamp conversion
// are exercised against a real database.
//
// Opt-in, because it loads the plugin's native library (downloaded into the
// user's cache on the first run):
//
//	FLOTER_SQLITE_TEST=1 go test ./internal/browser -run TestReadsRealDatabase
func TestReadsRealDatabase(t *testing.T) {
	if os.Getenv("FLOTER_SQLITE_TEST") == "" {
		t.Skip("set FLOTER_SQLITE_TEST=1 to load the sqlite plugin's library")
	}
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "History")
	db, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := db.Execute(ctx, `CREATE TABLE urls (
		id INTEGER PRIMARY KEY, url LONGVARCHAR, title LONGVARCHAR, last_visit_time INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	_ = stats
	// Two rows, the newer last so the ordering is what puts it first.
	now := (chromeEpochOffset + 1_700_000_000) * 1_000_000
	if _, err := db.Execute(ctx, "INSERT INTO urls(url, title, last_visit_time) VALUES (?, ?, ?)",
		"https://old.example.com", "Old page", now-1_000_000); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Execute(ctx, "INSERT INTO urls(url, title, last_visit_time) VALUES (?, ?, ?)",
		"https://new.example.com", "New page", now); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	profile := Profile{Browser: "Google Chrome", HistoryDB: path}
	results := Search(ctx, []Profile{profile}, "", Options{})
	if len(results) != 2 {
		t.Fatalf("history = %+v", results)
	}
	if results[0].Title != "New page" || results[0].Browser != "Google Chrome" || results[0].Kind != "history" {
		t.Errorf("first result = %+v", results[0])
	}
	if !results[0].Visited.After(results[1].Visited) {
		t.Errorf("ordering = %v then %v", results[0].Visited, results[1].Visited)
	}
	// The query matches the title and the URL.
	if got := Search(ctx, []Profile{profile}, "new page", Options{}); len(got) != 1 {
		t.Errorf("title search = %+v", got)
	}
	if got := Search(ctx, []Profile{profile}, "old.example", Options{}); len(got) != 1 {
		t.Errorf("url search = %+v", got)
	}
	// A file that is not a database is skipped, not a failure.
	junk := filepath.Join(dir, "junk")
	if err := os.WriteFile(junk, []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Search(ctx, []Profile{{Browser: "X", HistoryDB: junk}}, "", Options{}); len(got) != 0 {
		t.Errorf("junk history = %+v", got)
	}
}
