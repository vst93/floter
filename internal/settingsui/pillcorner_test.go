package settingsui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-gn · E1 胶囊的四个圆角：直边上那**两条线**，走到 45° 上还在
// （`docs/liquid-glass.md` §6.159）。
//
// §6.154 把 E1（`Control` 面 + `ControlEdge` 发丝 + `LitFace` 一对内线）的**顶边**拆成
// 两条线：外面那行 `Border(1, ControlEdge)` 是**分离线**（跟着地面反相），它下面那行
// `InsetShadow(0, 1, 0, 0, LensRim)` 是**冠线**（压面，两种外观都亮）。§6.158 量了 E2
// （浮层）的四个圆角：那里直边上的四行墨在 45° 上**塌成一行**，因为 `RadiusMD` 10u 的弧
// 太紧、1px 环在 45° 只有约半像素覆盖率，描边与冠被混进同一个像素。
//
// 这一轮量的是 E1 自己的四个圆角——它的半径是 `Space(3.75)` = **15**，比 E2 的 10 大一档。
// 结论正好相反：**两条线都还在**。45° 对角线上，从盒角往里第 4 格是分离线、第 5 格是冠线
// （底角是 `LensEdge` 的对侧暗线）、第 6 格起才是面。半径 15 让 1px 环在 45° 上摊到两个
// 像素行里，于是 §6.154 的两条线绕到角落**没有被抹平**。探针实测：半径 10 → 一行（224 直接
// 接面），半径 12 → 勉强两行（232/234），半径 15 → 干净两行（227/234）。所以「圆角读得出
// 几条线」是半径的函数，E1 这一档刚好落在「两条都在」那一侧。
//
// 谁会让这些 pin 变红：把胶囊的 `Radius` 收到 10（M1，两条线塌成一行）、把 §6.154 那个
// 被否的候选（顶边也画成亮线）搬上来（M2，浅色分离线并入冠线、深色最亮行被推到冠外）。

// pillCornerArm is one 45° diagonal through an E1 pill's corner, in straight
// luminance levels, plus the diagonal step each ink row sits on.
type pillCornerArm struct {
	ground     int // the page the pill sits on, read at the box corner (i=0)
	border     int // the separation line: `Border(1, ControlEdge)`
	second     int // the row under it: `LensRim` on the top corners, `LensEdge` on the bottom
	face       int // the face, two steps inside the border
	borderStep int // diagonal steps from the box corner to the border row
	secondStep int // … to the second row
	faceStep   int // … to the first face row
}

// measurePillCorner builds one E1 pill — the settings action pill's own material
// (`rows.go` `extensionActionFace`) at its shipped `Space(3.75)` = 15 radius — on
// the real page ground, and walks the four 45° corner diagonals. `litTop` selects
// §6.154's rejected candidate (the border row drawn as a lit line too).
func measurePillCorner(t *testing.T, dark, litTop bool) (tl, tr, bl, br pillCornerArm) {
	t.Helper()
	ground := realPageGround(t, dark)
	s := settings.Default()
	s.UIScale = "default"
	s.Theme = appearanceName(dark)
	a := New(settings.NewStore(s), Actions{})
	var ref ui.Handle
	var box ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		th := c.Theme()
		tokens := a.tokens(c)
		edge := tokens.ControlEdge
		if litTop {
			// The only white the light sheet can spend on a raised edge is the
			// `LensRim` family's own 0.24 — §6.154's candidate, now on the corner.
			edge = ui.RGBA(255, 255, 255, 0.24)
		}
		c.Root().Background(ground)
		ui.Column(c).Padding(10.5).Width(320).Children(func() {
			ui.Column(c).Padding(th.Space(2.25)).Children(func() {
				pill := ui.ButtonBase(c).MinHeight(th.Space(7.5)).
					Padding(0, th.Space(2.5)).Radius(th.Space(3.75)).
					Border(1, edge).Background(tokens.Control)
				tokens.LitFace(pill)
				pill.Children(func() {
					ui.Text(c, "Run").FontSize(tokens.Body).FontWeight(600).
						TextColor(tokens.TextStrong)
				})
				pill.Bind(&ref)
				box = c.Resolve(ref).Bounds()
			})
		})
	}, 360, 140)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	img := tt.Image()
	if box.H < 12 || box.W < 20 {
		t.Fatalf("%s: measured a %v×%v box, not a control", appearanceName(dark), box.W, box.H)
	}
	// The framework snaps both edges with `math.Round` (`mygo@v0.7.3/ui/paint.go:160-167`).
	left, top := int(box.X+0.5), int(box.Y+0.5)
	right, bottom := int(box.X+box.W-0.5), int(box.Y+box.H-0.5)

	// walk reads one 45° diagonal inward from a box corner. The ground is the
	// corner pixel itself (the arc rounds it away to the page); the border is the
	// first step that leaves the ground by ≥ 8; the second row and the face follow.
	walk := func(x0, y0, dx, dy int) pillCornerArm {
		lum := func(i int) int { return int(imagePixel(img, x0+dx*i, y0+dy*i).R) }
		g := lum(0)
		arm := pillCornerArm{ground: g}
		for i := 1; i <= 9; i++ {
			if d := lum(i) - g; d >= 8 || d <= -8 {
				arm.border, arm.borderStep = lum(i), i
				arm.second, arm.secondStep = lum(i+1), i+1
				arm.face, arm.faceStep = lum(i+2), i+2
				return arm
			}
		}
		t.Fatalf("%s: no border row on the diagonal from (%d,%d)", appearanceName(dark), x0, y0)
		return arm
	}
	tl = walk(left, top, +1, +1)
	tr = walk(right, top, -1, +1)
	bl = walk(left, bottom, +1, -1)
	br = walk(right, bottom, -1, -1)
	return tl, tr, bl, br
}

