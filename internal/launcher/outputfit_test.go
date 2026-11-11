package launcher

import (
	"strings"
	"testing"

	"floter/internal/extensions"
	"github.com/egoist/mygo/ui"
)

// A captured run's page opens at the height it draws, so a short output does
// not arrive already scrolled.
//
// The report: 「现在显示的内容并不算很多，但还是触发了滚动条」. Two things were
// charged short and one thing charged too much, all on the same page:
//
//   - the strip above the body — the run's command line and its verdict — was
//     drawn but never priced, so every output was that strip short;
//   - the text form was priced in whole 40u bands, so a three-line block was
//     charged two bands while it drew three lines;
//   - and its floor was the reference's 72u of *its* line box, not this
//     build's, which left dead air under a one-line answer.
//
// The list form was the one arm that was already right, and it is pinned here
// too so a later change to the shared height chain cannot take it out.
func TestTheOutputPageOpensAtTheHeightItDraws(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stdout string
	}{
		{"text/one line", "one"},
		{"text/three lines", "a\nb\nc"},
		{"text/five lines", "a\nb\nc\nd\ne"},
		{"text/twelve lines", strings.Repeat("x\n", 11) + "x"},
		{"list/one row", listOutput(1)},
		{"list/three rows", listOutput(3)},
		{"list/ten rows", listOutput(10)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			entry := extensions.CommandEntry{
				IntegrationID: "test.ext",
				Command:       extensions.Command{ID: "list", Name: "List things"},
				Program:       "/bin/sh",
				Route:         extensions.RouteBackground,
			}
			a.SetCommands([]extensions.CommandEntry{entry})
			a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
				done(extensions.CapturedRun{Command: []string{"tool"}, Success: true, Stdout: tc.stdout}, nil)
			}
			var heights []int
			a.Actions.ResizeTo = func(h int) { heights = append(heights, h) }
			tt := ui.NewTester(themedView(a), InputWindowWidth, themedFrameHeight(maxResultSlots))
			tt.Frame()
			a.Query = "list"
			tt.Frame()
			tt.TypeKey(0, ui.KeyEnter, "")
			tt.Frame()
			if len(heights) == 0 {
				t.Fatal("the view asked for no window height")
			}
			// The window the page itself asked for, drawn at that size: the
			// body must fit without scrolling.
			tt.SetSize(InputWindowWidth, heights[len(heights)-1])
			for range 6 {
				tt.Frame()
			}
			if a.Output.MaxY > 0 {
				t.Errorf("the page opened %.1fpx scrolled: it draws more than the %dpx it asked for",
					a.Output.MaxY, heights[len(heights)-1])
			}
		})
	}
}

// The long output still scrolls inside itself: the ceiling the reference set is
// unchanged, and it is what keeps a hundred-line run from opening a window
// taller than the screen.
func TestALongOutputStillScrollsInsideItsOwnCeiling(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		done(extensions.CapturedRun{Command: []string{"tool"}, Success: true,
			Stdout: strings.Repeat("line\n", 99) + "line"}, nil)
	}
	var heights []int
	a.Actions.ResizeTo = func(h int) { heights = append(heights, h) }
	tt := ui.NewTester(themedView(a), InputWindowWidth, themedFrameHeight(maxResultSlots))
	tt.Frame()
	a.Query = "list"
	tt.Frame()
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	tt.SetSize(InputWindowWidth, heights[len(heights)-1])
	for range 6 {
		tt.Frame()
	}
	if a.Output.MaxY <= 0 {
		t.Fatalf("a hundred-line output does not scroll (%dpx window)", heights[len(heights)-1])
	}
	// The ceiling is the reference's ten band rows: past it the block scrolls
	// rather than growing the window, which is the rule that keeps a long
	// output from opening a window taller than a screen.
	if got := pluginTextRows(strings.Repeat("line\n", 99) + "line"); got != rowBudget {
		t.Errorf("the text ceiling is %d band rows, want the list budget %d", got, rowBudget)
	}
}

// The strip is charged on the page and nowhere else.
//
// It is the one band this round added to the height chain, so the surfaces
// around it must not grow by it: the search page behind the same query is the
// same window it was before the output landed, and leaving the page takes the
// strip with it. Measured on the query's own list rather than by comparing the
// two pages' heights — a result list and a one-line answer are different
// content, so their heights say nothing about the strip.
func TestTheOutputStripIsChargedOnlyOnItsOwnPage(t *testing.T) {
	a := testApp()
	entry := extensions.CommandEntry{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}
	a.SetCommands([]extensions.CommandEntry{entry})
	var land func(extensions.CapturedRun, error)
	a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
		land = done
	}
	var heights []int
	a.Actions.ResizeTo = func(h int) { heights = append(heights, h) }
	tt := ui.NewTester(themedView(a), InputWindowWidth, themedFrameHeight(maxResultSlots))
	tt.Frame()
	a.Query = "list"
	tt.Frame()
	search := heights[len(heights)-1]

	// While the run is in flight the page is still the search page: the strip
	// is not up yet, so the window does not move for it.
	tt.TypeKey(0, ui.KeyEnter, "")
	tt.Frame()
	if got := heights[len(heights)-1]; got != search {
		t.Errorf("the running band resized the page to %dpx, the search page is %dpx", got, search)
	}
	if a.OutputHeaderHeight() != 0 {
		t.Errorf("the strip is charged while no output is open: %v", a.OutputHeaderHeight())
	}

	land(extensions.CapturedRun{Command: []string{"tool"}, Success: true, Stdout: "one line"}, nil)
	tt.Frame()
	if a.OutputHeaderHeight() <= 0 {
		t.Errorf("the open output page charges no strip (%v), so it opens a band short",
			a.OutputHeaderHeight())
	}

	// Editing the field leaves the page, and the strip goes with it. The
	// window's height is not compared here: the edited query is different
	// content, so it legitimately asks for a different height. What must not
	// survive is the *strip* — the one band this round added.
	tt.Type("y")
	tt.Frame()
	if a.InOutputView() {
		t.Fatalf("a keystroke left the output page up: %q", tt.Texts())
	}
	if a.OutputHeaderHeight() != 0 {
		t.Errorf("the strip outlived the page: %v", a.OutputHeaderHeight())
	}
}
