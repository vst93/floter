package launcher

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/browser"
	"floter/internal/calculator"
	"floter/internal/clipboard"
	"floter/internal/drops"
	"floter/internal/extensions"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/tools"
)

// render draws one frame of the launcher at the given size and returns the
// tester, so a test can type, click and read what shows.
func render(t *testing.T, a *App) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
	}, InputWindowWidth, int(WindowHeight("small")))
	tt.Frame()
	return tt
}

func TestLauncherShowsTheCatalogWithTheFieldFocused(t *testing.T) {
	a := testApp()
	tt := render(t, a)

	for _, want := range []string{"Open settings", "Open terminal", "Quit floter", "Type to search", "Search"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
	if !tt.Focused("Search") {
		t.Errorf("the search field does not have the focus")
	}
}

func TestTypingFiltersAndEnterRunsTheTopResult(t *testing.T) {
	a := testApp()
	tt := render(t, a)

	tt.Type("terminal")
	tt.Frame()
	if !tt.HasText("Open terminal") {
		t.Fatalf("the terminal command is gone: %q", tt.Texts())
	}
	if tt.HasText("Open settings") || tt.HasText("Quit floter") {
		t.Errorf("the query did not filter: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if testRuns["terminal"] != 1 {
		t.Errorf("Enter ran %v, want terminal", testRuns)
	}
}

func TestArrowsMoveTheChoiceAndEnterRunsIt(t *testing.T) {
	a := testApp()
	tt := render(t, a)

	if a.Selected != 0 {
		t.Fatalf("selected = %d, want 0", a.Selected)
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if a.Selected != 1 {
		t.Fatalf("Down selected %d, want 1", a.Selected)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if testRuns["terminal"] != 1 {
		t.Errorf("Enter ran %v, want terminal", testRuns)
	}

	// Up wraps back to the last item.
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if a.Selected != 0 {
		t.Errorf("Up from 1 selected %d, want 0 (wrap to 0 of 3 after terminal ran)", a.Selected)
	}
}

func TestEscapeDismissesOnlyWithAnEmptyQuery(t *testing.T) {
	a := testApp()
	tt := render(t, a)

	tt.Type("set")
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if testRuns["dismiss"] != 0 {
		t.Errorf("Escape dismissed with a query typed")
	}
	if a.Query != "" {
		t.Errorf("Escape did not clear the query: %q", a.Query)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if testRuns["dismiss"] != 1 {
		t.Errorf("Escape with an empty query dismissed %d times, want 1", testRuns["dismiss"])
	}
}

func TestClickingARowRunsIt(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	if err := tt.Click("Open terminal"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if testRuns["terminal"] != 1 {
		t.Errorf("click ran %v, want terminal", testRuns)
	}
}

func TestNoResultsShowsTheEmptyMessage(t *testing.T) {
	a := testApp()
	a.Query = "zzzz"
	tt := render(t, a)
	if !tt.HasText("No results") {
		t.Errorf("missing the no-results message: %q", tt.Texts())
	}
}

func TestLauncherLanguageFollowsTheStore(t *testing.T) {
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	tt := render(t, a)
	if !tt.HasText("打开终端") {
		t.Errorf("missing the Chinese command: %q", tt.Texts())
	}
}

func TestCalculatorRowRendersAndCopiesOnEnter(t *testing.T) {
	a := testApp()
	tt := render(t, a)

	tt.Type("3*7")
	tt.Frame()
	if !tt.HasText("21") {
		t.Fatalf("the calculator row did not show: %q", tt.Texts())
	}
	if !tt.HasText("3*7") {
		t.Errorf("the expression is not shown: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if testRuns["copy"] != 1 {
		t.Errorf("Enter did not copy: %v", testRuns)
	}
	if !tt.HasText("Copied") {
		t.Errorf("no copy feedback: %q", tt.Texts())
	}
}

func TestScannedAppsAppearInTheList(t *testing.T) {
	a := testApp()
	a.SetApps([]apps.App{{Name: "Safari", Path: "/Applications/Safari.app"}})
	tt := render(t, a)

	if tt.HasText("Safari") {
		t.Errorf("an application showed before the user typed: %q", tt.Texts())
	}
	tt.Type("saf")
	tt.Frame()
	if !tt.HasText("Safari") {
		t.Errorf("the application did not show: %q", tt.Texts())
	}
	if err := tt.Click("Safari"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if testRuns["app:Safari"] != 1 {
		t.Errorf("the click did not open the app: %v", testRuns)
	}
}

func TestBigCatalogBuildsOnlyTheRowsInView(t *testing.T) {
	a := testApp()
	found := make([]apps.App, 0, 500)
	for i := 0; i < 500; i++ {
		found = append(found, apps.App{
			Name: fmt.Sprintf("App %03d", i),
			Path: fmt.Sprintf("/Applications/App %03d.app", i),
		})
	}
	a.SetApps(found)
	a.Query = "app"

	if got := a.Results(); len(got) < 500 {
		t.Fatalf("Results returned %d rows, want the 500 applications", len(got))
	}

	tt := render(t, a)
	built := 0
	for _, text := range tt.Texts() {
		if strings.HasPrefix(text, "App ") {
			built++
		}
	}
	if built == 0 {
		t.Fatalf("no result rows were built: %q", tt.Texts())
	}
	// The window shows about six rows; the list builds those and a few
	// beyond, not all 500.
	if built > 40 {
		t.Errorf("built %d rows, want only those in view", built)
	}
}

// commandApp is a launcher with one extension command that declares two
// arguments, for the command mode.
func commandApp() *App {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID:   "io.github.vst93.v",
		IntegrationName: "V Tools",
		ProviderName:    "V Tools",
		Command: extensions.Command{
			ID:          "jv",
			Name:        "JSON Viewer",
			Description: "View, format and edit JSON",
			Arguments: []extensions.Argument{
				{Names: []string{"-f", "--format"}, Kind: "flag", Description: "Format JSON"},
				{Names: []string{"-c"}, Kind: "flag", Description: "Compress JSON"},
				{Names: []string{"-mode"}, Kind: "enum", TakesValue: true, Description: "Output mode", Values: []string{"pretty", "plain"}},
			},
		},
		Program: "/usr/local/bin/v",
		Args:    []string{"jv"},
		Mode:    "pty",
	}})
	a.Query = "json"
	return a
}

func TestTabExpandsAnExtensionCommandIntoItsArguments(t *testing.T) {
	a := commandApp()
	tt := render(t, a)

	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.mode == nil {
		t.Fatalf("Tab did not enter the command mode: %q", tt.Texts())
	}
	if a.Query != "jv " {
		t.Errorf("the field holds %q, want the command id and a space", a.Query)
	}
	// The list offers the command's arguments (every name), not the enum
	// values, until the argument that takes them is typed.
	for _, want := range []string{"-f", "--format", "-c", "-mode"} {
		if !tt.HasText(want) {
			t.Errorf("missing argument %q in %q", want, tt.Texts())
		}
	}
	if tt.HasText("pretty") {
		t.Errorf("an enum value showed before its argument: %q", tt.Texts())
	}

	// Typing narrows the list to what prefixes it.
	tt.Type("-m")
	tt.Frame()
	if !tt.HasText("-mode") || tt.HasText("-c") {
		t.Errorf("typing did not narrow the list: %q", tt.Texts())
	}
	for range "-m" {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Frame()

	// After the argument that takes a value, its values are what is left.
	tt.Type("-mode ")
	tt.Frame()
	if !tt.HasText("pretty") || !tt.HasText("plain") {
		t.Errorf("the enum values did not show: %q", tt.Texts())
	}
	if tt.HasText("--format") {
		t.Errorf("argument names are still listed after a value argument: %q", tt.Texts())
	}
	tt.Frame()
}

func TestCommandModeFiltersCompletesAndRuns(t *testing.T) {
	a := commandApp()
	tt := render(t, a)
	tt.Key(0, ui.KeyTab)
	tt.Frame()

	// Typing filters the arguments.
	tt.Type("-m")
	tt.Frame()
	if !tt.HasText("-mode") {
		t.Fatalf("typing did not filter: %q", tt.Texts())
	}
	if tt.HasText("-c") {
		t.Errorf("a flag that does not match is still listed: %q", tt.Texts())
	}

	// Tab completes the chosen argument into the line.
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.Query != "jv -mode " {
		t.Fatalf("completion left %q, want \"jv -mode \"", a.Query)
	}

	// Enter runs the typed argv.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if testRuns["cmd:jv"] != 2 { // one run, plus the argument counted by the recorder
		t.Errorf("run recorded %v", testRuns)
	}
	if a.mode != nil || a.Query != "" {
		t.Errorf("the mode survived a run: mode=%v query=%q", a.mode, a.Query)
	}
}

func TestEscapeLeavesTheCommandMode(t *testing.T) {
	a := commandApp()
	tt := render(t, a)
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	tt.Type("-f")
	tt.Frame()

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.mode != nil {
		t.Error("Escape did not leave the command mode")
	}
	if a.Query != "" {
		t.Errorf("the field holds %q", a.Query)
	}
	if !tt.HasText("Open settings") {
		t.Errorf("the search did not come back: %q", tt.Texts())
	}
}

func TestDeletingTheCommandLeavesTheMode(t *testing.T) {
	a := commandApp()
	a.Query = "j"
	tt := render(t, a)
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.mode == nil {
		t.Fatal("Tab did not enter the mode")
	}

	// The user deletes the command id: the mode goes with it.
	for range "jv " {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Frame()
	if a.mode != nil {
		t.Errorf("the mode survived deleting the id: query=%q", a.Query)
	}
}

func TestClickingACommandRunsItWithNoArguments(t *testing.T) {
	a := commandApp()
	tt := render(t, a)
	if err := tt.Click("JSON Viewer"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if testRuns["cmd:jv"] != 1 {
		t.Errorf("clicking ran %v, want the bare command", testRuns)
	}
}

// clipboardApp is a launcher whose clipboard history holds two entries.
func clipboardApp(t *testing.T) (*App, *clipboard.Store) {
	t.Helper()
	store := clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), 0)
	if _, _, err := store.AddText("first clip"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.AddText("second clip with a secret"); err != nil {
		t.Fatal(err)
	}
	a := testApp()
	a.Clipboard = store
	return a, store
}

func TestClipboardModeSearchesAndCopies(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)

	// The built-in row enters the mode.
	tt.Type("clipboard")
	tt.Frame()
	if !tt.HasText("Clipboard history") {
		t.Fatalf("the clipboard row is missing: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if a.Query != "clipboard " || !a.clipboard {
		t.Fatalf("Enter left query %q, clipboard=%v", a.Query, a.clipboard)
	}
	// Both entries show while the query is empty.
	if !tt.HasText("first clip") || !tt.HasText("second clip with a secret") {
		t.Errorf("the entries did not show: %q", tt.Texts())
	}

	// The query filters the history.
	tt.Type("secret")
	tt.Frame()
	if tt.HasText("first clip") {
		t.Errorf("a non-matching entry is still listed: %q", tt.Texts())
	}
	if !tt.HasText("second clip with a secret") {
		t.Fatalf("the entry did not match: %q", tt.Texts())
	}

	// Enter copies it, leaves the mode and dismisses the window.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if testRuns["copy"] != 1 {
		t.Errorf("copy ran %v", testRuns)
	}
	if a.clipboard || a.Query != "" {
		t.Errorf("the mode survived: clipboard=%v query=%q", a.clipboard, a.Query)
	}
	if testRuns["dismiss"] != 1 {
		t.Errorf("the window was not dismissed: %v", testRuns)
	}
}

func TestClipboardModeLeavesWhenTheWordGoes(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the mode did not open")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.clipboard || a.Query != "" {
		t.Errorf("Escape left clipboard=%v query=%q", a.clipboard, a.Query)
	}
}

func TestClipboardModeWithoutAStore(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	// No store: the mode opens and shows nothing rather than crashing.
	if !a.clipboard {
		t.Fatal("the mode did not open")
	}
	if !tt.HasText("Enter runs it") {
		t.Errorf("the mode hint did not show: %q", tt.Texts())
	}
	a.activate(nil)
	if a.clipboard {
		t.Error("activating nothing did not leave the mode")
	}
}

func TestDynamicCompletionsMergeIntoTheCommandMode(t *testing.T) {
	a := commandApp()
	// The command declares one argument the provider completes.
	a.Commands[0].Command.Arguments = append(a.Commands[0].Command.Arguments,
		extensions.Argument{Names: []string{"-run"}, Kind: "command", TakesValue: true, Description: "Run a task"})
	asked := [][]string{}
	a.Actions.Complete = func(entry extensions.CommandEntry, tokens []string, done func([]extensions.Completion)) {
		asked = append(asked, tokens)
		done([]extensions.Completion{{Label: "deploy", Kind: "command", Detail: "Deploy it"}})
	}
	a.Query = "json"
	tt := render(t, a)
	tt.Key(0, ui.KeyTab)
	tt.Frame()

	// Nothing is asked for a plain flag.
	tt.Type("-f")
	tt.Frame()
	if len(asked) != 0 {
		t.Errorf("the provider was asked about a flag: %v", asked)
	}
	for range "-f" {
		tt.Key(0, ui.KeyBackspace)
	}
	tt.Frame()

	// A command-kind argument is what the provider completes.
	tt.Type("-run ")
	tt.Frame()
	if len(asked) == 0 {
		t.Fatalf("the provider was not asked: %q", tt.Texts())
	}
	tt.Frame()
	if !tt.HasText("deploy") {
		t.Errorf("the dynamic completion did not merge in: %q", tt.Texts())
	}
	// Tab takes it, and Enter runs the argv.
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.Query != "jv -run deploy " {
		t.Fatalf("completion left %q", a.Query)
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if testRuns["cmd:jv"] != 3 {
		t.Errorf("run recorded %v", testRuns)
	}
}

func TestTabPinsAClipboardEntry(t *testing.T) {
	a, _ := clipboardApp(t)
	pinnedTitle, pinnedText := "", ""
	a.Actions.PinText = func(title, text string) { pinnedTitle, pinnedText = title, text }
	tt := render(t, a)

	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	tt.Type("secret")
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if pinnedText != "second clip with a secret" {
		t.Errorf("pinned %q / %q", pinnedTitle, pinnedText)
	}
	// Tab pins; it does not copy or leave the mode.
	if testRuns["copy"] != 0 || !a.clipboard {
		t.Errorf("Tab copied or left the mode: %v clipboard=%v", testRuns, a.clipboard)
	}
}

func TestBrowserModeOpensAndCopies(t *testing.T) {
	a := testApp()
	found := []browser.Result{
		{URL: "https://go.dev/doc", Title: "Go Documentation", Kind: "history", BrowserID: "chrome", Browser: "Chrome", Visited: time.Unix(1_700_000_000, 0)},
		{URL: "https://rust-lang.org", Title: "Rust", Kind: "bookmark", BrowserID: "chrome", Browser: "Chrome"},
	}
	tabs := []browser.Tab{
		{BrowserID: "chrome", Index: 0, Title: "Rust Playground", URL: "https://play.rust-lang.org"},
	}
	a.Actions.SearchBrowser = func(query string, done func(BrowserResults)) {
		if query == "" {
			done(BrowserResults{Found: true, Results: found, Tabs: tabs})
			return
		}
		var out []browser.Result
		for _, result := range found {
			if strings.Contains(strings.ToLower(result.Title), strings.ToLower(query)) {
				out = append(out, result)
			}
		}
		done(BrowserResults{Found: true, Results: out, Tabs: tabs})
	}
	opened := ""
	a.Actions.OpenURL = func(browserID, url string) {
		if browserID != "chrome" {
			t.Errorf("opened %q in %q", url, browserID)
		}
		opened = url
	}

	// The trigger word enters the mode on its own, as the old build's did:
	// the word, a space, and the rest is the needle.
	tt := render(t, a)
	tt.Type("browser ")
	tt.Frame()
	if !a.browser || a.Query != "browser " {
		t.Fatalf("the browser mode did not open: %v %q", a.browser, a.Query)
	}
	if !tt.HasText("Go Documentation") || !tt.HasText("Rust") {
		t.Fatalf("the results did not show: %q", tt.Texts())
	}
	// The live tabs follow the file results, in their own group.
	if !tt.HasText("Rust Playground") {
		t.Fatalf("the tabs did not show: %q", tt.Texts())
	}
	// The detail line carries the URL and the visit time for history.
	// The detail carries the URL and the visit time (in the machine's own
	// zone, so only the separator is pinned).
	if !tt.HasText("https://go.dev/doc  \u00b7  ") {
		t.Errorf("the history detail is missing: %q", tt.Texts())
	}
	if !tt.HasText("https://rust-lang.org") {
		t.Errorf("a bookmark's detail is missing: %q", tt.Texts())
	}

	// Typing asks the shell again and shows what it answers.
	tt.Type("rust")
	tt.Frame()
	if tt.HasText("Go Documentation") {
		t.Errorf("the query did not narrow the results: %q", tt.Texts())
	}
	tt.Frame()

	// Tab copies the chosen URL without leaving the mode.
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if testRuns["copy"] != 1 || !a.browser {
		t.Errorf("Tab did not copy: %v mode=%v", testRuns, a.browser)
	}

	// Enter opens it and leaves.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if opened != "https://rust-lang.org" {
		t.Errorf("opened %q", opened)
	}
	if a.browser || a.Query != "" {
		t.Errorf("the mode survived Enter: %v %q", a.browser, a.Query)
	}
	if testRuns["dismiss"] != 1 {
		t.Errorf("the window was not hidden: %v", testRuns)
	}
}

// A live tab is its own action: Enter focuses the tab rather than opening its
// URL again.
func TestBrowserModeFocusesATab(t *testing.T) {
	a := testApp()
	tabs := []browser.Tab{{BrowserID: "chrome", Index: 1, Title: "Playground", URL: "https://play.example"}}
	a.Actions.SearchBrowser = func(query string, done func(BrowserResults)) {
		done(BrowserResults{Found: true, Tabs: tabs})
	}
	focused, opened := "", ""
	a.Actions.ActivateTab = func(tab browser.Tab) { focused = tab.URL }
	a.Actions.OpenURL = func(browserID, url string) { opened = url }

	tt := render(t, a)
	tt.Type("browser")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !tt.HasText("Playground") {
		t.Fatalf("the tab did not show: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if focused != "https://play.example" || opened != "" {
		t.Errorf("focused %q, opened %q", focused, opened)
	}
}

// The browser mode with no profile at all says so rather than claiming the
// query matched nothing.
func TestBrowserModeWithoutAProfile(t *testing.T) {
	a := testApp()
	a.Actions.SearchBrowser = func(query string, done func(BrowserResults)) {
		done(BrowserResults{})
	}
	tt := render(t, a)
	tt.Type("browser")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !tt.HasText("No browser profile was found") {
		t.Errorf("the empty state = %q", tt.Texts())
	}
}

func TestBrowserModeWithoutASearch(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Type("browser")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.browser {
		t.Fatal("the mode did not open")
	}
	// No action: the list is empty, and the hint explains the mode.
	if !tt.HasText("Enter runs it") {
		t.Errorf("the hint did not show: %q", tt.Texts())
	}
	// Escape leaves it.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.browser {
		t.Error("Escape did not leave the browser mode")
	}
}

func TestEmptyQueryOffersRecentApplications(t *testing.T) {
	a := testApp()
	found := []apps.App{
		{Name: "Safari", Path: "/Applications/Safari.app"},
		{Name: "Terminal", Path: "/System/Applications/Utilities/Terminal.app"},
		{Name: "Editor", Path: "/Applications/Editor.app"},
	}
	a.SetApps(found)
	a.SetRecent([]string{"/Applications/Editor.app", "/Applications/Gone.app", "/Applications/Safari.app"}, true)

	tt := render(t, a)
	// The built-in commands come first, then the recents, most-used first,
	// with the one that is gone left out.
	texts := tt.Texts()
	position := map[string]int{}
	for i, text := range texts {
		if _, seen := position[text]; !seen {
			position[text] = i
		}
	}
	if !tt.HasText("Editor") || !tt.HasText("Safari") {
		t.Fatalf("the recents did not show: %q", texts)
	}
	if tt.HasText("Terminal") {
		t.Errorf("an unlaunched application showed: %q", texts)
	}
	if position["Open settings"] > position["Editor"] {
		t.Errorf("the commands do not come first: %q", texts)
	}
	if position["Editor"] > position["Safari"] {
		t.Errorf("the recents are not in usage order: %q", texts)
	}

	// With the setting off, the empty query shows the commands alone.
	a.SetRecent([]string{"/Applications/Editor.app"}, false)
	tt.Frame()
	if tt.HasText("Editor") {
		t.Errorf("a recent showed with show_recent off: %q", tt.Texts())
	}
}

func TestSystemCommandsJoinTheSearchWhenEnabled(t *testing.T) {
	a := testApp()
	tools := []apps.App{
		{Name: "ripgrep", Path: "/usr/local/bin/ripgrep"},
		{Name: "tmux", Path: "/opt/homebrew/bin/tmux"},
	}
	a.SetTools(tools, true, nil)

	// An empty query stays the commands and the recents.
	tt := render(t, a)
	if tt.HasText("ripgrep") {
		t.Errorf("a tool showed for an empty query: %q", tt.Texts())
	}

	// A typed query matches them, and the row runs it in the terminal.
	tt.Type("ripgrep")
	tt.Frame()
	if !tt.HasText("ripgrep") {
		t.Fatalf("the tool did not show: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if testRuns["tool"] != 1 {
		t.Errorf("running the tool recorded %v", testRuns)
	}

	// With the setting off, the search does not offer them.
	a.SetTools(tools, false, nil)
	a.Query = "ripgrep"
	tt.Frame()
	if tt.HasText("ripgrep") {
		t.Errorf("a tool showed with the setting off: %q", tt.Texts())
	}
}

// An alias finds a PATH command, and an exact alias ranks as high as an exact
// name.
func TestToolAliasesJoinTheSearch(t *testing.T) {
	a := testApp()
	tools := []apps.App{
		{Name: "git", Path: "/usr/bin/git"},
		{Name: "ripgrep", Path: "/usr/local/bin/ripgrep"},
	}
	a.SetTools(tools, true, settings.CommandAliases{"git": "gfm", "ripgrep": "rg"})

	tt := render(t, a)
	tt.Type("gfm")
	tt.Frame()
	if !tt.HasText("git") {
		t.Fatalf("the alias did not find the command: %q", tt.Texts())
	}
	if tt.HasText("ripgrep") {
		t.Errorf("an unrelated command showed: %q", tt.Texts())
	}

	// The alias is matched, not shown: the row still reads as the command.
	if tt.HasText("gfm") {
		t.Errorf("the alias is drawn as a row: %q", tt.Texts())
	}

	// A prefix of the alias finds it too, and the command's own name keeps
	// working.
	for _, query := range []string{"gf", "git"} {
		a.Query = query
		tt.Frame()
		if !tt.HasText("git") {
			t.Errorf("query %q did not find the command: %q", query, tt.Texts())
		}
	}
}

// calculatorFixture is a history the calculator mode can be driven against.
type calculatorFixture struct {
	entries  []calculator.Entry
	added    [][2]string
	toggled  []string
	deleted  []string
	failures bool
}

func (f *calculatorFixture) Entries() []calculator.Entry { return f.entries }

func (f *calculatorFixture) Add(expression, result string) (calculator.Entry, error) {
	if f.failures {
		return calculator.Entry{}, errors.New("write failed")
	}
	f.added = append(f.added, [2]string{expression, result})
	entry := calculator.Entry{ID: "new", Expression: expression, Result: result, CreatedAt: 3}
	f.entries = append([]calculator.Entry{entry}, f.entries...)
	return entry, nil
}

func (f *calculatorFixture) ToggleFavorite(id string) (bool, error) {
	if f.failures {
		return false, errors.New("write failed")
	}
	f.toggled = append(f.toggled, id)
	for i := range f.entries {
		if f.entries[i].ID == id {
			f.entries[i].Favorite = !f.entries[i].Favorite
			return f.entries[i].Favorite, nil
		}
	}
	return false, calculator.ErrNoEntry
}

func (f *calculatorFixture) Delete(id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func TestCalculatorModeEvaluatesRecordsAndCopies(t *testing.T) {
	a := testApp()
	fixture := &calculatorFixture{entries: []calculator.Entry{
		{ID: "1", Expression: "10*3", Result: "30", CreatedAt: 2},
		{ID: "2", Expression: "sqrt(16)", Result: "4", CreatedAt: 1, Favorite: true},
	}}
	a.Calculator = fixture
	a.CopyMode = calculator.CopyResult
	copied := ""
	a.Actions.Copy = func(text string) { copied = text; testRuns["copy"]++ }

	tt := render(t, a)
	tt.Type("calculator")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.calculatorMode || a.Query != "calc " {
		t.Fatalf("the mode did not open: %v %q", a.calculatorMode, a.Query)
	}
	if !tt.HasText("10*3") || !tt.HasText("sqrt(16)") {
		t.Fatalf("the history did not show: %q", tt.Texts())
	}

	// A fresh expression offers its answer first; Enter evaluates it, records
	// it, copies per the copy mode and leaves the mode.
	tt.Type("0.1+0.2")
	tt.Frame()
	if !tt.HasText("0.3") {
		t.Fatalf("the answer row is missing: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if len(fixture.added) != 1 || fixture.added[0] != [2]string{"0.1+0.2", "0.3"} {
		t.Errorf("recorded %v", fixture.added)
	}
	if copied != "0.3" {
		t.Errorf("copied %q (result mode)", copied)
	}
	if a.calculatorMode || a.Query != "" {
		t.Errorf("the mode survived Enter: %v %q", a.calculatorMode, a.Query)
	}

	// A history row copies the whole line in full mode.
	a.CopyMode = calculator.CopyFull
	a.EnterCalculator()
	a.Query = "calc 10*3"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if copied != "10*3 = 30" {
		t.Errorf("copied %q (full mode)", copied)
	}
}

func TestCalculatorModeFiltersAndFavorites(t *testing.T) {
	a := testApp()
	fixture := &calculatorFixture{entries: []calculator.Entry{
		{ID: "1", Expression: "10*3", Result: "30", CreatedAt: 2},
		{ID: "2", Expression: "sqrt(16)", Result: "4", CreatedAt: 1, Favorite: true},
	}}
	a.Calculator = fixture
	a.Actions.Copy = func(string) { testRuns["copy"]++ }

	tt := render(t, a)
	a.EnterCalculator()
	tt.Frame()
	if a.calculatorFilter != calculatorFilterAll {
		t.Fatalf("the shipped filter is %q", a.calculatorFilter)
	}

	// Tab cycles to the starred rows and back.
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.calculatorFilter != calculatorFilterFavorites {
		t.Fatalf("Tab left the filter on %q", a.calculatorFilter)
	}
	if tt.HasText("10*3") {
		t.Errorf("an unstarred row showed under the favourites filter: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.calculatorFilter != calculatorFilterAll {
		t.Errorf("Tab did not cycle back: %q", a.calculatorFilter)
	}

	// The favourite key stars the selected row; the answer row is not one, so
	// the selection moves to the history first.
	a.Query = "calc 10*3"
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(ui.Super|ui.Ctrl, ui.KeyD)
	tt.Frame()
	if len(fixture.toggled) != 1 || fixture.toggled[0] != "1" {
		t.Errorf("toggled %v", fixture.toggled)
	}
	if !tt.HasText("★") {
		t.Errorf("the star is not shown: %q", tt.Texts())
	}

	// Escape leaves the mode.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.calculatorMode {
		t.Error("Escape did not leave the calculator mode")
	}
}

// Every trigger word the old build accepted enters the mode, and the field
// leaving it exits.
func TestCalculatorModeWords(t *testing.T) {
	a := testApp()
	a.Calculator = &calculatorFixture{}
	for _, word := range []string{"calc", "calculator", "计算器", "="} {
		a.ResetQuery()
		a.Query = word + " 2+2"
		a.syncTypedMode()
		if !a.calculatorMode {
			t.Errorf("%q did not enter the mode", word)
			continue
		}
		if got := a.calculatorQuery(); got != "2+2" {
			t.Errorf("%q kept needle %q", word, got)
		}
		a.Query = "2+2"
		a.syncCommandMode()
		if a.calculatorMode {
			t.Errorf("%q did not leave the mode when the word went", word)
		}
	}
}

// The clipboard mode's own keys and controls: the star key, and the delete
// control the selected row carries.
func TestClipboardModeFavoritesAndDelete(t *testing.T) {
	a, store := clipboardApp(t)
	a.Actions.Copy = func(string) { testRuns["copy"]++ }
	tt := render(t, a)

	a.EnterClipboard()
	tt.Frame()
	entries := store.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}

	// The star key toggles the selected entry, and the row shows it.
	tt.Key(ui.Super|ui.Ctrl, ui.KeyD)
	tt.Frame()
	if !store.Entries()[a.Selected].Favorite {
		t.Fatalf("the star did not land: %+v", store.Entries())
	}
	if !tt.HasText("★") {
		t.Errorf("the star is not shown: %q", tt.Texts())
	}

	// The selected row carries the delete control; pressing it removes the
	// row and leaves the mode open.
	if !tt.HasText("Delete") {
		t.Fatalf("the delete control is missing: %q", tt.Texts())
	}
	if err := tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := len(store.Entries()); got != 1 {
		t.Errorf("the delete did not land: %d entries", got)
	}
	if !a.clipboard {
		t.Error("deleting a row left the mode")
	}
}

// The calculator mode's rows carry the same delete control.
func TestCalculatorDeleteControl(t *testing.T) {
	a := testApp()
	fixture := &calculatorFixture{entries: []calculator.Entry{
		{ID: "1", Expression: "10*3", Result: "30", CreatedAt: 2},
	}}
	a.Calculator = fixture
	a.Actions.Copy = func(string) { testRuns["copy"]++ }
	tt := render(t, a)
	a.EnterCalculator()
	a.Query = "calc "
	tt.Frame()
	if err := tt.Click("Delete"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(fixture.deleted) != 1 || fixture.deleted[0] != "1" {
		t.Errorf("deleted %v", fixture.deleted)
	}
	if !tt.HasText("Calculation deleted") {
		t.Errorf("the feedback line is missing: %q", tt.Texts())
	}
}

// A command whose manifest sends its output to the background runs headless
// and shows its output in the launcher, where Enter copies it and Escape
// closes it.
func TestCapturedCommandOutputView(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Args:          []string{"-c", "echo hi"},
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	captured := 0
	a.Actions.RunCommandCaptured = func(got extensions.CommandEntry, args []string, done func(extensions.CapturedRun, error)) {
		captured++
		done(extensions.CapturedRun{
			Command: []string{"/bin/sh", "-c", "echo hi"},
			Stdout:  "hi\n",
			Success: true,
		}, nil)
	}
	copied := ""
	a.Actions.Copy = func(text string) { copied = text }

	tt := render(t, a)
	a.Query = "list"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if captured != 1 {
		t.Fatalf("the command did not run: %d", captured)
	}
	if !a.InOutputView() {
		t.Fatal("the output view did not open")
	}
	if !tt.HasText("hi") {
		t.Errorf("the output is missing: %q", tt.Texts())
	}
	if !tt.HasText("Finished") {
		t.Errorf("the status line is missing: %q", tt.Texts())
	}

	// Enter copies the text and keeps the view open.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if copied != "hi" {
		t.Errorf("copied %q", copied)
	}
	if !a.InOutputView() {
		t.Error("Enter closed the view")
	}

	// Escape closes it and returns to the search.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.InOutputView() || a.Query != "" {
		t.Errorf("Escape left the view open: %v %q", a.InOutputView(), a.Query)
	}
}

// A command that only complains still shows something, and a failed one says
// its exit code.
func TestCapturedCommandFailure(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "fail", Name: "Fail"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{
			Command:  []string{"tool"},
			Stderr:   "boom\n",
			ExitCode: 2,
		}, nil)
	}
	tt := render(t, a)
	a.Query = "fail"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !tt.HasText("boom") {
		t.Errorf("standard error is missing: %q", tt.Texts())
	}
	if !tt.HasText("Exit code 2") {
		t.Errorf("the exit code is missing: %q", tt.Texts())
	}
}

// A command that prints the list protocol is drawn as a list: its rows are
// walkable, Enter runs the chosen row's action, and a status row is not a
// door.
func TestCapturedCommandListProtocol(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "search", Name: "Search"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{
			Command: []string{"tool", "search"},
			Success: true,
			Stdout: `[
			  {"id":"a","title":"First result","subtitle":"context","icon":"star",
			   "action":{"type":"open","url":"https://example.com"}},
			  {"id":"b","title":"Copy me","action":{"type":"copy","text":"copied text"}},
			  {"id":"c","title":"Insert me","action":{"type":"insert","text":"typed"}},
			  {"id":"d","title":"No matches","kind":"status"}
			]`,
		}, nil)
	}
	opened := ""
	copied := ""
	a.Actions.OpenURL = func(browserID, url string) { opened = url }
	a.Actions.Copy = func(text string) { copied = text }

	tt := render(t, a)
	a.Query = "search"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.InOutputView() {
		t.Fatal("the output view did not open")
	}
	for _, want := range []string{"First result", "Copy me", "Insert me", "No matches"} {
		if !tt.HasText(want) {
			t.Fatalf("row %q is missing: %q", want, tt.Texts())
		}
	}
	if !tt.HasText("\u2191\u2193 choose") {
		t.Errorf("the list hint is missing: %q", tt.Texts())
	}

	// The first runnable row is chosen; Enter opens it and closes the view.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if opened != "https://example.com" {
		t.Errorf("opened %q", opened)
	}
	if a.InOutputView() {
		t.Error("the view survived running a row")
	}

	// Down moves to the copy row; Enter copies it and leaves the view open.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	a.Query = "search"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if copied != "copied text" {
		t.Errorf("copied %q", copied)
	}

	// The insert row puts its text back in the field and closes the view.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	a.Query = "search"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if a.Query != "typed" || a.InOutputView() {
		t.Errorf("insert left %q, view %v", a.Query, a.InOutputView())
	}
}