// TestThePillCornerKeepsTwoInkRows is the pin: on the 45° diagonal an E1 pill's
// corner keeps **both** of §6.154's lines (the separation row and the lit/counter
// row) rather than collapsing them into one like E2's tighter RadiusMD corner
// (§6.158), and the outer row still follows the ground's polarity.
func TestThePillCornerKeepsTwoInkRows(t *testing.T) {
	// P13-gn 重定基线（§6.159）：这四个圆角此前从未进帧（量于 P13-gn，与 §6.154
	// 的 `measureTopEdge`、§6.158 的 `measureFloatCorners` 同一把 45° 尺子）。
	want := map[string][4]pillCornerArm{
		"light": {
			{ground: 247, border: 227, second: 234, face: 230, borderStep: 4, secondStep: 5, faceStep: 6}, // TL
			{ground: 247, border: 227, second: 234, face: 230, borderStep: 4, secondStep: 5, faceStep: 6}, // TR
			{ground: 247, border: 227, second: 221, face: 230, borderStep: 4, secondStep: 5, faceStep: 6}, // BL
			{ground: 247, border: 227, second: 221, face: 230, borderStep: 4, secondStep: 5, faceStep: 6}, // BR
		},
		"dark": {
			{ground: 14, border: 40, second: 52, face: 39, borderStep: 4, secondStep: 5, faceStep: 6}, // TL
			{ground: 14, border: 40, second: 52, face: 39, borderStep: 4, secondStep: 5, faceStep: 6}, // TR
			{ground: 14, border: 40, second: 34, face: 39, borderStep: 4, secondStep: 5, faceStep: 6}, // BL
			{ground: 14, border: 40, second: 34, face: 39, borderStep: 4, secondStep: 5, faceStep: 6}, // BR
		},
	}
	names := [4]string{"TL", "TR", "BL", "BR"}
	for _, dark := range []bool{false, true} {
		name := appearanceName(dark)
		tl, tr, bl, br := measurePillCorner(t, dark, false)
		got := [4]pillCornerArm{tl, tr, bl, br}
		for i := range got {
			if got[i] != want[name][i] {
				t.Errorf("%s %s: the corner reads %+v, want %+v — if the material or the "+
					"radius moved, re-baseline §6.159 with the old → new pair rather than "+
					"letting the two lines drift into one", name, names[i], got[i], want[name][i])
			}
			// 159-2: two ink rows survive on the diagonal — the border, then a
			// distinct second row, then the face. This is the whole contrast with
			// §6.158, where E2's RadiusMD corner read one row (borderStep+1 == face).
			if got[i].faceStep != got[i].borderStep+2 {
				t.Errorf("%s %s: the face starts %d steps after the border (borderStep %d, faceStep %d), want 2 — "+
					"a gap of 1 means the corner collapsed both lines into one row (E2's §6.158 failure)",
					name, names[i], got[i].faceStep-got[i].borderStep, got[i].borderStep, got[i].faceStep)
			}
			if got[i].second == got[i].border || got[i].second == got[i].face {
				t.Errorf("%s %s: the second row (%d) is not distinct from the border (%d) and the face (%d) — "+
					"§6.154's two lines must both be readable on the corner",
					name, names[i], got[i].second, got[i].border, got[i].face)
			}
			// 159-3: the outer row separates, and its sign follows the ground.
			if sep := topEdgeAbs(got[i].border - got[i].ground); sep < 12 {
				t.Errorf("%s %s: the separation row leaves the ground by %d levels (border %d / ground %d), want ≥ 12",
					name, names[i], sep, got[i].border, got[i].ground)
			}
			if (got[i].border < got[i].ground) != (got[i].ground > 128) {
				t.Errorf("%s %s: the separation row runs the wrong way (border %d / ground %d): "+
					"a paper ground wants a darker line, a night ground a brighter one",
					name, names[i], got[i].border, got[i].ground)
			}
		}
		// 159-4: the second row lights the face on the top corners and shades it on
		// the bottom (the `LensRim` / `LensEdge` pair), same sign across appearances.
		if tl.second <= tl.face || tr.second <= tr.face {
			t.Errorf("%s: the top corners' second row (%d/%d) is not brighter than the face (%d/%d) — "+
				"the lit edge is the mark of a touchable face (§3), on the corner too",
				name, tl.second, tr.second, tl.face, tr.face)
		}
		if bl.second >= bl.face || br.second >= br.face {
			t.Errorf("%s: the bottom corners' second row (%d/%d) is not darker than the face (%d/%d) — "+
				"the counter-side line belongs under the face (§6.154's `LensEdge`)",
				name, bl.second, br.second, bl.face, br.face)
		}
		// 159-5: §6.154's rejected candidate, on the corner. Light: the separation
		// row jumps toward the ground and nearly merges the crown (border→ground gap
		// collapses). Dark: the brightest row is pushed outside the crown.
		ctl, ctr, _, _ := measurePillCorner(t, dark, true)
		shippedSep := topEdgeAbs(tl.border - tl.ground)
		for _, cand := range []struct {
			label string
			arm   pillCornerArm
		}{{"TL", ctl}, {"TR", ctr}} {
			candSep := topEdgeAbs(cand.arm.border - cand.arm.ground)
			if !dark {
				// On paper the lit border must not erase the separation: shipped
				// separates by 20, the candidate by 8 — it loses more than half.
				if candSep*2 > shippedSep {
					t.Errorf("light %s: the rejected candidate keeps %d of the separation (border %d / ground %d); "+
						"it used to lose more than half — a lit border row on paper is a white edge, not a lit one",
						cand.label, candSep, cand.arm.border, cand.arm.ground)
				}
				if cand.arm.border <= cand.arm.second {
					t.Errorf("light %s: the candidate reads border %d ≤ second %d; on paper the two rows "+
						"used to merge into one band (§6.154's forbidden 「类玻璃边」)",
						cand.label, cand.arm.border, cand.arm.second)
				}
			} else {
				// At night the shipped border is already bright; the candidate's
				// fault is that it becomes the brightest row, outside the crown.
				if cand.arm.border <= cand.arm.second {
					t.Errorf("dark %s: the rejected candidate reads border %d ≤ second %d; it used to put the "+
						"brightest row outside the crown (a halo round the face, not a lit edge on it)",
						cand.label, cand.arm.border, cand.arm.second)
				}
			}
			t.Logf("%s %s: shipped separates %d levels (border %d / second %d / face %d); the lit-top candidate %d (border %d / second %d)",
				name, cand.label, shippedSep, tl.border, tl.second, tl.face,
				candSep, cand.arm.border, cand.arm.second)
		}
	}
}

