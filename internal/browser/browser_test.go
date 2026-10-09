package browser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	localAppData := filepath.Join(home, "AppData", "Local")
	// The fixture is written where this platform's table says the browsers
	// live, so the test asserts the table rather than one platform's layout.
	chrome := filepath.Join(chromiumBase(t, "chrome", home, localAppData), "Default")
	chromeSecond := filepath.Join(chromiumBase(t, "chrome", home, localAppData), "Profile 1")
	brave := filepath.Join(chromiumBase(t, "brave", home, localAppData), "Default")
	write(t, filepath.Join(chrome, "History"), "")
	write(t, filepath.Join(chromeSecond, "History"), "")
	write(t, filepath.Join(brave, "History"), "")
	// Safari, only on macOS.
	if runtimeDarwin() {
		write(t, filepath.Join(home, "Library/Safari/History.db"), "")
	}
	// Firefox with one profile.
	firefox := filepath.Join(firefoxProfiles(t, home), "abc.default")
	write(t, filepath.Join(firefox, "places.sqlite"), "")

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
	// The bookmark file is named beside the history database, the profile
	// carries its browser's id, and the profile key is `id/dir`.
	chromeProfile := byBrowser["Google Chrome"][0]
	if filepath.Base(chromeProfile.BookmarksFile) != "Bookmarks" {
		t.Errorf("bookmarks = %q", chromeProfile.BookmarksFile)
	}
	if chromeProfile.BrowserID != BrowserChrome || chromeProfile.Key() != BrowserChrome+"/Default" {
		t.Errorf("profile = %+v (key %q)", chromeProfile, chromeProfile.Key())
	}
	if !chromeProfile.HasHistory() || chromeProfile.HasBookmarks() {
		t.Errorf("profile files = %+v", chromeProfile)
	}
	// Nothing installed is nothing found.
	if got := Profiles(t.TempDir()); len(got) != 0 {
		t.Errorf("an empty home found %+v", got)
	}
	if got := Profiles(""); got != nil {
		t.Errorf("no home found %+v", got)
	}
}

// chromiumBase is where this platform's table puts one Chromium browser.
func chromiumBase(t *testing.T, id, home, localAppData string) string {
	t.Helper()
	for _, browser := range chromiumBrowsers(runtimeGOOS(), home, localAppData) {
		if browser.id == id {
			return browser.base
		}
	}
	t.Fatalf("the %s table has no %s", runtimeGOOS(), id)
	return ""
}

// firefoxProfiles is where this platform's discovery looks for Firefox.
func firefoxProfiles(t *testing.T, home string) string {
	t.Helper()
	return firefoxRoot(runtimeGOOS(), home, filepath.Join(home, "AppData", "Roaming"))
}

// The browser tables are pure functions of the platform, so every platform's
// layout is asserted from any host — which is how the old build's
// `Microsoft/Edge` path shipped wrong once.
func TestBrowserTablesPerPlatform(t *testing.T) {
	home := "/home/u"
	local := "/home/u/AppData/Local"
	mac := chromiumBrowsers("darwin", home, local)
	if len(mac) != 8 {
		t.Fatalf("macOS table = %+v", mac)
	}
	wantMac := map[string]string{
		"chrome":      "/home/u/Library/Application Support/Google/Chrome",
		"edge":        "/home/u/Library/Application Support/Microsoft Edge",
		"edge-beta":   "/home/u/Library/Application Support/Microsoft Edge Beta",
		"edge-dev":    "/home/u/Library/Application Support/Microsoft Edge Dev",
		"edge-canary": "/home/u/Library/Application Support/Microsoft Edge Canary",
		"brave":       "/home/u/Library/Application Support/BraveSoftware/Brave-Browser",
		"chromium":    "/home/u/Library/Application Support/Chromium",
		"arc":         "/home/u/Library/Application Support/Arc/User Data",
	}
	for _, browser := range mac {
		if want := wantMac[browser.id]; want != browser.base {
			t.Errorf("macOS %s = %q, want %q", browser.id, browser.base, want)
		}
	}

	windows := chromiumBrowsers("windows", `C:\Users\u`, `C:\Users\u\AppData\Local`)
	if len(windows) != 4 || windows[0].base != `C:\Users\u\AppData\Local\Google\Chrome\User Data` {
		t.Errorf("windows table = %+v", windows)
	}
	if got := chromiumBrowsers("windows", home, ""); got != nil {
		t.Errorf("windows without LOCALAPPDATA = %+v", got)
	}

	linux := chromiumBrowsers("linux", "/home/u", "")
	if len(linux) != 4 || linux[0].base != "/home/u/.config/google-chrome" {
		t.Errorf("linux table = %+v", linux)
	}
	if got := chromiumBrowsers("plan9", home, local); got != nil {
		t.Errorf("an unknown platform found %+v", got)
	}

	// Firefox's profiles are in roaming application data on Windows and in
	// the application support directory on macOS.
	if got := firefoxRoot("darwin", home, ""); got != "/home/u/Library/Application Support/Firefox/Profiles" {
		t.Errorf("macOS firefox = %q", got)
	}
	if got := firefoxRoot("windows", home, `C:\Users\u\AppData\Roaming`); got != `C:\Users\u\AppData\Roaming\Mozilla\Firefox\Profiles` {
		t.Errorf("windows firefox = %q", got)
	}
	if got := firefoxRoot("linux", home, ""); got != "/home/u/.mozilla/firefox" {
		t.Errorf("linux firefox = %q", got)
	}
}

