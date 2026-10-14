package launcher

import (
	"fmt"
	"testing"

	"floter/internal/browser"
)

// browserRows feeds the browser mode an answer and returns its list, opening the
// mode on the `browser` trigger word with the given range chip. The answer is
// injected as the shell's own reply (the shell no longer caps; the mode does —
// see `internal/shell/shell.go`'s `searchBrowser`), so these tests pin the mode's
// half of the reference's two ceilings (`plugins/browser/mode.ts:70,84,199,218`).
func browserRows(a *App, chip string, results []browser.Result, tabs []browser.Tab) []Item {
	a.browser = true
	a.Query = "browser "
	a.browserFilter = chip
	a.browserAsked = true
	a.browserAskedFor = a.browserQuery()
	a.browserFound = BrowserResults{Found: true, Results: results, Tabs: tabs}
	return a.browserItems()
}

// The browser mode holds the reference's fetch depth, not a smaller private
// page size. `BROWSER_GROUP_LIMIT` (MAX_RESULTS = 10) is the plugin output
// view's own box-sized ceiling; the inline launcher raises it to
// `BROWSER_FETCH_LIMIT = 500` and pages what it holds. This build scrolls
// instead of paging, so 500 is the reachable depth: an empty query's first group
// is capped at exactly that, and the tabs are a second group added on top until
// the *total* reaches `cap * 2` (`mode.ts:218`).
func TestTheBrowserModeHoldsTheReferenceFetchDepth(t *testing.T) {
	if BrowserFetchLimit != 500 {
		t.Fatalf("BrowserFetchLimit = %d, want the reference's 500", BrowserFetchLimit)
	}
	results := make([]browser.Result, 0, BrowserFetchLimit+200)
	for i := 0; i < BrowserFetchLimit+200; i++ {
		results = append(results, browser.Result{
			URL:   fmt.Sprintf("https://history.example/%d", i),
			Title: fmt.Sprintf("history %d", i),
			Kind:  browser.KindHistory,
		})
	}
	tabs := make([]browser.Tab, 0, BrowserFetchLimit+200)
	for i := 0; i < BrowserFetchLimit+200; i++ {
		tabs = append(tabs, browser.Tab{
			BrowserID: "chrome",
			Window:    0,
			Index:     i,
			Title:     fmt.Sprintf("tab %d", i),
			URL:       fmt.Sprintf("https://tabs.example/%d", i),
		})
	}

	items := browserRows(testApp(), filterAll, results, tabs)

	first, liveTabs := 0, 0
	for _, item := range items {
		if item.tab != nil {
			liveTabs++
		} else {
			first++
		}
	}
	// The empty query's first group is the fetch depth: the reference's inline
	// mode holds 500 rows and pages them, not ten.
	if first != BrowserFetchLimit {
		t.Errorf("the empty query's first group = %d rows, want %d", first, BrowserFetchLimit)
	}
	// The tabs group goes on top of the first: the reference's `cap * 2` is a
	// *total* ceiling, so the first group can never hide the live tabs.
	if liveTabs != BrowserFetchLimit {
		t.Errorf("the live-tab group = %d rows, want %d", liveTabs, BrowserFetchLimit)
	}
	if len(items) != 2*BrowserFetchLimit {
		t.Errorf("the whole browser list = %d rows, want %d", len(items), 2*BrowserFetchLimit)
	}
}

// A page that is both a bookmark/history entry and an open tab is two rows, not
// one: the reference's merge deduplicates bookmarks against history only and
// leaves the tab group alone ("the merge does not fold the two" —
// `plugins/browser/mode.ts:90`), because "open it again" and "switch to it" are
// different actions.
func TestTheBrowserTabGroupIsNotMergedWithTheFirstGroup(t *testing.T) {
	shared := "https://go.dev/doc"
	items := browserRows(testApp(), filterAll, []browser.Result{{
		URL:   shared,
		Title: "The Go Programming Language",
		Kind:  browser.KindBookmark,
	}}, []browser.Tab{{
		BrowserID: "chrome",
		Window:    0,
		Index:     1,
		Title:     "The Go Programming Language",
		URL:       shared,
	}})

	if len(items) != 2 {
		t.Fatalf("rows = %d, want 2 (the same URL as a bookmark and as a tab)", len(items))
	}
	first, liveTabs := 0, 0
	for _, item := range items {
		if item.tab != nil {
			liveTabs++
		} else {
			first++
		}
	}
	if first != 1 || liveTabs != 1 {
		t.Errorf("first group = %d, tabs = %d; want 1 and 1", first, liveTabs)
	}
	if items[0].ID == items[1].ID {
		t.Errorf("the bookmark and the tab share the id %q", items[0].ID)
	}
}
