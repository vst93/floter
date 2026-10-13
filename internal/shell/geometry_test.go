package shell

import (
	"testing"

	"github.com/egoist/mygo"
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
				Font: a.Launcher.Font, Spacing: a.Launcher.Spacing,
				TitleLine: a.Launcher.TitleLine, SubtitleLine: a.Launcher.SubtitleLine,
				Rows: a.Launcher.Rows, Held: a.Launcher.HeldRows,
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

// The shell sizes the window with the same geometry the view measures with:
// the line box it measured and the sheet it is showing, so the window opens at
// the right height rather than converging on it a frame later.
func TestTargetSizeUsesTheMeasuredGeometry(t *testing.T) {
	a := New(Options{Store: settings.NewStore(settings.Default()), WorkAreaHeight: 900})
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, launcher.InputWindowWidth, 1000)
	tt.Frame()
	_, height := a.targetSize(a.Store.Snapshot())
	// The view's own target and the shell's agree: the same geometry.
	want := launcher.Geometry{
		Font: a.Launcher.Font, Spacing: a.Launcher.Spacing,
		TitleLine: a.Launcher.TitleLine, SubtitleLine: a.Launcher.SubtitleLine,
		Rows: a.Launcher.Rows, Held: a.Launcher.HeldRows,
		Filter: a.Launcher.FiltersVisible(), Feedback: a.Launcher.FeedbackVisible(),
		Cap: 900 - 24,
	}.Height()
	if height != want {
		t.Errorf("the shell sizes %d, the view asks for %d", height, want)
	}
	// And with a sheet open, the shell's height is the sheet's.
	a.Launcher.EnterClipboard()
	a.Launcher.OpenConfig(settings.CustomPluginClipboard)
	tt.Frame()
	_, sheetHeight := a.targetSize(a.Store.Snapshot())
	wantSheet := launcher.Geometry{
		Font: a.Launcher.Font, Spacing: a.Launcher.Spacing,
		TitleLine: a.Launcher.TitleLine, SubtitleLine: a.Launcher.SubtitleLine,
		Rows: a.Launcher.Rows, Held: a.Launcher.HeldRows,
		Filter: a.Launcher.FiltersVisible(), Feedback: a.Launcher.FeedbackVisible(),
		Config: a.Launcher.ConfigSchema(), Cap: 900 - 24,
	}.Height()
	if sheetHeight != wantSheet {
		t.Errorf("with a sheet the shell sizes %d, the view asks for %d", sheetHeight, wantSheet)
	}
	// The sheet's height is the sheet's own: the rows the list is holding
	// behind it are not charged, since the sheet takes the list's place.
	if sheetHeight == height {
		t.Errorf("the sheet's window is the list's %d", height)
	}
}

// The settings card's header and the launcher's field row are one band. The
// old build pinned both to `SEARCH_FIELD_HEIGHT_UNITS` (56u, R37) so switching
// surfaces does not move the window's first band; the user asked for it again
// (「设置页面的头部栏高度要和搜索框一致，这样才有更强的一体性」). The
// pixel-level check is the two rows' centres: the field's line box and the
// header's title sit on the same line, at the same band height, on both
// appearances.
func TestSettingsHeaderSharesTheLauncherFieldRowBand(t *testing.T) {
	store := settings.NewStore(settings.Default())
	a := New(Options{Store: store})

	a.Surf = SurfaceLauncher
	launcherFrame := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 420)
	launcherFrame.Frame()
	field, ok := launcherFrame.Find("Search")
	if !ok {
		t.Fatalf("no search field in %q", launcherFrame.Texts())
	}

	a.Surf = SurfaceSettings
	settingsFrame := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 580)
	settingsFrame.Frame()
	title, ok := settingsFrame.Find("Settings")
	if !ok {
		t.Fatalf("no settings header in %q", settingsFrame.Texts())
	}

	fieldCentre := field.Y + field.H/2
	titleCentre := title.Y + title.H/2
	t.Logf("launcher field: y=%.1f h=%.1f centre=%.1f", field.Y, field.H, fieldCentre)
	t.Logf("settings title: y=%.1f h=%.1f centre=%.1f", title.Y, title.H, titleCentre)
	if diff := fieldCentre - titleCentre; diff > 0.6 || diff < -0.6 {
		t.Errorf("the settings header's centre %.1f does not match the launcher field's %.1f", titleCentre, fieldCentre)
	}
}

// The panel is re-homed on the display it is already on, so a window the user
// carried to a second monitor is placed there rather than yanked back to the
// first. A window whose centre is on no display at all falls back to the
// primary one.
func TestWindowDisplay(t *testing.T) {
	primary := mygo.Display{
		ID: 1, Primary: true,
		Bounds: mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080},
	}
	second := mygo.Display{
		ID:     2,
		Bounds: mygo.Rectangle{X: 1920, Y: 0, Width: 2560, Height: 1440},
	}
	displays := []mygo.Display{primary, second}

	if got, ok := displayContaining(displays, 100, 100, 720, 400); !ok || got.ID != 1 {
		t.Errorf("a window on the first display resolved to %+v (%v)", got, ok)
	}
	if got, ok := displayContaining(displays, 2000, 200, 720, 400); !ok || got.ID != 2 {
		t.Errorf("a window on the second display resolved to %+v (%v)", got, ok)
	}
	// A window straddling the seam belongs to the display its centre is on.
	if got, ok := displayContaining(displays, 1560, 100, 720, 400); !ok || got.ID != 2 {
		t.Errorf("a straddling window resolved to %+v (%v)", got, ok)
	}
	// Nowhere: no display claims it, and the caller falls back.
	if _, ok := displayContaining(displays, -5000, -5000, 720, 400); ok {
		t.Error("a window off every display was claimed")
	}
	if _, ok := displayContaining(nil, 100, 100, 720, 400); ok {
		t.Error("a machine with no displays claimed a window")
	}
}
