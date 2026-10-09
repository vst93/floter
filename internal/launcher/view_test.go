package launcher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
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
