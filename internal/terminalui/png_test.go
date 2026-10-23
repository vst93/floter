package terminalui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// The P8-e visual evidence: the terminal page in both appearances, written to
// /tmp so the report can point at a file rather than at a description. What the
// two frames have to show is the bar's status dot — the live one, neutral and
// breathing, beside the session's name.
//
// The dump is opt-in (`FLOTER_PNG=1`) because a test run should not litter /tmp;
// the round that produced the report ran it once with the switch on.
func TestDumpTerminalPagePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	dir := "/tmp"
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{Settings: func() {}, Close: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		withTitle(a, "floter — zsh")
		a.Drawer = true
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokensOf(a, c).Theme)
			a.View(c)
		}, 900, 620)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		// The pointer rests on the bar so the identity row — and its status dot —
		// is in the picture: the dot is what P8-e §1 A is about.
		tt.Move(420, 12)
		tt.Frame()
		tt.Frame()
		path := filepath.Join(dir, "floter-p8e-terminal-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s (%dx%d)", path, 900, 620)
	}
}
