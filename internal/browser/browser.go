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
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

// The browser ids the plugin's vocabulary is built on: they name a browser in
// the `target` setting, they are the first half of a profile key, and they
// resolve the application a page opens in. They match the old build's ids, so
// a settings file written by it keeps working.
const (
	BrowserChrome     = "chrome"
	BrowserEdge       = "edge"
	BrowserEdgeBeta   = "edge-beta"
	BrowserEdgeDev    = "edge-dev"
	BrowserEdgeCanary = "edge-canary"
	BrowserBrave      = "brave"
	BrowserChromium   = "chromium"
	BrowserArc        = "arc"
	BrowserFirefox    = "firefox"
	BrowserSafari     = "safari"
	BrowserCustom     = "custom"
)

// Result is one history entry or bookmark.
type Result struct {
	// URL is the page's address and Title its name (empty when the browser
	// keeps none).
	URL   string
	Title string
	// BrowserID is the browser's id, Browser its display name, and Kind
	// "history" or "bookmark".
	BrowserID string
	Browser   string
	Kind      string
	// ProfileKey is the profile the entry came from, so an action can
	// resolve its target without a second discovery.
	ProfileKey string
	// Visited is when the page was last visited; the zero time for a
	// bookmark, which carries none.
	Visited time.Time
	// Visits is how many times the page was visited (0 when the browser
	// does not say).
	Visits int
	// Added is when a bookmark was saved; the zero time when the browser
	// does not say.
	Added time.Time
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
	// BrowserID is the browser's id ("chrome", "edge", …) and Browser its
	// display name.
	BrowserID string
	Browser   string
	// Name is the profile's own label — the one the user chose in the
	// browser — when the browser records one; the directory name otherwise.
	Name string
	// Dir is the profile's directory name inside the browser's base
	// directory ("Default", "Profile 1", …).
	Dir string
	// BaseDir is the directory the profile lives in, as discovery found it
	// (a custom base directory may name a browser's base or one profile).
	BaseDir string
	// HistoryDB and BookmarksFile are the files to read; either may be
	// empty.
	HistoryDB     string
	BookmarksFile string
}

// Key identifies a profile across runs: `browser-id/profile-dir`, the old
// build's `profile_key`. It is what an action resolves its target through.
func (p Profile) Key() string {
	if p.BrowserID == "" || p.Dir == "" {
		return ""
	}
	return p.BrowserID + "/" + p.Dir
}

// HasHistory and HasBookmarks report whether the profile's files are there:
// the default-profile choice prefers a browser the user actually browses with.
func (p Profile) HasHistory() bool   { return p.HistoryDB != "" && exists(p.HistoryDB) }
func (p Profile) HasBookmarks() bool { return p.BookmarksFile != "" && exists(p.BookmarksFile) }

// Label is the profile as a list shows it: the browser's name and the
// profile's own label when it has one of its own.
func (p Profile) Label() string {
	if p.Name == "" || p.Name == p.Dir {
		return p.Browser
	}
	return p.Browser + " · " + p.Name
}

// Profiles finds the profiles of the browsers installed for a user, under
// their home directory. Directories that do not exist are left out.
func Profiles(home string) []Profile {
	return ProfilesIn(home, "")
}

