package launcher

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// over lays a translucent color over an opaque one, the way the renderer
// composites a row's fill on the panel beneath it.
func over(over ui.Color, base color.RGBA) color.RGBA {
	a := float64(over.A) / 255
	mix := func(o, b uint8) uint8 {
		return uint8(float64(o)*a + float64(b)*(1-a) + 0.5)
	}
	return color.RGBA{mix(over.R, base.R), mix(over.G, base.G), mix(over.B, base.B), 255}
}

func closeRGB(a, b color.RGBA) bool {
	diff := func(x, y uint8) int {
		d := int(x) - int(y)
		if d < 0 {
			d = -d
		}
		return d
	}
	return diff(a.R, b.R) <= 2 && diff(a.G, b.G) <= 2 && diff(a.B, b.B) <= 2
}

// The selected result row's fill is the contract's **neutral** chosen-row rung,
// `RowSelected` (`docs/raycast-design.md` §1/§4.3), and it carries no accent.
//
// P8-a 重定基线：旧 base.css 值 → 合约 §4.3。旧 pin 钉的是「选中行 = accent 的
// 静默淡染」（`--accent-tint-quiet`，dark 0.085 / light 0.065 的蓝）。合约把启动
// 器列表的选中改成**中性**：`rgba(255,255,255,0.085)` / `rgba(0,0,0,0.06)`，且写明
// 「accent 不出现在启动器选中行」。所以这条测试量的事实换成了两件：
//
//  1. 选中行 = `RowSelected` 在面板上的合成值（两套外观各自的 rung 不同）；
//  2. 它**不含 accent**：合成后的像素三个通道相差不超过 2 级（中性填充不可能带
//     色偏），并且它也不等于 accent 的静默淡染（反证，防止回退到旧语言）。
func TestTheSelectedRowWearsThePerAppearanceQuietTint(t *testing.T) {
	rowFill := func(themeName string) (sel, rest color.RGBA, tokens theme.Tokens) {
		a := testApp()
		recentPage(a, 2) // P11-bk: the empty page's rows are the recents
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = themeName }); err != nil {
			t.Fatal(err)
		}
		tt := render(t, a)
		tt.Frame()
		img := tt.Image()
		first, ok := tt.Find("Recent 00") // the empty page's selected first row
		if !ok {
			t.Fatalf("%s: the first result was not drawn: %q", themeName, tt.Texts())
		}
		second, ok := tt.Find("Recent 01") // the unselected row one band below
		if !ok {
			t.Fatalf("%s: the second result was not drawn: %q", themeName, tt.Texts())
		}
		// A blank x well to the right of both titles: the row's fill with no
		// glyph or text ink over it.
		const x = 600
		sel = img.RGBAAt(x, int(first.Y+first.H/2))
		rest = img.RGBAAt(x, int(second.Y+second.H/2))
		tokens = theme.ForPreferences(a.settings(), themeName == "dark", ui.Preferences{})
		return sel, rest, tokens
	}

	lightSel, lightRest, lightTok := rowFill("light")
	darkSel, darkRest, darkTok := rowFill("dark")

	// The two appearances' tokens must differ (the contract splits them: dark
	// 0.085 of white, light 0.06 of black).
	if lightTok.RowSelected == darkTok.RowSelected {
		t.Fatalf("the chosen-row rung is one value for both appearances: %v", lightTok.RowSelected)
	}

	for _, tc := range []struct {
		name      string
		sel, rest color.RGBA
		want      ui.Color
		accent    ui.Color
	}{
		{"light", lightSel, lightRest, lightTok.RowSelected, lightTok.AccentTintQuiet},
		{"dark", darkSel, darkRest, darkTok.RowSelected, darkTok.AccentTintQuiet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := over(tc.want, tc.rest)
			if !closeRGB(tc.sel, want) {
				t.Errorf("the selected row is %v, want %v (the neutral chosen-row rung %v over the row's own %v)",
					tc.sel, want, tc.want, tc.rest)
			}
			// No accent: a *neutral* fill cannot shift one channel against the
			// other, so the composite's own spread across R/G/B is a couple of
			// levels at most. An accent-tinted row (the accent is a saturated
			// red) would read tens of levels of R against B.
			spread := 0
			for _, d := range []int{int(tc.sel.R) - int(tc.sel.G), int(tc.sel.G) - int(tc.sel.B), int(tc.sel.B) - int(tc.sel.R)} {
				if d < 0 {
					d = -d
				}
				if d > spread {
					spread = d
				}
			}
			if spread > 2 {
				t.Errorf("the selected row is %v: its channels differ by %d levels, so the fill carries a hue "+
					"(the contract's chosen row is neutral)", tc.sel, spread)
			}
			// …and it is not the old accent wash: the anti-regression half. The
			// accent's quiet tint over the same row is a *red* plane, so the two
			// must be visibly apart.
			if wash := over(tc.accent, tc.rest); closeRGB(tc.sel, wash) {
				t.Errorf("the %s selection is painted with the accent's quiet tint %v (%v); "+
					"the contract's row fill is neutral", tc.name, tc.accent, wash)
			}
		})
	}
}

