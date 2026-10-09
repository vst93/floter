package launcher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/clipboard"
	"floter/internal/extensions"
	"floter/internal/settings"
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
	tt.Type("\b\b\b")
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
	tt.Type("clipboard history")
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
