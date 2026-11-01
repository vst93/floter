package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The opt-in dump for P13-aw: the plugin sheet scrolled to its middle, both
// appearances. Run it with FLOTER_P13AW_DUMP=1, and FLOTER_P13AW_TAG=before
// while the two scroll edges are re-injected as absent, to get the comparison
// pair (before: 刀切; after: 两头退让).
func TestDumpP13AWPluginSheet(t *testing.T) {
	if os.Getenv("FLOTER_P13AW_DUMP") == "" {
		t.Skip("set FLOTER_P13AW_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13AW_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		tt, a := pluginSheetFrame(t, dark)
		scroll := &a.PluginScroll
		if scroll.MaxY <= 0 {
			t.Fatalf("the sheet does not overflow (MaxY = %v)", scroll.MaxY)
		}
		scroll.Y = scroll.MaxY / 2
		tt.Frame()
		tt.Frame()
		name := fmt.Sprintf("/tmp/floter-P13-aw-pluginsheet-%s-%s.png", appearanceName(dark), tag)
		f, err := os.Create(filepath.Clean(name))
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

// The opt-in dump for P13-dc: the sheet parked at its start, both appearances,
// so the header and the group headings are in frame. Run with FLOTER_P13DC_DUMP=1;
// FLOTER_P13DC_TAG=before while `pluginsheet.go` still says `FontSize(t.FontSize+2)`
// gives the comparison pair.
func TestDumpP13DCPluginSheetLadder(t *testing.T) {
	if os.Getenv("FLOTER_P13DC_DUMP") == "" {
		t.Skip("set FLOTER_P13DC_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13DC_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		tt, a := pluginSheetFrame(t, dark)
		a.PluginScroll.Y = 0
		tt.Frame()
		tt.Frame()
		name := fmt.Sprintf("/tmp/floter-P13-dc-pluginsheet-%s-%s.png", appearanceName(dark), tag)
		f, err := os.Create(filepath.Clean(name))
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
