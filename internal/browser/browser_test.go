package browser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProfilesFindsWhatsInstalled(t *testing.T) {
	home := t.TempDir()
	// A Chromium browser with two profiles, one of them without history.
	write(t, filepath.Join(home, "Library/Application Support/Google/Chrome/Default/History"), "")
	write(t, filepath.Join(home, "Library/Application Support/Google/Chrome/Profile 1/History"), "")
	write(t, filepath.Join(home, "Library/Application Support/BraveSoftware/Brave-Browser/Default/History"), "")
	// Safari, only on macOS.
	if runtimeDarwin() {
		write(t, filepath.Join(home, "Library/Safari/History.db"), "")
	}
	// Firefox with one profile.
	write(t, filepath.Join(home, "Library/Application Support/Firefox/Profiles/abc.default/places.sqlite"), "")

	profiles := Profiles(home)
	byBrowser := map[string][]Profile{}
	for _, profile := range profiles {
		byBrowser[profile.Browser] = append(byBrowser[profile.Browser], profile)
	}
	if got := len(byBrowser["Google Chrome"]); got != 2 {
		t.Errorf("Chrome profiles = %d, want 2", got)
	}
	if got := len(byBrowser["Brave"]); got != 1 {
		t.Errorf("Brave profiles = %d, want 1", got)
	}
	if got := len(byBrowser["Firefox"]); got != 1 {
		t.Errorf("Firefox profiles = %d, want 1", got)
	}
	if runtimeDarwin() {
		if got := len(byBrowser["Safari"]); got != 1 {
			t.Errorf("Safari profiles = %d, want 1", got)
		}
	}
	// The bookmark file is named beside the history database.
	chrome := byBrowser["Google Chrome"][0]
	if filepath.Base(chrome.BookmarksFile) != "Bookmarks" {
		t.Errorf("bookmarks = %q", chrome.BookmarksFile)
	}
	// Nothing installed is nothing found.
	if got := Profiles(t.TempDir()); len(got) != 0 {
		t.Errorf("an empty home found %+v", got)
	}
	if got := Profiles(""); got != nil {
		t.Errorf("no home found %+v", got)
	}
}

func TestChromiumBookmarks(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "Library/Application Support/Google/Chrome/Default/Bookmarks")
	write(t, path, `{
  "roots": {
    "bookmark_bar": {
      "name": "Bookmarks bar", "type": "folder",
      "children": [
        {"name": "Docs", "type": "url", "url": "https://example.com/docs"},
        {"name": "Nested", "type": "folder", "children": [
          {"name": "Deep", "type": "url", "url": "https://example.com/deep"}
        ]}
      ]
    },
    "other": {"name": "Other", "type": "folder", "children": [
      {"name": "Blog", "type": "url", "url": "https://blog.example.com"}
    ]}
  }
}`)
	profile := Profile{Browser: "Google Chrome", BookmarksFile: path}
	results := readBookmarks(profile)
	if len(results) != 3 {
		t.Fatalf("bookmarks = %+v", results)
	}
	titles := map[string]string{}
	for _, result := range results {
		titles[result.Title] = result.URL
		if result.Kind != "bookmark" || result.Browser != "Google Chrome" {
			t.Errorf("result = %+v", result)
		}
		if !result.Visited.IsZero() {
			t.Errorf("a bookmark carries a visit time: %v", result.Visited)
		}
	}
	if titles["Docs"] != "https://example.com/docs" || titles["Deep"] != "https://example.com/deep" || titles["Blog"] != "https://blog.example.com" {
		t.Errorf("titles = %v", titles)
	}

	// A missing or malformed file yields nothing rather than an error.
	if got := readBookmarks(Profile{BookmarksFile: filepath.Join(home, "nope")}); got != nil {
		t.Errorf("missing file = %+v", got)
	}
	write(t, filepath.Join(home, "bad.json"), "{not json")
	if got := readBookmarks(Profile{BookmarksFile: filepath.Join(home, "bad.json")}); got != nil {
		t.Errorf("malformed file = %+v", got)
	}
}

