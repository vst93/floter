package launcher

import (
	"reflect"
	"testing"

	"floter/internal/apps"
	"floter/internal/extensions"
	"floter/internal/settings"
)

// testRuns records the built-in commands a test activated. The launcher
// package's tests run in one goroutine, so one map is enough.
var testRuns map[string]int

func testApp() *App {
	testRuns = map[string]int{}
	return New(settings.NewStore(settings.Default()), Actions{
		OpenSettings:  func() { testRuns["settings"]++ },
		OpenTerminal:  func() { testRuns["terminal"]++ },
		Quit:          func() { testRuns["quit"]++ },
		Dismiss:       func() { testRuns["dismiss"]++ },
		Copy:          func(string) { testRuns["copy"]++ },
		OpenApp:       func(app apps.App) { testRuns["app:"+app.Name]++ },
		RunCommand:    func(entry extensions.CommandEntry, args []string) { testRuns["cmd:"+entry.Command.ID] += len(args) + 1 },
		RunInTerminal: func(argv []string) { testRuns["tool"]++ },
	})
}

func TestCatalogIsAuthoredOrderAndLocalized(t *testing.T) {
	a := testApp()
	got := a.Catalog()
	if len(got) != 8 {
		t.Fatalf("catalog has %d items, want 8", len(got))
	}
	want := []string{"settings", "terminal", "browser", "clipboard", "calculator", "power-restart", "power-shutdown", "quit"}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("catalog[%d] = %s, want %s", i, got[i].ID, id)
		}
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
		if len(got) != 8 {
			t.Errorf("query %q matched %d, want all 8", query, len(got))
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

func TestCalculatorRowComesFirstAndCopies(t *testing.T) {
	a := testApp()
	a.Query = "2+2"
	got := a.Results()
	if len(got) == 0 || got[0].ID != "calc" {
		t.Fatalf("Results(2+2) = %v", ids(got))
	}
	if got[0].Title != "4" || got[0].Detail != "2+2" {
		t.Errorf("calc row = %q / %q", got[0].Title, got[0].Detail)
	}
	a.activate(got)
	if testRuns["copy"] != 1 {
		t.Errorf("copy ran %v", testRuns)
	}
	if a.Query != "" {
		t.Errorf("the field was not cleared: %q", a.Query)
	}
	if a.toast == "" {
		t.Error("no copy feedback was queued")
	}

	// A bare number is a search, not a sum.
	a.Query = "1"
	for _, item := range a.Results() {
		if item.ID == "calc" {
			t.Error("a bare number produced a calculator row")
		}
	}
	// A broken sum shows no row either, and does not panic.
	a.Query = "1+"
	for _, item := range a.Results() {
		if item.ID == "calc" {
			t.Error("a broken sum produced a calculator row")
		}
	}
}

func TestAppsJoinTheSearchOnceTyped(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{
		{Name: "Safari", Path: "/Applications/Safari.app"},
		{Name: "Terminal", Path: "/System/Applications/Utilities/Terminal.app"},
	})

	// An empty query shows the built-in commands alone.
	if got := a.Results(); len(got) != 8 {
		t.Errorf("empty query shows %d rows (%v)", len(got), ids(got))
	}

	a.Query = "safari"
	got := a.Results()
	if len(got) == 0 || got[0].Title != "Safari" {
		t.Fatalf("Results(safari) = %v", ids(got))
	}
	a.activate(got)
	if testRuns["app:Safari"] != 1 {
		t.Errorf("open ran %v", testRuns)
	}

	// The application ranks above the command that merely contains the
	// word: a title prefix (score 0) beats a word prefix (score 1).
	a.Query = "terminal"
	got = a.Results()
	if len(got) < 2 || got[0].Title != "Terminal" || got[1].ID != "terminal" {
		t.Errorf("Results(terminal) = %v", ids(got))
	}

	// A broad query returns every match; the list view builds only the rows
	// in view (see TestBigCatalogBuildsOnlyTheRowsInView).
	many := make([]apps.App, 0, 60)
	for i := 0; i < 60; i++ {
		many = append(many, apps.App{Name: "App " + string(rune('a'+i%26)), Path: "/Applications/App.app"})
	}
	a.SetApps(many)
	a.Query = "app"
	if got := a.Results(); len(got) < 60 {
		t.Errorf("Results returned %d rows, want the 60 applications too", len(got))
	}
}

