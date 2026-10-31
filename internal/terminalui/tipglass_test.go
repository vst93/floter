package terminalui

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// tipWords is the tip the P13-ax pin rests on.
const tipWords = "the quick brown fox"

// tipGround is the opaque ground each appearance's float face is measured
// over, so the composited colour is the token arithmetic itself rather than
// whatever the window's own glass adds.
func tipGround(dark bool) ui.Color {
	if dark {
		return ui.RGBA(20, 21, 24, 1)
	}
	return ui.RGBA(247, 247, 249, 1)
}

// tipFrame renders one labelled anchor carrying a `theme.Tokens.Tip` over the
// appearance's ground. rest is how long the pointer stays on the anchor: the
// tip shows once it has rested 0.6s (`ui/tooltip.go:13-16`), so 0 is the before
// picture and 700ms the after one.
func tipFrame(t *testing.T, tok theme.Tokens, dark bool, rest time.Duration) *ui.Tester {
	t.Helper()
	ground := tipGround(dark)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tok.Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(60, 40).Children(func() {
			anchor := ui.ButtonBase(c).Label("anchor").Width(120).Height(30).
				Background(tok.Control)
			tok.Tip(c, anchor, tipWords)
		})
	}, 460, 380)
	tt.SetPreferences(ui.Preferences{})
	tt.Frame()
	r, ok := tt.Find("anchor")
	if !ok {
		t.Fatal("the anchor is not in the frame")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if rest > 0 {
		tt.Advance(rest)
		tt.Frame()
	}
	return tt
}

// tipBox is the float face's pixel box, in the tester's logical coordinates.
type tipBox struct{ x0, y0, x1, y1 int }

func (b tipBox) cx() int { return (b.x0 + b.x1) / 2 }
func (b tipBox) cy() int { return (b.y0 + b.y1) / 2 }

// tipFace returns how many pixels carry the float face — the one colour
// `FaceOver(ground, Float)` predicts — and their box. That colour being there
// *is* the E2 face (§1 「浮」= 窗框 tint + floatStep，永远比它压着的那张面密).
func tipFace(tt *ui.Tester, want ui.Color) (int, tipBox) {
	img := tt.Image()
	box := tipBox{x0: 1 << 30, y0: 1 << 30, x1: -1, y1: -1}
	count := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if !near(pxTip(img, x, y), want, 2) {
				continue
			}
			count++
			if x < box.x0 {
				box.x0 = x
			}
			if y < box.y0 {
				box.y0 = y
			}
			if x > box.x1 {
				box.x1 = x
			}
			if y > box.y1 {
				box.y1 = y
			}
		}
	}
	return count, box
}

func pxTip(img *image.RGBA, x, y int) ui.Color {
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// near reports whether two colours agree to within tol on every channel.
func near(a, b ui.Color, tol int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x) - int(y)
		}
		return int(y) - int(x)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

func lum(c ui.Color) int { return (int(c.R) + int(c.G) + int(c.B)) / 3 }

// TestTheTipIsAFloatingPane is the P13-ax pin for `theme.Tokens.Tip`
// (`docs/liquid-glass.md` §6.20): the tip is an E2 float — the `Float` face
// stepping off the ground, one lit top edge, and two shadows — instead of the
// framework's **opaque inverse chip** with a single blur and no edge.
//
// 旧值 → 新值（深色）：面 `Text`（近白 ≈229）→ `Float`（压在 20/21/24 的地上，
// ≈33）；圆角 5（`KeycapRadius`）→ 8（`RadiusSM`）；影一层 `0 2px 8px 0.2` →
// 两层（环境 `0 6px 18px 0.30` + 接触 `0 1px 2px 0.20`）；边 无 → `InputStroke`
// 发丝一圈 + 顶边 `EdgeHighlight`。
func TestTheTipIsAFloatingPane(t *testing.T) {
	for _, dark := range []bool{true, false} {
		name := "light"
		if dark {
			name = "dark"
		}
		tok := theme.ForPreferences(settings.Settings{}, dark, ui.Preferences{})
		t.Run(name, func(t *testing.T) {
			face := theme.FaceOver(tipGround(dark), tok.Float)

			// The material changed; the clock did not. Before 0.6s the frame is
			// the bare ground: no tip, not even a tinted pane.
			early := tipFrame(t, tok, dark, 400*time.Millisecond)
			if early.HasText(tipWords) {
				t.Fatal("the tip shows after 0.4s: the framework's 0.6s rest is gone")
			}
			if n, _ := tipFace(early, face); n > 0 {
				t.Fatalf("a float face is painted before the tip shows (%d pixels)", n)
			}

			tt := tipFrame(t, tok, dark, 700*time.Millisecond)
			if !tt.HasText(tipWords) {
				t.Fatal("no tip after the pointer rested on the anchor")
			}
			if dump := os.Getenv("FLOTER_P13AX_DUMP"); dump != "" {
				f, err := os.Create("/tmp/floter-P13-ax-tip-" + name + ".png")
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
			n, box := tipFace(tt, face)
			if n < 500 {
				t.Fatalf("the tip's face is %d pixels of FaceOver(ground, Float): the tip is not wearing the float face", n)
			}
			img := tt.Image()
			facePx := pxTip(img, box.x0+3, box.cy()) // clear of the glyphs
			if !near(facePx, face, 2) {
				t.Fatalf("the tip's middle is %v, not the float face %v", facePx, face)
			}

			// A float never reads brighter than what it covers: the framework's
			// chip was the inverse pair, i.e. an opaque near-white pane on the
			// dark appearance (≈229 here).
			if dark && lum(facePx) > 90 {
				t.Fatalf("the tip's face is %v (luminance %d): that is the inverse chip, not the float face",
					facePx, lum(facePx))
			}

			// The lit top edge: `EdgeHighlight` 1px along the top of the pane,
			// brighter than the face it crowns. `FaceOver`'s exact colour starts
			// one row in (the crown covers the first), so read it one row up.
			// The light appearance is exempt and documented: a near-white crown
			// on a near-white pane is 白压白 — there the hairline speaks instead
			// (§3 「浅色的受光边只能是近白…它靠对侧的 EdgeShadow 才成立」).
			if dark {
				crown := pxTip(img, box.x0+3, box.y0-1)
				if lum(crown)-lum(facePx) < 6 {
					t.Fatalf("the tip's top edge is %v against the face %v (%+d levels): no lit edge",
						crown, facePx, lum(crown)-lum(facePx))
				}
			}

		})
	}
}
