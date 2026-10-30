package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-n · E1 是**一对**线：先量，再决定改哪一半。
//
// 任务书点名要量的是「`LensRim` 0.09 在浅色面上是否真看得见」。量下来的答案
// 是「看不见」（面 230 上只抬 +2 级），但**改它会把 accent 面洗成类玻璃边**：
// 浅色档的 accent CTA 顶行在 0.09 时还能读出强调色（`TestThePrimaryCTAIsAnAccentButtonAtRadiusSM`
// 靠它量圆角），抬到 0.55 之后那一行整条变成 55% 的白 —— 正是 §1 禁令 3 点名的
// 「渐变白边模拟高光」。而且浅色档并不缺这半条线：它的对侧（`LensEdge` 0.06）
// 实测把面 230 压到 216（**−13 级**），块靠它站起来。
//
// 真正只剩一半的是**深色**档：它的面是 39，同一条 0.06 的黑线只降 **−2 级**，
// 「底暗一分」在 1x 屏上不存在。所以这一轮改的是深色档的 `LensEdge`：
// **0.06 → 0.20**（实测底内线 37 → 31，−2 → −8；面 65 上 −4 → −13）。
// 黑线在 accent 面上会被色相吃掉（§6.2 的 2-6），所以它不会变成高光。
//
// 为什么量在**真页面的地面**上：`lensProbe` 用的是 `#808080` 中灰地面，浅色
// `Control`（0.07 的黑）落在上面是 119，而真页面的面是 230 —— 在中灰上量一条
// 白线会把抬高**高估一倍**，量一条黑线会把压暗**低估一倍**。所以这条 pin 先渲染
// 真的设置页取地面像素，再把参照面画在那块地面上。

// realPageGround renders the settings page on one appearance and returns the
// pixel the page paints itself with — the ground an E1 face actually sits on.
func realPageGround(t *testing.T, dark bool) ui.Color {
	t.Helper()
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.selectPage(PageGeneral)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, 860, 900)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return imagePixel(tt.Image(), 5, 5)
}

// counterPane renders the **real** E1 material (`theme.Tokens.LitFace`) on the
// page ground and returns the three pixels the pair is read from: the brightest
// of the first three inner rows (the lit edge), the darkest of the last three
// (the counter-side) and the face between them. It does not take the two line
// colours as arguments — that is the point: a pin that painted its own
// reference lines would stay green while `LitFace` painted something else.
func counterPane(t *testing.T, tokens theme.Tokens, dark bool, ground ui.Color) (crown, shade, face ui.Color) {
	t.Helper()
	var box ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(ground)
		ui.Column(c).Padding(20).Width(300).Children(func() {
			e := ui.Row(c).FillWidth().Height(48).Radius(tokens.RadiusSM).Background(tokens.Control)
			tokens.LitFace(e)
			e.Bind(&ref)
		})
		box = c.Resolve(ref).Bounds()
	}, 400, 120)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()
	x := int(box.X) + int(box.W)/2
	face = imagePixel(img, x, int(box.Y+box.H/2))
	crown = imagePixel(img, x, int(box.Y))
	for dy := 1; dy < 3; dy++ {
		if q := imagePixel(img, x, int(box.Y)+dy); liteBrighter(q, crown) {
			crown = q
		}
	}
	shade = imagePixel(img, x, int(box.Y+box.H)-1)
	for dy := 1; dy < 3; dy++ {
		if q := imagePixel(img, x, int(box.Y+box.H)-1-dy); liteDarker(q, shade) {
			shade = q
		}
	}
	return crown, shade, face
}

// TestTheCounterSideIsVisibleOnTheDarkAppearance pins §6.0 的 0-9 行：E1 的一对
// 线里，「底暗」那一半在深色档必须真的压得下去（≥ 6 级）。浅色档的同一条判据
// 早就过了（−13 级），这条 pin 把它一起量着，免得将来有人为了「对称」把浅色的
// 对侧也抬上去 —— 那会把 accent 面洗成类玻璃边。
//
// Mutations:
//   - 深色 `lensEdge` 改回 0.06 → 深色压暗 −2，红；
//   - 深色 `lensEdge` 改到 0.60 → 面被压成灰饼，`liteDarker` 仍绿，但
//     `launcher/raycast_test.go` 的对侧上限档红（0.20 是该外观的上限）；
//   - 把 `LitFace` 的底边换成 `LensRim`（顶亮顶亮）→ `liteDarker` 红。
func TestTheCounterSideIsVisibleOnTheDarkAppearance(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			tokens := theme.ForPreferences(settings.Default(), appearance.dark, ui.Preferences{})
			ground := realPageGround(t, appearance.dark)
			crown, shade, face := counterPane(t, tokens, appearance.dark, ground)
			if !liteDarker(shade, face) {
				t.Fatalf("[%s] the counter-side (%v) is not darker than the face (%v): E1 is a pair (§3)",
					appearance.name, shade, face)
			}
			drop := int(face.R) - int(shade.R)
			if drop < 6 {
				t.Errorf("[%s] the counter-side drops its face by only %d levels (%v under %v on the page ground %v): "+
					"at 1x that is the same pixel, not a shade (liquid-glass.md §1 的 0-9 行 / §3)",
					appearance.name, drop, shade, face, ground)
			}
			// 一半就够：这一支的两套外观各自用**有空间的那一半**说话 —— 深色的面是
			// 39（顶亮 +19、底暗 −8），浅色的面是 230（顶亮 +2、底暗 −13）。谁的一半
			// 抬不起来，另一半必须抬得起来，否则这个块就贴在底上了。
			lift := int(crown.R) - int(face.R)
			if lift < 6 && drop < 10 {
				t.Errorf("[%s] neither side of the pair reads: the lit edge lifts %d, the counter-side drops %d "+
					"(face %v) — the block sits flat on its ground", appearance.name, lift, drop, face)
			}
			t.Logf("[%s] ground %v face %v crown %v (%+d) shade %v (−%d)",
				appearance.name, ground, face, crown, lift, shade, drop)
		})
	}
}

// TestDumpP13NCounterSidePNGs is this round's visual evidence: the same
// reference pane drawn twice on the real page ground — once with the old 0.06
// counter-side, once with the new one. Opt-in (`FLOTER_PNG=1`).
func TestDumpP13NCounterSidePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-n-counter-<appearance>.png")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		tokens := theme.ForPreferences(settings.Default(), dark, ui.Preferences{})
		ground := realPageGround(t, dark)
		old := tokens.LensEdge
		old.A = 15 // 0.06, the value before this round
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(24).Gap(16).Children(func() {
				for _, shade := range []struct {
					label string
					c     ui.Color
				}{{"before 0.06", old}, {fmt.Sprintf("after %.2f", float32(tokens.LensEdge.A)/255), tokens.LensEdge}} {
					ui.Text(c, shade.label).FontSize(tokens.Caption).TextColor(tokens.TextSecondary)
					e := ui.Row(c).FillWidth().Height(48).Radius(tokens.RadiusSM).Background(tokens.Control)
					e.InsetShadow(0, 1, 0, 0, tokens.LensRim)
					e.InsetShadow(0, -1, 0, 0, shade.c)
				}
			})
		}, 420, 240)
		tt.SetDark(dark)
		tt.Frame()
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P13-n-counter-"+appearance+".png")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