// Output that is not the list protocol stays text, even when it looks like
// JSON.
func TestCapturedCommandNonListStaysText(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "dump", Name: "Dump"},
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{
			Command: []string{"tool"},
			Success: true,
			Stdout:  `{"not":"a list"}` + "\n",
		}, nil)
	}
	tt := render(t, a)
	a.Query = "dump"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !tt.HasText(`{"not":"a list"}`) {
		t.Errorf("the text is missing: %q", tt.Texts())
	}
	if !tt.HasText("Enter copies") {
		t.Errorf("the text hint is missing: %q", tt.Texts())
	}
}

// A row with an application icon draws it, and the icon is decoded once.
func TestApplicationIconIsDrawn(t *testing.T) {
	root := t.TempDir()
	bundle := filepath.Join(root, "Iconed.app")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "Resources"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(bundle, "Contents", "Info.plist"), `<?xml version="1.0"?>
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>Iconed</string>
  <key>CFBundleIconFile</key><string>AppIcon</string>
</dict></plist>`)
	// A real PNG inside a real .icns container.
	var image bytes.Buffer
	if err := png.Encode(&image, imageNewRGBA(32)); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(bundle, "Contents", "Resources", "AppIcon.icns"), string(icnsFile(t, image.Bytes())))

	found := apps.ScanDarwin(root)
	if len(found) != 1 {
		t.Fatalf("apps = %+v", found)
	}
	a := testApp()
	a.SetApps(found)
	tt := render(t, a)
	tt.Type("iconed")
	tt.Frame()
	if !tt.HasText("Iconed") {
		t.Fatalf("the row did not show: %q", tt.Texts())
	}
	// The bitmap is cached after the first draw.
	if a.appIcon(found[0]) == nil {
		t.Error("the icon was not decoded")
	}
	if len(a.icons) != 1 {
		t.Errorf("icons cache = %v", a.icons)
	}
}

