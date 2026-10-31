package settingsui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-ag · 对比度模式里的受光边不许被压到看不见。
//
// `LensRim` 0.09 是 P12-a 挑出来「1x 屏上真的看得见」的那条线（§1）。它跟着
// `LensScale`（`frosted` 0.5 / `regular` 1 / `liquid` 1.6）缩放，而对比度模式
// 又把这个系数乘一道 0.6——于是「增加对比度」的用户在深色面上拿到的受光边是
// 0.09 × 0.5 × 0.6 = **0.027**，白线压在面 65 上只抬 **6 级**：比 P12-a 之前
// 那条 0.04 还低。一个要求「把边给我看清楚」的设置，反而把边抹掉了。
//
// 这条 pin 量的是**像素**：`counterPane` 画的是真的 `theme.Tokens.LitFace`，
// 所以它不预设渲染器算出来的值（换成别的线也会红）。
//
// 同时它守着另一半：普通档位**没有**被抬——`frosted` 依然比 `regular` 安静
// （参考的 `--glass-lens-scale` 是一格真阶梯，本轮的 bug 只是「看不见」，
// 不是「不够响」）。深色 `frosted` 的 10 级是量出来的现状，如实留在那里。
//
// Mutations:
//   - `litEdge` 不看 `contrast`（退回 `scaleAlpha(c, factor)`）→ 四条对比度臂
//     全红（深色 6 级 < 12，浅色 1 级 < 2）；
//   - `LitEdgeFloor` 改到 0.6（对比度模式的 `regular` 系数本身）→ `regular` 臂
//     红（10 级 < 12：地板没起来）；
//   - `factor <= 0` 的守卫删掉 → `theme.TestLensScalesWithTheGlassStop` 的
//     「off kept a lens」红（`off` 档被地板复活出一条线）。
func TestTheContrastModeKeepsTheLitEdge(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			ground := realPageGround(t, appearance.dark)
			lift := func(step string, contrast bool) int {
				tokens := rimStopTokens(appearance.dark, step, contrast)
				crown, _, face := counterPane(t, tokens, appearance.dark, ground)
				return int(crown.R) - int(face.R)
			}
			// A dark face has to show the line P12-a put there; a near-white one
			// can only take a whisper of white whatever the token says (P13-n),
			// so its floor only asks the edge to still exist.
			floor := 2
			if appearance.dark {
				floor = 12
			}
			for _, step := range []string{"frosted", "regular"} {
				if got := lift(step, true); got < floor {
					t.Errorf("[%s] the contrast mode leaves the lit edge at %d levels on the `%s` stop "+
						"(floor %d): asking for more contrast must not erase the face's edge",
						appearance.name, got, step, floor)
				}
			}
			// …and the plain ladder is untouched: the thinner material is still
			// the quieter edge.
			if appearance.dark {
				thin, plain := lift("frosted", false), lift("regular", false)
				if thin > plain {
					t.Errorf("[%s] `frosted` lifts %d, `regular` %d: the stop ladder still has to grade "+
						"the edge (0.5 / 1 / 1.6)", appearance.name, thin, plain)
				}
				if plain < floor {
					t.Errorf("[%s] the plain `regular` stop lifts the edge by only %d levels (floor %d)",
						appearance.name, plain, floor)
				}
			}
		})
	}
}

// rimStopTokens is the app's own token table at one glass stop.
func rimStopTokens(dark bool, step string, contrast bool) theme.Tokens {
	s := settings.Default()
	s.GlassStep = step
	return theme.ForPreferences(s, dark, ui.Preferences{HighContrast: contrast, ReduceMotion: true})
}

// The P13-ag visual evidence: the same E1 face at the two contrast settings,
// side by side, so the `high-contrast` leg can be looked at instead of only
// counted.
//
// Opt-in (`FLOTER_PNG=1`) like the other dumps; `FLOTER_PNG_SUFFIX` names the
// file so the round can write the old and the new table (`-before`).
func TestDumpP13AGRimStopPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	suffix := os.Getenv("FLOTER_PNG_SUFFIX")
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		plain := rimStopTokens(dark, "regular", false)
		contrast := rimStopTokens(dark, "regular", true)
		ground := realPageGround(t, dark)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(plain.Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(20).Width(300).Gap(12).Children(func() {
				for _, tokens := range []theme.Tokens{plain, contrast} {
					e := ui.Row(c).FillWidth().Height(48).
						Radius(plain.RadiusSM).Background(tokens.Control)
					tokens.LitFace(e)
				}
			})
		}, 340, 140)
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		name := "floter-P13-ag-rim-" + appearance + suffix + ".png"
		writePNG(t, filepath.Join("/tmp", name), tt)
	}
}
