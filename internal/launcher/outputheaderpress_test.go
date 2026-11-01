package launcher

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-bw · 捕获输出页页头的两枚 28u 方键：指针的第三档
// （`docs/rounds/P13-bw-spec.md`、`docs/liquid-glass.md` §6.40 第 40-3 行）。
//
// 这一族（`hover=2 / press=0`）在本轮之前只剩这两枚键：`output.go` 的 `control`
// 闭包只有一条 `if button.Hovered()` 臂，**没有 `Pressed()` 分支**——按住不放的那
// 一帧与悬停帧逐像素相同。用户读到的不是「按到了」，而是「按坏了、没反应」。
//
// 量四件事（深/浅各一遍，全部读像素）：
//
//	① 悬停真的画了面（悬停面 ≠ 条子自己的面）；
//	② 按下真的画了**另一个**面（按下面 ≠ 悬停面）——旧实现这一条逐像素相同；
//	③ 按下的面在**条子面与悬停面之间**（第三档是「往静息的回退」，
//	   `docs/liquid-glass.md` §5 的 `ControlPress`，同 P13-bv）；
//	④ 按下只改自己那一块：条子自己的面与键的布局盒在按下帧里一个字节都没动。
//
// 出图：`FLOTER_PNG=1 go test ./internal/launcher/ -run TestDumpP13BW`，
// 落到 `/tmp/floter-P13-bw-outputheader-<light|dark>-<rest|hover|press>.png`。

// outputHeaderProbe is how far inside the 28u key the material is read: the mark
// is centred and 28u is wide, so 3px in is the face, never the glyph.
const outputHeaderProbe = 3

// outputHeaderState is one pointer state's reading: the key's face, the strip's
// own plane beside it (the ground both faces are graded against), and the box.
type outputHeaderState struct {
	key   ui.Color
	strip ui.Color
	box   ui.Rect
}

// outputHeaderFrame draws the capture-output page with both header keys present.
// The strip's own plane never changes between the three frames, so the pointer's
// state is the only thing that moves.
func outputHeaderFrame(t *testing.T, dark bool) (*ui.Tester, string) {
	t.Helper()
	a := testApp()
	a.Actions.PinText = func(string, string) {}
	a.Actions.DetachOutput = func(OutputView) {}
	tt := runListOutput(t, a, `[{"id":"r0","title":"Alpha"}]`)
	tt.SetDark(dark)
	tt.Frame()
	return tt, a.copy().HistoryPin
}

// outputHeaderRead parks the pointer and reads the two faces.
func outputHeaderRead(t *testing.T, dark bool, state string) outputHeaderState {
	t.Helper()
	tt, label := outputHeaderFrame(t, dark)
	box, ok := tt.Find(label)
	if !ok {
		t.Fatalf("dark=%v: the output strip's pin key is not drawn: %q", dark, tt.Texts())
	}
	x, y := int(box.X)+outputHeaderProbe, int(box.Y+box.H/2)
	// The strip's own plane is read to the left of the key's box, on the same row:
	// the title's ellipsis never reaches there.
	groundX := int(box.X) - 20
	switch state {
	case "rest":
		tt.Move(float32(groundX), float32(y))
		tt.Frame()
	case "hover":
		tt.Move(float32(x), float32(y))
		tt.Frame()
	case "press":
		tt.Move(float32(x), float32(y))
		tt.Frame()
		tt.Press(float32(x), float32(y))
		// No colour transition is set on this key, so the frame after the press is
		// the settled face; the second frame only proves it is a state.
		tt.Frame()
		tt.Frame()
	default:
		t.Fatalf("unknown state %q", state)
	}
	return outputHeaderState{
		key:   outputHeaderPixel(tt.Image(), x, y),
		strip: outputHeaderPixel(tt.Image(), groundX, y),
		box:   box,
	}
}

func outputHeaderPixel(img image.Image, x, y int) ui.Color {
	r, g, b, _ := img.At(x, y).RGBA()
	return ui.RGBA(uint8(r>>8), uint8(g>>8), uint8(b>>8), 255)
}

