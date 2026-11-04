package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The opt-in dump for P13-gg: the real settings page, four stops of the same
// walk.
//
//	-top     = three tabs: the first screenful, body at 0
//	-scroll  = four tabs: the first stop that scrolls (body at 26.5)
//	-deep    = sixteen tabs: the last stop of the page (body at 1210)
//	-wrap    = seventeen tabs: the keyboard back at the top (body at 184)
//
// Run with FLOTER_P13GG_DUMP=1.
func TestDumpP13GGTabWalk(t *testing.T) {
	if os.Getenv("FLOTER_P13GG_DUMP") == "" {
		t.Skip("set FLOTER_P13GG_DUMP=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a, tt := tabWalkPage(t, dark)
		emit := func(name string) {
			path := filepath.Join(os.TempDir(), fmt.Sprintf("floter-P13-gg-tabwalk-%s-%s.png", appearance, name))
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s body Y=%.1f -> %s", appearance, a.Body.Y, path)
		}
		tabWalkSteps(t, a, tt, 3, false)
		emit("top")
		tabWalkSteps(t, a, tt, 1, false)
		emit("scroll")
		tabWalkSteps(t, a, tt, 12, false)
		emit("deep")
		tabWalkSteps(t, a, tt, 1, false)
		emit("wrap")
	}
}
