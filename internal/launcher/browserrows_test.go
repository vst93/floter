package launcher

import (
	"testing"
	"time"

	"floter/internal/browser"
)

// The browser mode's rows follow the reference's own row content and glyph
// vocabulary (`src/launcher/LauncherResults.tsx:290-294`, `src/plugins/browser/mode.ts`):
// a history entry leads with the clock, a bookmark with the bookmark mark, a
// live tab with the app-window mark; every one of them prints the URL alone as
// its subtitle, and a tab's identity is its *position* — the same page open in
// two windows is two rows.
func TestBrowserRowsFollowTheReferenceRowContent(t *testing.T) {
	a := testApp()
	history := browser.Result{
		URL: "https://go.dev/doc", Title: "Go Documentation",
		Kind: browser.KindHistory, BrowserID: "chrome", Browser: "Chrome",
		Visited: time.Unix(1_700_000_000, 0),
	}
	bookmark := browser.Result{
		URL: "https://rust-lang.org", Title: "Rust",
		Kind: browser.KindBookmark, BrowserID: "chrome", Browser: "Chrome",
	}
	// The same URL in two windows: position, not URL, keeps the rows distinct.
	tabA := browser.Tab{BrowserID: "chrome", Window: 0, Index: 0, Title: "Rust Playground", URL: "https://play.rust-lang.org"}
	tabB := browser.Tab{BrowserID: "chrome", Window: 1, Index: 2, Title: "Rust Playground", URL: "https://play.rust-lang.org"}
	a.Actions.SearchBrowser = func(query string, done func(BrowserResults)) {
		done(BrowserResults{Found: true, Results: []browser.Result{history, bookmark}, Tabs: []browser.Tab{tabA, tabB}})
	}

	tt := render(t, a)
	tt.Type("browser ")
	tt.Frame()
	if !a.browser {
		t.Fatalf("the browser mode did not open: %v %q", a.browser, a.Query)
	}

	byTitle := map[string][]Item{}
	for _, item := range a.Results() {
		byTitle[item.Title] = append(byTitle[item.Title], item)
	}

	// The clock, not the shared globe: a history entry is a page *at a time*.
	if items := byTitle["Go Documentation"]; len(items) != 1 {
		t.Fatalf("history rows = %d, want 1", len(items))
	} else if glyph, ok := rowGlyph(items[0]); !ok || glyph != glyphClock {
		t.Errorf("the history row's glyph = %v, want the clock", ok)
	} else if items[0].Detail != history.URL {
		t.Errorf("the history subtitle = %q, want the bare URL %q", items[0].Detail, history.URL)
	}
	if items := byTitle["Rust"]; len(items) != 1 {
		t.Fatalf("bookmark rows = %d, want 1", len(items))
	} else if glyph, ok := rowGlyph(items[0]); !ok || glyph != glyphBookmark {
		t.Errorf("the bookmark row's glyph = %v, want the bookmark", ok)
	}

	// Two tabs, two rows: the URL is the subtitle, the position is the id.
	tabs := byTitle["Rust Playground"]
	if len(tabs) != 2 {
		t.Fatalf("tab rows = %d, want 2 (the same URL twice)", len(tabs))
	}
	for _, item := range tabs {
		if glyph, ok := rowGlyph(item); !ok || glyph != glyphWindow {
			t.Errorf("the tab row's glyph = %v, want the app window", ok)
		}
		if item.Detail != tabA.URL {
			t.Errorf("the tab subtitle = %q, want the bare URL %q", item.Detail, tabA.URL)
		}
	}
	if tabs[0].ID == tabs[1].ID {
		t.Fatalf("the two tabs share an id: %q", tabs[0].ID)
	}
	want := map[string]bool{"tab:chrome:0:0": true, "tab:chrome:1:2": true}
	for _, item := range tabs {
		if !want[item.ID] {
			t.Errorf("tab id = %q, want one of %v", item.ID, want)
		}
	}
}
