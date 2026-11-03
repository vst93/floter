package terminalui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-fb · `docs/liquid-glass.md` §6.123 —— **先量，量出不该改。**
//
// 命题（本轮开工时的假设）：终端的外观抽屉是设置页那一行的**第二份实现**，
// 而设置页的行名穿 `tokens.Emphasis` + 580 + `TextStrong`（§6.117），抽屉的行名
// 却穿 `t.FontSize`（= `tokens.Body`）+ 560 + `TextSecondary`——同一角色两个寄存器。
//
// 量下来这个假设**不成立**，而且它撞两条已经落地的契约：
//
//  1. 抽屉的层级是「**值**响、名字静」：读数是 `tokens.Body` + mono + 600 +
//     `TextStrong`，名字是同档字号 + 560 + `TextSecondary`
//     （`TestTheDrawerReadoutWearsNeutralInk`，`raycast_test.go`）。把名字抬到
//     `TextStrong` 后实测读数 707 对名字 713 —— 名字比它命名的值还亮，那条 pin
//     当场变红。抽屉不是描述行，是一排**测量行**：值才是要读的东西。
//  2. 条带高 = 画布高 = PTY 的行数（`draweredge_test.go`）。名字抬一档后行盒
//     17 → 18，两条堆叠行各长 1px，实测 640×280 的滚动量 54.4 → **56.4**，
//     终端画布少 2px。视觉轮只改墨与面，不许动高度链。
//
// 所以这一族**故意不做**：抽屉的行名留在正文档 + 次墨。下面两条 pin 把「量出来的
// 现状」和它的理由钉住，免得下一轮再提一次同一把刀。

// drawerRegisterFrame draws the real drawer (or one synthetic row shape) and
// returns the tester together with the drawer's tokens.
func drawerRegisterFrame(t *testing.T, dark bool, draw func(a *App, c *ui.Context)) (*ui.Tester, theme.Tokens) {
	t.Helper()
	a := drawerApp(t)
	if err := a.Store.Update(func(s *settings.Settings) {
		if dark {
			s.Theme = "dark"
		} else {
			s.Theme = "light"
		}
	}); err != nil {
		t.Fatal(err)
	}
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		draw(a, c)
	}, 900, 400)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt, tok
}

// drawerStepBox is the box a string occupies at one rung, drawn with the
// drawer's own tokens inside a `Shrink(0)` row so its box is its own ink's.
func drawerStepBox(t *testing.T, tok theme.Tokens, text string, size float32, weight int) ui.Rect {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		ui.Row(c).Children(func() {
			ui.Text(c, text).FontSize(size).FontWeight(weight).
				TextColor(tok.TextStrong).SingleLine().Shrink(0)
		})
	}, 400, 200)
	tt.Frame()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the reference frame does not draw %q: %v", text, tt.Texts())
	}
	return box
}

// TestTheDrawerRowsNameIsDeliberatelyTheBodyStep: all three row shapes the
// drawer has draw their name at the body step — never the settings page's
// emphasis step. This is the §6.123 decision, held as a number.
func TestTheDrawerRowsNameIsDeliberatelyTheBodyStep(t *testing.T) {
	const label = "Label"
	scopy := i18n.For("en").Settings
	for _, dark := range []bool{true, false} {
		shapes := map[string]func(a *App, c *ui.Context){
			"drawerRow": func(a *App, c *ui.Context) {
				a.drawerRow(c, c.Theme().Space(20), label, func() {
					ui.Text(c, "value").FontSize(tokensOf(a, c).Body)
				})
			},
			"drawerRowStacked": func(a *App, c *ui.Context) {
				a.drawerRowStacked(c, c.Theme().Space(20), label, func() {
					ui.Text(c, "value").FontSize(tokensOf(a, c).Body)
				})
			},
		}
		for name, draw := range shapes {
			tt, tok := drawerRegisterFrame(t, dark, draw)
			box, ok := tt.Find(label)
			if !ok {
				t.Fatalf("%s (%v) does not draw its label: %v", name, dark, tt.Texts())
			}
			body := drawerStepBox(t, tok, label, tok.Body, 560)
			emph := drawerStepBox(t, tok, label, tok.Emphasis, 560)
			t.Logf("%s dark=%v: name %v, body %v, emphasis %v", name, dark, box, body, emph)
			if box.H != body.H {
				t.Errorf("%s dark=%v: the row's name is %.1f high, not the body step's %.1f — the drawer's name register moved (§6.123)",
					name, dark, box.H, body.H)
			}
			if box.H == emph.H {
				t.Errorf("%s dark=%v: the row's name is at the emphasis step (%.1f) — §6.123 measured that this also costs the strip 2px of the canvas",
					name, dark, box.H)
			}
		}

		// The palette row names itself through the same family; its label is the
		// copy's own word, so the real string is measured.
		tt, tok := drawerRegisterFrame(t, dark, func(a *App, c *ui.Context) {
			a.drawerPalette(c, tokensOf(a, c), c.Theme().Space(40), scopy, a.settings())
		})
		box, ok := tt.Find(scopy.Palette)
		if !ok {
			t.Fatalf("drawerPalette (%v) does not draw its label: %v", dark, tt.Texts())
		}
		body := drawerStepBox(t, tok, scopy.Palette, tok.Body, 560)
		if box.H != body.H {
			t.Errorf("drawerPalette dark=%v: its name is %.1f high, not the body step's %.1f (§6.123)",
				dark, box.H, body.H)
		}
	}
}

// TestDumpP13FBDrawerPNGs writes the round's visual evidence: the real drawer
// in both appearances. Opt-in, like every other dump in this package.
func TestDumpP13FBDrawerPNGs(t *testing.T) {
	dir := os.Getenv("FLOTER_P13FB_DIR")
	if dir == "" {
		t.Skip("set FLOTER_P13FB_DIR=<dir> to write the round's visual evidence")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := drawerApp(t)
		if err := a.Store.Update(func(s *settings.Settings) {
			if dark {
				s.Theme = "dark"
			} else {
				s.Theme = "light"
			}
		}); err != nil {
			t.Fatal(err)
		}
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokensOf(a, c).Theme)
			c.Root().Background(ui.RGBA(0, 0, 0, 1))
			a.drawer(c, i18n.For("en").Terminal)
		}, 900, 400)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		name := dir + "/floter-P13-fb-drawer-" + appearance + ".png"
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
		t.Logf("wrote %s", name)
	}
}
