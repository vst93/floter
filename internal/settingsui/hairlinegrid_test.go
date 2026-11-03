package settingsui

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-fu · 1px 发丝线落在**分数坐标**上的剖面（`docs/liquid-glass.md` §6.140）。
//
// 这一族 pin 回答的是「界面为什么糊」这个问题里唯一还没被量过的物理解释：
// 1px 的线会不会因为盒顶落在 17.7 / 18.6 / 19.5 / 20.4 这样的分数上，被画成
// **两行各半**（那是抗锯齿的典型症状：一条 0.06 的黑线摊成两行 0.03，两行都
// 在噪声里）。
//
// 答案是不会：框架在 `mygo@v0.7.3/ui/paint.go:160-167` 的 `snap` 里把矩形的
// **两条边都四舍五入**到整设备像素（注释原文 “so that edges stay crisp”），
// 所以一条 `Border(1, …)` 永远是**一行**；分数坐标留下的只是 ≤0.5px 的落点偏移。
//
// 实测（真 `actionButton`，父级 `Padding(t.Space(2.25))`，四个界面台阶：
// `Space(2.25)` = 7.200 / 8.100 / 9.000 / 9.900，盒顶 = 17.700 / 18.600 /
// 19.500 / 20.400，小数部分各不相同 .7/.6/.5/.4）：
//
//	浅色  边 216 · 冠 236 · 面 230（地面 247）
//	深色  边  54 · 冠  58 · 面  39（地面  14）
//
// 四个台阶**逐值相同**——线的强度与缩放无关，也没有任何一行被摊成两行。
// 顺带量到一条不对称（不改，见 §6.140 的「需拍板」）：同一条顶边在浅色上读作
// 「暗描边 + 亮点」（边比面暗 14），在深色上读作「亮线 + 更亮的线」（边比面亮 15）。
//
// 谁会让这些 pin 变红：把 `Border` / `InsetShadow` 换成 `Shadow` 或带模糊的画法、
// 在胶囊外面套一层 `Opacity`（那正是「线被摊成两半」的症状）、或者哪天盒顶的
// 四舍五入不再由框架统一做。

// hairlineReading is the three rows a raised control's top edge owns: its own
// `Border(1, ControlEdge)` row, the `LensRim` inset under it, and the face.
type hairlineReading struct {
	top, y    int
	edge      int
	rim       int
	face      int
	frac      float32 // the box top's fractional part — must not be 0
	faceRows  int     // how many consecutive rows below the rim carry the face
	groundRow int     // the row above the edge: the page, untouched
}

// hairlineProbe builds one real action pill under a **fractional** padding and
// reads the four rows its top edge owns.
func hairlineProbe(t *testing.T, step string, dark bool) hairlineReading {
	t.Helper()
	ground := realPageGround(t, dark)
	s := settings.Default()
	s.UIScale = step
	s.Theme = appearanceName(dark)
	a := New(settings.NewStore(s), Actions{})
	var ref ui.Handle
	var box ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		th := c.Theme()
		c.Root().Background(ground)
		ui.Column(c).Padding(10.5).Width(320).Children(func() {
			// `Space(2.25)` is 8.1 / 9.0 / 9.9 / 7.2 depending on the step: the
			// box top lands on a fraction at three of the four, which is the
			// whole point of the pin.
			ui.Column(c).Padding(th.Space(2.25)).Children(func() {
				a.actionButton(c, "Run").Bind(&ref)
				box = c.Resolve(ref).Bounds()
			})
		})
	}, 400, 160)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()
	x := int(box.X + box.W/2)
	// The framework snaps both edges with `math.Round` (`ui/paint.go:160-167`), so
	// the row the top edge lands on is the *rounded* Y, not the truncated one.
	top := int(math.Round(float64(box.Y)))
	row := func(dy int) int { return int(imagePixel(img, x, top+dy).R) }
	r := hairlineReading{
		top: top, y: top,
		groundRow: row(-1),
		edge:      row(0),
		rim:       row(1),
		face:      row(2),
		frac:      box.Y - float32(math.Floor(float64(box.Y))),
	}
	for dy := 2; dy <= 6; dy++ {
		if row(dy) == r.face {
			r.faceRows++
			continue
		}
		break
	}
	if box.H < 12 || box.W < 20 {
		t.Fatalf("%s/%s: measured a %v×%v box, not a control", step, appearanceName(dark), box.W, box.H)
	}
	return r
}

// TestTheHairlineIsOneDeviceRowAtEveryScale is the pin: at all four interface
// steps, on both sheets, a 1px line is one row, its strength does not change
// with the step, and the face starts exactly two rows down.
func TestTheHairlineIsOneDeviceRowAtEveryScale(t *testing.T) {
	// P13-fu 重定基线：这些数此前没有 pin（量于 P13-fu，HEAD `266f3760`）。
	// 左 = 浅色，右 = 深色。
	want := map[string]struct{ edge, rim, face, ground int }{
		"light": {edge: 216, rim: 236, face: 230, ground: 247},
		"dark":  {edge: 54, rim: 58, face: 39, ground: 14},
	}
	for _, step := range []string{"tiny", "small", "default", "large"} {
		for _, dark := range []bool{false, true} {
			r := hairlineProbe(t, step, dark)
			w := want[appearanceName(dark)]
			name := step + "/" + appearanceName(dark)
			// (a) this frame really exercised a fractional box top.
			if r.frac == 0 {
				t.Errorf("%s: the box top sits on %d.0 — this frame measures an integer coordinate, "+
					"so it says nothing about fractional ones; fix the padding, not the pin", name, r.top)
			}
			// (b) one row of edge, and the row above it is still the page.
			if r.groundRow != w.ground {
				t.Errorf("%s: the row above the edge reads %d, want the page %d — the line bled upward",
					name, r.groundRow, w.ground)
			}
			// (c) the three rows, value for value.
			if r.edge != w.edge || r.rim != w.rim || r.face != w.face {
				t.Errorf("%s: edge/rim/face = %d/%d/%d, want %d/%d/%d. A line drawn across two rows "+
					"would land near half the edge's strength in each (%d and %d); the framework's "+
					"`snap` (ui/paint.go:160-167) rounds both edges, so this must stay one row",
					name, r.edge, r.rim, r.face, w.edge, w.rim, w.face,
					(w.ground+w.edge)/2, w.face)
			}
			// (d) the face is flat from two rows down: the line does not seep.
			if r.faceRows < 3 {
				t.Errorf("%s: the face holds for %d rows below the rim, want ≥3 — the edge is bleeding into the pane",
					name, r.faceRows)
			}
			// (e) the lit rim survives at device resolution on both sheets: it is
			// brighter than the face it sits on (§3's one sentence).
			if r.rim <= r.face {
				t.Errorf("%s: the rim reads %d over a %d face — the lit edge went out", name, r.rim, r.face)
			}
		}
	}
}
