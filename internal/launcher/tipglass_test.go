package launcher

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The launcher's seven tips (P13-ay, `docs/liquid-glass.md` §6.20 20-2).
//
// Every one of them used to ride the framework's `Element.Tooltip`
// (`ui/tooltip.go:127-133`): a 5px corner (the keycap's ladder, not a floating
// pane's 8), the **opaque inverse pair** as its fill, one `0 2px 8px 0.2` blur
// and no edge at all. This repo never sets `ui.Theme.Inverse`, so `inverse()`
// falls back to `Text` (`ui/theme.go:71-80`) — on the dark appearance that is a
// near-white chip over dark glass, the system-tooltip look this generation
// retired (P13-ax did the terminal page; these are the launcher's).
//
// The pin reads the tip off the pixels, not off the code: it renders the frame
// before and after the pointer rests on an anchor and takes the tip's face as
// the largest single colour inside the region that changed. Two things are
// then true of a pane that is on the z ladder and false of the chip:
//
//	① the face is the appearance's `Float` step of the ground, i.e. **dark on
//	   dark / light on light** — the chip is the opposite by construction;
//	② it floats: the anchor's top edge and the face's bottom edge are apart by
//	   the framework's 1.5u gap (`ui/tooltip.go:132`), not zero.
func TestTheLaunchersTipsAreFloatingPanes(t *testing.T) {
	cases := []struct {
		name   string
		app    func(t *testing.T) *App
		drive  func(tt *ui.Tester) // what puts the anchor on screen
		anchor func(a *App) string // the anchor's own text (its Label / mark)
	}{
		{
			name: "direct-keycap",
			app: func(t *testing.T) *App {
				a := testApp()
				a.SetCommands([]extensions.CommandEntry{{
					IntegrationID: "x", IntegrationName: "X",
					Command: extensions.Command{ID: "jv", Name: "JSON Viewer"},
					Program: "/bin/echo", Mode: "pty", Route: extensions.RouteBackground,
				}})
				return a
			},
			drive:  func(tt *ui.Tester) { tt.Type("jv") },
			anchor: func(*App) string { return "\u2423" },
		},
		{
			name: "command-conflict",
			app: func(t *testing.T) *App {
				a := testApp()
				a.SetCommands([]extensions.CommandEntry{
					{IntegrationID: "a", IntegrationName: "A", Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}, Program: "/bin/echo", Mode: "pty"},
					{IntegrationID: "b", IntegrationName: "B", Command: extensions.Command{ID: "jv", Name: "JSON Viewer Two"}, Program: "/bin/echo", Mode: "pty"},
				})
				return a
			},
			drive:  func(tt *ui.Tester) { tt.Type("jv") },
			anchor: func(a *App) string { return a.copy().CommandConflict },
		},
	}
	for _, tc := range cases {
		for _, appearance := range []struct {
			name string
			dark bool
		}{{"light", false}, {"dark", true}} {
			t.Run(tc.name+"/"+appearance.name, func(t *testing.T) {
				a := tc.app(t)
				tt := renderIn(t, a, appearance.dark)
				tc.drive(tt)
				tt.Frame()
				anchorText := tc.anchor(a)
				if anchorText == "" {
					t.Fatal("the fixture has no anchor text to hover")
				}
				rect, ok := tt.Find(anchorText)
				if !ok {
					t.Fatalf("no %q anchor to hover: %q", anchorText, tt.Texts())
				}
				shot, ok := tipShot(t, tt, rect)
				if !ok {
					t.Fatalf("hovering %q painted no tip at all: %q", anchorText, tt.Texts())
				}
				// ① 面：浮层比它压着的那张面沉（深）或浮（浅），反色片子正好相反。
				level := lum(shot.face)
				if appearance.dark && level > 90 {
					t.Fatalf("the tip's face is %v (level %d): that is the inverse chip, not the float face",
						shot.face, level)
				}
				if !appearance.dark && level < 190 {
					t.Fatalf("the tip's face is %v (level %d): that is the inverse chip, not the float face",
						shot.face, level)
				}
				// ② 空间：提示与锚点之间是 1.5u 的缝，不是 0——这一条在下面的
				// `TestTheTipFloatsOffItsAnchor` 上量：launcher 里提示是半透明面，
				// 它压着的内容逐行不同，面色的 bbox 会跟着内容漂（实测有 6px 与
				// 4px 两种读数），而平地上的同一枚提示是确定的。
				t.Logf("face %v level %d, box %.0f×%.0f at (%.0f,%.0f), anchor top %.0f",
					shot.face, level, shot.box.W, shot.box.H, shot.box.X, shot.box.Y, rect.Y)
				if os.Getenv("FLOTER_P13AY_DUMP") != "" {
					name := "floter-P13-ay-" + tc.name + "-" + appearance.name
					f, err := os.Create(filepath.Join("/tmp", name+".png"))
					if err != nil {
						t.Fatal(err)
					}
					if err := png.Encode(f, tt.Image()); err != nil {
						t.Fatal(err)
					}
					f.Close()
				}
			})
		}
	}
}