// writeFixture writes a file under a fixture directory.
func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// icnsFile wraps a PNG payload in an .icns container.
func icnsFile(t *testing.T, pngData []byte) []byte {
	t.Helper()
	var body bytes.Buffer
	body.WriteString("ic08")
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(pngData)+8))
	body.Write(length)
	body.Write(pngData)
	out := bytes.NewBufferString("icns")
	total := make([]byte, 4)
	binary.BigEndian.PutUint32(total, uint32(body.Len()+8))
	out.Write(total)
	out.Write(body.Bytes())
	return out.Bytes()
}

// imageNewRGBA is a plain square image for the fixture.
func imageNewRGBA(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 200, A: 255})
		}
	}
	return img
}

// The first ten rows that can be run answer to the app modifier plus their
// number, and the badge shows it.
func TestResultShortcutsRunTheNumberedRow(t *testing.T) {
	a := testApp()
	found := make([]apps.App, 0, 12)
	for i := 0; i < 12; i++ {
		found = append(found, apps.App{Name: fmt.Sprintf("App %02d", i), Path: fmt.Sprintf("/Applications/App %02d.app", i)})
	}
	a.SetApps(found)
	ran := []string{}
	a.Actions.OpenApp = func(app apps.App) { ran = append(ran, app.Name) }

	tt := render(t, a)
	a.Query = "app"
	tt.Frame()
	// The numbers are assigned to the rows in view, 1..9 then 0.
	if got := a.numbers[0]; got != 1 {
		t.Errorf("the first row's number = %d", got)
	}
	if got := a.numbers[9]; got != 0 {
		t.Errorf("the tenth row's number = %d", got)
	}
	if _, ok := a.numbers[10]; ok {
		t.Errorf("an eleventh row was numbered: %v", a.numbers)
	}
	if !tt.HasText("1") {
		t.Errorf("the badge is missing: %q", tt.Texts())
	}

	// The digit keys run the numbered rows; the modifiers are the settings'
	// select_result binding (Cmd on macOS, Ctrl elsewhere).
	mods, _, ok := shortcuts.Parse(settings.SelectResultDigit(a.settings(), 2))
	if !ok {
		t.Fatal("the second result's key did not parse")
	}
	tt.Key(mods, ui.Key2)
	tt.Frame()
	if len(ran) != 1 || ran[0] != "App 01" {
		t.Fatalf("ran %v", ran)
	}
	mods, _, ok = shortcuts.Parse(settings.SelectResultDigit(a.settings(), 0))
	if !ok {
		t.Fatal("the tenth result's key did not parse")
	}
	tt.Key(mods, ui.Key0)
	tt.Frame()
	if len(ran) != 2 || ran[1] != "App 09" {
		t.Errorf("ran %v", ran)
	}

	// Rebinding the family moves the numbers with it.
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutSelectResult, "Alt+1")
	}); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(ui.Alt, ui.Key3)
	tt.Frame()
	if len(ran) != 3 || ran[2] != "App 02" {
		t.Errorf("after rebinding: ran %v", ran)
	}
}

