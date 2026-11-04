package shell

import (
	"image"
	"image/png"
	"os"
	"testing"
)

// P13-gl · E2 的左/右竖边：影与线已经把它分开了，那条「耳语」没有位置
// （`docs/liquid-glass.md` §6.157）。
//
// §6.155（P13-gj）拆的是 E2 的**顶边**（影 / 描边 / 冠三段），§6.156（P13-gk）拆的是
// **下沿**（影 79 级、线 ±21/22）。同一个盒子的第三对边——**左/右竖边**——从来没进过
// 帧。本轮要回答三件事：
//
//  1. 竖边靠什么分离：影，还是盒子自己那条 `InputStroke`？
//  2. `EdgeHighlightX`（theme 里写着「两侧的耳语描边」，浅 0.6 / 深 0.04）**全仓从未
//     被画过**。它是这一支唯一的「死 token」：真画上去读得出来吗？
//  3. 竖边那条线与下沿那条线是同一个数吗（同一根 `InputStroke`，两条边）。
//
// 场地与顶边/下沿两轮完全相同（真 toast、settings 面、720×520、默认 47% 框 alpha、
// 入场弹簧 settle 后、`straightLuma` 除掉框自身 alpha）。取 **top+12 行**：它已经出了
// 10u 的圆角（圆角在 top+10 结束），又还在 toast 的上下 padding 里，左侧文字列
// （`Space(3)` = 12u）和右侧 ✕ 的右边都没到——两侧的边因此是干净的一条竖线。
//
// 一件必须写清的事实：这个 toast 横跨两片地面——左半压设置面板的不透明纸，右半多出
// 去 4u 压在 47% 的窗玻璃上。所以**左右两侧的读数不是同一张纸上的两个镜像**：
// 左边（纸面）是 pin 的基准，右边用来验证「同一条线换地面会怎么读」。

// floatSideArm is one column-band through a floating pane's vertical edge, in
// straight luminance levels: the ground the shadow fell on (the frame before
// the pane existed), the darkest column of the cast shadow outside the box, the
// box's own 1px line, the column just inside it (where a side crown would live)
// and the face.
type floatSideArm struct {
	ground int
	shadow int
	line   int
	inner  int
	face   int
}

// floatSideReading is both vertical edges of one pane, plus the bottom edge's
// line from the same frame so "one token, two edges" is never a cross-render
// comparison.
type floatSideReading struct {
	left, right floatSideArm
	bottomLine  int // §6.156: face − border on the bottom edge, signed
}

// measureFloatSide reads the two vertical edges of the toast at top+12 and the
// bottom edge's line from the same two frames.
//
// The columns are found by ink, not by layout: the alpha run on the top face row
// brackets the box (a face laid over the 47% glass steps the alpha), then the
// line is the **first** column outward from the face whose luma steps at least
// 12 levels — outward, because on paper the shadow is darker than the line and
// an argmax would pick the shadow instead of the box's own edge.
func measureFloatSide(t *testing.T, dark bool) floatSideReading {
	t.Helper()
	before, after, _, top := floatEdgeFrame(t, dark)

	// Bracket the box on the top face row: this row is over the glass, where the
	// pane's face is an alpha step. The corners (10u) make this run ~2 columns
	// narrower than the box, which is why the line search starts at the run.
	faceRow := top + 2
	left, right := -1, -1
	for x := 0; x < after.Rect.Dx(); x++ {
		if alphaAt(after, x, faceRow) >= 190 {
			if left < 0 {
				left = x
			}
			right = x
		}
	}
	if left < 0 || right <= left {
		t.Fatalf("dark=%v: no pane on the top face row %d", dark, faceRow)
	}

	row := top + 12
	lum := func(img *image.RGBA, x int) int { return straightLuma(shellPixelAt(img, x, row)) }

	arm := func(anchor, dir int) floatSideArm {
		face := lum(after, anchor)
		line := -1
		for i := 0; i <= 12; i++ {
			x := anchor + dir*i
			v := lum(after, x)
			if d := v - face; d >= 12 || d <= -12 {
				line = x
				break
			}
		}
		if line < 0 {
			t.Fatalf("dark=%v: no vertical line within 12 columns of x=%d", dark, anchor)
		}
		shadow := lum(after, line+dir)
		for i := 2; i <= 4; i++ {
			if v := lum(after, line+dir*i); v < shadow {
				shadow = v
			}
		}
		return floatSideArm{
			ground: lum(before, line+dir),
			shadow: shadow,
			line:   lum(after, line),
			inner:  lum(after, line-dir),
			face:   lum(after, line-dir*4),
		}
	}

	// Anchors sit inside the padding band on each side: 6 columns in from the
	// alpha run, i.e. past the line and clear of the message text on the left
	// and of the ✕ on the right.
	reading := floatSideReading{
		left:  arm(left+6, -1),
		right: arm(right-6, +1),
	}
	bottom := measureFloatBottom(t, dark)
	reading.bottomLine = bottom.border - bottom.face
	return reading
}

