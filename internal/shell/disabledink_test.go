package shell

import (
	"fmt"
	"image/png"
	"os"
	"testing"
)

func TestDumpP13HLRunningOutput(t *testing.T) {
	if os.Getenv("FLOTER_P13HL_DUMP") == "" {
		t.Skip("opt-in screenshot: FLOTER_P13HL_DUMP=1")
	}
	tag := os.Getenv("FLOTER_P13HL_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tt, _ := barDetached(t, dark, true)
		name := fmt.Sprintf("/tmp/floter-P13-hl-output-%s-%s.png", appearance, tag)
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
		t.Log(name)
	}
}
