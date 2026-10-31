package settingsui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The opt-in dump for P13-at: the run-output block scrolled to its middle, both
// appearances. Run it with FLOTER_P13AT_DUMP=1, and FLOTER_P13AT_TAG=before
// while the two scroll edges are re-injected as absent, to get the comparison
// pair (before: 刀切; after: 两头退让).
func TestDumpP13ATRunOutput(t *testing.T) {
	if os.Getenv("FLOTER_P13AT_DUMP") == "" {
		t.Skip("set FLOTER_P13AT_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13AT_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		a, text := runOutputScrollApp(t)
		tt := integrationPage(t, a, 720, 2400)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		scroll := a.runOutputScroll("dev.floter.tool")
		if scroll.MaxY <= 0 {
			t.Fatalf("the output block does not overflow (MaxY = %v)", scroll.MaxY)
		}
		scroll.Y = scroll.MaxY / 2
		tt.Frame()
		tt.Frame()
		box := runOutputBox(t, tt, text)
		// The crop: the block plus a line of its own ground on each side, so
		// the reader can see both edges' neighbours.
		name := fmt.Sprintf("/tmp/floter-P13-at-runoutput-%s-%s.png", appearanceName(dark), tag)
		f, err := os.Create(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		crop := tt.Image()
		sub := crop.SubImage(image.Rect(0, box.Min.Y-24, crop.Bounds().Dx(), box.Max.Y+24))
		if err := png.Encode(f, sub); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", name)
	}
}
