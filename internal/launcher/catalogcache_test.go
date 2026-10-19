package launcher

import (
	"strings"
	"testing"

	"floter/internal/apps"
	"floter/internal/extensions"
	"floter/internal/settings"
)

// The catalog is built once per set of inputs, not once per keystroke.
//
// Every keystroke walks the whole catalog, and building it first is the larger
// half of that walk: an `Item` per application, a `strings.Join` of its aliases,
// and a lowered copy of its four search registers. With a machine-wide catalog
// (600 applications: ~200µs and 3.4k allocations, measured) a fast typist pays
// it thirty times to draw the same rows — the drawing side of 「很多界面经常
// 卡死」, on the one path a user reads as a stall because they are watching the
// list answer them.
func TestTheCatalogIsBuiltOncePerKeystroke(t *testing.T) {
	a := testApp()
	found := make([]apps.App, 0, 300)
	for i := 0; i < 300; i++ {
		found = append(found, apps.App{Name: "App", Path: "/usr/share/applications/app.desktop"})
	}
	a.SetApps(found)
	// The rows are made once for the scan that arrived...
	a.Catalog()
	if got := a.catalogBuilds; got != 1 {
		t.Fatalf("the catalog was built %d times for one list, want 1", got)
	}
	// ...and every keystroke after it reuses them.
	for _, query := range []string{"a", "ap", "app", "appl"} {
		a.Query = query
		a.Results()
	}
	if got := a.catalogBuilds; got != 1 {
		t.Errorf("typing rebuilt the catalog %d times, want the one build: the rows do not depend on the query", got)
	}
}

// The cache is keyed on the inputs, not on "has it been built": a rescan that
// returns the same number of applications, a new command set, a changed alias
// and a new language all have to rebuild it.
func TestTheCatalogRebuildsWhenItsInputsChange(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{{Name: "First", Path: "/first.desktop"}})
	first := a.Catalog()
	if !hasRow(first, "app:/first.desktop") {
		t.Fatalf("the first scan produced no row: %v", ids(first))
	}

	// A rescan that happens to return the same *count* but a different
	// application: the shape's length term cannot see it, the revision can.
	a.SetApps([]apps.App{{Name: "Second", Path: "/second.desktop"}})
	second := a.Catalog()
	if hasRow(second, "app:/first.desktop") || !hasRow(second, "app:/second.desktop") {
		t.Fatalf("the rescan did not replace the row: %v", ids(second))
	}

	// A PATH tool on, then off with the same list.
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	if !hasRow(a.Catalog(), "tool:/usr/bin/git") {
		t.Fatalf("the tool row was not built: %v", ids(a.Catalog()))
	}
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, false)
	if hasRow(a.Catalog(), "tool:/usr/bin/git") {
		t.Fatalf("the tool row survived the switch: %v", ids(a.Catalog()))
	}

	// An alias: it is the row's own search register, so the built row carries
	// the old one until the setter invalidates.
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	a.SetCommandAliases(settings.CommandAliases{"git": "gfm"})
	found := false
	for _, item := range a.Catalog() {
		if item.ID == "tool:/usr/bin/git" && item.alias == "gfm" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the alias did not reach the built row: %v", ids(a.Catalog()))
	}

	// A new language: the built-in commands' titles are the dictionary's.
	before := commandTitle(a.Catalog(), "terminal")
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatalf("could not change the language: %v", err)
	}
	after := commandTitle(a.Catalog(), "terminal")
	if after == before {
		t.Fatalf("the catalog kept the old language's wording: %q", after)
	}
	if after != StringsFor("zh").CommandTerminal {
		t.Errorf("the rebuilt row is %q, want the new dictionary's %q",
			after, StringsFor("zh").CommandTerminal)
	}
}

