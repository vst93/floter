package browser

import (
	"os"
	"strings"

	"floter/internal/plist"
)

// Safari's bookmarks, which live in a property list rather than in a database
// or a JSON file: `~/Library/Safari/Bookmarks.plist`, a binary plist whose
// nodes are lists (folders) and leaves (bookmarks). Reading it is the same
// soft contract as every other source here — a missing, unreadable or
// unexpected file yields nothing rather than an error.

// readSafariBookmarks flattens Safari's bookmark tree.
func readSafariBookmarks(profile Profile) []Result {
	data, err := os.ReadFile(profile.BookmarksFile)
	if err != nil {
		return nil
	}
	value, err := plist.Parse(data)
	if err != nil {
		return nil
	}
	var results []Result
	walkSafariBookmarks(value, profile, &results)
	return results
}

// walkSafariBookmarks visits one node and its children.
//
// The node types are Safari's own: a list has `Children`, a leaf has
// `URLString` and its name in `URIDictionary.title` (with `Title` as the
// fallback), and a proxy is one of Safari's own folders — recursed into like a
// list, because that is where the user's bookmarks bar actually lives.
func walkSafariBookmarks(value any, profile Profile, out *[]Result) {
	record, ok := value.(map[string]any)
	if !ok {
		return
	}
	switch plist.String(record, "WebBookmarkType") {
	case "WebBookmarkTypeLeaf":
		url := strings.TrimSpace(plist.String(record, "URLString"))
		if url == "" {
			return
		}
		title := ""
		if uri, ok := record["URIDictionary"].(map[string]any); ok {
			title = plist.String(uri, "title")
		}
		if title == "" {
			title = plist.String(record, "Title")
		}
		*out = append(*out, Result{
			URL:        url,
			Title:      title,
			BrowserID:  profile.BrowserID,
			Browser:    profile.Browser,
			Kind:       KindBookmark,
			ProfileKey: profile.Key(),
		})
	}
	children, ok := record["Children"].([]any)
	if !ok {
		return
	}
	for _, child := range children {
		walkSafariBookmarks(child, profile, out)
	}
}
