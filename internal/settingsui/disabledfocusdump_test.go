package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The opt-in dump for P13-gf: three boxes, the middle one disabled.
//
//	-stranded = the keyboard was on the middle box before it went disabled
//	-stepped  = the keyboard steps over it (Tab, Tab: the ring is on box 3)
//
// Run with FLOTER_P13GF_DUMP=1.
func TestDumpP13GFDisabledFocus(t *testing.T) {
	if os.Getenv("FLOTER_P13GF_DUMP") == "" {
		t.Skip("set FLOTER_P13GF_DUMP=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, arm := range []string{"stepped", "stranded"} {
			// In the stepped arm the middle box is disabled from the start, so
			// two Tabs put the ring on box 3. In the stranded arm it is still
			// enabled while the keyboard lands on it, and only then goes
			// disabled — the in-flight arm of every locked control.
			dis := arm == "stepped"
			tt, _ := lensProbe(t, dark, 260, 100, func(c *ui.Context, a *App) {
				ui.Row(c).Gap(8).Children(func() {
					for i := 0; i < 3; i++ {
						box := ui.Row(c).Focusable().Size(60, 40).Radius(6)
						if i == 1 {
							box.Disabled(dis)
						}
					}
				})
			})
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			tt.Key(0, ui.KeyTab)
			tt.Frame()
			if arm == "stranded" {
				dis = true
				tt.Frame()
			}
			name := filepath.Join(os.TempDir(), fmt.Sprintf("floter-P13-gf-disabledfocus-%s-%s.png", appearance, arm))
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", name)
		}
	}
}
