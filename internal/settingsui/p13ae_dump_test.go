package settingsui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The P13-ae visual evidence: the two pages whose rows name something the
// program chose — the sessions list and the integrations list — each rendered
// with a name long enough to be interesting, once per appearance, so the
// report can point at a file instead of describing one.
//
// The dump is opt-in (`FLOTER_P13AE_DUMP=1`) because a test run should not
// litter /tmp; the round that produced the report ran it once with the switch
// on, and again with the fix reverted for the "before" frames.
func TestDumpP13aeLongNames(t *testing.T) {
	if os.Getenv("FLOTER_P13AE_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13AE_DUMP=1 to write /tmp/floter-P13-ae-*.png")
	}
	const (
		width = 460
		name  = "vst@build-host: ~/workspace/floter/internal/settingsui (mygo-rewrite)"
		pkg   = "com.vst93.floter.extensions.markdown-preview-themes"
	)
	for _, dark := range []bool{true, false} {
		dark := dark
		appearance := "light"
		if dark {
			appearance = "dark"
		}

		// ① The sessions page: a 44u row must stay 44u however long the name is.
		store := settings.NewStore(settings.Default())
		a := New(store, Actions{SelectSession: func(int) {}, CloseSessionAt: func(int) {}})
		created := time.Date(2024, 1, 2, 15, 4, 0, 0, time.Local)
		a.Sessions = func() []Session {
			return []Session{
				{ID: 1, Title: name, Running: true, Index: 0,
					Note: "Running · attached", Size: "80×24", Created: created,
					Dir: "/home/vst/workspace/floter/internal/settingsui"},
				{ID: 2, Title: "build", Running: false, Index: 1,
					Note: "Exited · exit 1", Size: "100×30", Created: created,
					Dir: "/home/vst/workspace/floter"},
			}
		}
		a.selectPage(PageSessions)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, width, 620)
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P13-ae-sessions-"+appearance+".png"), tt)

		// ② The integrations page: the name truncates, the version stays on the
		// title line beside it.
		ia := New(settings.NewStore(settings.Default()), Actions{})
		ia.Integrations = func() []Integration {
			return []Integration{{
				ID: "ext", Name: pkg, Version: "1.2.3", Publisher: "vst",
				Enabled: true, Running: true, RuntimeAvailable: true,
				Description: "Renders markdown previews with the theme of your choice.",
			}}
		}
		ia.selectPage(PageIntegrations)
		itt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(ia.tokens(c).Theme)
			ia.View(c)
		}, width, 620)
		itt.SetDark(dark)
		itt.Frame()
		itt.Frame()
		writePNG(t, filepath.Join("/tmp", "floter-P13-ae-integrations-"+appearance+".png"), itt)
	}
}
