package launcher

import (
	"fmt"
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
