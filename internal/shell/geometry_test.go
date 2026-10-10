package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/apps"
	"floter/internal/launcher"
	"floter/internal/plugincfg"
	"floter/internal/settings"
)

// The window's height is the card's own: what the view draws, the launcher's
// geometry charges, and the shell walks the window to. Rendered at exactly the
// charged height, every row the list holds has to be inside the window — a row
// of text whose line box is taller than the model assumed would show up here as
// a row below the card's edge.
func TestChargedHeightHoldsTheList(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*App)
		want  []string
	}{
		{"the catalog", func(a *App) {}, []string{"Open settings", "Quit floter"}},
		{"the clipboard mode's chips", func(a *App) { a.Launcher.EnterClipboard() }, []string{"All", "Favorites"}},
		{"a plugin's configuration sheet", func(a *App) {
			a.Launcher.EnterClipboard()
			a.Launcher.OpenConfig(settings.CustomPluginClipboard)
		}, []string{"Clipboard history", "Clear history"}},
		{"the docked feedback row", func(a *App) { a.Launcher.Feedback("Copied") }, []string{"Copied"}},
		{"a list of applications", func(a *App) {
			a.Launcher.SetApps([]apps.App{
				{Name: "Alpha", Path: "/a/Alpha.app"},
				{Name: "Beta", Path: "/a/Beta.app"},
			})
			a.Launcher.SetQuery("a")
		}, []string{"Alpha", "Beta"}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			a := New(Options{Store: settings.NewStore(settings.Default())})
			tc.setup(a)
			// One frame measures the list, the next sizes the window to it.
			tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, launcher.InputWindowWidth, 1000)
			tt.Frame()
			var schema *plugincfg.Schema
			if config := a.Launcher.Config(); config != nil {
				schema = config.Schema
			}
			height := launcher.Geometry{
				Font: a.Launcher.Font, Line: a.Launcher.Line, Spacing: a.Launcher.Spacing,
				RowLines: a.Launcher.RowLines, Held: a.Launcher.HeldRows,
				Filter:   a.Launcher.FiltersVisible(),
				Feedback: a.Launcher.FeedbackVisible(),
				Config:   schema,
			}.Height()
			if height <= 0 || height >= 1000 {
				t.Fatalf("charged height = %d", height)
			}
			tt.SetSize(launcher.InputWindowWidth, height)
			tt.Frame()
			for _, want := range tc.want {
				rect, ok := tt.Find(want)
				if !ok {
					t.Errorf("missing %q at the charged height: %q", want, tt.Texts())
					continue
				}
				if bottom := rect.Y + rect.H; bottom > float32(height) {
					t.Errorf("%q ends at %.0f, below the window's %d", want, bottom, height)
				}
			}
		})
	}
}
