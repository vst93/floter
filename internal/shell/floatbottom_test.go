package shell

import (
	"image"
	"image/png"
	"os"
	"testing"
)

// P13-gk · E2 的下沿：影拿走 79 级，那根线白天是暗的、夜里是亮的
// （`docs/liquid-glass.md` §6.156）。
//
// §6.155（P13-gj）用一行 ✕ 中心把 E2 的**顶边**拆成三段（影 / 描边 / 冠），
// 结论是「纸面上层级全在影上，夜里改由描边行扛」。本轮把同一把尺子翻到
// **下沿**，回答另外三件事：
//
//  1. 下沿靠什么分离——影，还是盒子自己那条 `InputStroke`？
//  2. 那条线在深浅两套外观下是同一个符号吗（顶边那一轮发现「谁在分离」会换人，
//     这里要问「那条线自己是暗的还是亮的」）。
//  3. 「两层影都朝下偏移」这件事，在下沿量出来是多少——以及它会不会被误读成
//     「夜里影不够深」。
//
// 场地与顶边那一轮完全相同（真 toast、settings 面、720×520、默认 47% 框 alpha、
// 入场弹簧 settle 后、`straightLuma` 除掉框自身 alpha）。唯一的差别是列：这里取
// **浮层自己的水平中线**，避开左侧的 mark、右侧的 ✕，也避开盒子的圆角。
//
// 一个必须先说清的事实：这个 toast 挂在设置面板的**顶边上方**，所以它的顶边落在
// 47% 的窗玻璃上（地面 248），而它的**下沿已经压在设置面板自己那张不透明的纸上**
// （地面 246）。两个地面只差 2 级，读数因此可比——但它们不是同一张纸，报告里不许
// 混着说。

// floatBottomArm is one column through a floating pane's bottom edge, in
// straight luminance levels: the pane's face row, its own last (border) row,
// the darkest row of the cast shadow under the box, the ground that shadow fell
// on (the frame before the pane existed), and how many rows the throw takes to
// settle back onto that ground.
type floatBottomArm struct {
	face    int
	border  int
	contact int
	ground  int
	throw   int
}

// topTake is how many levels the pane's cast shadow takes off the page above
// the box (the §6.155 reading, taken from the same two frames).
type floatBottomReading struct {
	floatBottomArm
	topTake int
}

// measureFloatBottom reads one floating pane's bottom edge. The column is the
// pane's own horizontal middle; the rows are found by ink, not by layout, so a
// spacing change moves the whole arm instead of silently re-pointing it.
func measureFloatBottom(t *testing.T, dark bool) floatBottomReading {
	t.Helper()
	before, after, dismiss, top := floatEdgeFrame(t, dark)

	// The pane's horizontal run, read on its own face row.
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
	if left < 0 {
		t.Fatalf("dark=%v: no pane in row %d", dark, faceRow)
	}
	x := (left + right) / 2
	row := func(img *image.RGBA, y int) int { return straightLuma(shellPixelAt(img, x, y)) }

	// The box's own last row: the first row below the face whose luma steps by
	// at least 12 levels. On paper the line is darker than the face, at night
	// it is lighter, so the test has to accept either sign here.
	face := row(after, faceRow)
	border := -1
	for y := faceRow + 1; y < after.Rect.Dy(); y++ {
		if d := row(after, y) - face; d >= 12 || d <= -12 {
			border = y
			break
		}
	}
	if border < 0 {
		t.Fatalf("dark=%v: no bottom border below the face in column %d", dark, x)
	}

	contact := row(after, border+1)
	for y := border + 1; y <= border+6; y++ {
		if v := row(after, y); v < contact {
			contact = v
		}
	}

	// The ground is the same row in the frame *before* the pane existed: the
	// settings panel's own paper, which is what the shadow fell on.
	ground := row(before, border+2)

	last := border
	for y := border + 1; y < after.Rect.Dy(); y++ {
		if row(after, y) >= ground-1 {
			last = y
			break
		}
	}

	// The top edge's own take, from the same two frames, so the "the bottom is
	// the deeper of the two" comparison never compares two different renders.
	xTop := int(dismiss.X + dismiss.W/2)
	topTake := straightLuma(shellPixelAt(before, xTop, top-1)) - straightLuma(shellPixelAt(after, xTop, top-1))

	return floatBottomReading{
		floatBottomArm: floatBottomArm{
			face:    row(after, border-1),
			border:  row(after, border),
			contact: contact,
			ground:  ground,
			throw:   last - border,
		},
		topTake: topTake,
	}
}

