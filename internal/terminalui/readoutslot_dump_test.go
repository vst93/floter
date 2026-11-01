package terminalui

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The P13-co visual evidence: the drawer's font-size row at both ends of its
// range, in both appearances, written to /tmp so the report can point at a file
// rather than at a description. What the pairs have to show is the track: it
// stands at the same x in both frames, and only the number in its slot changes.
//
// Opt-in (`FLOTER_PNG=1`), like every other dump in this package.
func TestDumpDrawerReadoutPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	suffix := os.Getenv("FLOTER_PNG_SUFFIX")
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, size := range []int{settings.MinFontSize, settings.MaxFontSize} {
			store := settings.NewStore(settings.Default())
			if err := store.Update(func(s *settings.Settings) {
				s.Theme = appearance
				s.FontSize = size
			}); err != nil {
				t.Fatal(err)
			}
			a := drawerApp(t)
			a.Store = store
			var tok theme.Tokens
			tt := ui.NewTester(func(c *ui.Context) {
				tok = tokensOf(a, c)
				c.SetTheme(tok.Theme)
				c.Root().Background(ui.RGBA(0, 0, 0, 1))
				a.drawer(c, i18n.For("en").Terminal)
			}, 900, 400)
			tt.Frame()
			path := fmt.Sprintf("/tmp/floter-P13-co-readout-%s-%d%s.png", appearance, size, suffix)
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", path)
		}
	}
}
