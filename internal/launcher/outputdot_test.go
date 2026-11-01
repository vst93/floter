package launcher

import (
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/extensions"
	"floter/internal/theme"
)

// P13-cs · `docs/liquid-glass.md` §6.62 — 分隔点只有一档。
//
// 这一行里的 `·` 此前是 `TextTertiary.Alpha(0.7)`：参考的 `.7`（`extensions.css:2096-2103`）
// 是叠在**不透明**的墨上的 CSS 乘法，本仓把它接在了半透明的 token 上，于是 token 自己的
// alpha 又乘一次 0.7。实测这枚点只走到档位的 0.63，而且**换一张地面就换一个高度**：
// 捕获输出页的地面上 70 级，设置页的卡片面上 48 级。
//
// 这条 pin 不写死任何一个渲染器算出来的像素值：它在**同一张地面**上用同一枚字形画出
// 两个候选——阶梯的第四档 `TextTertiary`，和本轮删掉的那把 0.7 乘以它——然后要求页面上
// 那枚点离「档位」比离「0.7」更近。两次参考渲染与页面同字号、同地面，所以唯一被量的
// 是墨。

// statusDot reads the separator's own pixel out of the strip.
//
// The line is `verdict  ·  legend` in one `RichText`, so the dot is not a node of
// its own: it is the narrowest run of ink columns inside the line's own box.
// The two two-space gaps around it (the reference's `·` sits between two pads)
// are what keep it a run of its own even at 11px.
func statusDot(img *image.RGBA, box ui.Rect, ground ui.Color) (ui.Color, bool) {
	g := statusLuma(ground)
	ink := func(c ui.Color) bool {
		if c.A == 0 {
			return false
		}
		d := g - statusLuma(c)
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
			if ink(statusPixel(img, x, y)) {
				hasInk[i] = true
				break
			}
		}
	}
	at, width, pad := -1, 0, -1
	run := -1
	for i := 0; i <= cols; i++ {
		on := i < cols && hasInk[i]
		if on && run < 0 {
			run = i
		}
		if !on && run >= 0 {
			// The separator is the one short run isolated by the reference's
			// two two-space pads; inter-letter gaps are 1–2px, the pads are 8.
			// Choosing "the narrowest run" would land on a narrow letter (a `t`
			// stem is 2px), so the pick is "short run, widest blank on both
			// sides".
			left, right := 0, 0
			for j := run - 1; j >= 0 && !hasInk[j]; j-- {
				left++
			}
			for j := i; j < cols && !hasInk[j]; j++ {
				right++
			}
			if w := i - run; w <= 6 && left+right > pad {
				at, width, pad = run, w, left+right
			}
			run = -1
		}
	}
	if at < 0 {
		return ui.Color{}, false
	}
	var darkest ui.Color
	best := -1
	for x := int(box.X) + at; x < int(box.X)+at+width; x++ {
		for y := int(box.Y); y < int(box.Y+box.H); y++ {
			c := statusPixel(img, x, y)
			if !ink(c) {
				continue
			}
			if d := statusWeight(ground, c); d > best {
				best, darkest = d, c
			}
		}
	}
	return darkest, best > 0
}

// dotCandidates paints the same `·` twice on the page's own plane: once with the
// rung itself and once with the `0.7` this round removed. A pin then asks which
// one the surface is drawn with, so no number the renderer computed is written
// down here.
func dotCandidates(t *testing.T, tokens theme.Tokens, dark bool, plane ui.Color, size float32) (rung, faded, above ui.Color) {
	t.Helper()
	var a, b, d ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(plane)
		ui.Row(c).Padding(8).Gap(12).AlignItems(ui.Center).Children(func() {
			var ha, hb, hd ui.Handle
			ui.Text(c, "\u00b7").FontSize(size).TextColor(tokens.TextTertiary).Bind(&ha)
			ui.Text(c, "\u00b7").FontSize(size).TextColor(tokens.TextTertiary.Alpha(0.7)).Bind(&hb)
			ui.Text(c, "\u00b7").FontSize(size).TextColor(tokens.Theme.TextMuted).Bind(&hd)
			a, b, d = c.Resolve(ha).Bounds(), c.Resolve(hb).Bounds(), c.Resolve(hd).Bounds()
		})
	}, 200, 60)
	tt.SetDark(dark)
	tt.Frame()
	img := tt.Image()
	return dotDarkest(img, a, plane), dotDarkest(img, b, plane), dotDarkest(img, d, plane)
}

// darkestIn is the farthest pixel from the plane inside one box — the same
// reading `statusSplit` takes off the strip's two halves.
func dotDarkest(img *image.RGBA, box ui.Rect, plane ui.Color) ui.Color {
	var out ui.Color
	best := -1
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			c := statusPixel(img, x, y)
			if d := statusWeight(plane, c); d > best {
				best, out = d, c
			}
		}
	}
	return out
}

