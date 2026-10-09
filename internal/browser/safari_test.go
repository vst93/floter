package browser

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// safariBookmarksB64 is a Safari `Bookmarks.plist` — the binary property list
// Safari actually writes — holding a bookmarks bar with a nested folder, one
// of Safari's own proxy folders, and a bookmark at the root.
const safariBookmarksB64 = "YnBsaXN0MDDTAQIDBCwZWENoaWxkcmVuVVRpdGxlXxAPV2ViQm9va21hcmtUeXBlogUn1AECAwYHJRkmXxAPV2ViQm9va21hcmtVVUlEowgRG9QJCgMGCw4PEF1VUklEaWN0aW9uYXJ5WVVSTFN0cmluZ9EMDVV0aXRsZVREb2NzXxAYaHR0cHM6Ly9leGFtcGxlLmNvbS9kb2NzXxATV2ViQm9va21hcmtUeXBlTGVhZlFB1AECAwYSGBkaoRPUCQoDBhQWDxfRDBVYSW50cmFuZXRfEBlodHRwczovL2ludHJhbmV0LmV4YW1wbGUvUUJUV29ya18QE1dlYkJvb2ttYXJrVHlwZUxpc3RRV9QBAgMGHCIjJKEd1AkKAwYeIA8h0QwfV1Byb3hpZWRfEBZodHRwczovL3Byb3h5LmV4YW1wbGUvUUNeQm9va21hcmtzIE1lbnVfEBRXZWJCb29rbWFya1R5cGVQcm94eVFQXEJvb2ttYXJrc0JhclNCQVLUCQoDBigqDyvRDClUTmV3c18QFWh0dHBzOi8vbmV3cy5leGFtcGxlL1FEWUJvb2ttYXJrcwAIAA8AGAAeADAAMwA8AE4AUgBbAGkAcwB2AHwAgQCcALIAtAC9AL8AyADLANQA8ADyAPcBDQEPARgBGgEjASYBLgFHAUkBWAFvAXEBfgGCAYsBjgGTAasBrQAAAAAAAAIBAAAAAAAAAC0AAAAAAAAAAAAAAAAAAAG3"

func TestSafariBookmarks(t *testing.T) {
	data, err := base64.StdEncoding.DecodeString(safariBookmarksB64)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "Bookmarks.plist")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	profile := Profile{BrowserID: BrowserSafari, Browser: "Safari", Dir: "Safari", BookmarksFile: path}
	results := readBookmarks(profile)
	if len(results) != 4 {
		t.Fatalf("bookmarks = %+v", results)
	}
	titles := map[string]string{}
	for _, result := range results {
		titles[result.Title] = result.URL
		if result.Kind != KindBookmark || result.BrowserID != BrowserSafari || result.ProfileKey != "safari/Safari" {
			t.Errorf("result = %+v", result)
		}
	}
	// A nested folder and a proxy folder are both recursed into.
	for title, url := range map[string]string{
		"Docs":     "https://example.com/docs",
		"Intranet": "https://intranet.example/",
		"Proxied":  "https://proxy.example/",
		"News":     "https://news.example/",
	} {
		if titles[title] != url {
			t.Errorf("%s = %q, want %q", title, titles[title], url)
		}
	}

	// A missing, malformed or unexpected file yields nothing.
	if got := readBookmarks(Profile{BookmarksFile: filepath.Join(dir, "nope.plist")}); got != nil {
		t.Errorf("missing file = %+v", got)
	}
	bad := filepath.Join(dir, "bad.plist")
	if err := os.WriteFile(bad, []byte("not a plist"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readBookmarks(Profile{BookmarksFile: bad}); got != nil {
		t.Errorf("malformed file = %+v", got)
	}
	plain := filepath.Join(dir, "plain.plist")
	if err := os.WriteFile(plain, []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>WebBookmarkType</key><string>WebBookmarkTypeList</string></dict></plist>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readBookmarks(Profile{BookmarksFile: plain}); got != nil {
		t.Errorf("an empty list = %+v", got)
	}
	// A leaf with no URL is skipped rather than shown as an empty row.
	if got := walkResults(t, `<?xml version="1.0"?><plist version="1.0"><dict>
	  <key>WebBookmarkType</key><string>WebBookmarkTypeLeaf</string>
	  <key>URIDictionary</key><dict><key>title</key><string>Nameless</string></dict>
	</dict></plist>`); len(got) != 0 {
		t.Errorf("a leaf with no URL = %+v", got)
	}
	// A leaf with no title falls back to its URL.
	got := walkResults(t, `<?xml version="1.0"?><plist version="1.0"><dict>
	  <key>WebBookmarkType</key><string>WebBookmarkTypeLeaf</string>
	  <key>URLString</key><string>https://untitled.example/</string>
	</dict></plist>`)
	if len(got) != 1 || got[0].URL != "https://untitled.example/" || got[0].Title != "" {
		t.Errorf("an untitled leaf = %+v", got)
	}
}

// walkResults reads one XML fixture through the Safari walker.
func walkResults(t *testing.T, document string) []Result {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Bookmarks.plist")
	if err := os.WriteFile(path, []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	return readSafariBookmarks(Profile{BrowserID: BrowserSafari, Browser: "Safari", Dir: "Safari", BookmarksFile: path})
}
