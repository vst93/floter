package settingsui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
)

// The P8-c visual evidence: the integrations page and the plugin's
// configuration sheet in both appearances, written to /tmp so the report can
// point at a file rather than at a description.
//
// The dump is opt-in (`FLOTER_PNG=1`) because a test run should not litter /tmp;
// the round that produced the report ran it once with the switch on.
func TestDumpSettingsPagePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		a.Integrations = func() []Integration {
			return []Integration{
				{ID: "dev.floter.plain", Name: "Plain Tool", Enabled: true, Running: true,
					RuntimeAvailable: true, Version: "1.0.0", Publisher: "vst",
					Description: "A tool that does one thing."},
				{ID: "dev.floter.reg", Name: "Just Connected", Enabled: true, Running: true,
					RuntimeAvailable: true, Version: "2.1.0", Publisher: "vst", Highlighted: true,
					Description: "The row a floter://register link named."},
				{ID: "dev.floter.bad", Name: "Broken Tool", Enabled: true, Broken: true,
					RuntimeAvailable: false, Version: "0.9.0", Description: "Its runtime is gone."},
			}
		}
		a.selectPage(PageIntegrations)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 860, 900)
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		path := filepath.Join("/tmp", "floter-p8c-integrations-"+appearance+".png")
		writePNG(t, path, tt)
	}
}

// TestDumpPluginSheetPNGs draws the plugin's own configuration overlay — the
// third surface the task book asks for — in both appearances.
func TestDumpPluginSheetPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	copy := i18n.For("en").Settings
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		schema, values := plugincfg.Browser(copy, settings.DefaultBrowserPlugin(),
			[]i18n.Option{{ID: "brave", Label: "Brave"}, {ID: "safari", Label: "Safari"}})
		sheet := PluginSheet{
			Schema: schema, Values: values,
			Change: func(string, any) {}, Arm: func(string) {}, Run: func(string) {}, Unarm: func(string) {},
		}
		a := New(store, Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.PluginSheet(c, sheet)
		}, 720, 900)
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		path := filepath.Join("/tmp", "floter-p8c-sheet-"+appearance+".png")
		writePNG(t, path, tt)
	}
}

// writePNG encodes one tester frame to path and logs it, so a failing run names
// the file it could not write.
func writePNG(t *testing.T, path string, tt *ui.Tester) {
	t.Helper()
	f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, tt.Image()); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	img := tt.Image()
	t.Logf("%s (%dx%d)", path, img.Rect.Dx(), img.Rect.Dy())
}