func TestTimestamps(t *testing.T) {
	// Chrome: microseconds since 1601-01-01.
	chrome := chromeMicros((chromeEpochOffset + 1_000_000_000) * 1_000_000)
	if want := time.Unix(1_000_000_000, 0).UTC(); !chrome.Equal(want) {
		t.Errorf("chromeMicros = %v, want %v", chrome, want)
	}
	// Safari: seconds since 2001-01-01.
	apple := appleSeconds(1_000_000_000 - appleEpochOffset)
	if want := time.Unix(1_000_000_000, 0).UTC(); !apple.Equal(want) {
		t.Errorf("appleSeconds = %v, want %v", apple, want)
	}
	// Firefox: microseconds since 1970.
	firefox := unixMicros(1_000_000_000 * 1_000_000)
	if want := time.Unix(1_000_000_000, 0).UTC(); !firefox.Equal(want) {
		t.Errorf("unixMicros = %v, want %v", firefox, want)
	}
	// A missing timestamp is the zero time.
	for _, got := range []time.Time{chromeMicros(0), appleSeconds(0), unixMicros(-1)} {
		if !got.IsZero() {
			t.Errorf("a missing timestamp = %v", got)
		}
	}
}

func TestSearchMatchesAndOrders(t *testing.T) {
	historyDir := t.TempDir()
	// Two SQLite-free profiles: bookmarks only, which need no database.
	write(t, filepath.Join(historyDir, "one.json"), `{"roots": {"bar": {"name": "bar", "children": [
		{"name": "Go Documentation", "type": "url", "url": "https://go.dev/doc"},
		{"name": "Rust Book", "type": "url", "url": "https://doc.rust-lang.org/book"}
	]}}}`)
	write(t, filepath.Join(historyDir, "two.json"), `{"roots": {"bar": {"name": "bar", "children": [
		{"name": "Go Blog", "type": "url", "url": "https://go.dev/blog"}
	]}}}`)
	profiles := []Profile{
		{Browser: "A", BookmarksFile: filepath.Join(historyDir, "one.json")},
		{Browser: "B", BookmarksFile: filepath.Join(historyDir, "two.json")},
	}

	// Every term must match, in the title or the URL.
	results := Search(context.Background(), profiles, "go", Options{})
	if len(results) != 2 {
		t.Fatalf("search go = %+v", results)
	}
	if results := Search(context.Background(), profiles, "rust book", Options{}); len(results) != 1 || results[0].Title != "Rust Book" {
		t.Errorf("search rust book = %+v", results)
	}
	if results := Search(context.Background(), profiles, "rust-lang", Options{}); len(results) != 1 {
		t.Errorf("search by URL = %+v", results)
	}
	if results := Search(context.Background(), profiles, "nothing", Options{}); len(results) != 0 {
		t.Errorf("search nothing = %+v", results)
	}
	// An empty query returns everything, and the limit holds.
	if results := Search(context.Background(), profiles, "", Options{}); len(results) != 3 {
		t.Errorf("empty query = %+v", results)
	}
	if results := Search(context.Background(), profiles, "", Options{Limit: 2}); len(results) != 2 {
		t.Errorf("limited = %+v", results)
	}
	// Undated entries sort by title.
	results = Search(context.Background(), profiles, "", Options{})
	if results[0].Title != "Go Blog" || results[2].Title != "Rust Book" {
		t.Errorf("order = %v", []string{results[0].Title, results[1].Title, results[2].Title})
	}
}

func TestLabel(t *testing.T) {
	if got := (Result{URL: "https://example.com"}).Label(); got != "https://example.com" {
		t.Errorf("label without a title = %q", got)
	}
	if got := (Result{URL: "https://example.com", Title: "  Docs  "}).Label(); got != "Docs" {
		t.Errorf("label = %q", got)
	}
}

func runtimeDarwin() bool { return runtimeGOOS() == "darwin" }
