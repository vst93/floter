package settingsui

import (
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/theme"
)

// P13-cs · `docs/liquid-glass.md` §6.62 — 分隔点只有一档。
//
// meta 行的 `·` 此前是 `TextTertiary.Alpha(0.7)`。参考的 `.7`
//（`extensions.css:979-984`）是叠在一条不透明墨上的 CSS 乘法；本仓把它接在**半透明**的
// token 上，于是一次乘法变成两次。实测这枚点只走到档位的 0.55（浅）/0.53（深），
// 深色卡片面上只剩 40 级——与 `HairlineSoft` 同级，读起来像屏幕上的脏而不像标点。
//
// 这条 pin 用同一张面板面、同一字号画出两个候选（阶梯第四档 / 那一档乘 0.7），要求
// 页面上的 `·` 离「档位」更近。没有一处写死渲染器算出来的像素值。

// metaDot reads the narrowest ink run inside one text node — the `·` that
// introduces the fact it rides in.
func metaDot(img *image.RGBA, box ui.Rect, plane ui.Color) (ui.Color, int, bool) {
	p := dotLuma(plane)
	ink := func(c ui.Color) bool {
		if c.A == 0 {
			return false
		}
		d := p - dotLuma(c)
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
			if ink(imagePixel(img, x, y)) {
				hasInk[i] = true
				break
			}
		}
	}
	at, width := 0, cols+1
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
	if width > cols {
		return ui.Color{}, 0, false
	}
	var out ui.Color
	best := -1
	for x := int(box.X) + at; x < int(box.X)+at+width; x++ {
		for y := int(box.Y); y < int(box.Y+box.H); y++ {
			c := imagePixel(img, x, y)
			if !ink(c) {
				continue
			}
			d := p - dotLuma(c)
			if d < 0 {
				d = -d
			}
			if d > best {
				best, out = d, c
			}
		}
	}
	return out, best, best > 0
}

// metaDotCandidates paints the same `·` twice on the row's own plane.
func metaDotCandidates(t *testing.T, tokens theme.Tokens, dark bool, plane ui.Color) (rung, faded, above ui.Color) {
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
	return metaDotDarkest(img, a, plane), metaDotDarkest(img, b, plane), metaDotDarkest(img, d, plane)
}

func metaDotDarkest(img *image.RGBA, box ui.Rect, plane ui.Color) ui.Color {
	p := dotLuma(plane)
	var out ui.Color
	best := -1
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			c := imagePixel(img, x, y)
			d := p - dotLuma(c)
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

// TestTheMetaDotIsTheFactRung pins both sites on the Integrations page: the dot
// in front of a fact (`metaFact`) and the one in front of the state word
// (`stateMark`).
func TestTheMetaDotIsTheFactRung(t *testing.T) {
	en := i18n.For("en").Settings
	for _, appearance := range appearanceNames {
		a := oneIntegrationApp(metaFixture(), Actions{})
		tt, tokens := renderIn(t, a, 860, 900, appearance.dark)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		for _, label := range []string{"· Managed", "· " + en.IntegrationsRunning} {
			box := mustFind(t, tt, label)
			img := tt.Image()
			plane := imagePixel(img, int(box.X)+2, int(box.Y)-8)
			dot, got, ok := metaDot(img, box, plane)
			if !ok {
				t.Fatalf("%s %q: no `·` is drawn ahead of the fact", appearance.name, label)
			}
			rung, faded, above := metaDotCandidates(t, tokens, appearance.dark, plane)
			lum := dotLuma(plane)
			wRung, wFaded, wAbove := mdotAbs(lum-dotLuma(rung)), mdotAbs(lum-dotLuma(faded)), mdotAbs(lum-dotLuma(above))
			// Nearest of three inks painted with the same glyph on the same
			// plane: the removed 0.7, the `TextTertiary` rung, and one rung
			// above it. Neither a dot left below the ladder nor one that climbed
			// a rung can pass.
			if pick := dotNearest(got, wFaded, wRung, wAbove); pick != 1 {
				t.Errorf("%s %q: the `·` reads %d levels off the panel (%v on %v); the removed 0.7 reads %d (%v), the TextTertiary rung %d (%v), one rung above %d (%v) — nearest is %s, want the rung",
					appearance.name, label, got, dot, plane, wFaded, faded, wRung, rung, wAbove, above,
					[]string{"the removed 0.7", "the rung", "the rung above"}[pick])
			}
			t.Logf("%s %q: dot %v (%d) plane %v / faded %v (%d) / rung %v (%d) / above %v (%d)",
				appearance.name, label, dot, got, plane, faded, wFaded, rung, wRung, above, wAbove)
		}
	}
}

// TestTheMetaDotDump writes the pair P13-cs is about (opt-in, FLOTER_PNG=1;
// FLOTER_PNG_TAG names the pair).
func TestTheMetaDotDump(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-cs-meta-<tag>-<appearance>.png")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, appearance := range appearanceNames {
		a := oneIntegrationApp(metaFixture(), Actions{})
		tt, _ := renderIn(t, a, 860, 900, appearance.dark)
		if err := tt.Click("Integrations"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		writePNG(t, fmt.Sprintf("/tmp/floter-P13-cs-meta-%s-%s.png", tag, appearance.name), tt)
	}
}

// dotLuma is an ink's brightness, the reading this pin compares inks with.
func dotLuma(c ui.Color) int { return (int(c.R) + int(c.G) + int(c.B)) / 3 }

func mdotAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func dotNearest(got int, weights ...int) int {
	pick, best := 0, -1
	for i, w := range weights {
		d := mdotAbs(got - w)
		if best < 0 || d < best {
			pick, best = i, d
		}
	}
	return pick
}