// A drop offers the three safe actions per file, and none of them runs the
// file itself.
func TestFilesModeOffersTheDropActions(t *testing.T) {
	a := testApp()
	opened, cdTo, copied := "", "", ""
	a.Actions.OpenPath = func(path string) { opened = path }
	a.Actions.OpenInTerminal = func(dir string) { cdTo = dir }
	a.Actions.Copy = func(text string) { copied = text }
	a.SetDropped([]drops.File{
		{Path: "/tmp/report.pdf", Name: "report.pdf", Directory: "/tmp"},
		{Path: "/tmp/projects", Name: "projects", Directory: "/tmp", IsDirectory: true},
	})

	tt := render(t, a)
	tt.Type("files ")
	tt.Frame()
	if !a.InFilesMode() {
		t.Fatal("the files mode did not open")
	}
	for _, want := range []string{"Open  report.pdf", "Open a terminal in  report.pdf", "Copy the path of  report.pdf"} {
		if !tt.HasText(want) {
			t.Fatalf("row %q is missing: %q", want, tt.Texts())
		}
	}
	// A folder says so, and its cd enters it.
	if !tt.HasText("Folder") {
		t.Errorf("the folder mark is missing: %q", tt.Texts())
	}

	// Open hands the path to the system, and leaves the mode.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if opened != "/tmp/report.pdf" || a.InFilesMode() {
		t.Errorf("open: %q, mode %v", opened, a.InFilesMode())
	}

	// A file's cd stands next to it; a folder's enters it.
	a.EnterFiles()
	a.Query = "files report"
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if cdTo != "/tmp" {
		t.Errorf("cd = %q", cdTo)
	}
	a.EnterFiles()
	a.Query = "files projects"
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if cdTo != "/tmp/projects" {
		t.Errorf("a folder's cd = %q", cdTo)
	}

	// Copy puts the absolute path on the clipboard.
	a.EnterFiles()
	a.Query = "files report"
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if copied != "/tmp/report.pdf" {
		t.Errorf("copied %q", copied)
	}
}

