package launcher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
)

// listOutput builds the list protocol's JSON for n generic rows.
func listOutput(n int) string {
	var b strings.Builder
	b.WriteString("[")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":"r%d","title":"Row %d"}`, i, i)
	}
	b.WriteString("]")
	return b.String()
}

// runListOutput opens the output view on one captured emission.
func runListOutput(t *testing.T, a *App, stdout string) *ui.Tester {
	t.Helper()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{Command: []string{"tool"}, Success: true, Stdout: stdout}, nil)
	}
	tt := render(t, a)
	a.Query = "list"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	return tt
}

// A plugin that hands over its whole list is windowed in memory: the first
// emission loads two pages, the footer says more exist, and the window grows
// by one page when the end is reached. A complete list carries no footer.
func TestCapturedListWindowsInMemory(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, listOutput(25))
	if !a.InOutputView() {
		t.Fatal("the output view did not open")
	}
	if got := len(a.output.Rows); got != extensions.InitialPages*extensions.PageSize {
		t.Fatalf("the first window is %d rows", got)
	}
	if a.output.Page == nil || !a.output.Page.HasMore {
		t.Fatalf("the window's page = %+v", a.output.Page)
	}
	if !tt.HasText("Scroll for more") {
		t.Errorf("the footer is missing: %q", tt.Texts())
	}
	if tt.HasText("Row 24") {
		t.Errorf("a row past the window is drawn: %q", tt.Texts())
	}

	// Reaching the end loads the rest: every row is reachable and the footer
	// goes away when the list is complete.
	a.loadMoreOutputPage()
	if got := len(a.output.Rows); got != 25 {
		t.Fatalf("after one more page the window is %d rows", got)
	}
	if a.output.Page != nil {
		t.Errorf("a complete list kept a page block: %+v", a.output.Page)
	}
}

// A short list is complete on the first emission: every row and no footer.
func TestCapturedShortListHasNoFooter(t *testing.T) {
	a := testApp()
	tt := runListOutput(t, a, listOutput(extensions.PageSize))
	if got := len(a.output.Rows); got != extensions.PageSize {
		t.Fatalf("the window is %d rows", got)
	}
	if a.output.Page != nil {
		t.Errorf("a complete list got a page block: %+v", a.output.Page)
	}
	if tt.HasText("Scroll for more") || tt.HasText("No more results") {
		t.Errorf("a complete list drew a footer: %q", tt.Texts())
	}
}

// A plugin that pages itself owns its cursor: the rows are not windowed, the
// footer reflects its block, and loading more re-runs the command with the
// cursor it named.
func TestCapturedSelfPagingListReplaysCursor(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "paged", Name: "Paged"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	// The first page fills more than the viewport, so its end is not on screen
	// and the scroll trigger has not fired yet.
	first := `{"output":` + listOutput(12) + `,"page":{"cursor":"c1","hasMore":true}}`
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{Command: []string{"tool"}, Success: true, Stdout: first}, nil)
	}
	var gotEntry extensions.CommandEntry
	var gotArgs []string
	var gotCursor string
	a.Actions.LoadOutputPage = func(e extensions.CommandEntry, args []string, cursor string, done func(extensions.CapturedRun, error)) {
		gotEntry, gotArgs, gotCursor = e, args, cursor
		done(extensions.CapturedRun{
			Command: []string{"tool"},
			Success: true,
			Stdout:  `{"output":[{"id":"r0","title":"Row 0"},{"id":"last","title":"Last"}],"page":{"cursor":null,"hasMore":false}}`,
		}, nil)
	}
	tt := render(t, a)
	a.Query = "paged"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if a.output == nil || !a.output.Declared {
		t.Fatal("the self-paging list was not marked as such")
	}
	if len(a.output.Rows) != 12 || a.output.Page == nil || !a.output.Page.HasMore {
		t.Fatalf("view = %+v", a.output)
	}
	if !tt.HasText("Scroll for more") {
		t.Errorf("the footer is missing: %q", tt.Texts())
	}

	a.loadMoreOutputPage()
	if gotCursor != "c1" {
		t.Errorf("the plugin's cursor was not replayed: %q", gotCursor)
	}
	if gotEntry.Command.ID != "paged" || len(gotArgs) != 0 {
		t.Errorf("re-ran %+v with %q", gotEntry, gotArgs)
	}
	// The repeated id is dropped and the plugin's end is honoured.
	if len(a.output.Rows) != 13 {
		t.Fatalf("the next page did not append: %+v", a.output.Rows)
	}
	if a.output.Page == nil || a.output.Page.HasMore {
		t.Errorf("the end was not honoured: %+v", a.output.Page)
	}
}

// A self-paging plugin that stops speaking the protocol is not asked forever:
// the list keeps what it has and says it is done.
func TestCapturedSelfPagingListStopsOnBadPage(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "paged", Name: "Paged"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{
			Success: true,
			Stdout:  `{"output":` + listOutput(12) + `,"page":{"cursor":"c1","hasMore":true}}`,
		}, nil)
	}
	a.Actions.LoadOutputPage = func(_ extensions.CommandEntry, _ []string, _ string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{Success: true, Stdout: "not a list"}, nil)
	}
	tt := render(t, a)
	a.Query = "paged"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()

	a.loadMoreOutputPage()
	if len(a.output.Rows) != 12 {
		t.Fatalf("rows changed: %+v", a.output.Rows)
	}
	if a.output.Page == nil || a.output.Page.HasMore {
		t.Errorf("a bad page did not stop the list: %+v", a.output.Page)
	}
}
