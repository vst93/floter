package launcher

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
)

// BenchmarkLauncherFrame measures one frame of the launcher with a full
// catalog and a typed query, which is the frame the user sees on every
// keystroke: the list builds only the rows in view, so this must not grow with
// the catalog.
func BenchmarkLauncherFrame(b *testing.B) {
	a := testApp()
	found := make([]apps.App, 0, 500)
	for i := 0; i < 500; i++ {
		found = append(found, apps.App{
			Name: fmt.Sprintf("Application %d", i),
			Path: fmt.Sprintf("/Applications/App %d.app", i),
		})
	}
	a.SetApps(found)
	a.Query = "application 4"

	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
}

// BenchmarkLauncherEmptyFrame measures the frame an empty query shows: the
// built-in commands and the recent applications.
func BenchmarkLauncherEmptyFrame(b *testing.B) {
	a := testApp()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	tt.Frame()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tt.Frame()
	}
}

// BenchmarkMatch measures the ranking alone, over a catalog the size of a
// loaded desktop: it is the one part of a keystroke that walks every row, so
// its allocations are what the frame's garbage is made of (the ranking reads
// the catalog's rows in place and copies only the winners — see Match).
func BenchmarkMatch(b *testing.B) {
	items := make([]Item, 0, 500)
	for i := 0; i < 500; i++ {
		title := fmt.Sprintf("Application %d", i)
		detail := "/Applications/App " + fmt.Sprintf("%d.app", i)
		id := fmt.Sprintf("app:%d", i)
		// The catalog's own pre-lowered columns (see App.Catalog): without
		// them the benchmark would measure strings.ToLower, which a real
		// catalog never pays per keystroke.
		items = append(items, Item{
			ID: id, Title: title, Detail: detail,
			lowerID: strings.ToLower(id), lowerTitle: strings.ToLower(title),
			lowerDetail: strings.ToLower(detail),
		})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := Match(items, "application 4"); len(got) == 0 {
			b.Fatal("no match")
		}
	}
}
