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

// The P11-z visual evidence: the terminal page in both appearances, written to
// /tmp so the report can point at a file rather than at a description. What the
// two frames have to show is the panel's frame — the canvas used to draw its own
// 1px stroke a rung inside the panel's, which read as the 「类玻璃边」 the user
// reported; the panel is now the only frame and the canvas is one fill.
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps: a test run
// should not litter /tmp.
func TestDumpP11ZTerminalPagePNGs(t *testing.T) {
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
		a := New(store, Actions{Settings: func() {}, Close: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		withTitle(a, "floter — zsh")
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokensOf(a, c).Theme)
			a.View(c)
		}, 900, 620)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P11-z-terminal-"+appearance+".png")
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

// ⑥ P11-z（用户实测 F3）· The canvas draws **no second frame**: the panel is the
// page's one frame, and the mount inside it is a fill.
//
// The view used to paint `Border(1, InputStrokeActive)` on the mount — a 1px
// ring a rung inside the panel's own 12px corner, in the accent's red while the
// page had the keyboard. Two rounded frames around one rectangle is what the
// user read as 「终端页面外圈那圈类玻璃边」; the accent half of it is F2's.
//
// The pin reads the page's outermost column at mid height: with the ring it was
// the stroke blended over the canvas fill, without it the column is the canvas
// fill itself, i.e. the same pixel as a few columns in.
func TestTheTerminalCanvasDrawsNoSecondFrame(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := New(newStore(), Actions{}, nil)
		withSession(a, &terminal.Terminal{})
		tt, _ := raycastFrame(t, a, dark, 900, 620)
		img := tt.Image()
		midY := img.Rect.Dy() / 2
		for _, side := range []struct{ x, inward int }{{0, 4}, {img.Rect.Dx() - 1, -4}} {
			x := side.x
			edge, face := pixel(tt, x, midY), pixel(tt, x+side.inward, midY)
			if delta(edge, face) > 2 {
				t.Errorf("dark=%v: the canvas's column %d is %v but its face is %v — a stroke is drawn there "+
					"(the panel is the page's one frame, §6.2 第 1 条 / P11-z F3)", dark, x, edge, face)
			}
			// ...and no red: F2 retired the accent from this ring, so even if a
			// stroke came back it must not be the accent's red.
			if warm := int(edge.R) - int(edge.B); warm > 16 {
				t.Errorf("dark=%v: the canvas's column %d is %d levels warm (R−B): an accent ring is back", dark, x, warm)
			}
		}
	}
}