// TestTheFloatingBottomEdgeSeparatesWithTheShadowAndTheLineFlipsSign is the pin.
//
// E2's bottom edge is three readings — face, the box's own 1px line, the darkest
// row of the cast shadow — and on paper it is the shadow that separates the pane
// (79 levels, against the top edge's 30) because both shadows are offset down.
// The line itself flips sign between appearances: on paper it is 22 levels
// *darker* than the face, at night 21 levels *lighter* — one token, two
// readings, the same rule §6.154 found on E1's top edge.
func TestTheFloatingBottomEdgeSeparatesWithTheShadowAndTheLineFlipsSign(t *testing.T) {
	// P13-gk 重定基线（§6.156）：这四行此前没有字面量 pin（量于 P13-gk，
	// 真 toast，720×520，默认 47% 框 alpha，浮层水平中线的 straight luma）。
	want := map[string]floatBottomArm{
		"light": {face: 249, border: 227, contact: 167, ground: 246, throw: 33},
		"dark":  {face: 24, border: 45, contact: 16, ground: 23, throw: 18},
	}
	got := map[string]floatBottomReading{}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		arm := measureFloatBottom(t, dark)
		got[name] = arm
		if arm.floatBottomArm != want[name] {
			t.Errorf("%s: the bottom edge reads %+v, want %+v — if the material moved, "+
				"re-baseline §6.156 with the old → new pair rather than letting the "+
				"three rows drift apart", name, arm.floatBottomArm, want[name])
		}

		// 156-3: the line's *sign* follows the ground, exactly as E1's top edge
		// does (§6.154). Paper's line is a dark hairline on a bright face;
		// night's is a lit one. Stated as an absolute value because both are
		// legitimate — what is not legitimate is the line vanishing (which is
		// what the `EdgeShadow` candidate did, see below).
		line := arm.face - arm.border
		if line < 0 {
			line = -line
		}
		if line < 12 {
			t.Errorf("%s: the box's own line sits %d levels off the face (%d vs %d), want ≥ 12 — "+
				"the bottom of an E2 pane is three readings (face, line, shadow); an extra ink "+
				"on its inside collapses two of them into one band", name, line, arm.border, arm.face)
		}
		if !dark && arm.face-arm.border < 12 {
			t.Errorf("light: the bottom line reads %+d against the face (%d vs %d) — on paper the "+
				"pane's own edge is the dark one (the crown only exists at the top, §3)",
				arm.face-arm.border, arm.border, arm.face)
		}
		if dark && arm.border-arm.face < 12 {
			t.Errorf("dark: the bottom line reads %+d against the face (%d vs %d) — at night the "+
				"line is lit, the mirror of the same token on paper",
				arm.border-arm.face, arm.border, arm.face)
		}

		if !dark {
			// 156-2 (paper): the shadow is the separator, and it must be deep —
			// the measured 79 is 2.6× the top edge's 30 because both throw
			// layers are offset downward, so the bottom catches the offsets and
			// the top only catches the blur.
			if take := arm.ground - arm.contact; take < 24 {
				t.Errorf("light: the cast shadow takes only %d levels off the panel (%d → %d), want ≥ 24 — "+
					"on paper the bottom of a floating pane is held up by its shadow, not by its line",
					take, arm.ground, arm.contact)
			}
			// 156-4: and it must be the deeper of the two, by a lot. If this ever
			// stops holding, the shadows stopped being directional (or the top
			// grew one) and the pane's depth moved to the wrong end of the box.
			if arm.ground-arm.contact < 2*arm.topTake {
				t.Errorf("light: the bottom shadow takes %d levels against the top edge's %d — "+
					"both throw layers are offset down, so the bottom is where the depth lives "+
					"(want ≥ 2× the top's)", arm.ground-arm.contact, arm.topTake)
			}
			// 156-5: the throw reaches far enough to read as a throw rather than
			// as one dark row under the box.
			if arm.throw < 24 {
				t.Errorf("light: the throw settles back onto the panel after %d rows, want ≥ 24 — "+
					"a contact edge alone is not a throw", arm.throw)
			}
			t.Logf("light: face %d, line %d (%+d vs face), shadow %d (%d levels off the panel, %+d over the line), "+
				"throw %d rows, top edge takes %d", arm.face, arm.border, arm.border-arm.face,
				arm.contact, arm.ground-arm.contact, arm.border-arm.contact, arm.throw, arm.topTake)
			continue
		}

		// 156-2 (night): the ground is 23, so the shadow cannot separate
		// anything — 7 levels is the whole budget before it is a black band.
		take := arm.ground - arm.contact
		if take > 8 {
			t.Errorf("dark: the cast shadow takes %d levels off the panel (%d → %d), more than the 8 "+
				"this appearance has to spend — at night a deeper shadow is a black band, not depth",
				take, arm.ground, arm.contact)
		}
		if lift := arm.border - arm.contact; lift < 16 {
			t.Errorf("dark: the line stands %d levels over the shadow row (%d vs %d), want ≥ 16 — "+
				"at night the shadow cannot carry the boundary, so the line must",
				lift, arm.border, arm.contact)
		}
		t.Logf("dark: face %d, line %d (%+d vs face), shadow %d (%d levels off the panel, %+d over the line), "+
			"throw %d rows, top edge takes %d", arm.face, arm.border, arm.border-arm.face,
			arm.contact, take, arm.border-arm.contact, arm.throw, arm.topTake)
	}

	// 156-5 (cross appearance): at night the same geometry settles in half the
	// rows. That is saturation, not a shorter shadow — the night ground is 23
	// and the shadow only has 7 levels to give, so it reaches the ground sooner.
	// The guard is here to stop the misreading "the night throw is short, make
	// it bigger": growing it would push the take past 8 and turn it into a band.
	if light, dark := got["light"], got["dark"]; dark.throw >= light.throw {
		t.Errorf("the night throw settles in %d rows against paper's %d — at night the shadow has "+
			"fewer levels to give, so it must land sooner; equal or longer throws mean the two "+
			"appearances are no longer the same geometry", dark.throw, light.throw)
	}
}

// TestDumpP13GkFloatBottom writes the pane the round measured. Opt-in, as every
// dump in this package is:
//
//	FLOTER_P13GK_DUMP=1 go test -count=1 -run TestDumpP13GkFloatBottom ./internal/shell/
func TestDumpP13GkFloatBottom(t *testing.T) {
	if os.Getenv("FLOTER_P13GK_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13GK_DUMP=1 to write /tmp/floter-P13-gk-floatbottom-*.png")
	}
	for _, dark := range []bool{false, true} {
		tt := toastTester(t, dark)
		tt.SetScale(3)
		tt.Frame()
		name := "/tmp/floter-P13-gk-floatbottom-light.png"
		if dark {
			name = "/tmp/floter-P13-gk-floatbottom-dark.png"
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
