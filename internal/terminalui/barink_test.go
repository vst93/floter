package terminalui

import (
	"image/png"
	"os"
	"sort"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P11-ae：bar 的墨只回答「它浮在哪块面」这一个问题。
//
// 反证（用户实测 F2 的验收标准里的「可读性与对比度达标」，本轮把它量到 bar 上）：
// bar 浮在画布上，画布是调色板自己的底色；窗口外观与调色板相反时（浅色窗口 +
// `ink`/`dusk`/`forest` 等深色调色板），窗口的 `TextSecondary` 落在近黑面上不到
// 2:1。这条 pin 对**每一套调色板 × 两套窗口外观**重新算一遍：bar 自己的墨在
// 它自己的面上必须 ≥ 4.5:1，且那层脸是「够用就最薄」——再薄一档就不过线。
func TestTheBarWearsTheColourOfTheCanvasItFloatsOn(t *testing.T) {
	names := make([]string, 0, len(palettes))
	for name := range palettes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, dark := range []bool{true, false} {
		tokens := theme.For(settings.Default(), dark)
		window := windowBarInks(tokens.Theme, tokens)
		for _, name := range names {
			inks := barInksFor(tokens, window, name, true)
			// Alpha 0 is the honest answer for a palette whose own ink already
			// clears on this window's face (a light palette in a light window):
			// the bar needs no face there, so it paints none.
			face := inks.Face.Over(tokens.Theme.Background)
			got := inkContrast(inks.Ink, face)
			if got < barFaceContrast {
				t.Errorf("%s (window dark=%v): the bar's ink is %.2f:1 on its own face, want ≥ %.1f",
					name, dark, got, barFaceContrast)
			}
			// The bar's ink is the palette's own, not the window's: the two
			// only coincide for `contrast`'s white on black in a dark window.
			if sameRGB(inks.Ink, window.Ink) && inkContrast(window.Ink, face) < barFaceContrast {
				t.Errorf("%s (window dark=%v): the bar still reads the window's secondary rung", name, dark)
			}
			// The quieter rungs are the same ink at the least opacity that
			// still clears its floor — 3:1 for the 16u glyphs (WCAG 1.4.11's
			// non-text contrast), 2.5:1 for the 1.75u status dot, which says
			// "alive" by breathing rather than by being read. Both are pinned
			// as the *quietest* rung that clears: one step fainter and the
			// floor is missed.
			for _, rung := range []struct {
				name  string
				ink   ui.Color
				floor float64
			}{{"muted", inks.Muted, 3.0}, {"tertiary", inks.Tertiary, 2.5}} {
				c := rung.ink.Over(face)
				if got := inkContrast(c, face); got < rung.floor {
					t.Errorf("%s (window dark=%v): the %s rung is %.2f:1, want ≥ %.1f",
						name, dark, rung.name, got, rung.floor)
				}
				if a := float32(rung.ink.A) / 255; a > 1.0/32 {
					fainter := packed(palettes[name].fg).Alpha(a - 1.0/32).Over(face)
					if inkContrast(fainter, face) >= rung.floor {
						t.Errorf("%s (window dark=%v): the %s rung is not the quietest that clears %.1f (alpha %.3f)",
							name, dark, rung.name, rung.floor, a)
					}
				}
			}
			// As light as it can be: the painted face is the least alpha that
			// clears, and one rung thinner the ink fails.
			want := barFaceAlpha(packed(palettes[name].fg), packed(palettes[name].bg), tokens.Theme.Background)
			if a := float32(inks.Face.A) / 255; a < want-1.0/255 || a > want+1.0/255 {
				t.Errorf("%s (window dark=%v): the bar's face is at %.3f, want the least readable %.3f",
					name, dark, a, want)
			}
			if a := float32(inks.Face.A) / 255; a > 1.0/32 {
				thinner := packed(palettes[name].bg).Alpha(a - 1.0/32)
				if inkContrast(packed(palettes[name].fg), thinner.Over(tokens.Theme.Background)) >= barFaceContrast {
					t.Errorf("%s (window dark=%v): the bar's face is denser than it has to be (alpha %.3f)", name, dark, a)
				}
			}
		}
	}
}

// 正面：出厂默认（`inherit`）、未知 id、以及空页（没有会话，也就没有它的调色板）
// 都拿窗口自己的四档、且 bar 不画自己的面——逐像素与参考的浮条一致。
func TestTheShippedBarKeepsTheWindowInk(t *testing.T) {
	for _, dark := range []bool{true, false} {
		tokens := theme.For(settings.Default(), dark)
		window := windowBarInks(tokens.Theme, tokens)
		for _, tc := range []struct {
			name    string
			id      string
			running bool
		}{
			{"inherit", "inherit", true},
			{"unknown id", "not-a-palette", true},
			{"empty page", "ink", false},
		} {
			if got := barInksFor(tokens, window, tc.id, tc.running); got != window {
				t.Errorf("%s (window dark=%v) = %+v, want the window's rungs %+v", tc.name, dark, got, window)
			}
		}
	}
}

// barFrameWithPalette renders the bar over a black root with the stored palette
// and window appearance given, the pointer resting on it so the identity line
// and the bar's own face are both on screen.
func barFrameWithPalette(t *testing.T, id string, dark bool) *ui.Tester {
	t.Helper()
	store := newStore()
	if err := store.Update(func(s *settings.Settings) {
		s.TerminalTheme = id
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		s.Theme = appearance
	}); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, barTitle)
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokensOf(a, c).Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.titleBar(c, copy)
	}, 900, 40)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Move(200, 14)
	tt.Frame()
	return tt
}

