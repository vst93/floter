package terminalui

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
)

// P13-ex · 抽屉的满态下沿（`docs/liquid-glass.md` §6.119）。
//
// P11-au 把条带本身做成了滚动容器，于是它有了 `MaxY > 0` 的满态；但两端都没有
// 边。滚到中间时网格最后一行在条带下沿（= 窗口底边）被**水平切断**，墨一点没退
// ——「滚动收口」这一族（§6.16–§6.19、§6.113）里最后一把刀。
//
// 两条 pin：
//  1. 下沿那一行必须是**退后**的，不是满对比；同一帧里、边上四行之外必须有满
//     对比的墨（对照，否则探针落在空白上，这条 pin 会白绿）；
//  2. 边是几何自门控的：滚到底时最后一行停在离下沿 9px 处，5.4px 的边整个落在
//     空白上，那几行必须读成裸面（与 §6.19 同一条，不读 `ScrollState`）。

// drawerLuma is one pixel's brightness, as the settings surface's own probes read
// it (three channels over three).
func drawerLuma(r, g, b uint8) int { return (int(r) + int(g) + int(b)) / 3 }

// drawerSpread is one pixel row's ink: max luma − min luma. A row holding a glyph
// reads in the hundreds; a bare pane reads 0-2.
func drawerSpread(img *image.RGBA, y, x0, x1 int) int {
	lo, hi := 999, -1
	for x := x0; x < x1; x++ {
		i := y*img.Stride + x*4
		v := drawerLuma(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if hi < 0 {
		return 0
	}
	return hi - lo
}

// drawerFullState renders the terminal at a window short enough that the strip
// does not fit (P13-dp: 640×280 is the smallest frame that still overflows —
// `MaxY` 54.4), and hands back the tester, the strip's rect and the edge's own
// height in pixels.
func drawerFullState(t *testing.T, dark bool) (*App, *ui.Tester, ui.Rect, int) {
	t.Helper()
	a := New(newStore(), Actions{Close: func() {}, NewCommand: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	a.Drawer = true
	band := 0
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokensOf(a, c).Theme)
		band = int(c.Theme().Space(1.5))
		a.View(c)
	}, 640, 280)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	if a.DrawerScroll.MaxY <= 0 {
		t.Fatalf("the drawer does not overflow this frame (MaxY=%.1f), so there is no full "+
			"state to measure", a.DrawerScroll.MaxY)
	}
	strip, ok := tt.Find(i18n.For("en").Settings.GroupTerminal)
	if !ok {
		t.Fatalf("the drawer's strip is not on the frame: %q", tt.Texts())
	}
	if band < 3 {
		t.Fatalf("the edge resolves to %d px, too thin to measure", band)
	}
	// 高度链红线：本轮只加一层画在视口上的面，条带的内边距（= 抽屉的高度、进而
	// 画布的高度、进而 PTY 的行数）一个字节都不许动。P13-dp 在这张取景上量到
	// `MaxY` 54.4；边若偷偷改了内边距，这个数会先变。
	if m := a.DrawerScroll.MaxY; m < 53.9 || m > 54.9 {
		t.Fatalf("the drawer's scroll range moved to %.1f (P13-dp measured 54.4 at 640×280): the "+
			"edge must not touch the strip's padding — it is the drawer's height, and the height is "+
			"the canvas's, and the canvas is the PTY's", m)
	}
	return a, tt, strip, band
}

func TestTheFullDrawerFadesAtItsBottomEdge(t *testing.T) {
	// 下沿那一行的墨展开度上限。P13-ex 实测：改前（边高 0）最坏 147（浅）/ 216（深），
	// 改后（`Space(1.5)` 的边）35 / 51——两套外观都落在 70 以内，而改前都远过。
	const faded = 70
	// 对照：夹具必须真的把一行字切在下沿上，否则探针落在空白上，这条 pin 白绿。
	const ink = 100
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a, tt, strip, band := drawerFullState(t, dark)
			x0 := int(strip.X) + 6
			x1 := int(strip.X+strip.W) - 6
			H := tt.Image().Bounds().Dy()

			// 九档滚动位置：下沿那一行的最坏值，与边上四行之外见过的最重墨（对照）。
			worstLast, mostAbove := 0, 0
			for k := 1; k <= 9; k++ {
				a.DrawerScroll.Y = a.DrawerScroll.MaxY * float32(k) / 10
				tt.Frame()
				tt.Frame()
				img := tt.Image()
				if s := drawerSpread(img, H-1, x0, x1); s > worstLast {
					worstLast = s
				}
				for y := H - band - 4; y < H-band; y++ {
					if s := drawerSpread(img, y, x0, x1); s > mostAbove {
						mostAbove = s
					}
				}
			}
			if mostAbove < ink {
				t.Fatalf("the fixture never cuts a glyph at the bottom edge (the heaviest ink above it is "+
					"%d levels): the pins below would pass on a drawer that never overflows", mostAbove)
			}
			if worstLast > faded {
				t.Errorf("the drawer's last pixel row reads %d levels of ink (limit %d) while the rows above it "+
					"carry %d: the grid is cut at the window's bottom edge instead of fading into the strip's "+
					"face — the drawer is the last scrolling face without a `glass.ScrollEdge`", worstLast, faded, mostAbove)
			}
		})
	}
}

func TestTheDrawerEdgeLeavesNoBandAtTheEnd(t *testing.T) {
	// 滚到底时最后一行停在离下沿 9px 处（`Padding(t.Space(2.5))`），5.4px 的边整个
	// 落在空白上，那几行必须读成裸面（实测两套外观都是 0）。
	const quiet = 6
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a, tt, strip, band := drawerFullState(t, dark)
			x0 := int(strip.X) + 6
			x1 := int(strip.X+strip.W) - 6
			a.DrawerScroll.Y = math.MaxFloat32
			tt.Frame()
			tt.Frame()
			img := tt.Image()
			H := img.Bounds().Dy()
			for y := H - band; y < H; y++ {
				if s := drawerSpread(img, y, x0, x1); s > quiet {
					t.Errorf("at the end of the scroll the bottom edge still lays a band down: row y=%d reads %d "+
						"levels of ink (limit %d) — the edge is not gated by the strip's own end padding, "+
						"so it paints a grey band on blank face", y, s, quiet)
				}
			}
		})
	}
}

// TestDumpP13EXDrawerEdge is the opt-in frame dump: FLOTER_P13EX_DIR names the
// directory, and the tag names the run (the `before` frames are rendered from a
// `git worktree` at the previous commit).
func TestDumpP13EXDrawerEdge(t *testing.T) {
	dir := os.Getenv("FLOTER_P13EX_DIR")
	if dir == "" {
		t.Skip("set FLOTER_P13EX_DIR to dump the P13-ex frames")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tag := os.Getenv("FLOTER_P13EX_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		a, tt, _, _ := drawerFullState(t, dark)
		for _, at := range []string{"mid", "end"} {
			if at == "mid" {
				a.DrawerScroll.Y = a.DrawerScroll.MaxY / 2
			} else {
				a.DrawerScroll.Y = math.MaxFloat32
			}
			tt.Frame()
			tt.Frame()
			path := filepath.Join(dir, "floter-P13-ex-drawer-"+name+"-"+at+".png")
			f, err := os.Create(path)
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
		}
	}
}