// TestDumpP13GnPillCorner writes the four corners this round argues about, at 3×.
// Opt-in, as every dump in this package is:
//
//	FLOTER_P13GN_DUMP=1 go test -count=1 -run TestDumpP13GnPillCorner ./internal/settingsui/
func TestDumpP13GnPillCorner(t *testing.T) {
	if os.Getenv("FLOTER_P13GN_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13GN_DUMP=1 to write /tmp/floter-P13-gn-pillcorner-*.png")
	}
	for _, dark := range []bool{false, true} {
		dark := dark
		s := settings.Default()
		s.Theme = appearanceName(dark)
		a := New(settings.NewStore(s), Actions{})
		ground := realPageGround(t, dark)
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			th := c.Theme()
			tokens := a.tokens(c)
			c.Root().Background(ground)
			ui.Row(c).Padding(14).Gap(th.Space(4)).Children(func() {
				for _, edge := range []ui.Color{tokens.ControlEdge, ui.RGBA(255, 255, 255, 0.24)} {
					edge := edge
					ui.Column(c).Gap(th.Space(1.5)).Children(func() {
						pill := ui.ButtonBase(c).MinHeight(th.Space(7.5)).Width(180).
							Padding(0, th.Space(2.5)).Radius(th.Space(3.75)).
							Border(1, edge).Background(tokens.Control)
						tokens.LitFace(pill)
						pill.Children(func() {
							ui.Text(c, "Run").FontSize(tokens.Body).FontWeight(600).
								TextColor(tokens.TextStrong)
						})
					})
				}
			})
		}, 440, 120)
		tt.SetDark(dark)
		tt.SetScale(3)
		tt.Frame()
		tt.Frame()
		name := "/tmp/floter-P13-gn-pillcorner-light.png"
		if dark {
			name = "/tmp/floter-P13-gn-pillcorner-dark.png"
		}
		f, err := os.Create(name)
		if err != nil {
			t.Fatalf("creating %s: %v", name, err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatalf("encoding %s: %v", name, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("closing %s: %v", name, err)
		}
	}
}
