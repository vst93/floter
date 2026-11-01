package shell

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/launcher"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-cs · `docs/liquid-glass.md` §6.62 — 分隔点只有一档。
//
// 浮窗口条上的 `·`（`detach.go` 的 `detachedVerdict`）是同一族的第三处：此前
// `TextTertiary.Alpha(0.7)`，参考的 `.7` 叠在半透明的 token 上 → 档位的 0.55–0.63。
// 这里钉的是「这条带上的 `·` 用的是阶梯第四档，不是那一档乘 0.7」。
func dotStripPlane(img *image.RGBA, box ui.Rect, fallback ui.Color) ui.Color {
	for dy := 2; dy < 6; dy++ {
		y := int(box.Y) - dy
		if y < 0 {
			break
		}
		c := shellDotPixel(img, int(box.X)+1, y)
		if c.A != 0 {
			return c
		}
	}
	return fallback
}

func shellDotPixel(img *image.RGBA, x, y int) ui.Color {
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

func shellDotLuma(c ui.Color) int { return (int(c.R) + int(c.G) + int(c.B)) / 3 }

// shellDotDarkest is the farthest pixel from the plane inside one box.
func shellDotDarkest(img *image.RGBA, box ui.Rect, plane ui.Color) ui.Color {
	p := shellDotLuma(plane)
	var out ui.Color
	best := -1
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			c := shellDotPixel(img, x, y)
			d := p - shellDotLuma(c)
			if d < 0 {
				d = -d
			}
			if d > best {
				best, out = d, c
			}
		}
	}
	return out
}

// shellDotCandidates paints the same `·` twice on the band's own plane.
func shellDotCandidates(t *testing.T, tokens theme.Tokens, dark bool, plane ui.Color) (rung, faded, above ui.Color) {
	t.Helper()
	var a, b, d ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(plane)
		ui.Row(c).Padding(8).Gap(12).AlignItems(ui.Center).Children(func() {
			var ha, hb, hd ui.Handle
			ui.Text(c, "\u00b7").FontSize(tokens.Caption).TextColor(tokens.TextTertiary).Bind(&ha)
			ui.Text(c, "\u00b7").FontSize(tokens.Caption).TextColor(tokens.TextTertiary.Alpha(0.7)).Bind(&hb)
			ui.Text(c, "\u00b7").FontSize(tokens.Caption).TextColor(tokens.Theme.TextMuted).Bind(&hd)
			a, b, d = c.Resolve(ha).Bounds(), c.Resolve(hb).Bounds(), c.Resolve(hd).Bounds()
		})
	}, 200, 60)
	tt.SetDark(dark)
	tt.Frame()
	img := tt.Image()
	return shellDotDarkest(img, a, plane), shellDotDarkest(img, b, plane), shellDotDarkest(img, d, plane)
}

func TestTheDetachedBarDotIsTheFactRung(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a := New(Options{Store: settings.NewStore(settings.Default())})
		detached := &detachedOutput{
			title: "jv --floter",
			entry: &extensions.CommandEntry{IntegrationID: "local.tool", Command: extensions.Command{ID: "jv", Name: "JSON Viewer"}},
			ran:   true,
			view: launcher.OutputView{
				Title:   "jv --floter",
				Text:    "{\"a\": 1}",
				Verdict: "Finished",
				Facts:   "1.2s",
			},
		}
		var tokens theme.Tokens
		tt := ui.NewTester(func(c *ui.Context) {
			tokens = launcher.Resolve(a.Store.Snapshot(), c.Theme().Dark, c.Preferences()).Tokens
			a.detachedView(c, detached)
		}, 720, 460)
		tt.SetDark(dark)
		tt.Frame()
		lineBox, ok := tt.Find("Finished  \u00b7  1.2s")
		if !ok {
			t.Fatalf("dark=%v: the bar draws no verdict line: %q", dark, tt.Texts())
		}
		img := tt.Image()
		plane := dotStripPlane(img, lineBox, tokens.Opaque)
		dotBox, ok := shellNarrowest(img, lineBox, plane)
		if !ok {
			t.Fatalf("dark=%v: the bar's verdict line has no separate `·` run", dark)
		}
		got := shellDotDarkest(img, dotBox, plane)
		g := shellDotLuma(plane) - shellDotLuma(got)
		if g < 0 {
			g = -g
		}
		rung, faded, above := shellDotCandidates(t, tokens, dark, plane)
		lum := shellDotLuma(plane)
		wRung, wFaded, wAbove := shellAbs(lum-shellDotLuma(rung)), shellAbs(lum-shellDotLuma(faded)), shellAbs(lum-shellDotLuma(above))
		if pick := shellNearest(g, wFaded, wRung, wAbove); pick != 1 {
			t.Errorf("dark=%v: the bar's `·` reads %d levels off its band (%v on %v); the removed 0.7 reads %d (%v), the TextTertiary rung %d (%v), one rung above %d (%v) — nearest is %s, want the rung",
				dark, g, got, plane, wFaded, faded, wRung, rung, wAbove, above,
				[]string{"the removed 0.7", "the rung", "the rung above"}[pick])
		}
		t.Logf("dark=%v: dot %v (%d) plane %v / faded %v (%d) / rung %v (%d) / above %v (%d)",
			dark, got, g, plane, faded, wFaded, rung, wRung, above, wAbove)
	}
}

// shellNarrowest is the narrowest run of ink columns inside one text node — the
// `·` that the verdict line draws between its two facts.
func shellNarrowest(img *image.RGBA, box ui.Rect, plane ui.Color) (ui.Rect, bool) {
	p := shellDotLuma(plane)
	ink := func(c ui.Color) bool {
		if c.A == 0 {
			return false
		}
		d := p - shellDotLuma(c)
		if d < 0 {
			d = -d
		}
		return d > 6
	}
	cols := int(box.W)
	hasInk := make([]bool, cols)
	for i := 0; i < cols; i++ {
		x := int(box.X) + i
		for y := int(box.Y); y < int(box.Y+box.H); y++ {
			if ink(shellDotPixel(img, x, y)) {
				hasInk[i] = true
				break
			}
		}
	}
	at, width := -1, cols+1
	run := -1
	for i := 0; i <= cols; i++ {
		on := i < cols && hasInk[i]
		if on && run < 0 {
			run = i
		}
		if !on && run >= 0 {
			if w := i - run; w < width {
				at, width = run, w
			}
			run = -1
		}
	}
	if at < 0 {
		return ui.Rect{}, false
	}
	return ui.Rect{X: box.X + float32(at), Y: box.Y, W: float32(width), H: box.H}, true
}

func shellAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func shellNearest(got int, weights ...int) int {
	pick, best := 0, -1
	for i, w := range weights {
		d := shellAbs(got - w)
		if best < 0 || d < best {
			pick, best = i, d
		}
	}
	return pick
}