// A drop of nothing the app can read leaves the launcher alone, and the mode
// leaves with its word.
func TestFilesModeLeavesWithItsWord(t *testing.T) {
	a := testApp()
	a.SetDropped([]drops.File{{Path: "/tmp/a.txt", Name: "a.txt", Directory: "/tmp"}})
	tt := render(t, a)
	tt.Type("files ")
	tt.Frame()
	if !a.InFilesMode() {
		t.Fatal("the mode did not open")
	}
	a.Query = "a.txt"
	tt.Frame()
	if a.InFilesMode() {
		t.Error("the mode survived losing its word")
	}
}

// A tool the catalog knows, which this machine does not have, is offered as an
// install row whose Enter copies the command — never runs it.
func TestToolInstallRows(t *testing.T) {
	a := testApp()
	states := tools.Look([]string{t.TempDir()}, tools.Platform())
	a.SetToolCatalog(states)
	copied := ""
	ran := 0
	a.Actions.Copy = func(text string) { copied = text }
	a.Actions.RunInTerminal = func([]string) { ran++ }

	tt := render(t, a)
	tt.Type("ripgrep")
	tt.Frame()
	if !tt.HasText("Install ripgrep") {
		t.Fatalf("the install row is missing: %q", tt.Texts())
	}
	// The row shows the command it would copy, and the manager it is for.
	if !tt.HasText("cargo install ripgrep") && !tt.HasText("sudo apt install ripgrep") &&
		!tt.HasText("brew install ripgrep") && !tt.HasText("sudo pacman -S ripgrep") {
		t.Errorf("the command is missing: %q", tt.Texts())
	}
	// A Chinese keyword finds the tool too.
	a.Query = "搜索"
	tt.Frame()
	if !tt.HasText("Install ripgrep") {
		t.Errorf("a keyword did not find the tool: %q", tt.Texts())
	}

	// Enter copies the command; nothing runs.
	a.Query = "ripgrep"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if copied == "" || !strings.Contains(copied, "ripgrep") {
		t.Errorf("copied %q", copied)
	}
	if ran != 0 {
		t.Errorf("an install ran %d times", ran)
	}

	// An installed tool gets no install row: the PATH command scan covers it.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rg"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.SetToolCatalog(tools.Look([]string{dir}, tools.Platform()))
	a.Query = "ripgrep"
	tt.Frame()
	if tt.HasText("Install ripgrep") {
		t.Errorf("an installed tool was offered for install: %q", tt.Texts())
	}
	// An empty query offers no install rows.
	a.Query = ""
	tt.Frame()
	if tt.HasText("Install ") {
		t.Errorf("an empty query offered installs: %q", tt.Texts())
	}
}

