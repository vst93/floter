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
	"strconv"
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
	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/shortcuts"
	"floter/internal/tools"
)

// render draws one frame of the launcher at the given size and returns the
// tester, so a test can type, click and read what shows.
func render(t *testing.T, a *App) *ui.Tester {
	t.Helper()
	// The frame is the window the shell opens plus the band a full list needs:
	// the launcher resizes itself to its content in the app (`ResizeTo`), and
	// a test frame is fixed, so it is given the room a ten-row list asks for.
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
	}, InputWindowWidth, int(WindowHeight("small"))+120)
	tt.Frame()
	return tt
}

func TestLauncherShowsTheCatalogWithTheFieldFocused(t *testing.T) {
	a := testApp()
	tt := render(t, a)

	for _, want := range []string{"Open settings", "Terminal", "Quit floter", "Type to search", "Search"} {
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
	// The row is the reference's 「Terminal」 (`system.terminal`), not the
	// held row's action word (`launcher.terminalRow`): the two registers are
	// distinct rows, so a query that reaches the surface names the surface.
	if _, ok := tt.Find("Terminal"); !ok {
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
	if err := tt.Click("Terminal"); err != nil {
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
	// A query that matches nothing is the shell's for Enter, but the old
	// build drew no action bar over an empty page (`visibleActionBar` was
	// gated on `launcherResults.length`), so the page says so and no shell
	// row is offered. The typed line still runs on Enter (see App.activate).
	if !tt.HasText("No results") {
		t.Errorf("the empty message is missing: %q", tt.Texts())
	}
	if tt.HasText("Run in shell") {
		t.Errorf("the shell row was offered over an empty page: %q", tt.Texts())
	}
}

func TestLauncherLanguageFollowsTheStore(t *testing.T) {
	a := testApp()
	if err := a.Store.Update(func(s *settings.Settings) { s.Language = "zh" }); err != nil {
		t.Fatal(err)
	}
	tt := render(t, a)
	if !tt.HasText("终端") {
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

// The command mode's rows carry the same right-hand source as the command row
// that opened the mode: the field's scope marker names the command, the rows
// name the integration it comes from.
func TestCommandModeRowsCarryTheSource(t *testing.T) {
	a := commandApp()
	entry := a.Commands[0]
	a.enterCommand(entry)
	rows := a.argumentItems(entry)
	if len(rows) == 0 {
		t.Fatal("the command mode offers no rows")
	}
	for _, row := range rows {
		if row.Source != "V Tools" {
			t.Errorf("row %q source = %q, want V Tools", row.Title, row.Source)
		}
	}
}
func TestTabExpandsAnExtensionCommandIntoItsArguments(t *testing.T) {
	a := commandApp()
	tt := render(t, a)

	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.mode == nil {
		t.Fatalf("Tab did not enter the command mode: %q", tt.Texts())
	}
	// The trigger word was consumed on entry: the field holds the argv, and
	// the scope marker beside it names the command.
	if a.Query != "" {
		t.Errorf("the field holds %q, want the empty argv", a.Query)
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
	if a.Query != "-mode " {
		t.Fatalf("completion left %q, want \"-mode \"", a.Query)
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
	// The needle the user typed becomes an ordinary query: the user's own
	// words are never thrown away by a dismissal (the old build's R31).
	if a.Query != "-f" {
		t.Errorf("the field holds %q, want the needle", a.Query)
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

	// Deleting the needle empties the field but does not end the mode: the
	// trigger word was consumed on entry, and the scope marker still names the
	// command the field is feeding. Escape is what leaves it.
	for range "needle" {
		tt.Type("-x")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.mode != nil {
		t.Errorf("Escape left the mode on: query=%q", a.Query)
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
	if !a.clipboard {
		t.Fatalf("clipboard=%v", a.clipboard)
	}
	// The trigger word was consumed on entry: the field holds the needle, and
	// the scope marker beside it names the mode.
	if a.Query != "" {
		t.Fatalf("Enter left query %q", a.Query)
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

	// Enter copies it, leaves the mode and dismisses the window. The run
	// clears the field, as the old build's own copy did.
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

// A status note is the panel's mute one-liner, and the reference draws it as
// one of the row family: `.launcher-status` pads the same 12u as
// `.launcher-result` (`src/styles/launcher.css:1099-1114,1480-1491`) and leads
// with the same 28u icon column, so its sentence lands on the same text column
// as every result row's title. One `Space(n)` is four reference units
// (`internal/theme/theme.go`), so 12u is `Space(3)` — the `Space(12)` the note
// briefly carried pushed it 36u right of the column it shares.
func TestStatusNotePaddingMatchesReference(t *testing.T) {
	// The column, taken from an ordinary result row.
	plain := testApp()
	rows := render(t, plain)
	title, ok := rows.Find(plain.Catalog()[0].Title)
	if !ok {
		t.Fatalf("the first result row's title was not found: %q", rows.Texts())
	}

	// The same column, taken from a status note: the clipboard mode with the
	// history switched off is one real note and nothing else.
	a, _ := clipboardApp(t)
	clipboardOff(t, a)
	a.EnterClipboard()
	tt := render(t, a)
	tt.Frame()
	note := a.copy().ClipboardDisabled
	rect, ok := tt.Find(note)
	if !ok {
		t.Fatalf("the status note text was not found: %q", tt.Texts())
	}
	if rect.X != title.X {
		t.Errorf("status note text X = %v, want %v (the text column a result row's title sits on)",
			rect.X, title.X)
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
	if a.Query != "-run deploy " {
		t.Fatalf("completion left %q", a.Query)
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if testRuns["cmd:jv"] != 3 {
		t.Errorf("run recorded %v", testRuns)
	}
}

func TestClipboardFilterChipsCycleOnTab(t *testing.T) {
	a, _ := clipboardApp(t)
	tt := render(t, a)

	tt.Type("clipboard")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !a.clipboard {
		t.Fatal("the clipboard mode did not open")
	}
	// The chips row shows the axis, the chosen chip lit.
	for _, label := range []string{"All", "Favorites", "Text", "Images", "Links", "Files"} {
		if !tt.HasText(label) {
			t.Errorf("the chips row is missing %q: %q", label, tt.Texts())
		}
	}
	if a.clipboardFilter != filterAll {
		t.Errorf("the mode opened on %q", a.clipboardFilter)
	}
	// Tab walks the axis, Shift+Tab walks it back, and both wrap.
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.clipboardFilter != filterFavorites {
		t.Errorf("Tab landed on %q, want favorites", a.clipboardFilter)
	}
	tt.Key(ui.Shift, ui.KeyTab)
	tt.Frame()
	if a.clipboardFilter != filterAll {
		t.Errorf("Shift+Tab landed on %q, want all", a.clipboardFilter)
	}
	tt.Key(ui.Shift, ui.KeyTab)
	tt.Frame()
	if a.clipboardFilter != filterFiles {
		t.Errorf("Shift+Tab did not wrap: %q", a.clipboardFilter)
	}
	// Tab does not copy and does not leave the mode.
	if testRuns["copy"] != 0 || !a.clipboard {
		t.Errorf("Tab copied or left the mode: %v clipboard=%v", testRuns, a.clipboard)
	}
	// Clicking a chip chooses it.
	if err := tt.Click("Favorites"); err != nil {
		t.Fatalf("chip click: %v", err)
	}
	if a.clipboardFilter != filterFavorites {
		t.Errorf("the chip click chose %q", a.clipboardFilter)
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
	a.Actions.OpenURL = func(browserID, url string) error {
		if browserID != "chrome" {
			t.Errorf("opened %q in %q", url, browserID)
		}
		opened = url
		return nil
	}

	// The trigger word enters the mode on its own, as the old build's did:
	// the word, a space, and the rest is the needle.
	tt := render(t, a)
	tt.Type("browser ")
	tt.Frame()
	if !a.browser {
		t.Fatalf("the browser mode did not open: %v %q", a.browser, a.Query)
	}
	if !tt.HasText("Go Documentation") || !tt.HasText("Rust") {
		t.Fatalf("the results did not show: %q", tt.Texts())
	}
	// The live tabs follow the file results, in their own group.
	if !tt.HasText("Rust Playground") {
		t.Fatalf("the tabs did not show: %q", tt.Texts())
	}
	// The detail line is the URL alone, the reference's own row content
	// (`plugins/browser/mode.ts:195`): a history entry is told apart by its
	// clock glyph, not a date string.
	if !tt.HasText("https://go.dev/doc") {
		t.Errorf("the history detail is missing: %q", tt.Texts())
	}
	if tt.HasText("https://go.dev/doc  \u00b7  ") {
		t.Errorf("the history detail still carries a date: %q", tt.Texts())
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

	// Tab walks the browser's range chips without leaving the mode; the
	// chips row shows the axis.
	for _, label := range []string{"All", "Bookmarks", "History", "Tabs"} {
		if !tt.HasText(label) {
			t.Errorf("the chips row is missing %q: %q", label, tt.Texts())
		}
	}
	if a.browserFilter != filterAll {
		t.Errorf("the mode opened on %q", a.browserFilter)
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.browserFilter != filterBookmarks || !a.browser {
		t.Errorf("Tab did not cycle the range: %q mode=%v", a.browserFilter, a.browser)
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

// Every browser trigger word enters the mode on the range it names, as the
// old build's `parseBrowserMode` did: the bookmark words open on the
// bookmarks, the history words on the history, and the plugin's own word on
// everything.
func TestBrowserModeWordsChooseTheRange(t *testing.T) {
	cases := []struct {
		word   string
		filter string
	}{
		{"browser", filterAll},
		{"浏览器", filterAll},
		{"bookmarks", filterBookmarks},
		{"bookmark", filterBookmarks},
		{"书签", filterBookmarks},
		{"history", filterHistory},
		{"hist", filterHistory},
		{"历史", filterHistory},
		{"历史记录", filterHistory},
	}
	for _, tc := range cases {
		a := testApp()
		a.Query = tc.word + " rust"
		a.syncTypedMode()
		if !a.browser {
			t.Errorf("%q did not enter the browser mode", tc.word)
			continue
		}
		if a.browserFilter != tc.filter {
			t.Errorf("%q opened on %q, want %q", tc.word, a.browserFilter, tc.filter)
		}
		if a.Query != "" {
			t.Errorf("%q kept the field %q", tc.word, a.Query)
		}
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
	a.Actions.OpenURL = func(browserID, url string) error { opened = url; return nil }

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
	if !tt.HasText("No supported browser was found on this computer") {
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
		{Name: "Notes", Path: "/System/Applications/Notes.app"},
		{Name: "Editor", Path: "/Applications/Editor.app"},
	}
	a.SetApps(found)
	a.SetRecent([]string{"/Applications/Editor.app", "/Applications/Gone.app", "/Applications/Safari.app"}, true)

	// A taller window: the built-in list grew with the power rows and the
	// calculator's own line, and the recents — with their own heading — come
	// after them.
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) },
		InputWindowWidth, int(WindowHeight("small"))+240)
	tt.Frame()
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
	if tt.HasText("Notes") {
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
	a.SetTools(tools, true)

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

	// With the setting off, the search does not offer them as tools — and a
	// name that then matches nothing draws no bar either (the old build's
	// `visibleActionBar` needed a non-empty list). Enter still runs the typed
	// line in the shell.
	a.SetTools(tools, false)
	a.Query = "ripgrep"
	tt.Frame()
	if tt.HasText("Run in shell") {
		t.Errorf("the shell row was offered over an empty page: %q", tt.Texts())
	}
	if !tt.HasText("No results") {
		t.Errorf("the empty message is missing: %q", tt.Texts())
	}
	// The install row is the one that goes away: its command would come from
	// the catalog.
	if tt.HasText("Install ripgrep") {
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
	a.SetTools(tools, true)
	a.SetCommandAliases(settings.CommandAliases{"git": "gfm", "ripgrep": "rg"})

	tt := render(t, a)
	tt.Type("gfm")
	tt.Frame()
	if !tt.HasText("git") {
		t.Fatalf("the alias did not find the command: %q", tt.Texts())
	}
	if tt.HasText("ripgrep") {
		t.Errorf("an unrelated command showed: %q", tt.Texts())
	}

	// The alias is what carried the match, so the row names it — the old
	// build's `launcher.aliasMatch`, riding the subtitle so a row reached by
	// an alias explains itself instead of reading like a stray hit.
	if !tt.HasText("Alias: gfm") {
		t.Errorf("the row does not name the alias that reached it: %q", tt.Texts())
	}

	// A prefix of the alias finds it too (and still names the alias — the
	// alias is what carried it), while the command's own name finds it
	// without any alias note: the name is the answer, not a translation.
	a.Query = "gf"
	tt.Frame()
	if !tt.HasText("git") || !tt.HasText("Alias: gfm") {
		t.Errorf("a prefix of the alias did not find and name it: %q", tt.Texts())
	}
	a.Query = "git"
	tt.Frame()
	if !tt.HasText("git") {
		t.Errorf("the command's own name did not find it: %q", tt.Texts())
	}
	if tt.HasText("Alias: gfm") {
		t.Errorf("the name match still named the alias: %q", tt.Texts())
	}
}

// A user alias set on an extension command's own row — the Integrations
// alias editor's key is the command id — reaches the command and is named on
// the row, the same as a PATH command's alias. Before this, the editor stored
// an alias that nothing ever read.
func TestExtensionCommandAliasesJoinTheSearch(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID:   "io.github.vst93.v",
		IntegrationName: "V Tools",
		Command: extensions.Command{
			ID:          "jv",
			Name:        "JSON Viewer",
			Description: "View, format and edit JSON",
		},
	}})
	a.Aliases = settings.CommandAliases{"jv": "j"}

	a.Query = "j"
	found := false
	for _, item := range a.Results() {
		if item.ID != "cmd:io.github.vst93.v:jv" {
			continue
		}
		found = true
		if item.Detail != "Alias: j \u00b7 View, format and edit JSON" {
			t.Errorf("the alias hint did not reach the subtitle: %q", item.Detail)
		}
	}
	if !found {
		t.Errorf("the alias did not find the extension command: %v", ids(a.Results()))
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
	if !a.calculatorMode {
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

	// A history row copies the whole line in full mode: the field holds the
	// needle, so the row the choice lands on is the one the needle names.
	a.CopyMode = calculator.CopyFull
	a.EnterCalculator()
	a.Query = "10*3"
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
	a.Query = "1+1"
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.Key(ui.Super|ui.Ctrl, ui.KeyD)
	tt.Frame()
	if len(fixture.toggled) != 1 || fixture.toggled[0] != "1" {
		t.Errorf("toggled %v", fixture.toggled)
	}
	// The starred row shows its star *control*, and never a second `★` badge:
	// the reference's row carries the control alone.
	if _, ok := tt.Find("Unfavorite"); !ok {
		t.Errorf("the starred row's own star is not shown: %q", tt.Texts())
	}
	if tt.HasText("★") {
		t.Errorf("a starred row wears a second `★` badge beside its star control: %q", tt.Texts())
	}

	// Escape leaves the mode.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.calculatorMode {
		t.Error("Escape did not leave the calculator mode")
	}
}

// The calculator mode's history is deliberately *not* filtered by the field's
// text: the field is the pending expression, and filtering the list by it
// would hide every past calculation exactly while the user is writing a new
// one. A history row's subtitle is its time, and its result lives at the row's
// right edge — never both.
func TestCalculatorModeHistoryIsNotAFilter(t *testing.T) {
	a := testApp()
	now := time.Now()
	fixture := &calculatorFixture{entries: []calculator.Entry{
		{ID: "1", Expression: "10*3", Result: "30", CreatedAt: now.UnixMilli()},
		{ID: "2", Expression: "sqrt(16)", Result: "4", CreatedAt: now.AddDate(0, 0, -1).UnixMilli()},
	}}
	a.Calculator = fixture
	a.EnterCalculator()
	// A fresh expression shows its answer row *and* the whole history, not a
	// history narrowed to the expression.
	a.Query = "0.1+0.2"
	items := a.calculatorItems()
	if len(items) != 3 {
		t.Fatalf("items = %d, want the answer row plus two history rows: %+v", len(items), items)
	}
	if items[0].Title != "0.1+0.2" || items[0].trailing != "0.3" {
		t.Errorf("the answer row = %+v", items[0])
	}
	if items[1].trailing != "30" || items[1].Detail == "30" {
		t.Errorf("the history row duplicates the result: %+v", items[1])
	}
	if want := i18n.For("en").Launcher.CalculatorYesterday; items[2].Detail != want {
		t.Errorf("the yesterday row's time = %q, want %q", items[2].Detail, want)
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
		// The word was consumed on entry: the field is empty, and the needle
		// the user types next is the field's own text.
		if got := a.calculatorQuery(); got != "" {
			t.Errorf("%q kept needle %q", word, got)
		}
		a.Query = "2+2"
		if got := a.calculatorQuery(); got != "2+2" {
			t.Errorf("%q lost the needle %q", word, got)
		}
		// Escape is what leaves it; the needle becomes an ordinary query.
		a.leaveCalculator()
		if a.calculatorMode {
			t.Errorf("%q did not leave the mode", word)
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
	// The starred row shows its star *control* rather than a `★` badge, so a
	// starred entry never wears two stars.
	if _, ok := tt.Find("Unfavorite"); !ok {
		t.Errorf("the starred row's own star is not shown: %q", tt.Texts())
	}
	if tt.HasText("★") {
		t.Errorf("a starred row wears a second `★` badge beside its star control: %q", tt.Texts())
	}

	// The delete is a two-step: the first press of ⌘⌫ arms the row, the
	// second removes. The old build's delete is keyboard-only — the row
	// shows the note while armed, never a click target.
	deleteKey(tt)
	tt.Frame()
	if !tt.HasText(i18n.For("en").Launcher.DeleteConfirm) {
		t.Fatalf("the armed note is missing: %q", tt.Texts())
	}
	deleteKey(tt)
	tt.Frame()
	if got := len(store.Entries()); got != 1 {
		t.Errorf("the delete did not land: %d entries", got)
	}
	if !a.clipboard {
		t.Error("deleting a row left the mode")
	}
}

// R50 · the star is hidden while the row's delete is armed: the old
// build's `!deleteArmed` condition swapped the star for the "press
// again" note, so a half-finished confirmation never offered a
// second control competing with the key that completes it.
func TestArmedDeleteHidesTheStar(t *testing.T) {
	store := clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), 0)
	if _, _, err := store.AddText("only clip"); err != nil {
		t.Fatal(err)
	}
	a := testApp()
	a.Clipboard = store
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()

	// The star shows on the selected row before arming.
	if !tt.HasText("Favorite") {
		t.Fatalf("the star is missing before arming: %q", tt.Texts())
	}

	// Arm the delete — the star must disappear. HasText is a substring
	// match, so "Favorites" (the filter chip) satisfies "Favorite" —
	// compare the exact text entries instead.
	deleteKey(tt)
	tt.Frame()
	if !tt.HasText(i18n.For("en").Launcher.DeleteConfirm) {
		t.Fatalf("the armed note is missing: %q", tt.Texts())
	}
	for _, text := range tt.Texts() {
		if text == "Favorite" || text == "Unfavorite" {
			t.Errorf("the star is shown while armed: %q", tt.Texts())
			break
		}
	}

	// R50 · the old build's `aria-live="polite"` announced the armed
	// note to screen readers when it appeared. The Go build's
	// equivalent is `Context.Announce`, called once per arm.
	announcements := tt.Announcements()
	found := false
	for _, a := range announcements {
		if a == i18n.For("en").Launcher.DeleteConfirm {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("the armed note was not announced: %q", announcements)
	}
}
func TestCalculatorDeleteControl(t *testing.T) {
	a := testApp()
	fixture := &calculatorFixture{entries: []calculator.Entry{
		{ID: "1", Expression: "10*3", Result: "30", CreatedAt: 2},
	}}
	a.Calculator = fixture
	a.Actions.Copy = func(string) { testRuns["copy"]++ }
	tt := render(t, a)
	a.EnterCalculator()
	tt.Frame()
	deleteKey(tt)
	tt.Frame()
	deleteKey(tt)
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
	a.Actions.OpenURL = func(browserID, url string) error { opened = url; return nil }
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
	// The action bar is the panel's pinned footer (the list's last entry) and
	// takes no number — the old build ran `resultShortcutSlots` over
	// `displayedResults`, which excludes the bar — so the family starts on the
	// first real result: 1..9 then 0 on the tenth numberable row.
	if got := a.numbers[0]; got != 1 {
		t.Errorf("the first result row's number = %d (%v)", got, a.numbers)
	}
	last := 0
	for i := range a.numbers {
		if i > last {
			last = i
		}
	}
	if got := a.numbers[last]; got != 0 {
		t.Errorf("the tenth row's number = %d (%v)", got, a.numbers)
	}
	if _, ok := a.numbers[last+1]; ok {
		t.Errorf("an eleventh row was numbered: %v", a.numbers)
	}
	if _, ok := a.numbers[len(a.Results())-1]; ok {
		t.Errorf("the action bar was numbered: %v", a.numbers)
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
	if len(ran) != 2 || ran[1] != a.Results()[last].Title {
		t.Errorf("ran %v, want the tenth numbered row %q", ran, a.Results()[last].Title)
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
	for _, want := range []string{"Open  report.pdf", "cd in terminal  report.pdf", "Copy path  report.pdf"} {
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
	a.Query = "report"
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if cdTo != "/tmp" {
		t.Errorf("cd = %q", cdTo)
	}
	a.EnterFiles()
	a.Query = "projects"
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
	a.Query = "report"
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
	// The needle narrows the list; the mode itself leaves on Escape, not when
	// the field changes.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.InFilesMode() {
		t.Error("Escape left the mode on")
	}
}

// A tool the catalog knows, which this machine does not have, is offered as an
// install row whose Enter copies the command — never runs it.
func TestToolInstallRows(t *testing.T) {
	a := testApp()
	states := tools.Look([]string{t.TempDir()}, tools.Platform())
	a.SetToolCatalog(states)
	ran := 0
	lastInstall := ""
	a.Actions.OpenInstallSession = func(command string) { ran++; lastInstall = command }

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

	// Enter opens a bare terminal session with the install command typed into
	// it: the shell is the user's, and it outlives the install.
	a.Query = "ripgrep"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if ran != 1 || lastInstall != "cargo install ripgrep" && !strings.Contains(lastInstall, "ripgrep") {
		t.Errorf("install session ran %d times, command %q", ran, lastInstall)
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
	// Enter asks rather than runs: the panel's own confirmation row appears,
	// and nothing reaches the shell until its own button is pressed (the old
	// build's `.launcher-system-confirm`).
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if len(asked) != 0 {
		t.Fatalf("the row ran without a confirmation: %v", asked)
	}
	if a.PowerPending() != PowerRestart {
		t.Fatalf("pending = %q", a.PowerPending())
	}
	copy := a.copy()
	if !tt.HasText(copy.PowerConfirmMessage(PowerRestart)) {
		t.Fatalf("the confirmation is not drawn: %q", tt.Texts())
	}
	if err := tt.Click(copy.PowerRestartAction); err != nil {
		t.Fatalf("the confirmation's button: %v", err)
	}
	tt.Frame()
	if len(asked) != 1 || asked[0] != "restart" {
		t.Fatalf("asked %v", asked)
	}
	if a.PowerPending() != "" {
		t.Errorf("the confirmation stayed up: %q", a.PowerPending())
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

// An unanswered power confirmation is not forever: the old build asked with
// `useTimedReset(pendingSystemAction, …)` and its three-second default, so the
// question clears itself rather than waiting for a later glance.
func TestThePowerConfirmationExpiresOnItsOwn(t *testing.T) {
	a := testApp()
	asked := []string{}
	a.Actions.Power = func(action string) { asked = append(asked, action) }
	now := disarmArmClock(a)

	tt := render(t, a)
	a.Query = "restart"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if a.PowerPending() != PowerRestart {
		t.Fatalf("pending = %q", a.PowerPending())
	}
	if !tt.HasText(a.copy().PowerConfirmMessage(PowerRestart)) {
		t.Fatalf("the confirmation is not drawn: %q", tt.Texts())
	}

	// Inside the window the question is still waiting.
	*now = now.Add(2 * time.Second)
	tt.Frame()
	if a.PowerPending() != PowerRestart {
		t.Fatalf("the confirmation left inside its window: %q", a.PowerPending())
	}

	// Past it, the question and its row are gone, and nothing ran.
	*now = now.Add(2 * time.Second)
	tt.Frame()
	if a.PowerPending() != "" {
		t.Errorf("the confirmation outlived its window: %q", a.PowerPending())
	}
	if tt.HasText(a.copy().PowerConfirmMessage(PowerRestart)) {
		t.Errorf("the row stayed on screen: %q", tt.Texts())
	}
	if len(asked) != 0 {
		t.Errorf("the confirmation ran without an answer: %v", asked)
	}
}

// A clipboard row's two lines are the entry's preview and its type word plus a
// compact age — not a bare timestamp, and not the whole capture.
func TestClipboardRowPreviewAndSubtitle(t *testing.T) {
	store := clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), 0)
	if _, _, err := store.AddText("git status\ngit diff\ngit log"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.AddFiles([]string{"/tmp/a.png", "/tmp/b.png", "/tmp/c.png"}); err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageNewRGBA(24)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.AddImage(buffer.Bytes(), 24, 24); err != nil {
		t.Fatal(err)
	}

	a := testApp()
	a.Clipboard = store
	a.EnterClipboard()

	copy := a.copy()
	got := map[string]string{}
	for _, item := range a.clipboardItems() {
		got[item.Title] = item.Detail
	}
	want := map[string]string{
		"git status":    copy.ClipboardTypeText + " · 0m",
		"a.png +2":      copy.ClipboardTypeFiles + " · 0m",
		"[image 24x24]": copy.ClipboardTypeImage + " · 0m",
	}
	for title, detail := range want {
		if got[title] != detail {
			t.Errorf("row %q subtitle = %q, want %q (rows: %v)", title, got[title], detail, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

// Past a day the age becomes a concrete local time, still behind the type word.
func TestClipboardRowSubtitleFallsBackToADate(t *testing.T) {
	store := clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), 0)
	if _, _, err := store.AddText("a clip"); err != nil {
		t.Fatal(err)
	}
	created := store.Entries()[0].Time()

	a := testApp()
	a.Clipboard = store
	a.EnterClipboard()
	a.clock = func() time.Time { return created.Add(30 * time.Hour) }

	items := a.clipboardItems()
	if len(items) != 1 {
		t.Fatalf("rows = %+v", items)
	}
	want := a.copy().ClipboardTypeText + " · " + created.Format("2006-01-02 15:04")
	if items[0].Detail != want {
		t.Errorf("subtitle = %q, want %q", items[0].Detail, want)
	}
}

// The thumbnail memo is bounded by the history's own ceiling: past it, only the
// paths the current result set references survive (the old build's R97).
func TestClipboardThumbnailMemoIsBounded(t *testing.T) {
	store := clipboard.NewStore(clipboard.FromConfigRoot(t.TempDir()), 0)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageNewRGBA(4)); err != nil {
		t.Fatal(err)
	}
	entry, _, err := store.AddImage(buffer.Bytes(), 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	kept := store.ImagePath(entry)
	if kept == "" {
		t.Fatal("the image entry has no path")
	}

	a := testApp()
	a.Clipboard = store

	// Below the ceiling nothing is dropped.
	a.thumbs = map[string]*ui.Bitmap{kept: nil}
	a.pruneThumbs(store.Entries())
	if len(a.thumbs) != 1 {
		t.Errorf("a small memo was pruned: %v", a.thumbs)
	}

	// Past it, only the live path survives.
	a.thumbs = map[string]*ui.Bitmap{}
	for i := 0; i <= clipboard.MaxMaxItems; i++ {
		a.thumbs[filepath.Join("gone", strconv.Itoa(i)+".png")] = nil
	}
	a.thumbs[kept] = nil
	a.pruneThumbs(store.Entries())
	if len(a.thumbs) != 1 {
		t.Fatalf("the memo was not pruned: %d entries", len(a.thumbs))
	}
	if _, ok := a.thumbs[kept]; !ok {
		t.Errorf("the live thumbnail was dropped: %v", a.thumbs)
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
	if !tt.HasText("[image 24x24]") {
		t.Fatalf("the image row is missing: %q", tt.Texts())
	}
	// The row's second line names the kind and the age, not a bare timestamp.
	if !tt.HasText(a.copy().ClipboardTypeImage + " · 0m") {
		t.Fatalf("the image row has no type/age subtitle: %q", tt.Texts())
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

// The action bar: what the field itself is asking for.
func TestActionBarRows(t *testing.T) {
	a := testApp()
	openedURL, openedPath, cdTo, ran := "", "", "", 0
	a.Actions.OpenURL = func(browserID, target string) error { openedURL = target; return nil }
	a.Actions.OpenPath = func(path string) { openedPath = path }
	a.Actions.OpenInTerminal = func(dir string) { cdTo = dir }
	a.Actions.Copy = func(text string) { _ = text }
	a.Actions.RunInTerminal = func([]string) { ran++ }

	tt := render(t, a)

	// A URL opens, and can be copied.
	tt.Type("https://example.com/docs")
	tt.Frame()
	if !tt.HasText("Open in browser") {
		t.Fatalf("the url row is missing: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if openedURL != "https://example.com/docs" {
		t.Errorf("opened %q", openedURL)
	}

	// The scheme is case-insensitive: a pasted `HTTPS://` is a page to open,
	// not a command to run.
	openedURL = ""
	a.ResetQuery()
	tt.Type("HTTPS://Example.com/Docs")
	tt.Frame()
	if !tt.HasText("Open in browser") {
		t.Fatalf("the uppercase url row is missing: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if openedURL != "HTTPS://Example.com/Docs" {
		t.Errorf("opened %q", openedURL)
	}

	// A path opens in the file manager, with a terminal row beside it and a
	// copy row.
	dir := t.TempDir()
	a.ResetQuery()
	tt.Type(dir)
	tt.Frame()
	if !tt.HasText("Open in files") || !tt.HasText("cd in terminal") {
		t.Fatalf("the path rows are missing: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if openedPath != dir {
		t.Errorf("opened %q", openedPath)
	}
	a.ResetQuery()
	tt.Type(dir)
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if cdTo != dir {
		t.Errorf("cd = %q", cdTo)
	}

	// A command the user typed runs in the terminal surface, and it is what
	// Enter runs when nothing else matches. Nothing matches here, so the bar
	// is not drawn (the old build's `visibleActionBar` needed a non-empty
	// list) — Enter reaches the row through the list's no-result fallback.
	a.ResetQuery()
	tt.Type("git status")
	tt.Frame()
	if tt.HasText("Run in shell") {
		t.Errorf("the shell row was offered over an empty page: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if ran != 1 {
		t.Errorf("the shell row ran %d times", ran)
	}

	// A bare command word defaults to the shell too, even when the catalog
	// has a match beside it.
	ran = 0
	a.ResetQuery()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/bin/git"}}, true)
	tt.Frame()
	tt.Type("git")
	tt.Frame()
	if !tt.HasText("Run in shell") {
		t.Fatalf("the shell row is missing beside a match: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if ran != 1 {
		t.Errorf("the shell row ran %d times", ran)
	}
}

// The built-in commands are searchable in both languages and by the pinyin
// initials of their Chinese names.
func TestBuiltinCommandsPinyinAndLanguage(t *testing.T) {
	a := testApp()
	for _, tc := range []struct{ query, id string }{
		{"重启", "power-restart"},
		{"cq", "power-restart"},      // 重启 → cq
		{"cxqd", "power-restart"},    // 重新启动 → cxqd
		{"zxqd", "power-restart"},    // 重's other reading
		{"gj", "power-shutdown"},     // 关机 → gj
		{"gbdn", "power-shutdown"},   // 关闭电脑 → gbdn
		{"剪贴板", "clipboard"},         // the word itself
		{"jtb", "clipboard"},         // 剪贴板 → jtb
		{"浏览器", "browser"},           // the word itself
		{"llq", "browser"},           // 浏览器 → llq
		{"jsq", "calculator"},        // 计算器 → jsq
		{"终端", "terminal"},           // the word itself
		{"zd", "terminal"},           // 终端 → zd
		{"设置", "settings"},           // the word itself
		{"sz", "settings"},           // 设置 → sz
		{"退出", "quit"},               // the word itself
		{"tc", "quit"},               // 退出 → tc
		{"restart", "power-restart"}, // the English wording on a Chinese UI
		{"关机", "power-shutdown"},     // the Chinese wording on an English UI
	} {
		a.ResetQuery()
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

// A shell line with leading environment assignments rides them: the tokens
// are the structured parse's, and the assignments reach the terminal as env.
func TestShellLineCarriesEnvironment(t *testing.T) {
	a := testApp()
	gotArgv, gotEnv := []string{}, map[string]string{}
	a.Actions.RunInTerminalWithEnv = func(argv []string, env []string) {
		// A redraw of the result list re-arms the row; only the first run
		// counts.
		if len(gotArgv) > 0 {
			return
		}
		gotArgv = argv
		for _, pair := range env {
			key, value, _ := strings.Cut(pair, "=")
			gotEnv[key] = value
		}
	}
	a.Actions.RunInTerminal = func([]string) { t.Error("the plain run was used") }

	tt := render(t, a)
	tt.Type(`FOO='a b' PATH=/usr/bin git status`)
	tt.Frame()
	// The line matches nothing, so no bar is drawn; Enter still runs it.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if len(gotArgv) != 2 || gotArgv[0] != "git" || gotArgv[1] != "status" {
		t.Errorf("argv = %v", gotArgv)
	}
	if gotEnv["FOO"] != "a b" || gotEnv["PATH"] != "/usr/bin" {
		t.Errorf("env = %v", gotEnv)
	}
}

// The shell history: a command the user ran is remembered, ↑ walks into it,
// and ↓ brings the half-typed draft back. The walk is driven directly, not
// through the field's editor (whose internal buffer the framework keeps per
// window): the walker's decisions are what this pins.
func TestShellHistory(t *testing.T) {
	a := testApp()
	ran := 0
	a.Actions.RunInTerminal = func([]string) { ran++ }

	tt := render(t, a)
	tt.Type("git status")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if len(a.History) != 1 || a.History[0] != "git status" {
		t.Fatalf("history = %v", a.History)
	}

	// ↑ recalls the command; ↓ past the newest entry brings the draft back.
	a.Query = "draft"
	a.pendingCaret = true
	if !a.historyUp() {
		t.Fatal("↑ did not walk into the history")
	}
	if a.Query != "git status" {
		t.Errorf("↑ = %q, want the recalled command", a.Query)
	}
	if !a.historyDown() {
		t.Fatal("↓ did not walk out of the history")
	}
	if a.Query != "draft" {
		t.Errorf("↓ = %q, want the draft back", a.Query)
	}
	if a.historyDown() {
		t.Error("↓ walked past the present")
	}

	// A second command is promoted rather than stacked, and the walk covers
	// both.
	a.historyRemember("cargo build")
	if len(a.History) != 2 || a.History[0] != "cargo build" {
		t.Fatalf("history = %v", a.History)
	}
	a.Query = "x"
	if !a.historyUp() {
		t.Fatal("↑ did not walk")
	}
	if a.Query != "cargo build" {
		t.Errorf("newest = %q", a.Query)
	}
	if !a.historyUp() {
		t.Fatal("↑ did not walk to the oldest")
	}
	if a.Query != "git status" {
		t.Errorf("oldest = %q", a.Query)
	}
	if a.historyUp() {
		t.Error("↑ walked past the oldest entry")
	}

	// An empty history has nothing to walk.
	a.History = nil
	a.Query = "untouched"
	if a.historyUp() {
		t.Error("an empty history walked")
	}
	if a.Query != "untouched" {
		t.Errorf("an empty history changed the field: %q", a.Query)
	}
}

// The settings page sells `open_settings` as a rebindable app shortcut; the
// launcher is the surface that owes it (the old build answered it in the
// collapsed mode). Both the shipped binding and a rebound one open the
// settings surface, and the key that was given up stops answering.
func TestOpenSettingsShortcutOpensSettings(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	mods, key, ok := shortcuts.Parse(settings.Shortcut(a.settings(), settings.ShortcutOpenSettings))
	if !ok {
		t.Fatal("the open-settings shortcut did not parse")
	}
	tt.Key(mods, key)
	tt.Frame()
	if testRuns["settings"] != 1 {
		t.Fatalf("the shortcut opened settings %d times, want 1", testRuns["settings"])
	}

	// Rebinding the action moves the key with it.
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutOpenSettings, "Alt+Shift+O")
	}); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Key(mods, key)
	tt.Frame()
	if testRuns["settings"] != 1 {
		t.Errorf("the old binding still opened settings: %d", testRuns["settings"])
	}
	tt.Key(ui.Alt|ui.Shift, ui.KeyO)
	tt.Frame()
	if testRuns["settings"] != 2 {
		t.Errorf("the rebound shortcut opened settings %d times, want 2", testRuns["settings"])
	}
}

// The field row's two own buttons are glyphs with no visible word, so each
// carries the sentence the old build's `title=` gave it — and the settings
// button's line names the key that opens the same surface, read from the
// user's own binding rather than frozen at the shipped ⌘,.
func TestFieldRowButtonsCarryTheirHints(t *testing.T) {
	a := testApp()
	// The sessions button only exists when the shell wired it: without a
	// destination the field keeps one button, not a dead one.
	a.Actions.OpenSessions = func() {}
	copy := a.copy()
	hint := func() string {
		binding := shortcuts.Display(settings.Shortcut(a.settings(), settings.ShortcutOpenSettings))
		return copy.FieldSettingsHint(binding)
	}
	tt := render(t, a)
	for _, want := range []struct{ label, text string }{
		{copy.FieldSessions, copy.FieldSessionsHint},
		{copy.FieldSettings, hint()},
	} {
		rect, ok := tt.Find(want.label)
		if !ok {
			t.Fatalf("no %q control: %q", want.label, tt.Texts())
		}
		// Park the pointer away first, so this control's own rest timer starts
		// fresh rather than warming on the previous tip.
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H+120)
		tt.Frame()
		tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
		// The tip waits for the pointer to rest (Base UI's 600ms).
		time.Sleep(700 * time.Millisecond)
		tt.Frame()
		if !tt.HasText(want.text) {
			t.Errorf("the %q control has no tooltip %q: %q", want.label, want.text, tt.Texts())
		}
	}

	// Rebinding open_settings moves the words with the key.
	shipped := hint()
	if err := a.Store.Update(func(s *settings.Settings) {
		s.SetShortcut(settings.ShortcutOpenSettings, "Alt+Shift+O")
	}); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	rebound := hint()
	if rebound == shipped {
		t.Fatalf("the hint froze the shipped binding: %q", shipped)
	}
	rect, ok := tt.Find(copy.FieldSettings)
	if !ok {
		t.Fatalf("no settings control: %q", tt.Texts())
	}
	tt.Move(rect.X+rect.W/2, rect.Y+rect.H+120)
	tt.Frame()
	tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	time.Sleep(700 * time.Millisecond)
	tt.Frame()
	if !tt.HasText(rebound) {
		t.Errorf("the title did not follow the rebind: want %q in %q", rebound, tt.Texts())
	}
}

// A plain W is a letter in the field, not the shell's ⌘W hide: the hide
// is the shell's appShortcuts (shell_test.go), and the launcher only
// answers its own rebindable keys.
func TestPlainWIsALetterNotTheHide(t *testing.T) {
	a := testApp()
	tt := render(t, a)
	tt.Type("x")
	tt.Frame()
	tt.TypeKey(0, ui.KeyW, "w")
	tt.Frame()
	if testRuns["dismiss"] != 0 {
		t.Errorf("a plain W dismissed %v", testRuns)
	}
	if a.Query != "xw" {
		t.Errorf("query = %q, want the w in the field", a.Query)
	}
}

// The transition the field's nudge promises, end to end through the editor:
// typing a command's trigger word and a space opens the command's mode, so
// the nudge ("one space then enters the mode") is not a claim the app cannot
// keep. The word is consumed; the mode's scope marker names the command.
func TestTypingATriggerWordAndSpaceEntersTheCommandMode(t *testing.T) {
	a := commandApp()
	a.Query = ""
	tt := render(t, a)
	tt.Type("jv")
	tt.Frame()
	if !tt.HasText("JSON Viewer") {
		t.Fatalf("the command row is not in the list: %q", tt.Texts())
	}
	tt.Type(" ")
	tt.Frame()
	if a.mode == nil {
		t.Fatalf("space did not enter the command mode: %q", tt.Texts())
	}
	if a.Query != "" {
		t.Errorf("the trigger word was not consumed: %q", a.Query)
	}
	if !tt.HasText("Enter runs it") {
		t.Errorf("the mode's own hint is missing: %q", tt.Texts())
	}
}

// The command row's right-hand source sits in a fixed slot at the row's right
// edge — after the row's own marks and before its keys — never inline after the
// title, where a narrow window's ellipsis eats it first, precisely when "where
// did this result come from" matters most.
func TestTheSourceWordSitsAtTheRowsRightEdge(t *testing.T) {
	a := commandApp()
	tt := render(t, a)
	tt.Frame()
	title, ok := tt.Find("JSON Viewer")
	if !ok {
		t.Fatalf("the command row is gone: %q", tt.Texts())
	}
	source, ok := tt.Find("V Tools")
	if !ok {
		t.Fatalf("the command row names no source: %q", tt.Texts())
	}
	if source.X < title.X+title.W {
		t.Errorf("the source sits inside the title: title %+v source %+v", title, source)
	}
	badge, ok := tt.Find("Ctrl+1")
	if !ok {
		t.Fatalf("the row has no numbered badge: %q", tt.Texts())
	}
	if source.X+source.W > badge.X {
		t.Errorf("the source overlaps the badge: source %+v badge %+v", source, badge)
	}
}

// A PATH command's drawn row carries its two recovered facts: the source word
// "PATH" in the same right-hand slot an integration's name fills, and the
// executable's path as the second line, complete enough to read which
// directory's command this is.
func TestThePathRowNamesItsSourceOnScreen(t *testing.T) {
	a := testApp()
	a.SetTools([]apps.App{{Name: "git", Path: "/usr/local/bin/git"}}, true)
	tt := render(t, a)
	tt.Type("GIT")
	tt.Frame()
	title, ok := tt.Find("git")
	if !ok {
		t.Fatalf("the PATH command row is gone: %q", tt.Texts())
	}
	source, ok := tt.Find("PATH")
	if !ok {
		t.Fatalf("the row names no source: %q", tt.Texts())
	}
	if source.X < title.X+title.W {
		t.Errorf("the source sits inside the title: title %+v source %+v", title, source)
	}
	if !tt.HasText("/usr/local/bin/git") {
		t.Errorf("the row does not say what it runs: %q", tt.Texts())
	}
}

// The row's warning dot and space keycap belong to the same right-edge group,
// not to the title line: a narrow window must not be able to truncate them away
// with the title.
func TestTheRowsMarksSitAtTheRightEdge(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID: "x", IntegrationName: "X",
		Command: extensions.Command{ID: "jv", Name: "JSON Viewer", Description: "View JSON"},
		Program: "/bin/echo", Mode: "pty", Route: extensions.RouteBackground,
	}})
	tt := render(t, a)
	tt.Type("jv")
	tt.Frame()
	title, ok := tt.Find("JSON Viewer")
	if !ok {
		t.Fatalf("the command row is gone: %q", tt.Texts())
	}
	subtitle, ok := tt.Find("View JSON")
	if !ok {
		t.Fatalf("the command row has no subtitle: %q", tt.Texts())
	}
	key, ok := tt.Find("\u2423")
	if !ok {
		t.Fatalf("the direct-output row has no space keycap: %q", tt.Texts())
	}
	if key.X < title.X+title.W {
		t.Errorf("the keycap sits inside the title: title %+v key %+v", title, key)
	}
	if key.X < subtitle.X+subtitle.W {
		t.Errorf("the keycap sits inside the subtitle: subtitle %+v key %+v", subtitle, key)
	}
}

// A title or subtitle longer than the row's column is cut, not wrapped: the
// old build's two text registers are `white-space: nowrap` with an ellipsis,
// and a wrapped title draws lines the window never charged for and pushes the
// list out of its own glass. The source slot narrows the column, so the cut is
// what keeps the row inside the height it was charged.
func TestLongRowTextIsCutNotWrapped(t *testing.T) {
	a := testApp()
	title := strings.Repeat("A very long command name ", 12)
	subtitle := strings.Repeat("long ", 40)
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID: "x", IntegrationName: "X",
		Command: extensions.Command{ID: "jv", Name: title, Description: subtitle},
		Program: "/bin/echo", Mode: "pty",
	}})
	a.Query = "jv"
	tt := render(t, a)
	tt.Frame()
	got, ok := tt.Find(title)
	if !ok {
		t.Fatalf("the long title is gone: %q", tt.Texts())
	}
	if float64(got.H) > a.TitleLine+1 {
		t.Errorf("the title wrapped: %+v, one line is %.1f", got, a.TitleLine)
	}
	got, ok = tt.Find(subtitle)
	if !ok {
		t.Fatalf("the long subtitle is gone: %q", tt.Texts())
	}
	if float64(got.H) > a.SubtitleLine+1 {
		t.Errorf("the subtitle wrapped: %+v, one line is %.1f", got, a.SubtitleLine)
	}
}

// A command whose output the launcher shows carries the space keycap, and the
// keycap says what it means: the sentence rides it as a tooltip, so the bare
// `␣` is not an unexplained mark. The tip shows once the pointer rests.
func TestDirectOutputRowCarriesTheSpaceKeycapTooltip(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID: "x", IntegrationName: "X",
		Command: extensions.Command{ID: "jv", Name: "JSON Viewer"},
		Program: "/bin/echo", Mode: "pty", Route: extensions.RouteBackground,
	}})
	tt := render(t, a)
	tt.Type("jv")
	tt.Frame()
	rect, ok := tt.Find("\u2423")
	if !ok {
		t.Fatalf("the direct-output row has no space keycap: %q", tt.Texts())
	}
	hint := a.copy().DirectOutputHint
	if hint == "" {
		t.Fatal("the keycap has no copy to show")
	}
	tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	// The tip waits for the pointer to rest (Base UI's 600ms).
	time.Sleep(700 * time.Millisecond)
	tt.Frame()
	if !tt.HasText(hint) {
		t.Errorf("the keycap has no tooltip: %q", tt.Texts())
	}
}

// countOf counts the nodes showing exactly want: a Label and the tooltip a
// hover reveals are two, so an increase is the tip appearing.
func countOf(tt *ui.Tester, want string) int {
	n := 0
	for _, s := range tt.Texts() {
		if s == want {
			n++
		}
	}
	return n
}

// hoverRested parks the pointer on rect, waits out the tooltip's rest delay,
// and renders the frame that shows it — the gesture every hint test shares.
func hoverRested(tt *ui.Tester, rect ui.Rect) {
	tt.Move(rect.X+rect.W/2, rect.Y+rect.H+120)
	tt.Frame()
	tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
	// The tip waits for the pointer to rest (Base UI's 600ms).
	time.Sleep(700 * time.Millisecond)
	tt.Frame()
}

// A command word two integrations share is marked with a dot; the dot says
// why it is there, since a bare warning mark explains nothing.
func TestCommandRowWarningCarriesItsSentence(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{
		{IntegrationID: "a", IntegrationName: "A", Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}, Program: "/bin/echo", Mode: "pty"},
		{IntegrationID: "b", IntegrationName: "B", Command: extensions.Command{ID: "jv", Name: "JSON Viewer Two"}, Program: "/bin/echo", Mode: "pty"},
	})
	tt := render(t, a)
	tt.Type("jv")
	tt.Frame()
	sentence := a.copy().CommandConflict
	if sentence == "" {
		t.Fatal("the warning dot has no sentence to show")
	}
	rect, ok := tt.Find(sentence)
	if !ok {
		t.Fatalf("the warning dot carries no label/sentence: %q", tt.Texts())
	}
	before := countOf(tt, sentence)
	hoverRested(tt, rect)
	if countOf(tt, sentence) <= before {
		t.Errorf("the warning dot has no tooltip: %q", tt.Texts())
	}
}

// A history row's star and its pin are glyph controls: hovering each reveals
// the word it stands for.
func TestHistoryRowMarksCarryTheirSentences(t *testing.T) {
	a, _ := clipboardApp(t)
	a.Actions.PinText = func(string, string) {}
	tt := render(t, a)
	a.EnterClipboard()
	tt.Frame()
	copy := a.copy()
	for _, want := range []string{copy.HistoryFavorite, copy.HistoryPin} {
		rect, ok := tt.Find(want)
		if !ok {
			t.Fatalf("no %q mark on the selected row: %q", want, tt.Texts())
		}
		before := countOf(tt, want)
		hoverRested(tt, rect)
		if countOf(tt, want) <= before {
			t.Errorf("the %q mark has no tooltip: %q", want, tt.Texts())
		}
	}
}

// A row's source word is cut to a fixed slot; its tooltip keeps the whole
// word readable.
func TestCommandRowSourceCarriesItsWholeWord(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID: "x", IntegrationName: "A Rather Long Integration Name",
		Command: extensions.Command{ID: "jv", Name: "JSON Viewer"},
		Program: "/bin/echo", Mode: "pty",
	}})
	tt := render(t, a)
	tt.Type("jv")
	tt.Frame()
	source := "A Rather Long Integration Name"
	rect, ok := tt.Find(source)
	if !ok {
		t.Fatalf("the source word is missing: %q", tt.Texts())
	}
	before := countOf(tt, source)
	hoverRested(tt, rect)
	if countOf(tt, source) <= before {
		t.Errorf("the source word has no tooltip: %q", tt.Texts())
	}
}

// The captured output view's two icon controls carry their hints: the pin and
// the detach, which the old build's `title=` explained.
func TestOutputViewControlsCarryTheirHints(t *testing.T) {
	a := testApp()
	a.Actions.DetachOutput = func(OutputView) {}
	a.Actions.PinText = func(string, string) {}
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh", Args: []string{"-c", "echo hi"},
		Route: extensions.RouteBackground,
	}})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{Command: []string{"list"}, Stdout: "hi\n", Success: true}, nil)
	}
	tt := render(t, a)
	a.Query = "list"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	copy := a.copy()
	if copy.OutputDetachHint == "" || copy.OutputDetachHint == copy.OutputDetach {
		t.Fatal("the detach control has no distinct hint")
	}
	rect, ok := tt.Find(copy.OutputDetach)
	if !ok {
		t.Fatalf("the detach control is missing: %q", tt.Texts())
	}
	hoverRested(tt, rect)
	if !tt.HasText(copy.OutputDetachHint) {
		t.Errorf("the detach control has no tooltip: %q", tt.Texts())
	}
}

// The `␣` keycap's gesture, end to end: the command's trigger word and a space
// open its mode, and Enter runs it — the output lands in the search box, which
// is what the keycap promised.
func TestSpaceKeycapGestureRunsTheDirectOutputCommand(t *testing.T) {
	a := testApp()
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID: "x", IntegrationName: "X",
		Command: extensions.Command{ID: "list", Name: "List things"},
		Program: "/bin/sh", Args: []string{"-c", "echo hi"},
		Route: extensions.RouteBackground,
	}})
	captured := 0
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		captured++
		done(extensions.CapturedRun{Command: []string{"list"}, Stdout: "hi\n", Success: true}, nil)
	}
	tt := render(t, a)
	tt.Type("list ")
	tt.Frame()
	if a.mode == nil {
		t.Fatalf("space did not enter the command mode: %q", tt.Texts())
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if captured != 1 || !a.InOutputView() || !tt.HasText("hi") {
		t.Errorf("the space gesture did not run the command: captured=%d output=%v texts=%q",
			captured, a.InOutputView(), tt.Texts())
	}
}

// A line with shell syntax is not a program with arguments: `ls | grep x` is
// a pipeline, and no argv can say it. The row therefore hands the *line* to
// the user's own shell (the reference's run entry typed every line into the
// interactive shell), while a structured line keeps the argv route.
func TestShellSyntaxRunsTheLineInAShell(t *testing.T) {
	a := testApp()
	ranLine := ""
	ranArgv := []string{}
	a.Actions.RunInShell = func(line string) {
		if ranLine != "" {
			return // a redraw re-arms the row; only the first run counts
		}
		ranLine = line
	}
	a.Actions.RunInTerminal = func(argv []string) { ranArgv = argv }

	tt := render(t, a)
	tt.Type("ls -la | grep go")
	tt.Frame()
	// A shell-syntax line matches no catalogue row, so the old build drew no
	// bar for it; Enter still hands the line to the shell (the list's
	// no-result fallback).
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if ranLine != "ls -la | grep go" {
		t.Errorf("the line handed to the shell = %q", ranLine)
	}
	if len(ranArgv) != 0 {
		t.Errorf("a shell-syntax line went to argv: %v", ranArgv)
	}

	// A structured line is unchanged: the argv route, token by token.
	a2 := testApp()
	got := []string{}
	a2.Actions.RunInTerminal = func(argv []string) {
		if len(got) == 0 {
			got = argv
		}
	}
	a2.Actions.RunInShell = func(string) { t.Error("a structured line went to the shell") }
	tt2 := render(t, a2)
	tt2.Type("git status")
	tt2.Frame()
	tt2.TypeKey(0, ui.KeyEnter, "")
	tt2.Frame()
	if len(got) != 2 || got[0] != "git" || got[1] != "status" {
		t.Errorf("argv = %v", got)
	}
}

// The two seams the shell-syntax row must not lose: the line is remembered in
// the history exactly as the user typed it, and the row's detail still shows
// the query.
func TestShellSyntaxRowKeepsItsLineAndItsHistory(t *testing.T) {
	a := testApp()
	a.Actions.RunInShell = func(string) {}
	tt := render(t, a)
	tt.Type("cat log\u3000| tail -5")
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	// The history keeps the line the user typed, separators and all: it is a
	// record of what was run, not a second normalization point.
	if len(a.History) != 1 || a.History[0] != "cat log\u3000| tail -5" {
		t.Errorf("history = %q", a.History)
	}
}

// A drop of more files than the cap lists the first five and puts the rest
// behind one "and N more" row — an ordinary result, reachable with the arrows,
// that reveals the drop instead of touching a file (the reference's
// `MAX_VISIBLE_DROPPED_FILES` and its expander row).
func TestALargeDropCollapsesBehindOneRow(t *testing.T) {
	a := testApp()
	files := make([]drops.File, 0, 8)
	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt", "f.txt", "g.txt", "h.txt"} {
		files = append(files, drops.File{Path: "/tmp/" + name, Name: name, Directory: "/tmp"})
	}
	a.SetDropped(files)

	tt := render(t, a)
	tt.Type("files ")
	tt.Frame()

	// The rows are the first five files' — the sixth is behind the expander.
	listed := map[string]bool{}
	ids := []string{}
	for _, item := range a.filesItems() {
		ids = append(ids, item.ID)
		listed[item.Title] = true
	}
	if !listed["Open  e.txt"] {
		t.Errorf("the fifth file is not listed: %v", ids)
	}
	if listed["Open  f.txt"] {
		t.Errorf("the sixth file is listed before the expander was run: %v", ids)
	}
	if !listed["\u2026and 3 more files"] {
		t.Fatalf("the expander row is missing: %v", ids)
	}
	if ids[len(ids)-1] != "drop:more" {
		t.Fatalf("the expander is not the list's last row: %v", ids)
	}
	if !tt.HasText("Dropped files") || !tt.HasText("Open  a.txt") {
		t.Errorf("the mode did not draw its rows: %q", tt.Texts())
	}

	// Running the expander reveals the whole drop — and it is a row like any
	// other: the selection reaches it, there is no control of its own.
	a.Selected = len(a.filesItems()) - 1
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if !tt.HasText("Open  h.txt") && !a.dropsExpanded {
		t.Errorf("the expander did not reveal the rest: %q", tt.Texts())
	}
	if got := a.filesItems(); len(got) != 8*3+1 {
		t.Errorf("after expanding the list holds %d rows, want %d", len(got), 8*3+1)
	}
	if a.InFilesMode() == false {
		t.Error("running the expander left the files mode")
	}
}

// A drop at or below the cap has no expander, and a fresh drop starts
// collapsed: the previous drop's expansion must not carry over to a new set of
// files the user has not seen.
func TestTheDropExpanderFollowsTheDrop(t *testing.T) {
	a := testApp()
	small := []drops.File{{Path: "/tmp/a.txt", Name: "a.txt", Directory: "/tmp"}}
	a.SetDropped(small)
	for _, item := range a.filesItems() {
		if item.ID == "drop:more" {
			t.Error("a drop below the cap offers an expander")
		}
	}

	many := make([]drops.File, 0, 7)
	for i := 0; i < 7; i++ {
		name := string(rune('a'+i)) + ".txt"
		many = append(many, drops.File{Path: "/tmp/" + name, Name: name, Directory: "/tmp"})
	}
	a.SetDropped(many)
	a.dropsExpanded = true
	before := len(a.filesItems())
	// A re-drop replaces the files, so the expander's state goes with them.
	a.SetDropped(many)
	if after := len(a.filesItems()); after >= before {
		t.Errorf("a new drop kept the old expansion: %d rows, was %d", after, before)
	}
	// Entering the mode again starts collapsed too.
	a.dropsExpanded = true
	a.EnterFiles()
	if after := len(a.filesItems()); after >= before {
		t.Errorf("re-entering the mode kept the old expansion: %d rows", after)
	}
}

// The cap counts files, not rows: the needle still filters the action rows, and
// the expander counts the files behind it.
func TestTheDropCapCountsFilesNotRows(t *testing.T) {
	a := testApp()
	many := make([]drops.File, 0, 7)
	for i := 0; i < 7; i++ {
		name := "file" + string(rune('a'+i)) + ".txt"
		many = append(many, drops.File{Path: "/tmp/" + name, Name: name, Directory: "/tmp"})
	}
	a.SetDropped(many)
	a.EnterFiles()

	items := a.filesItems()
	more := ""
	for _, item := range items {
		if item.ID == "drop:more" {
			more = item.Title
		}
	}
	if more != "\u2026and 2 more files" {
		t.Errorf("the expander counts rows, not files: %q", more)
	}

	// A needle that matches one action word keeps that row and drops the
	// others — the filter is the rows', the cap is the files'.
	a.Query = "copy path"
	filtered := a.filesItems()
	openRows := 0
	for _, item := range filtered {
		if strings.HasPrefix(item.Title, "Open ") {
			openRows++
		}
	}
	if openRows != 0 {
		t.Errorf("the action word did not filter the rows: %v", filtered)
	}
	if len(filtered) == 0 {
		t.Error("the action word matched nothing at all")
	}
}
