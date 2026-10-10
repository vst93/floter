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
	for i := range items {
		items[i] = apps.App{Name: "App" + string(rune('A'+i)), Path: "/app/x"}
	}
	a.SetApps(items)
	tt := render(t, a)
	tt.Frame()
	results := a.Results()
	if len(results) < 3 {
		t.Fatalf("results = %d", len(results))
	}
	// The apps join the catalog when a query names them; one that does,
	// sits beside the built-ins, and the second row is under the scroll
	// edge band either way.
	a.Query = "AppB"
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