// tipShot rests the pointer on anchor and returns the tip's face: the largest
// single colour in the region the tip changed.
//
// The pointer's own row may react to the hover as well, so the probe only looks
// **above** the anchor: a tip shows over its anchor (`ui/tooltip.go:132`), and
// the row it belongs to is at or below that line.
func tipShot(t *testing.T, tt *ui.Tester, anchor ui.Rect) (tipCapture, bool) {
	t.Helper()
	tt.Move(anchor.X+anchor.W/2, anchor.Y+anchor.H+160)
	tt.Frame()
	before := copyImage(tt.Image())
	tt.Move(anchor.X+anchor.W/2, anchor.Y+anchor.H/2)
	tt.Advance(700 * time.Millisecond)
	tt.Frame()
	after := tt.Image()

	type change struct{ x, y int }
	var changed []change
	limit := int(anchor.Y) - 2
	for y := 0; y < after.Bounds().Dy() && y < limit; y++ {
		for x := 0; x < after.Bounds().Dx(); x++ {
			if differs(pxOf(before, x, y), pxOf(after, x, y)) {
				changed = append(changed, change{x, y})
			}
		}
	}
	if len(changed) < 200 {
		return tipCapture{}, false
	}
	counts := map[ui.Color]int{}
	for _, p := range changed {
		counts[pxOf(after, p.x, p.y)]++
	}
	type colourCount struct {
		c ui.Color
		n int
	}
	ranked := make([]colourCount, 0, len(counts))
	for c, n := range counts {
		ranked = append(ranked, colourCount{c, n})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].n != ranked[j].n {
			return ranked[i].n > ranked[j].n
		}
		return fmt.Sprint(ranked[i].c) < fmt.Sprint(ranked[j].c)
	})
	if ranked[0].n < 300 {
		t.Logf("the changed region has no dominant colour: top %v", ranked[:min(4, len(ranked))])
		return tipCapture{}, false
	}
	face := ranked[0].c
	box := ui.Rect{X: 1 << 30, Y: 1 << 30}
	x1, y1 := -1, -1
	for _, p := range changed {
		if pxOf(after, p.x, p.y) != face {
			continue
		}
		box.X = min(box.X, float32(p.x))
		box.Y = min(box.Y, float32(p.y))
		x1 = max(x1, p.x)
		y1 = max(y1, p.y)
	}
	box.W = float32(x1) - box.X
	box.H = float32(y1) - box.Y
	return tipCapture{face: face, box: box}, true
}

// tipCapture is one tip read off the pixels.
type tipCapture struct {
	face ui.Color
	box  ui.Rect
}

// lum is a colour's average channel, the level the pins below speak in.
func lum(c ui.Color) int { return (int(c.R) + int(c.G) + int(c.B)) / 3 }

