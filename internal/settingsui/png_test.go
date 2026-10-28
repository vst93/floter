package settingsui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/plugincfg"
	"floter/internal/settings"
	"floter/internal/theme"
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

// The P8-d visual evidence: §6 the task book's four frames — the integrations
// page (permission dots, the running state word and the healthy status word all
// in one screen) and the sessions page (the reset action and the resumable dot)
// — in both appearances, as the pixels the pins above are reading.
//
// Opt-in for the same reason as the P8-c dump: a test run should not litter
// /tmp, so the round that produced the report ran it once with `FLOTER_PNG=1`.
func TestDumpP8DIntegrationAndSessionPNGs(t *testing.T) {
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
		// ① The integrations page: the enforced/declared permission dots, the
		// running state word beside a disabled one, and the diagnostics block's
		// healthy summary — the three inks P8-d re-routed.
		record := &ApprovalRecord{
			At: 1700000000, ApprovedDigest: "sha256-aaaa", CurrentDigest: "sha256-aaaa",
			Enforced: []string{"clipboard-write"}, Disclosure: []string{"filesystem-read"},
		}
		a := New(store, Actions{})
		// The permission audit is folded away until asked for; this frame is the
		// one the task book wants (permission dots + the running word + the
		// healthy status word on one screen), so it is opened.
		a.auditOpen = map[string]bool{"dev.floter.on": true}
		a.Integrations = func() []Integration {
			return []Integration{
				{ID: "dev.floter.on", Name: "Plain Tool", Enabled: true, Running: true,
					RuntimeAvailable: true, Version: "1.0.0", Publisher: "vst",
					Description: "A tool that does one thing.", Approval: record,
					Permissions:     []string{"clipboard-write", "filesystem-read"},
					Enforced:        map[string]bool{"clipboard-write": true},
					DiagnosisStatus: "healthy",
					DiagnosisChecks: []DiagnosisCheck{{ID: "version", Status: "healthy", Message: "answered"}}},
				{ID: "dev.floter.off", Name: "Off Tool", Enabled: false,
					RuntimeAvailable: true, Version: "0.4.1", Description: "Turned off on purpose."},
				{ID: "dev.floter.bad", Name: "Broken Tool", Enabled: true, Broken: true,
					RuntimeAvailable: false, Version: "0.9.0", Description: "Its runtime is gone.",
					DiagnosisStatus: "error",
					DiagnosisChecks: []DiagnosisCheck{{ID: "version", Status: "error", Message: "no answer"}}},
			}
		}
		a.selectPage(PageIntegrations)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 860, 1400)
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-p8d-integrations-"+appearance+".png"), tt)

		// ② The sessions page: the reset action (neutral at rest and on hover)
		// and the resumable dot in the resume button's corner.
		sessions := settings.NewStore(settings.Default())
		if err := sessions.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		sa := New(sessions, Actions{SelectSession: func(int) {}, CloseSessionAt: func(int) {}})
		created := time.Date(2024, 1, 2, 15, 4, 0, 0, time.Local)
		sa.Sessions = func() []Session {
			return []Session{
				{ID: 1, Title: "zsh", Running: true, Index: 0,
					Note: "Running · attached", Size: "80×24", Created: created},
				{ID: 2, Title: "build", Running: false, Index: 1,
					Note: "Exited", Size: "100×30", Created: created},
			}
		}
		sa.selectPage(PageSessions)
		stt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(sa.tokens(c).Theme)
			sa.View(c)
		}, 860, 760)
		stt.SetDark(dark)
		stt.Frame()
		stt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-p8d-sessions-"+appearance+".png"), stt)
	}
}

// The P11-aa visual evidence: the settings page's General tab in four frames —
// {light, dark} × {more contrast off, more contrast on} — because this round is
// about **words** and the only honest way to show it is the ink they are painted
// in. The page's section hints and row sublabels are the quiet rungs
// (`TextTertiary` / `TextMuted`), which is exactly what the switch used to leave
// alone (light 3.04:1, dark 3.82:1).
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps.
func TestDumpP11AAHighContrastPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, contrast := range []bool{false, true} {
			state := "off"
			if contrast {
				state = "on"
			}
			store := settings.NewStore(settings.Default())
			if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
				t.Fatal(err)
			}
			a := New(store, Actions{})
			a.selectPage(PageGeneral)
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(a.tokens(c).Theme)
				a.View(c)
			}, 860, 1100)
			tt.SetDark(dark)
			tt.SetPreferences(ui.Preferences{HighContrast: contrast})
			tt.Frame()
			tt.Frame()
			path := filepath.Join("/tmp", "floter-P11-aa-contrast-"+appearance+"-"+state+".png")
			writePNG(t, path, tt)
			tokens := theme.ForPreferences(a.Store.Snapshot(), dark, ui.Preferences{HighContrast: contrast})
			t.Logf("%s: TextSecondary=%v TextTertiary=%v", path, tokens.TextSecondary, tokens.TextTertiary)
		}
	}
}

// The P9-c evidence: the settings page itself in both appearances, read through
// the same settled `renderIn` the dark subtests use. It exists because the dark
// half of those pins is a *picture*, and a picture is worth checking with eyes
// once per round even when the numbers are green.
func TestDumpTheSettingsPageInBothAppearances(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			a := New(newStore(t), Actions{})
			a.Integrations = func() []Integration {
				return []Integration{
					{ID: "dev.floter.plain", Name: "Plain Tool", Enabled: true, Running: true,
						RuntimeAvailable: true, Version: "1.0.0", Publisher: "vst",
						Description: "A tool that does one thing."},
					{ID: "dev.floter.bad", Name: "Broken Tool", Enabled: true, Broken: true,
						RuntimeAvailable: false, Version: "0.9.0", Description: "Its runtime is gone."},
				}
			}
			tt, _ := renderIn(t, a, 860, 900, appearance.dark)
			writePNG(t, filepath.Join("/tmp", "floter-P9-c-settings-"+appearance.name+".png"), tt)
		})
	}
}
