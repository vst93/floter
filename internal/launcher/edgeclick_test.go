package launcher

import (
	"testing"

	"floter/internal/apps"
)

// The scroll edge over the first rows must not eat their clicks: the strip
// is PassThrough, so a click at a row the strip covers still runs the row.
func TestClickingThroughTheScrollEdge(t *testing.T) {
	a := testApp()
	items := make([]apps.App, 12)
	paths := make([]string, 12)
	for i := range items {
		items[i] = apps.App{Name: "App" + string(rune('A'+i)), Path: "/app/x"}
		paths[i] = items[i].Path
	}
	a.SetApps(items)
	// P11-bk: the empty page's rows are the recents; the built-in commands no
	// longer stand in for them.
	a.SetRecent(paths, true)
	tt := render(t, a)
	tt.Frame()
	results := a.Results()
	if len(results) < 3 {
		t.Fatalf("results = %d", len(results))
	}
	// The apps join the catalog when a query names them; one that does,
	// sits beside the built-ins, and the second row is under the scroll
	// edge band either way. The query is lowercase on purpose: the action
	// bar's shell row shows the typed text as its detail, and a lowercased
	// query keeps that from colliding with the row's own (capitalized) title,
	// so the click lands on the app and not on the bar.
	a.Query = "appb"
	tt.Frame()
	results = a.Results()
	var title string
	for _, item := range results {
		if item.Title == "AppB" {
			title = item.Title
		}
	}
	if title == "" {
		t.Fatalf("AppB is not in %v", results)
	}
	if _, ok := tt.Find(title); !ok {
		t.Fatalf("row %q not found", title)
	}
	if err := tt.Click(title); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if testRuns["app:AppB"] != 1 {
		t.Errorf("the click through the edge did not run the row: %v", testRuns)
	}
}
