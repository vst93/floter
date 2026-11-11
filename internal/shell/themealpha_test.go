package shell

import (
	"testing"

	"floter/internal/settings"
	"floter/internal/settingsui"
	"github.com/egoist/mygo/ui"
)

// Every settings page keeps the window's glass, including the ones reached
// through a control that pins the card's own spacing.
//
// The report was 「添加工具中 本地工具页…没有走整体的主题颜色和透明度」: the page
// drew the right colours but at alpha 255 where every other page is 207, so it
// sat on the desktop as a solid slab. The cause was `Context.SetTheme`, which is
// not a scoped switch — it also writes the *root's* background
// (`ui/context.go:349-355`: `c.root.bg = t.Background`) — and the shell paints
// that root transparent so the glass reads the desktop through it
// (`internal/shell/shell.go`, `c.Root().Background(ui.Transparent)`). A
// segmented control restoring its pinned theme therefore repainted the whole
// window with an opaque background, and only the pages that drew one — the
// integration pages, whose metrics are the reference's literal pixels — were
// affected. That is why General and the other pages looked right.
//
// The reading is taken on the page's own face, below the last control, and
// compared against the General page's: a page that loses the glass reports both
// numbers.
func TestEverySettingsPageKeepsTheWindowsGlass(t *testing.T) {
	const x, y = 700, 660
	render := func(dark bool, setup func(a *App)) uint8 {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		s := settings.Default()
		s.Theme, s.Language, s.GlassStep, s.MainOpacity = appearance, "en", "liquid", 47
		a := New(Options{Store: settings.NewStore(s)})
		a.Surf = SurfaceSettings
		setup(a)
		tt := ui.NewTester(a.View, 720, 700)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		return tt.Image().RGBAAt(x, y).A
	}

	for _, dark := range []bool{false, true} {
		want := render(dark, func(a *App) { a.Settings.Page = settingsui.PageGeneral })
		for _, tc := range []struct {
			name  string
			setup func(a *App)
		}{
			{"integrations", func(a *App) { a.Settings.Page = settingsui.PageIntegrations }},
			{"add tools", func(a *App) {
				a.Settings.Page = settingsui.PageIntegrations
				a.Settings.OpenAddTools()
			}},
			{"local tool", func(a *App) {
				a.Settings.Page = settingsui.PageIntegrations
				a.Settings.OpenAddTools()
				a.Settings.OpenLocalTool()
			}},
			{"local tool form", func(a *App) {
				a.Settings.Page = settingsui.PageIntegrations
				a.Settings.OpenAddTools()
				a.Settings.OpenLocalTool()
				a.Settings.OpenLocalToolForm()
			}},
		} {
			got := render(dark, tc.setup)
			if got != want {
				t.Errorf("dark=%v %s: page face at alpha %d, the General page at %d — the page lost the window's glass",
					dark, tc.name, got, want)
			}
		}
	}
}