// outputHeaderDelta is how far apart two colours are, in levels of their widest
// channel.
func outputHeaderDelta(a, b ui.Color) int {
	d := func(x, y uint8) int {
		if x > y {
			return int(x) - int(y)
		}
		return int(y) - int(x)
	}
	dr, dg, db := d(a.R, b.R), d(a.G, b.G), d(a.B, b.B)
	if dg > dr {
		dr = dg
	}
	if db > dr {
		dr = db
	}
	return dr
}

// outputHeaderLuma is the perceptual weight of a colour, used only to ask "which
// side of which" — never to assert an exact composited value.
func outputHeaderLuma(c ui.Color) int { return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000 }

// outputHeaderBetween reports whether mid's luma sits strictly between a and b.
func outputHeaderBetween(a, mid, b ui.Color) bool {
	lo, hi := outputHeaderLuma(a), outputHeaderLuma(b)
	if lo > hi {
		lo, hi = hi, lo
	}
	return outputHeaderLuma(mid) > lo && outputHeaderLuma(mid) < hi
}

// TestTheOutputHeaderKeysSinkWhenThePointerGoesDown is ① ② ③ ④ on both keys.
func TestTheOutputHeaderKeysSinkWhenThePointerGoesDown(t *testing.T) {
	for _, dark := range []bool{false, true} {
		rest := outputHeaderRead(t, dark, "rest")
		hover := outputHeaderRead(t, dark, "hover")
		press := outputHeaderRead(t, dark, "press")
		t.Logf("dark=%v: strip %v rest %v hover %v press %v",
			dark, rest.strip, rest.key, hover.key, press.key)
		if outputHeaderDelta(hover.key, rest.key) < 4 {
			t.Errorf("dark=%v: the hovered key %v is the strip's own plane %v — the hover arm painted "+
				"nothing, so this pin would be grading the strip, not the key", dark, hover.key, rest.key)
			continue
		}
		if outputHeaderDelta(press.key, hover.key) < 3 {
			t.Errorf("dark=%v: the key under the pointer is %v whether or not the button is down (strip %v): "+
				"the press rung paints no face of its own — one step back toward the strip is what "+
				"`--glass-control-press` means (docs/liquid-glass.md §5, §6.40 第 40-3 行)",
				dark, press.key, rest.key)
			continue
		}
		if !outputHeaderBetween(rest.key, press.key, hover.key) {
			t.Errorf("dark=%v: the pressed face %v is not between the strip's own plane %v and the hovered %v "+
				"— the third rung is a step back toward rest, not a third colour",
				dark, press.key, rest.key, hover.key)
		}
		if press.strip != hover.strip {
			t.Errorf("dark=%v: the press repainted the strip itself (%v → %v) — the third rung belongs to "+
				"the key's own box, not to the band it sits on", dark, hover.strip, press.strip)
		}
		if press.box != hover.box {
			t.Errorf("dark=%v: the press moved the key's layout box (%v → %v) — the scale is a transform, "+
				"the strip must not re-flow under the pointer", dark, hover.box, press.box)
		}
	}
}

// TestDumpP13BW writes the three frames of both appearances for the eye.
func TestDumpP13BW(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to dump the P13-bw frames")
	}
	for _, dark := range []bool{false, true} {
		for _, state := range []string{"rest", "hover", "press"} {
			tt, label := outputHeaderFrame(t, dark)
			box, ok := tt.Find(label)
			if !ok {
				t.Fatalf("the output strip's pin key is not drawn: %q", tt.Texts())
			}
			x, y := int(box.X)+outputHeaderProbe, int(box.Y+box.H/2)
			switch state {
			case "rest":
				tt.Move(float32(int(box.X)-20), float32(y))
				tt.Frame()
			case "hover":
				tt.Move(float32(x), float32(y))
				tt.Frame()
			case "press":
				tt.Move(float32(x), float32(y))
				tt.Frame()
				tt.Press(float32(x), float32(y))
				tt.Frame()
				tt.Frame()
			}
			name := "light"
			if dark {
				name = "dark"
			}
			path := fmt.Sprintf("/tmp/floter-P13-bw-outputheader-%s-%s.png", name, state)
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(file, tt.Image()); err != nil {
				file.Close()
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
