package settingsui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The P13-af visual evidence: the integration meta line at 460u, where every
// fact takes a line of its own, in both appearances.
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps: a test run
// should not litter /tmp. `FLOTER_PNG_SUFFIX` names the file, so the round can
// write the same surface from the old and the new code (`-before`).
func TestDumpP13AFMetaPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	suffix := os.Getenv("FLOTER_PNG_SUFFIX")
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a := oneIntegrationApp(metaFixture(), Actions{})
		a.Store = store
		a.selectPage(PageIntegrations)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			a.View(c)
		}, 460, 900)
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		name := "floter-P13-af-meta-" + appearance + suffix + ".png"
		writePNG(t, filepath.Join("/tmp", name), tt)
	}
}
