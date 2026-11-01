package settingsui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// P13-cr 的截图：扩展页命令列表，深/浅两套外观的静息帧。
//
// 这一轮唯一的墨改是命令板上「Summon with: …」那一行（`TextMuted × 0.85` →
// `tokens.TextTertiary`）。出图是为了给「浅色几乎不动、深色退一档」一句像素证据：
//
//	FLOTER_P13CR_DUMP=before go test -count=1 -run TestDumpP13crTriggerRung ./internal/settingsui/
//	FLOTER_P13CR_DUMP=after  go test -count=1 -run TestDumpP13crTriggerRung ./internal/settingsui/
//
// 第二次跑（`after`）会自动读回 `before` 那张、打印差分像素数与包围盒。默认跳过。
func TestDumpP13crTriggerRung(t *testing.T) {
	suffix := os.Getenv("FLOTER_P13CR_DUMP")
	if suffix == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13CR_DUMP=<before|after> to write /tmp/floter-P13-cr-commands-*.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := New(newStore(t), Actions{})
		a.Integrations = func() []Integration { return []Integration{commandListIntegration()} }
		tt, _ := renderIn(t, a, 720, 900, dark)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()

		name := "/tmp/floter-P13-cr-commands-" + appearance + "-" + suffix + ".png"
		p13crWritePNG(t, name, tt.Image())
		if suffix != "after" {
			continue
		}
		before := p13crReadPNG(t, "/tmp/floter-P13-cr-commands-"+appearance+"-before.png")
		n, box := p13crPixelDiff(before, tt.Image())
		t.Logf("%s: %d px differ, bbox %v", appearance, n, box)
	}
}

func p13crWritePNG(t *testing.T, name string, img image.Image) {
	t.Helper()
	f, err := os.Create(name)
	if err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding %s: %v", name, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing %s: %v", name, err)
	}
}

func p13crReadPNG(t *testing.T, name string) image.Image {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("opening %s: %v", name, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decoding %s: %v", name, err)
	}
	return img
}

// p13crPixelDiff counts the pixels that moved by more than 8 levels in a channel
// and the box they occupy.
func p13crPixelDiff(before, after image.Image) (int, image.Rectangle) {
	b := after.Bounds()
	x0, y0, x1, y1 := 1<<30, 1<<30, -1, -1
	n := 0
	at := func(img image.Image, x, y int) color.RGBA {
		return color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p, q := at(before, x, y), at(after, x, y)
			if abs8(int(p.R)-int(q.R))+abs8(int(p.G)-int(q.G))+abs8(int(p.B)-int(q.B)) <= 8 {
				continue
			}
			n++
			if x < x0 {
				x0 = x
			}
			if y < y0 {
				y0 = y
			}
			if x > x1 {
				x1 = x
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	if n == 0 {
		return 0, image.Rectangle{}
	}
	return n, image.Rect(x0, y0, x1+1, y1+1)
}

func abs8(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