func TestProfileOrderAndDetection(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"Profile 10", "Default", "Profile 2", "Guest Profile", "Crashpad"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A folder with a profile file is one too, whatever it is called; a stray
	// folder with none is not.
	write(t, filepath.Join(base, "Zed", "Preferences"), "")
	write(t, filepath.Join(base, "Crashpad", "settings.dat"), "")

	want := []string{"Default", "Profile 2", "Profile 10", "Guest Profile", "Zed"}
	got := chromiumProfileDirs(base)
	if len(got) != len(want) {
		t.Fatalf("profile dirs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("profile dirs = %v, want %v", got, want)
		}
	}
}

func TestLocalStateNamesAProfile(t *testing.T) {
	base := t.TempDir()
	write(t, filepath.Join(base, "Local State"), `{"profile":{"info_cache":{"Default":{"name":"Personal"}}}}`)
	if got := localStateProfileName(base, "Default"); got != "Personal" {
		t.Errorf("profile name = %q", got)
	}
	// A profile the file does not name, or a file that is not there, falls
	// back to the directory name.
	if got := localStateProfileName(base, "Profile 1"); got != "Profile 1" {
		t.Errorf("unknown profile name = %q", got)
	}
	if got := localStateProfileName(t.TempDir(), "Default"); got != "Default" {
		t.Errorf("missing Local State name = %q", got)
	}
	write(t, filepath.Join(base, "bad"), "{}")
	if got := localStateProfileName(filepath.Dir(filepath.Join(base, "bad")), "Default"); got != "Personal" {
		t.Errorf("name after a bad file = %q", got)
	}
}

func TestCustomBaseDirIsSearched(t *testing.T) {
	home := t.TempDir()
	custom := t.TempDir()
	write(t, filepath.Join(custom, "Profile 3", "History"), "")
	profiles := ProfilesIn(home, custom)
	if len(profiles) != 1 || profiles[0].BrowserID != BrowserCustom || profiles[0].Key() != "custom/Profile 3" {
		t.Fatalf("custom profiles = %+v", profiles)
	}
	// A custom directory that is itself one profile is used as one.
	only := t.TempDir()
	write(t, filepath.Join(only, "History"), "")
	profiles = ProfilesIn(home, only)
	if len(profiles) != 1 || profiles[0].Dir != filepath.Base(only) {
		t.Fatalf("single-profile custom dir = %+v", profiles)
	}
}

func TestDefaultProfile(t *testing.T) {
	profiles := []Profile{
		{BrowserID: BrowserChrome, Browser: "Google Chrome", Dir: "Default"},
		{BrowserID: BrowserBrave, Browser: "Brave", Dir: "Default", HistoryDB: writeFixture(t, "History")},
		{BrowserID: BrowserFirefox, Browser: "Firefox", Dir: "x", BookmarksFile: writeFixture(t, "places.sqlite")},
	}
	// Auto mode skips a browser with no data at all.
	if got, ok := DefaultProfile(profiles, "auto"); !ok || got.BrowserID != BrowserBrave {
		t.Errorf("auto = %+v", got)
	}
	// A target naming a browser picks it, data or not.
	if got, ok := DefaultProfile(profiles, "chrome"); !ok || got.BrowserID != BrowserChrome {
		t.Errorf("target chrome = %+v", got)
	}
	// A target naming a browser that is not installed falls back to auto.
	if got, ok := DefaultProfile(profiles, "edge"); !ok || got.BrowserID != BrowserBrave {
		t.Errorf("unknown target = %+v", got)
	}
	if _, ok := DefaultProfile(nil, "auto"); ok {
		t.Error("no profiles found one")
	}
	// A profile key resolves back to its profile.
	if got, ok := FindProfile(profiles, "brave/Default"); !ok || got.BrowserID != BrowserBrave {
		t.Errorf("FindProfile = %+v", got)
	}
	if _, ok := FindProfile(profiles, "nope/Default"); ok {
		t.Error("an unknown key resolved")
	}
}