func copyImage(img *image.RGBA) *image.RGBA {
	out := image.NewRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	return out
}

func pxOf(img *image.RGBA, x, y int) ui.Color {
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// differs reports whether a pixel changed enough to be the tip rather than
// rounding in the frame it sits on.
func differs(a, b ui.Color) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x) - int(y)
		}
		return int(y) - int(x)
	}
	return d(a.R, b.R) > 2 || d(a.G, b.G) > 2 || d(a.B, b.B) > 2 || d(a.A, b.A) > 2
}

// TestTheLauncherHasNoFrameworkTips is the census half: §6.20 20-2 says seven
// sites, and a census is the only thing that stays true after the code moves.
// It names every site, so a regression report says which one went back.
func TestTheLauncherHasNoFrameworkTips(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	tipped := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if strings.Contains(line, ".Tooltip(") {
				t.Errorf("%s:%d still wears the framework's opaque chip: %s", file, i+1, strings.TrimSpace(line))
			}
			tipped += strings.Count(line, "tokens.Tip(c, ")
		}
	}
	if tipped != 7 {
		t.Errorf("the launcher carries %d tips on the liquid-glass material, want the seven §6.20 20-2 names", tipped)
	}
}

// TestTheTipFloatsOffItsAnchor pins the second half of a floating pane: the
// 1.5u gap between the pane and what it annotates.
//
// `TooltipBase`'s default attachment puts the tip flush against the anchor
// (`ui/tooltip.go:105-107`); the 1.5u comes from `Element.Tooltip` itself
// (`:132`). `Tokens.Tip` repaints the material and must carry that margin over
// — P13-ax dropped it, so every tip this design owns sat *touching* its anchor,
// which reads as a badge on the mark rather than as a pane over it.
//
// The measurement is on a flat ground: the launcher's own tips are translucent
// panes over text, so the composited face differs row by row and its pixel box
// drifts with whatever it covers (measured 6px and 4px for the same 1.5u). Over
// one opaque ground the face is exactly `FaceOver(ground, Float)` and the box is
// the pane.
func TestTheTipFloatsOffItsAnchor(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tok := theme.ForPreferences(settings.Default(), dark, ui.Preferences{})
			ground := ui.RGBA(247, 247, 249, 1)
			if dark {
				ground = ui.RGBA(20, 21, 24, 1)
			}
			tt := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(tok.Theme)
				c.Root().Background(ground)
				ui.Column(c).Padding(80, 60).Children(func() {
					anchor := ui.ButtonBase(c).Label("anchor").Width(120).Height(30).
						Background(tok.Control)
					tok.Tip(c, anchor, "the hint")
				})
			}, 460, 380)
			tt.SetPreferences(ui.Preferences{})
			tt.Frame()
			rect, ok := tt.Find("anchor")
			if !ok {
				t.Fatal("the anchor is not in the frame")
			}
			tt.Move(rect.X+rect.W/2, rect.Y+rect.H/2)
			tt.Advance(700 * time.Millisecond)
			tt.Frame()

			face := theme.FaceOver(ground, tok.Float)
			img := tt.Image()
			x1, y1 := -1, -1
			for y := 0; y < int(rect.Y)-2; y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					if !near(pxOf(img, x, y), face, 2) {
						continue
					}
					x1, y1 = max(x1, x), max(y1, y)
				}
			}
			if x1 < 0 {
				t.Fatalf("no %v face above the anchor: the tip did not paint", face)
			}
			want := ui.LightTheme().Space(1.5)
			gap := rect.Y - float32(y1+1)
			if gap < want-1 || gap > want+1 {
				t.Fatalf("the tip's face ends %.1fpx above its anchor, not the framework's 1.5u (%.1f): a pane that touches its anchor reads as a badge",
					gap, want)
			}
			t.Logf("gap %.1fpx (1.5u = %.1f)", gap, want)
		})
	}
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