// 像素 pin：一条 `ink` 调色板的 bar 在浅色窗口上画出两件肉眼可查的东西——
// 它自己的面（那条 28u 的带子不再是根窗口的黑），与调色板自己的墨（`#E6E9EF`
// 的亮字，而不是窗口的 `#6B6B70`）。`inherit` 的 bar 两件都不做：面透明、
// 墨仍是窗口的次级档。
func TestTheBarPaintsItsOwnFaceOverAPalette(t *testing.T) {
	// A column left of the trailing controls and clear of the identity line's
	// ink: the bar's own face, if it has one, is the only thing there.
	const px = 450
	faceRows := func(tt *ui.Tester) int {
		rows := 0
		for y := 0; y < 40; y++ {
			if channelDelta(sample(tt, px, y), ui.RGBA(0, 0, 0, 1)) > 6 {
				rows++
			}
		}
		return rows
	}
	palette := barFrameWithPalette(t, "ink", false)
	if rows := faceRows(palette); rows == 0 {
		t.Errorf("the bar over `ink` in a light window painted no face: column %d is the root's black", px)
	}
	inherit := barFrameWithPalette(t, "inherit", false)
	if rows := faceRows(inherit); rows != 0 {
		t.Errorf("an `inherit` bar painted a face: %d rows of column %d are not the root's black", rows, px)
	}
	// The brightest ink in the strip: the palette's foreground is a bright
	// neutral, the window's secondary step is a mid grey.
	brightest := func(tt *ui.Tester) int {
		img := tt.Image()
		max := 0
		for y := 6; y < 22; y++ {
			for x := 23; x < 700; x++ {
				i := y*img.Stride + x*4
				if v := int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2]); v > max {
					max = v
				}
			}
		}
		return max
	}
	if got, plain := brightest(palette), brightest(inherit); got <= plain {
		t.Errorf("the palette bar's ink (sum %d) is not brighter than the window's (sum %d)", got, plain)
	}
}

// The P11-ae visual evidence: the bar over `inherit` and over `ink`, in both
// window appearances, so the report can point at a file. Opt-in (`FLOTER_PNG=1`)
// like the other dumps: a test run should not litter /tmp.
func TestDumpP11AeBarPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, id := range []string{"inherit", "ink"} {
		for _, dark := range []bool{true, false} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			tt := barFrameWithPalette(t, id, dark)
			name := "/tmp/floter-P11-ae-bar-" + id + "-" + appearance + ".png"
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
}
