// Package browser reads the installed browsers' history and bookmarks, for
// the launcher's browser mode.
//
// It reads the browsers' own files where they keep them — Chrome's
// `History` SQLite database and `Bookmarks` JSON, Safari's `History.db`,
// Firefox's `places.sqlite` — and never writes to them: every connection is
// read-only, so a browser that holds the file open is not disturbed.
//
// The files can be unreadable (Safari's needs Full Disk Access) or in a
// schema this build does not know; each profile fails on its own and the
// rest still answer.
package browser

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// Result is one history entry or bookmark.
type Result struct {
	// URL is the page's address and Title its name (empty when the browser
	// keeps none).
	URL   string
	Title string
	// Browser is the profile's browser name, and Kind "history" or
	// "bookmark".
	Browser string
	Kind    string
	// Visited is when the page was last visited; the zero time for a
	// bookmark, which carries none.
	Visited time.Time
}

// Label is the one-line text a list shows.
func (r Result) Label() string {
	if strings.TrimSpace(r.Title) != "" {
		return strings.TrimSpace(r.Title)
	}
	return r.URL
}

// Profile is one browser profile's files.
type Profile struct {
	// Browser names the browser, as the user knows it.
	Browser string
	// HistoryDB and BookmarksFile are the files to read; either may be
	// empty.
	HistoryDB     string
	BookmarksFile string
}

// Profiles finds the profiles of the browsers installed for a user, under
// their home directory. Directories that do not exist are left out.
func Profiles(home string) []Profile {
	if home == "" {
		return nil
	}
	var profiles []Profile
	for _, browser := range chromiumBrowsers() {
		base := filepath.Join(home, browser.relative)
		for _, profile := range chromiumProfileDirs(base) {
			profiles = append(profiles, Profile{
				Browser:       browser.name,
				HistoryDB:     filepath.Join(base, profile, "History"),
				BookmarksFile: filepath.Join(base, profile, "Bookmarks"),
			})
		}
	}

	var firefoxRoot string
	switch runtime.GOOS {
	case "darwin":
		safari := filepath.Join(home, "Library", "Safari")
		if exists(filepath.Join(safari, "History.db")) {
			profiles = append(profiles, Profile{
				Browser:   "Safari",
				HistoryDB: filepath.Join(safari, "History.db"),
			})
		}
		firefoxRoot = filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles")
	case "windows":
		firefoxRoot = filepath.Join(os.Getenv("APPDATA"), "Mozilla", "Firefox", "Profiles")
	default:
		firefoxRoot = filepath.Join(home, ".mozilla", "firefox")
	}
	for _, dir := range subdirectories(firefoxRoot) {
		places := filepath.Join(dir, "places.sqlite")
		if exists(places) {
			profiles = append(profiles, Profile{Browser: "Firefox", HistoryDB: places, BookmarksFile: places})
		}
	}
	return profiles
}

type chromiumBrowser struct {
	name     string
	relative string
}

// chromiumBrowsers are the Chromium-family browsers and where their profiles
// live, per platform.
func chromiumBrowsers() []chromiumBrowser {
	if runtime.GOOS == "darwin" {
		return []chromiumBrowser{
			{"Google Chrome", "Library/Application Support/Google/Chrome"},
			{"Chromium", "Library/Application Support/Chromium"},
			{"Brave", "Library/Application Support/BraveSoftware/Brave-Browser"},
			{"Microsoft Edge", "Library/Application Support/Microsoft Edge"},
			{"Arc", "Library/Application Support/Arc/User Data"},
		}
	}
	return []chromiumBrowser{
		{"Google Chrome", ".config/google-chrome"},
		{"Chromium", ".config/chromium"},
		{"Brave", ".config/BraveSoftware/Brave-Browser"},
		{"Microsoft Edge", ".config/microsoft-edge"},
	}
}