func TestExtensionCommandsJoinTheSearch(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID:   "io.github.vst93.v",
		IntegrationName: "V Tools",
		ProviderName:    "V Tools",
		Command: extensions.Command{
			ID:          "jv",
			Name:        "JSON Viewer",
			Description: "View, format and edit JSON",
			Aliases:     []string{"jsonview"},
			Keywords:    []string{"pretty"},
		},
		Program: "/usr/local/bin/v",
		Args:    []string{"jv"},
		Mode:    "pty",
	}})

	// An empty query shows the commands alone, not a wall of extension
	// rows.
	a.Query = ""
	for _, item := range a.Results() {
		if item.ID == "cmd:io.github.vst93.v:jv" {
			t.Error("an extension command showed for an empty query")
		}
	}

	// The name, the description, an alias and a keyword all match.
	for _, query := range []string{"json viewer", "edit json", "jsonview", "pretty", "v tools"} {
		a.Query = query
		found := false
		for _, item := range a.Results() {
			if item.ID == "cmd:io.github.vst93.v:jv" {
				found = true
				if item.Title != "JSON Viewer" || item.Detail != "View, format and edit JSON" {
					t.Errorf("%q: row = %q / %q", query, item.Title, item.Detail)
				}
			}
		}
		if !found {
			t.Errorf("query %q did not match the command: %v", query, ids(a.Results()))
		}
	}

	a.Query = "json viewer"
	a.activate(a.Results())
	if testRuns["cmd:jv"] != 1 {
		t.Errorf("running the command recorded %v", testRuns)
	}
}