// TestTheStripDrawsItsDotFromTheLegendRung is the pin: the `·` between the
// verdict and the legend is the legend's own rung, in both appearances and on
// both the success and the fault arm.
func TestTheStripDrawsItsDotFromTheLegendRung(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, arm := range statusArms() {
			a := statusApp()
			tokens := appTokens(t, a, dark)
			p := probeStatus(t, dark, arm)
			dot, ok := statusDot(p.img, p.lineBox, p.ground)
			if !ok {
				t.Fatalf("dark=%v %s: the strip draws no separate `·` between its two halves", dark, arm.name)
			}
			rung, faded, _ := dotCandidates(t, tokens, dark, p.ground, tokens.Theme.FontSize-1)
			got := statusWeight(p.ground, dot)
			// Where the dot sits between three inks painted with the page's own
			// glyph, at the page's own size, on the page's own ground: the
			// removed `0.7`, the `TextTertiary` rung, and one rung *above* it
			// (`TextMuted`). Nearest wins, so neither a dot left below the ladder
			// nor one that climbed a rung can pass.
			wRung := statusWeight(p.ground, rung)
			wFaded := statusWeight(p.ground, faded)
			// The dot is drawn at a fractional x inside the strip, so its glyph
			// lands a dozen levels heavier than the same glyph painted at an
			// integer origin; the tolerance is that offset, not slack for the
			// ink. The rung-above candidate cannot be used here for the same
			// reason (its gap to the rung is smaller than the offset) — the
			// settings and detached-bar pins carry that half.
			if d := got - wRung; d > 15 || d < -15 {
				t.Errorf("dark=%v %s: the `·` reads %d levels off the ground, want the TextTertiary rung (%v, %d) within the glyph's own 15-level subpixel offset",
					dark, arm.name, got, rung, wRung)
			}
			if got-wFaded < 15 {
				t.Errorf("dark=%v %s: the `·` (%d) is within 15 levels of the removed ink (%v, %d): the separator is still below the ladder",
					dark, arm.name, got, faded, wFaded)
			}
			// And it is still quieter than the verdict it separates from.
			if v := statusWeight(p.ground, p.verdict); got >= v {
				t.Errorf("dark=%v %s: the `·` (%d) is as heavy as the verdict (%d)", dark, arm.name, got, v)
			}
			t.Logf("dark=%v %s: dot %v (%d) / faded %v (%d) / rung %v (%d); verdict %v (%d)",
				dark, arm.name, dot, got, faded, wFaded, rung, wRung,
				p.verdict, statusWeight(p.ground, p.verdict))
		}
	}
}

// statusApp is the fixture the strip's own probes use.
func statusApp() *App {
	a := testApp()
	a.Actions.PinText = func(string, string) {}
	a.Actions.DetachOutput = func(OutputView) {}
	a.SetCommands([]extensions.CommandEntry{{
		IntegrationID: "test.ext",
		Command:       extensions.Command{ID: "list", Name: "List things"},
		Program:       "/bin/sh",
		Route:         extensions.RouteBackground,
	}})
	return a
}

// appTokens resolves the launcher's tokens on one appearance without drawing
// the launcher: the candidates below need the palette, not the page.
func appTokens(t *testing.T, a *App, dark bool) theme.Tokens {
	t.Helper()
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) { tokens = a.tokens(c) }, 80, 60)
	tt.SetDark(dark)
	tt.Frame()
	return tokens
}

// TestTheStripDotDump writes the pair P13-cs is about (opt-in, FLOTER_PNG=1;
// FLOTER_PNG_TAG names the pair).
func TestTheStripDotDump(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-cs-strip-<tag>-<appearance>.png")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		a := statusApp()
		var run extensions.CapturedRun
		run.Success = true
		a.Actions.RunCommandCaptured = func(_ extensions.CommandEntry, _ []string, done func(extensions.CapturedRun, error)) {
			run.Stdout = "line 00\nline 01\nline 02"
			done(run, nil)
		}
		tt := render(t, a)
		a.Query = "list"
		tt.Frame()
		tt.TypeKey(0, ui.KeyEnter, "")
		tt.SetDark(dark)
		tt.Frame()
		writePNG(t, fmt.Sprintf("/tmp/floter-P13-cs-strip-%s-%s.png", tag, name), tt)
	}
}

// dotNearest is the index of the candidate closest to the measured weight.
func dotNearest(got int, weights ...int) int {
	pick, best := 0, -1
	for i, w := range weights {
		d := got - w
		if d < 0 {
			d = -d
		}
		if best < 0 || d < best {
			pick, best = i, d
		}
	}
	return pick
}