// chromiumProfileDirs lists the profile directories of a Chromium browser:
// "Default" and the numbered ones, as Chrome names them.
func chromiumProfileDirs(base string) []string {
	var out []string
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "Default" || strings.HasPrefix(name, "Profile ") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// Options narrow a search: how far back history goes, and how many results
// come back.
type Options struct {
	// Days limits history to the last N days; 0 means all of it. Bookmarks
	// are never filtered: they have no date.
	Days int
	// Limit caps the results; 0 means no cap.
	Limit int
}

// Search reads every profile's history and bookmarks and returns the entries
// matching the query, newest first.
//
// A profile that cannot be read is skipped: a locked or unreadable database
// must not stop the others.
func Search(ctx context.Context, profiles []Profile, query string, options Options) []Result {
	var results []Result
	for _, profile := range profiles {
		if profile.HistoryDB != "" {
			results = append(results, readHistory(ctx, profile)...)
		}
		if profile.BookmarksFile != "" {
			results = append(results, readBookmarks(profile)...)
		}
	}

	terms := strings.Fields(strings.ToLower(query))
	var cutoff time.Time
	if options.Days > 0 {
		cutoff = time.Now().Add(-time.Duration(options.Days) * 24 * time.Hour)
	}
	matched := make([]Result, 0, len(results))
	for _, result := range results {
		if !matches(result, terms) {
			continue
		}
		// A visit older than the window is out; a bookmark stays, since it
		// carries no date.
		if !cutoff.IsZero() && !result.Visited.IsZero() && result.Visited.Before(cutoff) {
			continue
		}
		matched = append(matched, result)
	}
	sortResults(matched)
	if options.Limit > 0 && len(matched) > options.Limit {
		matched = matched[:options.Limit]
	}
	return matched
}

// matches reports whether every term appears in the URL or the title.
func matches(result Result, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(result.Title + " " + result.URL)
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

// sortResults orders by visit time, newest first, with bookmarks and
// undated entries after the visits, by title.
func sortResults(results []Result) {
	sort.SliceStable(results, func(i, j int) bool {
		left, right := results[i], results[j]
		leftDated, rightDated := !left.Visited.IsZero(), !right.Visited.IsZero()
		switch {
		case leftDated && rightDated:
			return left.Visited.After(right.Visited)
		case leftDated != rightDated:
			return leftDated
		default:
			return strings.ToLower(left.Label()) < strings.ToLower(right.Label())
		}
	})
}

// maxRows is how many rows one profile contributes before matching: enough
// that the newest visits and bookmarks are all there, small enough that a
// history of a million rows is not read to answer one query.
const maxRows = 4000

// readHistory reads a profile's history database, newest first.
func readHistory(ctx context.Context, profile Profile) []Result {
	db, err := sqlite.Open(ctx, profile.HistoryDB, sqlite.OpenOptions{ReadOnly: true})
	if err != nil {
		return nil
	}
	defer db.Close()

	for _, query := range historyQueries(profile) {
		rows, err := db.Query(ctx, query.SQL)
		if err != nil {
			continue
		}
		results := make([]Result, 0, len(rows.Values))
		for _, values := range rows.Values {
			url, title := query.Map(values)
			if url == "" {
				continue
			}
			results = append(results, Result{
				URL:     url,
				Title:   title,
				Browser: profile.Browser,
				Kind:    "history",
				Visited: query.Visited(values),
			})
		}
		if len(results) > 0 {
			return results
		}
	}
	return nil
}

// historyQuery is one browser's history shape: how to read a row.
type historyQuery struct {
	SQL     string
	Map     func(values []any) (url, title string)
	Visited func(values []any) time.Time
}

// historyQueries are the shapes tried in order for a profile: Firefox's
// places, Safari's history, and Chrome's, each newest first.
func historyQueries(profile Profile) []historyQuery {
	return []historyQuery{
		{
			SQL: "SELECT url, title, last_visit_date FROM moz_places " +
				"WHERE last_visit_date IS NOT NULL ORDER BY last_visit_date DESC LIMIT " + itoa(maxRows),
			Map:     func(v []any) (string, string) { return text(v, 0), text(v, 1) },
			Visited: func(v []any) time.Time { return unixMicros(integer(v, 2)) },
		},
		{
			SQL:     "SELECT url, visit_count, last_visit_time FROM history_items ORDER BY last_visit_time DESC LIMIT " + itoa(maxRows),
			Map:     func(v []any) (string, string) { return text(v, 0), "" },
			Visited: func(v []any) time.Time { return appleSeconds(integer(v, 2)) },
		},
		{
			SQL:     "SELECT url, title, last_visit_time FROM urls ORDER BY last_visit_time DESC LIMIT " + itoa(maxRows),
			Map:     func(v []any) (string, string) { return text(v, 0), text(v, 1) },
			Visited: func(v []any) time.Time { return chromeMicros(integer(v, 2)) },
		},
	}
}

// readBookmarks reads a profile's bookmarks: Chrome's JSON file, or
// Firefox's places database.
func readBookmarks(profile Profile) []Result {
	if strings.HasSuffix(profile.BookmarksFile, "places.sqlite") {
		return readFirefoxBookmarks(profile)
	}
	return readChromiumBookmarks(profile)
}

// chromiumBookmarks is the shape of Chrome's Bookmarks file.
type chromiumBookmarks struct {
	Roots map[string]chromiumBookmark `json:"roots"`
}

type chromiumBookmark struct {
	Name     string             `json:"name"`
	Type     string             `json:"type"`
	URL      string             `json:"url"`
	Children []chromiumBookmark `json:"children"`
}

// readChromiumBookmarks flattens Chrome's bookmark tree.
func readChromiumBookmarks(profile Profile) []Result {
	data, err := os.ReadFile(profile.BookmarksFile)
	if err != nil {
		return nil
	}
	var bookmarks chromiumBookmarks
	if err := json.Unmarshal(data, &bookmarks); err != nil {
		return nil
	}
	var results []Result
	for _, root := range bookmarks.Roots {
		walkChromiumBookmarks(root, profile.Browser, &results)
	}
	return results
}

func walkChromiumBookmarks(node chromiumBookmark, browserName string, out *[]Result) {
	if node.Type == "url" && node.URL != "" {
		*out = append(*out, Result{
			URL:     node.URL,
			Title:   node.Name,
			Browser: browserName,
			Kind:    "bookmark",
		})
	}
	for _, child := range node.Children {
		walkChromiumBookmarks(child, browserName, out)
	}
}

// readFirefoxBookmarks reads Firefox's bookmarks from its places database.
func readFirefoxBookmarks(profile Profile) []Result {
	db, err := sqlite.Open(context.Background(), profile.BookmarksFile, sqlite.OpenOptions{ReadOnly: true})
	if err != nil {
		return nil
	}
	defer db.Close()
	rows, err := db.Query(context.Background(),
		"SELECT p.url, b.title FROM moz_bookmarks b JOIN moz_places p ON p.id = b.fk "+
			"WHERE b.type = 1 ORDER BY b.dateAdded DESC LIMIT "+itoa(maxRows))
	if err != nil {
		return nil
	}
	results := make([]Result, 0, len(rows.Values))
	for _, values := range rows.Values {
		url := text(values, 0)
		if url == "" {
			continue
		}
		results = append(results, Result{
			URL: url, Title: text(values, 1), Browser: profile.Browser, Kind: "bookmark",
		})
	}
	return results
}

// The epoch offsets the browsers record their timestamps with.
const (
	// chromeEpochOffset is the seconds between 1601-01-01 and 1970-01-01.
	chromeEpochOffset = 11644473600
	// appleEpochOffset is the seconds between 1904-01-01 and 1970-01-01 for
	// the Mac absolute time Safari uses.
	appleEpochOffset = 978307200
)

// chromeMicros converts Chrome's microseconds since 1601 to a time.
func chromeMicros(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	seconds := value/1_000_000 - chromeEpochOffset
	return time.Unix(seconds, (value%1_000_000)*1000).UTC()
}

// appleSeconds converts Safari's seconds since 2001 to a time.
func appleSeconds(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value+appleEpochOffset, 0).UTC()
}

// unixMicros converts Firefox's microseconds since 1970 to a time.
func unixMicros(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value/1_000_000, (value%1_000_000)*1000).UTC()
}

// text reads a column as a string, empty when it is not one.
func text(values []any, index int) string {
	if index >= len(values) {
		return ""
	}
	switch value := values[index].(type) {
	case string:
		return value
	case []byte:
		return string(value)
	default:
		return ""
	}
}

// integer reads a column as an integer, zero when it is not one.
func integer(values []any, index int) int64 {
	if index >= len(values) {
		return 0
	}
	switch value := values[index].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	default:
		return 0
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func subdirectories(root string) []string {
	var out []string
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			out = append(out, filepath.Join(root, entry.Name()))
		}
	}
	sort.Strings(out)
	return out
}

// ErrNoBrowsers is what a caller reports when nothing is installed.
var ErrNoBrowsers = errors.New("browser: no browser profiles found")