// The power rows' warm plate carries its own ink. The reference's
// `.launcher-result__icon--system` (`main:src/styles/launcher.css:1232-1235`)
// paints `--system-icon-surface` with `--system-icon-text` — the amber ink
// (`base.css:705,963`) — **not** the warning colour, which is a near-neutral
// red-grey reserved for a label. The port drew the glyph in `t.Warning` on a
// plate filled with `t.Warning.Alpha(0.14)`: both the plane and its ink were
// the warning colour, which is the wrong hue on either appearance.
//
// The plate is read at its centre (the glyph covers it, so the *ink* is what
// shows): over the warm plane the glyph's own hue must lead red over blue, as
// the amber ink does and the warning colour does not.
func TestThePowerGlyphsWearTheSystemIconInk(t *testing.T) {
	// P9-e: both appearances, with a plane-relative ink probe. The amber ink is
	// translucent on the dark side (`systemIconText` 0.82, `theme.go:798`), so
	// the expected value is that ink composited over the plate the glyph sits on
	// — `opaque()` (over white) is only the light half of the same fact, and on
	// the dark run it would name a bright ink nobody drew.
	for _, tc := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			// P11-bk: the power rows are drawn for a typed word now, not on the
			// empty page (`o` reaches `power off`).
			a.SetQuery("o")
			tt := renderIn(t, a, tc.dark)
			tt.Frame()

			// The row's 28u icon cell, inside the plate: the glyph and its plane,
			// nothing else.
			title, ok := tt.Find("Shut down the computer")
			if !ok {
				t.Fatalf("the power row was not drawn: %q", tt.Texts())
			}
			cell := ui.Rect{X: title.X - 34, Y: title.Y, W: 22, H: title.H}
			// The plate's own fill, read inside the 28u plate but left of the
			// 14u glyph: `imgPlane` over the whole cell is the *row* fill on the
			// dark side, where the 0.115 wash covers less of the window than the
			// row around it, and a top-edge sample lands on the row too (the
			// plate is 28u inside a taller row).
			img := tt.Image()
			plane := img.RGBAAt(int(title.X)-38, int(title.Y+title.H/2))
			ink := inkOn(tt, cell, plane)

			tokens := theme.ForPreferences(a.settings(), tc.dark, ui.Preferences{})
			want := inkOver(tokens.SystemIconText, plane)
			if !closeRGB(ink, want) {
				t.Errorf("the power glyph is %v, want the amber system-icon ink %v over its plate %v", ink, want, plane)
			}
			if warn := inkOver(tokens.Theme.Warning, plane); closeRGB(ink, warn) {
				t.Errorf("the power glyph wears the warning colour %v, not the amber system-icon ink", warn)
			}
		})
	}
}

// The power confirmation band is the reference's `.launcher-system-confirm`
// (`main:src/styles/launcher.css:756-770`): a `--system-icon-surface` fill —
// the warm amber plane a power entry wears — carrying the warning ink on its
// glyph and sentence. The port had filled the band with `t.Warning.Alpha(0.12)`,
// the *label's* colour as the pane's wash: the same mistake the launcher's
// feedback row made. The band's own fill is compared exactly against the warm
// token composited over the panel, and against the warning wash — the two
// candidate fills differ by a few units, so the assertion names the token
// rather than reading the pixel as "warm".
func TestThePowerConfirmBandWearsTheWarmPlane(t *testing.T) {
	// P9-e: both appearances. `IconSurface` is a translucent wash on either side
	// (light 0.13 / dark 0.115), and the pin already composites it over the panel
	// it samples — so the dark run is the same arithmetic over a different panel,
	// which is exactly the read that would catch a band filled with a fixed
	// light value.
	for _, tc := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(tc.name, func(t *testing.T) {
			a := testApp()
			a.Actions.Power = func(string) {}
			tt := renderIn(t, a, tc.dark)
			tt.Type("restart")
			tt.Frame()
			tt.TypeKey(0, ui.KeyEnter, "")
			tt.Frame()
			if a.PowerPending() != PowerRestart {
				t.Fatalf("pending = %q", a.PowerPending())
			}
			msg, ok := tt.Find(a.copy().PowerConfirmMessage(PowerRestart))
			if !ok {
				t.Fatalf("the confirmation is not drawn: %q", tt.Texts())
			}
			// The gap between the glyph and the sentence is the band's own fill;
			// the panel it sits on is read from the same line, left of the band's
			// own inset, rather than assumed. (The assumption used to be the literal
			// `{254 254 254}` the light sunken pane composited to; the pane's own
			// recess moved by a level when the content step was raised — see
			// glassmap.contentStep — and a constant would have to be re-tuned by hand
			// on every such change.)
			img := tt.Image()
			cy := int(msg.Y + msg.H/2)
			band := img.RGBAAt(int(msg.X)-2, cy)
			panel := img.RGBAAt(1, cy)
			tokens := theme.ForPreferences(a.settings(), tc.dark, ui.Preferences{})
			want := over(tokens.IconSurface, panel)
			wash := over(tokens.Theme.Warning.Alpha(0.12), panel)
			// One level of slack per channel: the band is composited by the
			// renderer and `want` by this file's own arithmetic, and the two round
			// the same alpha differently by a fraction. The wash it must not be is
			// 20 levels away, so the slack costs the assertion nothing.
			if d := channelGap(band, want); d > 1 {
				t.Errorf("the confirmation band is %v, want the warm plane %v over the panel (→ %v; %d levels off)",
					band, tokens.IconSurface, want, d)
			}
			if band == wash {
				t.Errorf("the confirmation band is the warning ink at 12%% (%v), not the warm plane", wash)
			}
		})
	}
}

// channelGap is the largest per-channel difference of two rendered pixels.
func channelGap(a, b color.RGBA) int {
	gap := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return max(gap(a.R, b.R), max(gap(a.G, b.G), gap(a.B, b.B)))
}