// ProfilesIn finds the profiles of the browsers installed for a user, with an
// extra directory to search as well: the custom base directory the browser
// plugin's settings name. An empty custom base directory means "the standard
// ones only".
func ProfilesIn(home, customBase string) []Profile {
	if home == "" {
		return nil
	}
	return profilesFor(runtime.GOOS, home, os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA"), customBase)
}

// profilesFor is the platform-independent discovery: the browser tables are
// pure functions of the platform and the environment paths, so every
// platform's layout is asserted from any host (a macOS-only branch would ship
// untested, which is how the old build's `Microsoft/Edge` path shipped wrong).
//
// customBase, when set, is searched as an extra Chromium base directory: the
// browser plugin's "custom profile directory" setting.
func profilesFor(goos, home, appData, localAppData, customBase string) []Profile {
	var profiles []Profile
	for _, browser := range chromiumBrowsers(goos, home, localAppData) {
		profiles = append(profiles, chromiumProfiles(goos, browser.id, browser.name, browser.base)...)
	}
	if customBase != "" {
		// A custom base directory may name a browser's base directory or one
		// profile inside it; a directory that is itself a profile is used as
		// one.
		if isProfileDir(filepath.Dir(customBase), filepath.Base(customBase)) && !looksLikeBaseDir(customBase) {
			profiles = append(profiles, chromiumProfile(goos, BrowserCustom, "Custom", customBase, filepath.Base(customBase)))
		} else {
			profiles = append(profiles, chromiumProfiles(goos, BrowserCustom, "Custom", customBase)...)
		}
	}

	if goos == "darwin" {
		safari := joinFor(goos, home, "Library", "Safari")
		history := joinFor(goos, safari, "History.db")
		if exists(history) {
			profiles = append(profiles, Profile{
				BrowserID: BrowserSafari,
				Browser:   "Safari",
				Dir:       "Safari",
				BaseDir:   safari,
				HistoryDB: history,
			})
		}
	}
	for _, dir := range subdirectories(firefoxRoot(goos, home, appData)) {
		places := joinFor(goos, dir, "places.sqlite")
		if exists(places) {
			profiles = append(profiles, Profile{
				BrowserID:     BrowserFirefox,
				Browser:       "Firefox",
				Name:          filepath.Base(dir),
				Dir:           filepath.Base(dir),
				BaseDir:       filepath.Dir(dir),
				HistoryDB:     places,
				BookmarksFile: places,
			})
		}
	}
	return profiles
}

// looksLikeBaseDir reports whether a directory holds Chromium profiles rather
// than being one: a custom base directory can be either, and the file layout
// is what tells them apart.
func looksLikeBaseDir(dir string) bool {
	for _, marker := range []string{"Local State", "Last Version"} {
		if exists(filepath.Join(dir, marker)) {
			return true
		}
	}
	return len(chromiumProfileDirs(dir)) > 1
}

// joinFor joins path elements with the separator the platform uses, so the
// browser tables stay pure functions of the platform — every platform's
// layout is then asserted from any host, which is how the old build's
// `Microsoft/Edge` path shipped wrong once.
func joinFor(goos string, parts ...string) string {
	separator := "/"
	if goos == "windows" {
		separator = `\`
	}
	return strings.Join(parts, separator)
}

// chromiumBrowser is one Chromium-family browser and the directory its
// profiles live in.
type chromiumBrowser struct {
	id   string
	name string
	base string
}

// chromiumBrowsers are the Chromium-family browsers and where their profiles
// live, per platform. The layout differs per browser and the differences are
// load-bearing:
//
//   - Chrome and Brave nest under a vendor folder; Chromium and every Edge
//     channel are a single top-level folder (Edge's space is part of the
//     name — the old `Microsoft/Edge` guess listed nothing).
//   - Windows keeps them under `%LOCALAPPDATA%`, each in its own `User Data`
//     directory, while Firefox's profiles are in roaming `%APPDATA%`.
//   - Linux uses the lower-case vendor names in `~/.config`.
func chromiumBrowsers(goos, home, localAppData string) []chromiumBrowser {
	switch goos {
	case "darwin":
		support := joinFor(goos, home, "Library", "Application Support")
		return []chromiumBrowser{
			{BrowserChrome, "Google Chrome", joinFor(goos, support, "Google", "Chrome")},
			{BrowserEdge, "Microsoft Edge", joinFor(goos, support, "Microsoft Edge")},
			{BrowserEdgeBeta, "Microsoft Edge Beta", joinFor(goos, support, "Microsoft Edge Beta")},
			{BrowserEdgeDev, "Microsoft Edge Dev", joinFor(goos, support, "Microsoft Edge Dev")},
			{BrowserEdgeCanary, "Microsoft Edge Canary", joinFor(goos, support, "Microsoft Edge Canary")},
			{BrowserBrave, "Brave", joinFor(goos, support, "BraveSoftware", "Brave-Browser")},
			{BrowserChromium, "Chromium", joinFor(goos, support, "Chromium")},
			{BrowserArc, "Arc", joinFor(goos, support, "Arc", "User Data")},
		}
	case "windows":
		if localAppData == "" {
			return nil
		}
		return []chromiumBrowser{
			{BrowserChrome, "Google Chrome", joinFor(goos, localAppData, "Google", "Chrome", "User Data")},
			{BrowserEdge, "Microsoft Edge", joinFor(goos, localAppData, "Microsoft", "Edge", "User Data")},
			{BrowserBrave, "Brave", joinFor(goos, localAppData, "BraveSoftware", "Brave-Browser", "User Data")},
			{BrowserChromium, "Chromium", joinFor(goos, localAppData, "Chromium", "User Data")},
		}
	case "linux":
		config := joinFor(goos, home, ".config")
		return []chromiumBrowser{
			{BrowserChrome, "Google Chrome", joinFor(goos, config, "google-chrome")},
			{BrowserEdge, "Microsoft Edge", joinFor(goos, config, "microsoft-edge")},
			{BrowserBrave, "Brave", joinFor(goos, config, "BraveSoftware", "Brave-Browser")},
			{BrowserChromium, "Chromium", joinFor(goos, config, "chromium")},
		}
	default:
		return nil
	}
}

// firefoxRoot is where Firefox keeps its profile directories: roaming
// application data on Windows, the application support directory on macOS,
// `~/.mozilla` elsewhere.
func firefoxRoot(goos, home, appData string) string {
	switch goos {
	case "darwin":
		return joinFor(goos, home, "Library", "Application Support", "Firefox", "Profiles")
	case "windows":
		if appData == "" {
			return ""
		}
		return joinFor(goos, appData, "Mozilla", "Firefox", "Profiles")
	default:
		return joinFor(goos, home, ".mozilla", "firefox")
	}
}

// chromiumProfiles lists one base directory's profiles, in the browser's own
// order: `Default` first, then `Profile N` by number, then the rest by name.
func chromiumProfiles(goos, id, name, base string) []Profile {
	dirs := chromiumProfileDirs(base)
	profiles := make([]Profile, 0, len(dirs))
	for _, dir := range dirs {
		profiles = append(profiles, chromiumProfile(goos, id, name, base, dir))
	}
	return profiles
}

// chromiumProfile is one profile of a Chromium base directory.
func chromiumProfile(goos, id, name, base, dir string) Profile {
	root := joinFor(goos, base, dir)
	return Profile{
		BrowserID:     id,
		Browser:       name,
		Name:          localStateProfileName(base, dir),
		Dir:           dir,
		BaseDir:       base,
		HistoryDB:     joinFor(goos, root, "History"),
		BookmarksFile: joinFor(goos, root, "Bookmarks"),
	}
}

// chromiumProfileDirs lists the profile directories of a Chromium browser.
// A profile is a directory named the way Chrome names them, or one holding
// one of the files every profile has — which keeps stray folders (`Crashpad`,
// `Shader Cache`, …) out of the list.
func chromiumProfileDirs(base string) []string {
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []string
	for _, entry := range entries {
		if entry.IsDir() && isProfileDir(base, entry.Name()) {
			out = append(out, entry.Name())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := profileRank(out[i]), profileRank(out[j])
		if left.class != right.class {
			return left.class < right.class
		}
		if left.number != right.number {
			return left.number < right.number
		}
		return left.name < right.name
	})
	return out
}

// profileOrder is a profile directory's place in the browser's own order.
type profileOrder struct {
	class  int
	number int
	name   string
}

// profileRank orders profile directories: `Default` (0) first, then
// `Profile N` (1) by number, then everything else (2) by name.
func profileRank(name string) profileOrder {
	if name == "Default" {
		return profileOrder{}
	}
	if rest, ok := strings.CutPrefix(name, "Profile "); ok {
		if number, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
			return profileOrder{class: 1, number: number}
		}
	}
	return profileOrder{class: 2, name: name}
}

// isProfileDir reports whether a directory inside a base directory looks like
// a Chromium profile: one of Chrome's own names, or one carrying a file every
// profile has. `Default` is accepted by name as well, because a profile that
// has never been opened has none of the files yet.
func isProfileDir(base, name string) bool {
	if name == "Default" || name == "Guest Profile" || strings.HasPrefix(name, "Profile ") {
		return true
	}
	for _, marker := range []string{"Preferences", "Bookmarks", "History"} {
		if exists(filepath.Join(base, name, marker)) {
			return true
		}
	}
	return false
}

// localStateProfileName reads the user's chosen label for a profile from the
// browser's `Local State` file, which is the only place it is recorded. A
// missing or unreadable file falls back to the directory name.
func localStateProfileName(base, dir string) string {
	data, err := os.ReadFile(filepath.Join(base, "Local State"))
	if err != nil {
		return dir
	}
	var state struct {
		Profile struct {
			InfoCache map[string]struct {
				Name string `json:"name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return dir
	}
	if name := strings.TrimSpace(state.Profile.InfoCache[dir].Name); name != "" {
		return name
	}
	return dir
}

// maxRows is how many rows one profile contributes before matching: enough
// that the newest visits and bookmarks are all there, small enough that a
// history of a million rows is not read to answer one query.
const maxRows = 4000

// readHistory reads a profile's history database, newest first, with the
// search terms applied in SQL before the LIMIT.
func readHistory(ctx context.Context, profile Profile, terms []string, field SearchField, days int) []Result {
	db, err := sqlite.Open(ctx, profile.HistoryDB, sqlite.OpenOptions{ReadOnly: true})
	if err != nil {
		return nil
	}
	defer db.Close()

	cutoff := int64(0)
	if days > 0 {
		cutoff = time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	}
	for _, shape := range historyQueries(profile) {
		sql, args := shape.statement(terms, field, cutoff)
		rows, err := db.Query(ctx, sql, args...)
		if err != nil {
			continue
		}
		results := make([]Result, 0, len(rows.Values))
		for _, values := range rows.Values {
			url, title := shape.Map(values)
			if url == "" {
				continue
			}
			results = append(results, Result{
				URL:        url,
				Title:      title,
				BrowserID:  profile.BrowserID,
				Browser:    profile.Browser,
				Kind:       KindHistory,
				ProfileKey: profile.Key(),
				Visited:    shape.Visited(values),
				Visits:     shape.Visits(values),
			})
		}
		if len(results) > 0 {
			return results
		}
	}
	return nil
}

// historyQuery is one browser's history shape: the table and its columns,
// and how to read a row.
type historyQuery struct {
	// table, urlColumn, titleColumn and timeColumn name the shape; a shape
	// with no title column passes an empty titleColumn.
	table       string
	urlColumn   string
	titleColumn string
	timeColumn  string
	visitCount  string
	// epoch converts the shape's timestamp column to a time, and inverse
	// converts a Unix second back to it, so the history window can be applied
	// in SQL.
	epoch   func(int64) time.Time
	inverse func(int64) int64
}

// statement is the shape's SELECT for a search: the terms and the history
// window are applied before the LIMIT, so a match older than the newest rows
// is still found.
func (shape historyQuery) statement(terms []string, field SearchField, cutoffUnix int64) (string, []any) {
	columns := shape.urlColumn
	if shape.titleColumn != "" {
		columns += ", " + shape.titleColumn
	} else {
		columns += ", ''"
	}
	columns += ", " + shape.timeColumn
	if shape.visitCount != "" {
		columns += ", " + shape.visitCount
	} else {
		columns += ", 0"
	}

	where := ""
	var args []any
	if shape.timeColumn != "" {
		where = " WHERE " + shape.timeColumn + " IS NOT NULL"
	}
	clause, clauseArgs := matchesClause(terms, field, shape.urlColumn, shape.titleColumn)
	where += clause
	args = append(args, clauseArgs...)
	if cutoffUnix > 0 && shape.timeColumn != "" {
		// The window is compared in the shape's own epoch, so the conversion
		// is the same one the reader applies.
		where += " AND " + shape.timeColumn + " >= ?"
		args = append(args, shape.inverse(cutoffUnix))
	}

	sql := "SELECT " + columns + " FROM " + shape.table + where +
		" ORDER BY " + shape.timeColumn + " DESC LIMIT " + itoa(maxRows)
	return sql, args
}

// Map reads a row's URL and title.
func (shape historyQuery) Map(values []any) (string, string) {
	return text(values, 0), text(values, 1)
}

// Visited reads a row's last visit time.
func (shape historyQuery) Visited(values []any) time.Time {
	return shape.epoch(integer(values, 2))
}

// Visits reads a row's visit count.
func (shape historyQuery) Visits(values []any) int {
	return int(integer(values, 3))
}

// historyQueries are the shapes tried in order for a profile: Firefox's
// places, Safari's history, and Chrome's, each newest first.
func historyQueries(profile Profile) []historyQuery {
	return []historyQuery{
		{
			table:       "moz_places",
			urlColumn:   "url",
			titleColumn: "title",
			timeColumn:  "last_visit_date",
			visitCount:  "visit_count",
			epoch:       unixMicros,
			inverse:     func(unix int64) int64 { return unix * 1_000_000 },
		},
		{
			table:      "history_items",
			urlColumn:  "url",
			timeColumn: "last_visit_time",
			visitCount: "visit_count",
			epoch:      appleSeconds,
			inverse:    func(unix int64) int64 { return unix - appleEpochOffset },
		},
		{
			table:       "urls",
			urlColumn:   "url",
			titleColumn: "title",
			timeColumn:  "last_visit_time",
			visitCount:  "visit_count",
			epoch:       chromeMicros,
			inverse:     func(unix int64) int64 { return (unix + chromeEpochOffset) * 1_000_000 },
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
	DateAdd  string             `json:"date_added"`
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
		walkChromiumBookmarks(root, profile, &results)
	}
	return results
}

func walkChromiumBookmarks(node chromiumBookmark, profile Profile, out *[]Result) {
	if node.Type == "url" && node.URL != "" {
		added := time.Time{}
		if micros, err := strconv.ParseInt(node.DateAdd, 10, 64); err == nil {
			added = chromeMicros(micros)
		}
		*out = append(*out, Result{
			URL:        node.URL,
			Title:      node.Name,
			BrowserID:  profile.BrowserID,
			Browser:    profile.Browser,
			Kind:       KindBookmark,
			ProfileKey: profile.Key(),
			Added:      added,
		})
	}
	for _, child := range node.Children {
		walkChromiumBookmarks(child, profile, out)
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
		"SELECT p.url, b.title, b.dateAdded FROM moz_bookmarks b JOIN moz_places p ON p.id = b.fk "+
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
			URL:        url,
			Title:      text(values, 1),
			BrowserID:  profile.BrowserID,
			Browser:    profile.Browser,
			Kind:       KindBookmark,
			ProfileKey: profile.Key(),
			Added:      unixMicros(integer(values, 2)),
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