// TestTheFloatingSideEdgesAreHeldByTheLineAndHaveNoWhisper is the pin.
//
// The vertical edges are **not** the top edge: there is no crown, and there is
// no `EdgeHighlightX` either — the token exists in `theme.Tokens` with a value
// (light 0.6 / dark 0.04) and has never been painted anywhere in the repo. This
// pin records why: the column just inside the box's own line *is the face*, and
// painting the whisper there reads +4 on paper / +9 at night — under the 8-level
// criterion in one appearance and at the threshold in the other, at the price of
// a second lit line 1px inside the first.
func TestTheFloatingSideEdgesAreHeldByTheLineAndHaveNoWhisper(t *testing.T) {
	// P13-gl 定基线（§6.157）：这些行此前没有字面量 pin（量于 P13-gl，真 toast，
	// 720×520，默认 47% 框 alpha，top+12 行的 straight luma）。
	want := map[string]floatSideReading{
		"light": {
			left:       floatSideArm{ground: 246, shadow: 215, line: 227, inner: 249, face: 249},
			right:      floatSideArm{ground: 248, shadow: 187, line: 223, inner: 251, face: 251},
			bottomLine: -22,
		},
		"dark": {
			left:       floatSideArm{ground: 24, shadow: 21, line: 45, inner: 24, face: 24},
			right:      floatSideArm{ground: 32, shadow: 24, line: 52, inner: 26, face: 26},
			bottomLine: +21,
		},
	}
	got := map[string]floatSideReading{}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		r := measureFloatSide(t, dark)
		got[name] = r
		w := want[name]
		if r != w {
			t.Errorf("%s: the vertical edges read %+v, want %+v — if the material moved, "+
				"re-baseline §6.157 with the old → new pair rather than letting the two "+
				"sides and the bottom line drift apart", name, r, w)
		}

		// 157-1: both vertical edges are held by the box's own line, and the sign
		// follows the ground — the same rule §6.154 found on E1's top edge and
		// §6.156 on E2's bottom. Paper: the line is darker than the face; night:
		// it is lit. Stated as an absolute value because both are legitimate.
		for _, side := range []struct {
			name string
			arm  floatSideArm
		}{{"left", r.left}, {"right", r.right}} {
			d := side.arm.line - side.arm.face
			if d < 0 {
				d = -d
			}
			if d < 12 {
				t.Errorf("%s/%s: the vertical line sits %d levels off the face (%d vs %d) — "+
					"the sides of an E2 pane have no crown (see 157-4), so this line is the "+
					"whole boundary", name, side.name, d, side.arm.line, side.arm.face)
			}
			if !dark && side.arm.line-side.arm.face > -12 {
				t.Errorf("%s/%s: the line reads %+d against the face (%d vs %d) — on paper the "+
					"pane's own edge is the dark one", name, side.name,
					side.arm.line-side.arm.face, side.arm.line, side.arm.face)
			}
			if dark && side.arm.line-side.arm.face < 12 {
				t.Errorf("%s/%s: the line reads %+d against the face (%d vs %d) — at night the "+
					"line is lit", name, side.name,
					side.arm.line-side.arm.face, side.arm.line, side.arm.face)
			}
		}

		// 157-2: one token, two edges. `InputStroke` draws the bottom edge and the
		// sides; on the paper side (the left, over the panel's own paper) it has to
		// read the same as the bottom edge in the same frame, or the box's outline
		// is two strengths of the same line.
		if d := (r.left.line - r.left.face) - r.bottomLine; d < -4 || d > 4 {
			t.Errorf("%s: the left vertical line is %+d against the face but the bottom edge "+
				"is %+d in the same frame — one `InputStroke` cannot have two strengths on the "+
				"outline of one box", name, r.left.line-r.left.face, r.bottomLine)
		}

		// 157-3: the paper side separates by its shadow by day and saturates at
		// night, exactly as §6.156 found below the box. (The right side sits on the
		// 47% glass, where the night ground is 32 rather than 24, so it has more to
		// give — that is a different ground, not a different rule.)
		take := r.left.ground - r.left.shadow
		if !dark && take < 24 {
			t.Errorf("light: the side shadow takes only %d levels off the paper (%d → %d), "+
				"want ≥ 24 — on paper the sides, like the bottom, are held up by the throw",
				take, r.left.ground, r.left.shadow)
		}
		if dark && take > 8 {
			t.Errorf("dark: the side shadow takes %d levels off the paper (%d → %d), more than "+
				"the 8 this appearance has to spend — at night a deeper shadow is a black band",
				take, r.left.ground, r.left.shadow)
		}

		// 157-4: there is no side crown. The column just inside the line is the
		// face itself. `EdgeHighlightX` — theme's "whisper of a stroke on both
		// sides", light 0.6 / dark 0.04, never painted anywhere in the repo —
		// reads +4 on paper and +9 at night if it is painted, so it is
		// deliberately not: see §6.157's candidate table.
		if d := r.left.inner - r.left.face; d < -2 || d > 2 {
			t.Errorf("%s: the column inside the box's line reads %d against the face %d — the "+
				"sides of an E2 pane carry no crown (`EdgeHighlight` is the top edge's, §3). "+
				"Painting `EdgeHighlightX` there is a decision, not a fix: it reads +4 on paper",
				name, r.left.inner, r.left.face)
		}
		if d := r.right.inner - r.right.face; d < -2 || d > 2 {
			t.Errorf("%s: the right side's inner column reads %d against its face %d — no crown "+
				"on either side; see the left side's error for the `EdgeHighlightX` reading",
				name, r.right.inner, r.right.face)
		}

		t.Logf("%s: left %+v, right %+v, bottom line %+d", name, r.left, r.right, r.bottomLine)
	}

	// 157-5: the two sides are two grounds, not two appearances: the right side
	// hangs 4u past the panel's paper onto the 47% glass, so by day its shadow has
	// more to give (70 against the paper side's 31) even though both shadows come
	// from the same two throw layers. If this ever inverts, the toast stopped
	// straddling the paper edge and the readings above describe a different box.
	for _, name := range []string{"light", "dark"} {
		l, r := got[name].left, got[name].right
		if takeL, takeR := l.ground-l.shadow, r.ground-r.shadow; takeR <= takeL {
			t.Errorf("%s: the right side's shadow takes %d levels against the paper side's %d — "+
				"the toast straddles the panel's paper edge, and the glass side is the one with "+
				"more headroom in both appearances", name, takeR, takeL)
		}
	}
}

// TestDumpP13GlFloatSide writes the pane this round measured. Opt-in, as every
// dump in this package is:
//
//	FLOTER_P13GL_DUMP=1 go test -count=1 -run TestDumpP13GlFloatSide ./internal/shell/
func TestDumpP13GlFloatSide(t *testing.T) {
	if os.Getenv("FLOTER_P13GL_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13GL_DUMP=1 to write /tmp/floter-P13-gl-floatside-*.png")
	}
	for _, dark := range []bool{false, true} {
		tt := toastTester(t, dark)
		tt.SetScale(3)
		tt.Frame()
		name := "/tmp/floter-P13-gl-floatside-light.png"
		if dark {
			name = "/tmp/floter-P13-gl-floatside-dark.png"
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
