package launcher

import (
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P11-cm visual evidence: the power confirmation's band over a full list, at
// the window height the shell charges for it — before (the shell's predicate
// left `pendingPower` out, so the window is a band short and the lowest row
// slides under the band) and after (one predicate, one height).
//
// Opt-in: `FLOTER_PNG=1 go test -run TestDumpP11CMBandPNGs ./internal/launcher/`
// writes /tmp/floter-P11-cm-band-<appearance>-<before|after>.png.
func TestDumpP11CMBandPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P11-cm-band-<appearance>-<before|after>.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, before := range []bool{true, false} {
			a := testApp()
			recentPage(a, 12)
			var charged int
			a.Actions.ResizeTo = func(h int) { charged = h }
			tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, InputWindowWidth, frameHeight(maxResultSlots))
			tt.SetDark(dark)
			tt.Frame()
			a.runPower(PowerRestart)
			tt.Frame()
			tt.Frame()

			// The shell's number for this state, as it was before the fix
			// (a predicate without `pendingPower`) or as it is now.
			shell := charged
			if before {
				shell = bandHeight(a, a.toast != "" || a.heldHint())
			}
			tt.SetSize(InputWindowWidth, shell)
			tt.Frame()
			tt.Frame()

			band, ok := tt.Find(a.powerMessage())
			if !ok {
				t.Fatalf("%s: the confirmation is not drawn: %q", appearance, tt.Texts())
			}
			var lowest string
			var lowestBottom float32
			for _, item := range a.Results() {
				if item.heading || item.status {
					continue
				}
				if rect, ok := tt.Find(item.Title); ok {
					if bottom := rect.Y + rect.H; bottom > lowestBottom {
						lowest, lowestBottom = item.Title, bottom
					}
				}
			}
			state := "after"
			if before {
				state = "before"
			}
			img := tt.Image()
			path := fmt.Sprintf("/tmp/floter-P11-cm-band-%s-%s.png", appearance, state)
			if img == nil {
				t.Fatalf("%s: no image", appearance)
			}
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s %s: window %d, band y=%.0f..%.0f, lowest row %q ends at %.0f (%s) -> %s",
				appearance, state, shell, band.Y, band.Y+band.H, lowest, lowestBottom,
				map[bool]string{true: "UNDER the band", false: "above the band"}[lowestBottom > band.Y],
				path)
		}
	}
}