// The cached rows are shared, so the search must never write back into them:
// the alias hint is the one field `matchedCatalog` sets, and a hint appended to
// a shared row would grow with every keystroke ("Alias: gfm · Alias: gfm · …").
func TestAQueryDoesNotWriteBackIntoTheCachedCatalog(t *testing.T) {
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	a.SetCommandAliases(settings.CommandAliases{"git": "gfm"})
	// The built row's own subtitle, before any query dresses it with a hint.
	built := ""
	for _, item := range a.Catalog() {
		if item.ID == "tool:/usr/bin/git" {
			built = item.Detail
		}
	}
	if built == "" {
		t.Fatal("the tool row has no subtitle to protect")
	}
	a.Query = "gfm"
	firstDetail := ""
	for _, item := range a.Results() {
		if item.ID == "tool:/usr/bin/git" {
			firstDetail = item.Detail
		}
	}
	if firstDetail == "" {
		t.Fatalf("the alias query reached no tool row: %v", ids(a.Results()))
	}
	if strings.Count(firstDetail, StringsFor(a.settings().Language).AliasMatch("gfm")) != 1 {
		t.Fatalf("the row's hint is %q, want the alias named once", firstDetail)
	}
	// The same query again, and a third time: the row says the same thing.
	for i := 0; i < 2; i++ {
		a.Results()
	}
	for _, item := range a.Results() {
		if item.ID == "tool:/usr/bin/git" && item.Detail != firstDetail {
			t.Errorf("the row's subtitle drifted on repeat: %q → %q", firstDetail, item.Detail)
		}
	}
	for _, item := range a.Catalog() {
		if item.ID == "tool:/usr/bin/git" && item.Detail != built {
			t.Errorf("the cached row was written through: %q, want %q", item.Detail, built)
		}
	}
}

// The lowered registers the catalog caches must answer exactly what folding on
// the spot did — the cache is a speed fact, not a behaviour one.
func TestTheLoweredRegistersMatchTheFoldedValues(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{
		{Name: "Visual Studio Code", Path: "/Applications/Code.app", Initials: "vsc", Comment: "Editor"},
		{Name: "网易云音乐", Path: "/Applications/Netease.app", Initials: "wyyyy"},
	})
	a.SetCommandAliases(settings.CommandAliases{"terminal": "Term"})
	for _, item := range a.Catalog() {
		if item.lowerTitle != strings.ToLower(item.Title) ||
			item.lowerDetail != strings.ToLower(item.Detail) ||
			item.lowerID != strings.ToLower(item.ID) ||
			item.lowerSearch != strings.ToLower(item.Search) ||
			item.lowerAlias != strings.ToLower(item.alias) {
			t.Fatalf("row %q cached an unfolded register: %+v", item.ID, item)
		}
	}
	// And the search still reaches a row by its pinyin initials, which is the
	// subsequence rung the cached registers feed.
	a.Query = "vsc"
	if got := a.Results(); len(got) == 0 || got[0].ID != "app:/Applications/Code.app" {
		t.Fatalf("the initials query reached %v", ids(got))
	}
	a.Query = "wyyyy"
	if got := a.Results(); len(got) == 0 || got[0].ID != "app:/Applications/Netease.app" {
		t.Fatalf("the pinyin query reached %v", ids(got))
	}
}

// The catalog cache must not hide a new command set: extensions are discovered
// after the first frame, and their rows are the catalog's own.
func TestANewCommandSetReachesTheSearch(t *testing.T) {
	a := testApp()
	a.Results()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID:   "demo",
		IntegrationName: "Demo",
		Command:         extensions.Command{ID: "hello", Name: "Say Hello"},
	}})
	a.Query = "hello"
	got := a.Results()
	if len(got) == 0 || !strings.HasPrefix(got[0].ID, "cmd:") {
		t.Fatalf("the new command did not reach the search: %v", ids(got))
	}
}

// hasRow reports whether the catalog carries a row with this id.
func hasRow(items []Item, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

// commandTitle is the built-in command's own title, empty when it has no row.
func commandTitle(items []Item, id string) string {
	for _, item := range items {
		if item.ID == id {
			return item.Title
		}
	}
	return ""
}
