package launcher

import (
	"strings"
	"testing"

	"floter/internal/extensions"
)

// outputHeight opens a captured run's output view on one emission and returns
// the window height the view asked for, with `query` typed in the field
// behind it.
func outputHeight(t *testing.T, stdout, query string) int {
	t.Helper()
	a := testApp()
	var heights []int
	a.Actions.ResizeTo = func(h int) { heights = append(heights, h) }
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Query = query
	a.showOutput(extensions.CapturedRun{Success: true, Stdout: stdout}, nil, &entry, nil)
	render(t, a)
	if len(heights) == 0 {
		t.Fatal("the view asked for no window height")
	}
	return heights[len(heights)-1]
}

// The output view's window is as tall as its output, not as the query behind
// it: the old build priced `pluginViewRows(pluginView)` while the plugin view
// was up (`main:src/App.tsx:2133-2137`). Before this, a two-row query in front
// of a twelve-line output sized the window to two rows.
func TestTheOutputViewPricesItsOwnContent(t *testing.T) {
	// The same output under a one-result query and under the many-result empty
	// page is the same window: the results behind it no longer price it.
	one := outputHeight(t, "a\nb\nc\nd", "list")
	many := outputHeight(t, "a\nb\nc\nd", "")
	if one != many {
		t.Errorf("the same output priced %d under one result and %d under many", one, many)
	}

	// A longer output is a taller window, and it climbs in the old build's own
	// band steps: three lines take the floor, twelve lines eight band rows.
	short := outputHeight(t, "a\nb\nc", "list")
	long := outputHeight(t, strings.Repeat("x\n", 11)+"x", "list")
	if long <= short {
		t.Errorf("a twelve-line output (%d) is not taller than a three-line one (%d)", long, short)
	}
}

// The output list's window is capped at the numbered budget: a twenty-five-row
// emission prices the same window a ten-row one does — the old build's
// `pluginViewRows = min(items, MAX_RESULTS)` — while a shorter list is exactly
// its own rows.
func TestTheOutputListWindowIsCappedAtTheBudget(t *testing.T) {
	three := outputHeight(t, listOutput(3), "list")
	ten := outputHeight(t, listOutput(10), "list")
	twentyFive := outputHeight(t, listOutput(25), "list")
	if three >= ten {
		t.Errorf("a three-row output (%d) is not shorter than a ten-row one (%d)", three, ten)
	}
	if ten != twentyFive {
		t.Errorf("a twenty-five-row output priced %d, a ten-row one %d", twentyFive, ten)
	}
}
