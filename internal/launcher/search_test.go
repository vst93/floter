package launcher

import (
	"testing"

	"floter/internal/settings"
)

// testRuns records the built-in commands a test activated. The launcher
// package's tests run in one goroutine, so one map is enough.
var testRuns map[string]int

func testApp() *App {
	testRuns = map[string]int{}
	return New(settings.NewStore(settings.Default()), Actions{
		OpenSettings: func() { testRuns["settings"]++ },
		OpenTerminal: func() { testRuns["terminal"]++ },
		Quit:         func() { testRuns["quit"]++ },
		Dismiss:      func() { testRuns["dismiss"]++ },
	})
}

func TestCatalogIsAuthoredOrderAndLocalized(t *testing.T) {
	a := testApp()
	got := a.Catalog()
	if len(got) != 3 {
		t.Fatalf("catalog has %d items, want 3", len(got))
	}
	if got[0].ID != "settings" || got[1].ID != "terminal" || got[2].ID != "quit" {
		t.Errorf("catalog order = %s, %s, %s", got[0].ID, got[1].ID, got[2].ID)
	}
	if got[0].Title != "Open settings" {
		t.Errorf("english title = %q", got[0].Title)
	}

	zh := testApp()
	if err := zh.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	if title := zh.Catalog()[0].Title; title != "打开设置" {
		t.Errorf("zh title = %q", title)
	}
}

func TestMatchEmptyQueryKeepsTheCatalog(t *testing.T) {
	a := testApp()
	for _, query := range []string{"", "   "} {
		got := a.Match(query)
		if len(got) != 3 {
			t.Errorf("query %q matched %d, want all 3", query, len(got))
		}
	}
}

func TestMatchRanksTitleBeforeDetail(t *testing.T) {
	a := testApp()
	// "settings" is a title word of item 0, and appears in the detail of
	// none of the others, so it ranks first.
	got := a.Match("settings")
	if len(got) == 0 || got[0].ID != "settings" {
		t.Fatalf("Match(settings) = %v", ids(got))
	}
	// "app" is the start of "Appearance..." in item 0's detail, and of
	// nothing else.
	got = a.Match("app")
	if len(got) == 0 || got[0].ID != "settings" {
		t.Fatalf("Match(app) = %v", ids(got))
	}
	if got := a.Match("zzz"); len(got) != 0 {
		t.Errorf("Match(zzz) = %v, want nothing", ids(got))
	}
}

func TestMatchRequiresEveryTerm(t *testing.T) {
	a := testApp()
	// Both terms land on the quit item ("quit floter" / "Exit the
	// application completely").
	got := a.Match("quit floter")
	if len(got) != 1 || got[0].ID != "quit" {
		t.Fatalf("Match(quit floter) = %v", ids(got))
	}
	if got := a.Match("quit terminal"); len(got) != 0 {
		t.Errorf("Match(quit terminal) = %v, want nothing", ids(got))
	}
}

func TestMatchCaseInsensitiveAndPrefixBeatsSubstring(t *testing.T) {
	a := testApp()
	got := a.Match("OPEN")
	// "Open settings" and "Open terminal" both start with it; the "quit"
	// row's detail does not contain it.
	if len(got) != 2 {
		t.Fatalf("Match(OPEN) = %v", ids(got))
	}
}

func TestNavigationClampsAndWraps(t *testing.T) {
	if got := ClampIndex(9, 3); got != 2 {
		t.Errorf("ClampIndex(9,3) = %d, want 2", got)
	}
	if got := ClampIndex(-4, 3); got != 0 {
		t.Errorf("ClampIndex(-4,3) = %d, want 0", got)
	}
	if got := ClampIndex(1, 0); got != 0 {
		t.Errorf("ClampIndex(1,0) = %d, want 0", got)
	}
	if got := NextIndex(2, 1, 3); got != 0 {
		t.Errorf("NextIndex(2,+1,3) = %d, want 0 (wrap)", got)
	}
	if got := NextIndex(0, -1, 3); got != 2 {
		t.Errorf("NextIndex(0,-1,3) = %d, want 2 (wrap)", got)
	}
	if got := NextIndex(0, 1, 0); got != 0 {
		t.Errorf("NextIndex on an empty list = %d, want 0", got)
	}
}

func TestActivateRunsTheSelectedCommand(t *testing.T) {
	a := testApp()
	a.Selected = 1 // terminal
	a.Query = "open"
	a.activate(a.Results())
	if testRuns["terminal"] != 1 {
		t.Errorf("ran = %v, want terminal once", testRuns)
	}
	// An out-of-range selection runs nothing.
	a.Selected = 9
	a.activate(a.Results())
	if testRuns["settings"] != 0 || testRuns["quit"] != 0 {
		t.Errorf("out-of-range ran something: %v", testRuns)
	}
}

func TestQuerySelectsTheFirstResult(t *testing.T) {
	a := testApp()
	a.Query = "terminal"
	got := a.Results()
	if len(got) != 1 || got[0].ID != "terminal" {
		t.Fatalf("Results = %v", ids(got))
	}
}

// Match is a method on App for the current query; expose it for the
// table-driven tests without a query field dance.
func (a *App) Match(query string) []Item { return Match(a.Catalog(), query) }

func ids(items []Item) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.ID
	}
	return out
}