// The power words claim the query, and the action goes to the shell.
func TestPowerRows(t *testing.T) {
	a := testApp()
	asked := []string{}
	a.Actions.Power = func(action string) { asked = append(asked, action) }
	a.SetApps([]apps.App{{Name: "Restart Helper", Path: "/Applications/Restart Helper.app"}})

	tt := render(t, a)
	tt.Type("restart")
	tt.Frame()
	if !tt.HasText("Restart the computer") {
		t.Fatalf("the restart row is missing: %q", tt.Texts())
	}
	// The word claims the query: the application whose name contains it does
	// not take the first row.
	if texts := tt.Texts(); len(texts) > 0 && texts[0] == "Restart Helper" {
		t.Errorf("the application outranked the power row: %q", texts)
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if len(asked) != 1 || asked[0] != "restart" {
		t.Fatalf("asked %v", asked)
	}

	// The shutdown words, including the Chinese one.
	for _, query := range []string{"shutdown", "power off", "关机"} {
		a.ResetQuery()
		a.Query = query
		tt.Frame()
		if !tt.HasText("Shut down the computer") {
			t.Errorf("query %q did not offer shutting down: %q", query, tt.Texts())
		}
	}
	// A query that merely contains the word does not claim it.
	a.ResetQuery()
	a.Query = "restart helper"
	tt.Frame()
	if tt.HasText("Restart the computer") {
		t.Errorf("a longer query claimed the power row: %q", tt.Texts())
	}
}

// An image clipboard entry's row carries a thumbnail, decoded once.
func TestClipboardImageThumbnail(t *testing.T) {
	store := clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), 0)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageNewRGBA(24)); err != nil {
		t.Fatal(err)
	}
	entry, _, err := store.AddImage(buffer.Bytes(), 24, 24)
	if err != nil {
		t.Fatal(err)
	}
	if entry.Kind != clipboard.KindImage {
		t.Fatalf("entry = %+v", entry)
	}

	a := testApp()
	a.Clipboard = store
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()
	if !tt.HasText("[image]") {
		t.Fatalf("the image row is missing: %q", tt.Texts())
	}
	// The thumbnail is decoded and cached by its file's path.
	thumb := a.clipThumb(entry)
	if thumb == nil {
		t.Fatal("the thumbnail was not decoded")
	}
	if len(a.thumbs) != 1 {
		t.Errorf("thumbs cache = %v", a.thumbs)
	}
	if again := a.clipThumb(entry); again != thumb {
		t.Error("the thumbnail was decoded twice")
	}

	// A text entry has none.
	text, _, err := store.AddText("plain")
	if err != nil {
		t.Fatal(err)
	}
	if got := a.clipThumb(text); got != nil {
		t.Errorf("a text entry produced a thumbnail: %v", got)
	}
}
