package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"floter/internal/settings"
)

// TestDumpP13DDFontLadder is this round's visual evidence: the General page at
// one interface step, in the frame production draws in. Set FLOTER_P13DD_DUMP=1
// to write it; FLOTER_P13DD_SCALE picks the step (tiny / small) and
// FLOTER_P13DD_TAG the file's suffix (before / after).
func TestDumpP13DDFontLadder(t *testing.T) {
	if os.Getenv("FLOTER_P13DD_DUMP") == "" {
		t.Skip("set FLOTER_P13DD_DUMP=1 to write the round's visual evidence")
	}
	scale := os.Getenv("FLOTER_P13DD_SCALE")
	if scale == "" {
		scale = "small"
	}
	tag := os.Getenv("FLOTER_P13DD_TAG")
	if tag == "" {
		tag = "after"
	}
	s := settings.Default()
	s.UIScale = scale
	for _, dark := range []bool{false, true} {
		if dark {
			s.Theme = "dark"
		} else {
			s.Theme = "light"
		}
		a := New(settings.NewStore(s), Actions{})

		tt := renderThemed(t, a, 760, 900)
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		name := filepath.Join(os.TempDir(),
			fmt.Sprintf("floter-P13-dd-settings-%s-%s-%s.png", scale, appearance, tag))
		f, err := os.Create(name)
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
		t.Logf("wrote %s", name)
	}
}

// TestDumpP13DDTextBoxes prints every drawn text's box, so two trees' frames can
// be compared without looking at antialiasing.
func TestDumpP13DDTextBoxes(t *testing.T) {
	if os.Getenv("FLOTER_P13DD_DUMP") == "" {
		t.Skip("opt-in")
	}
	s := settings.Default()
	a := New(settings.NewStore(s), Actions{})
	tt := renderThemed(t, a, 760, 900)
	for _, text := range tt.Texts() {
		if box, ok := tt.Find(text); ok {
			t.Logf("BOX %-40q %7.1f %7.1f %7.1f %7.1f", text, box.X, box.Y, box.W, box.H)
		}
	}
}
