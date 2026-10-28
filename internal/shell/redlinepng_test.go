package shell

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/launcher"
	"floter/internal/settings"
)

// The P11-z visual evidence: the launcher card **with the keyboard** in both
// appearances, written to /tmp so the report can point at a file rather than at
// a description. What the two frames have to show is the card's one edge: it
// was the accent's red (`InputStrokeActive`, 0.55) and is now the neutral
// focused hairline (0.16) — the user's 「整体红色线条太扎眼」.
//
// Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps: a test run
// should not litter /tmp.
func TestDumpP11ZLauncherCardPNGs(t *testing.T) {
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
		a := New(Options{Store: store})
		a.cardFocus = func() bool { return true }
		tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, launcher.InputWindowWidth, 420)
		tt.SetDark(dark)
		tt.Frame()
		tt.Type("ls")
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P11-z-red-"+appearance+".png")
		f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
		if err != nil {
			t.Fatal(err)
		}
		// The card is translucent (the window shows the desktop through it), and
		// a tester has no desktop behind it: composite the frame onto a neutral
		// one so the edge the report is about is a readable pixel rather than a
		// premultiplied alpha.
		desktop := image.NewRGBA(image.Rect(0, 0, tt.Image().Rect.Dx(), tt.Image().Rect.Dy()))
		backdrop := color.RGBA{R: 0xDF, G: 0xDF, B: 0xE4, A: 0xFF}
		if dark {
			backdrop = color.RGBA{R: 0x1C, G: 0x1C, B: 0x20, A: 0xFF}
		}
		draw.Draw(desktop, desktop.Rect, &image.Uniform{C: backdrop}, image.Point{}, draw.Src)
		draw.Draw(desktop, desktop.Rect, tt.Image(), image.Point{}, draw.Over)
		if err := png.Encode(f, desktop); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
		t.Logf("%s (%dx%d)", path, tt.Image().Rect.Dx(), tt.Image().Rect.Dy())
	}
}