// writeFixture makes a file that exists, for the has-data checks.
func writeFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	write(t, path, "")
	return path
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
	profile := Profile{BrowserID: BrowserChrome, Browser: "Google Chrome", BookmarksFile: path}
	results := readBookmarks(profile)
	if len(results) != 3 {
		t.Fatalf("bookmarks = %+v", results)
	}
	titles := map[string]string{}
	for _, result := range results {
		titles[result.Title] = result.URL
		if result.Kind != KindBookmark || result.Browser != "Google Chrome" || result.BrowserID != BrowserChrome {
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
		{"name": "Go Blog", "type": "url", "url": "https://go.dev/blog"},
		{"name": "Go Documentation", "type": "url", "url": "https://go.dev/doc"}
	]}}}`)
	profiles := []Profile{
		{BrowserID: "a", Browser: "A", Dir: "Default", BookmarksFile: filepath.Join(historyDir, "one.json")},
		{BrowserID: "b", Browser: "B", Dir: "Default", BookmarksFile: filepath.Join(historyDir, "two.json")},
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
	// The same URL in two profiles is one result.
	if results := Search(context.Background(), profiles, "documentation", Options{}); len(results) != 1 {
		t.Errorf("a duplicated URL = %+v", results)
	}
	// An empty query returns everything, and the limit holds.
	if results := Search(context.Background(), profiles, "", Options{}); len(results) != 3 {
		t.Errorf("empty query = %+v", results)
	}
	if results := Search(context.Background(), profiles, "", Options{Limit: 2}); len(results) != 2 {
		t.Errorf("limited = %+v", results)
	}

	// The search fields narrow the haystacks: a title-only search cannot hit
	// a URL and the other way round.
	if results := Search(context.Background(), profiles, "blog", Options{Field: FieldTitle}); len(results) != 1 {
		t.Errorf("title search = %+v", results)
	}
	if results := Search(context.Background(), profiles, "go.dev/blog", Options{Field: FieldTitle}); len(results) != 0 {
		t.Errorf("a title search hit a URL: %+v", results)
	}
	if results := Search(context.Background(), profiles, "go.dev/blog", Options{Field: FieldURL}); len(results) != 1 {
		t.Errorf("url search = %+v", results)
	}
	if results := Search(context.Background(), profiles, "documentation", Options{Field: FieldURL}); len(results) != 0 {
		t.Errorf("a URL search hit a title: %+v", results)
	}

	// The orderings: alphabetical by label, recent and visits by the stamped
	// times a history row carries.
	results = Search(context.Background(), profiles, "", Options{SortOrder: SortAlphabetical})
	if results[0].Title != "Go Blog" || results[2].Title != "Rust Book" {
		t.Errorf("alphabetical = %v", labels(results))
	}
}

func TestSortOrders(t *testing.T) {
	old := time.Unix(1_000, 0)
	recent := time.Unix(2_000, 0)
	results := []Result{
		{Title: "b", Visited: old, Visits: 9},
		{Title: "a", Visited: recent, Visits: 1},
		{Title: "c", Added: recent, Visits: 5},
	}
	orderResults(results, SortVisits)
	if got := labels(results); got[0] != "b" || got[1] != "c" {
		t.Errorf("by visits = %v", got)
	}
	orderResults(results, SortRecent)
	if got := labels(results); got[2] != "b" {
		t.Errorf("by recency = %v", got)
	}
	// Relevance is a no-op: the order the merge produced is the ranking.
	kept := []Result{{Title: "z"}, {Title: "a"}}
	orderResults(kept, SortRelevance)
	if got := labels(kept); got[0] != "z" || got[1] != "a" {
		t.Errorf("relevance reordered: %v", got)
	}
}

func labels(results []Result) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, result.Label())
	}
	return out
}

func TestMatchesClause(t *testing.T) {
	clause, args := matchesClause([]string{"a", "b"}, FieldAll, "url", "title")
	if clause != " AND (instr(lower(url), ?) > 0 OR instr(lower(title), ?) > 0) AND (instr(lower(url), ?) > 0 OR instr(lower(title), ?) > 0)" {
		t.Errorf("clause = %q", clause)
	}
	if len(args) != 4 || args[0] != "a" || args[3] != "b" {
		t.Errorf("args = %v", args)
	}
	// A shape with no title cannot answer a title-only search.
	clause, _ = matchesClause([]string{"a"}, FieldTitle, "url", "")
	if clause != " AND 0" {
		t.Errorf("title-only on a titleless shape = %q", clause)
	}
	// No terms is no clause at all.
	if clause, args := matchesClause(nil, FieldAll, "url", "title"); clause != "" || args != nil {
		t.Errorf("empty terms = %q %v", clause, args)
	}
}

func TestHistoryStatementAppliesTheWindow(t *testing.T) {
	var chrome historyQuery
	for _, shape := range historyQueries(Profile{}) {
		if shape.table == "urls" {
			chrome = shape
		}
	}
	sql, args := chrome.statement([]string{"go"}, FieldAll, 1_000_000_000)
	for _, want := range []string{"FROM urls", "instr(lower(url), ?) > 0", "last_visit_time >= ?", "ORDER BY last_visit_time DESC"} {
		if !strings.Contains(sql, want) {
			t.Errorf("statement %q lacks %q", sql, want)
		}
	}
	if len(args) != 3 {
		t.Fatalf("args = %v", args)
	}
	if got, want := args[2].(int64), (int64(1_000_000_000)+chromeEpochOffset)*1_000_000; got != want {
		t.Errorf("window bound = %d, want %d", got, want)
	}
	// No window means no bound.
	if _, args := chrome.statement(nil, FieldAll, 0); len(args) != 0 {
		t.Errorf("unbounded args = %v", args)
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