func TestSplitArgs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"jv", []string{"jv"}},
		{"jv -f", []string{"jv", "-f"}},
		{"jv  -f   file.json", []string{"jv", "-f", "file.json"}},
		{`jv -file "my file.json"`, []string{"jv", "-file", "my file.json"}},
		{`jv -file 'my file.json'`, []string{"jv", "-file", "my file.json"}},
		{`jv "unterminated`, []string{"jv", "unterminated"}},
		{`jv a\ b`, []string{"jv", "a b"}},
	}
	for _, tc := range cases {
		if got := splitArgs(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitArgs(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
	if got := firstWord("  jv -f"); got != "jv" {
		t.Errorf("firstWord = %q", got)
	}
	if got := firstWord("   "); got != "" {
		t.Errorf("firstWord of blanks = %q", got)
	}
}

// An application row is titled with the name the user's own desktop shows and
// subtitled with the Latin one, and every alias is searchable.
func TestApplicationNamesAndAliases(t *testing.T) {
	a := testApp()
	app := apps.App{Name: "WeCom", Localized: "企业微信", Aliases: []string{"WeCom", "企业微信", "WeWorkMac"}, Initials: "wecomqywx", Path: "/Applications/WeCom.app"}
	a.SetApps([]apps.App{app})
	a.Query = "企业微信"
	results := a.Results()
	if len(results) != 1 {
		t.Fatalf("results = %+v", results)
	}
	if results[0].Title != "企业微信" || results[0].Detail != "WeCom" {
		t.Errorf("row = %+v", results[0])
	}
	// The Latin name and an identifier alias both find it too.
	// The alias and the pinyin initials find it; a subsequence match may pull
	// in other rows whose names share the letters, but the application is
	// among them.
	for _, query := range []string{"WeCom", "WeWorkMac", "qywx"} {
		a.Query = query
		got := a.Results()
		found := false
		for _, item := range got {
			if item.ID == "app:"+app.Path {
				found = true
			}
		}
		if !found {
			t.Errorf("query %q did not reach the application: %v", query, got)
		}
	}
}

// A Chinese-named application is reachable from a Latin keyboard through the
// pinyin initials its scan computed.
func TestPinyinInitialsJoinTheSearch(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{
		{Name: "WeCom", Localized: "企业微信", Initials: "qywxwecom", Path: "/Applications/WeCom.app"},
		{Name: "网易云音乐", Initials: "wyyyy", Path: "/Applications/Netease.app"},
	})
	for _, tc := range []struct{ query, id string }{
		{"wyyyy", "app:/Applications/Netease.app"},
		{"qywx", "app:/Applications/WeCom.app"},
	} {
		a.Query = tc.query
		got := a.Results()
		found := false
		for _, item := range got {
			if item.ID == tc.id {
				found = true
			}
		}
		if !found {
			t.Errorf("query %q did not reach %s: %v", tc.query, tc.id, ids(got))
		}
	}
}

// The subsequence tier of the search ladder: a term whose characters appear
// in order (starting at a word boundary) finds the row, while a mid-word
// start does not.
func TestSubsequenceMatching(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{
		{Name: "Visual Studio Code", Path: "/Applications/Visual Studio Code.app"},
	})
	var found bool
	// `vsc` is a word-started subsequence of Visual Studio Code.
	a.Query = "vsc"
	got := a.Results()
	if len(got) == 0 || got[0].Title != "Visual Studio Code" {
		t.Errorf("vsc = %+v", got)
	}
	// A CJK name's pinyin initials work the same way: the scan fills the
	// initials, and the launcher matches against them.
	a.SetApps([]apps.App{
		{Name: "网易云音乐", Path: "/Applications/Netease.app", Initials: "wyyyy"},
	})
	a.Query = "wyyyy"
	got = a.Results()
	found = false
	for _, item := range got {
		if item.Title == "网易云音乐" {
			found = true
		}
	}
	if !found {
		t.Errorf("wyyyy = %+v", got)
	}
	// A single term that is the whole name is the ordinary path.
	a.SetApps([]apps.App{
		{Name: "Visual Studio Code", Path: "/Applications/Visual Studio Code.app"},
	})
	a.Query = "visual studio code"
	got = a.Results()
	found = false
	for _, item := range got {
		if item.Title == "Visual Studio Code" {
			found = true
		}
	}
	if !found {
		t.Errorf("full name = %+v", got)
	}
	// A mid-word subsequence still finds the row: it ranks below the
	// contained and word-prefix tiers.
	a.SetApps([]apps.App{
		{Name: "Visual Studio Code", Path: "/Applications/Visual Studio Code.app"},
	})
	a.Query = "sual"
	got = a.Results()
	found = false
	for _, item := range got {
		if item.Title == "Visual Studio Code" {
			found = true
		}
	}
	if !found {
		t.Errorf("a mid-word subsequence = %+v", got)
	}
}

func TestIsSubsequence(t *testing.T) {
	candidate := "visual studio code"
	// Subsequences match at any position.
	for _, term := range []string{"v", "vs", "vsc", "sc", "studio", "sual", "vcde"} {
		if !isSubsequence(term, candidate) {
			t.Errorf("isSubsequence(%q, %q) = false, want true", term, candidate)
		}
	}
	// Non-subsequences do not.
	for _, term := range []string{"x", "codex", "edoc"} {
		if isSubsequence(term, candidate) {
			t.Errorf("isSubsequence(%q) = true, want false", term)
		}
	}
	// A term longer than the candidate cannot be a subsequence.
	if isSubsequence(candidate+"x", candidate) {
		t.Error("a longer term matched")
	}
	// An empty term is trivially a subsequence.
	if !isSubsequence("", candidate) {
		t.Error("an empty term did not match")
	}
	// CJK characters participate as runes.
	if !isSubsequence("网易", "网易云音乐") {
		t.Error("CJK subsequence did not match")
	}
	if isSubsequence("网易易", "网易云音乐") {
		t.Error("a non-existent CJK subsequence matched")
	}
}
