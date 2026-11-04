package shell

import (
	"image"
	"image/png"
	"os"
	"testing"
)

// P13-gm · E2 的四个圆角：直边上那四行墨，走到 45° 上只剩一行
// （`docs/liquid-glass.md` §6.158）。
//
// §6.155 拆了 E2 的顶边（影 / 描边 / 冠 / 面，**四行**），§6.156 拆了下沿，
// §6.157 拆了两侧。四条直边量完，盒子还剩四个 10u 的圆角——`RadiusMD` 的弧上，
// 那四行墨各自落在哪一像素、还读不读得出来，从来没有进过帧。
//
// 量的是真 toast（`internal/shell/notices.go`：`Float` + `Border(1, InputStroke)` +
// `InsetShadow(0, 1, 0, 0, EdgeHighlight)` + `ElevThrow`），settings 面、720×520、
// 默认 47% 框 alpha、入场弹簧 settle 后。取每个角的 **45° 对角线**：从盒角开始向里
// 每次走一格（dx, dy 同号），记下影最深的那一格、弧上那一格、它里面那一格和面。
//
// 为什么是 45°：直边上的每一行只有一种墨（影 / 描边 / 冠），而弧上的 1px 环在 45°
// 只有约半个像素的覆盖率，于是描边与它里面那一行（冠）被混进**同一个像素**。这正是
// 直边读得出、圆角读不出的那件事——量它，才能知道「圆角要不要补一笔」是伪命题。
//
// 两件量法上的事实：
//   - 盒子的**左**边不能用 alpha 定位：toast 左半边压在设置面板不透明的纸上（alpha 255，
//     与盒内相同），alpha 台阶只存在于玻璃上。左边改用 §6.157 那把尺子（纸面 → 影 →
//     盒子自己的线），下沿用「面的某一列往下第一次跳 ≥ 12」。
//   - 影的最深一格先找，弧上那一格定义为**从影最深处往里第一次回升 ≥ 8 级**——
//     这在纸面、玻璃、深、浅四种地面下都是同一个规则，不看 alpha。

// floatCornerArm is one 45° diagonal through a floating pane's corner, in
// straight luminance levels.
type floatCornerArm struct {
	ground    int // the shadow's darkest pixel, read from the frame before the pane existed
	shadow    int // that same pixel with the pane's throw on it
	arc       int // the one inked pixel on the 45° arc (border + crown blended)
	inside    int // one step further in: the face, or the crown on the top corners
	face      int // the face, two steps inside the arc
	clearance int // diagonal steps from the box corner to the first face pixel
	arcStep   int // diagonal steps from the box corner to the arc pixel
}

// floatCornerReading is one pane's four corners.
type floatCornerReading struct {
	topLeft, topRight, bottomLeft, bottomRight floatCornerArm
}

// floatCornerBox locates the toast by ink rather than by layout. The top row
// comes from the shared helper (alpha step in the ✕'s column); the two side
// borders and the bottom border are found with the §6.157 rule: walk in from the
// ground, find the cast shadow's well, and take the first column that climbs ≥ 8
// levels back out of it — that column is the pane's own 1px line. It works on
// paper and on glass, light and dark, which is why the corners can be read on
// both grounds with one ruler.
func floatCornerBox(t *testing.T, after *image.RGBA, top int) (left, right, bottom int) {
	t.Helper()
	row := top + 20
	lum := func(x, y int) int { return straightLuma(shellPixelAt(after, x, y)) }

	// frame remembers the two ground polarities: on paper the cast shadow is
	// the first thing that steps away from the ground (light 21 levels), while at
	// night the same shadow only takes 3 and the pane's own line is the first
	// step (21). So the onset is "first step of ≥ 12", and if the ink keeps
	// sinking past it, the border is where it climbs back out of the well.
	edge := func(anchor, dir int) int {
		ground := lum(anchor, row)
		for i := 0; i < 400; i++ {
			x := anchor + dir*i
			if x < 1 || x >= after.Rect.Dx()-1 {
				break
			}
			d := lum(x, row) - ground
			if d >= -12 && d <= 12 {
				continue
			}
			min := lum(x, row)
			if d < 0 {
				for j := i; j < i+10; j++ {
					v := lum(anchor+dir*j, row)
					if v < min {
						min = v
					}
					if v-min >= 8 {
						return anchor + dir*j
					}
				}
			}
			return x
		}
		return -1
	}
	left = edge(350, +1)
	// The right edge cannot use that rule: at this row the pane's throw reaches
	// the window's right margin (light 177 against the ground's 246 at x=706, and
	// nothing but the window's own glass beyond x=709), so there is no clean
	// ground to step away from. The frame's alpha is the locator instead: the
	// pane's border is the last pixel of the run of ≥190 that ends at the pane.
	right = -1
	for x := after.Rect.Dx() - 1; x >= 0; x-- {
		if alphaAt(after, x, row) >= 190 {
			right = x
			break
		}
	}
	if left < 0 || right < 0 {
		t.Fatalf("no side borders on row %d (left=%d right=%d)", row, left, right)
	}

	// The bottom edge is the border row itself: it is read 40 columns in from the
	// left border (past the 10u arc, so the edge is flat) and from 30 rows down
	// (below the message's glyph ink, which stops around top+26).
	x, scan := left+40, top+30
	face := lum(x, scan)
	bottom = -1
	for y := scan; y < after.Rect.Dy(); y++ {
		if d := lum(x, y) - face; d >= 12 || d <= -12 {
			bottom = y
			break
		}
	}
	if bottom < 0 {
		t.Fatalf("no bottom edge under column %d", x)
	}
	return left, right, bottom
}

// measureFloatCorners walks the four 45° diagonals.
func measureFloatCorners(t *testing.T, dark bool) floatCornerReading {
	t.Helper()
	before, after, _, top := floatEdgeFrame(t, dark)
	left, right, bottom := floatCornerBox(t, after, top)
	lum := func(img *image.RGBA, x, y int) int { return straightLuma(shellPixelAt(img, x, y)) }

	corner := func(x0, y0, dx, dy int) floatCornerArm {
		val := func(i int) int { return lum(after, x0+dx*i, y0+dy*i) }
		// The shadow's darkest diagonal step, searched one to three steps out:
		// step 0 is the corner pixel itself and is always a partial blend.
		deep, deepest := 1, val(1)
		for i := 2; i <= 3; i++ {
			if v := val(i); v < deepest {
				deep, deepest = i, v
			}
		}
		arc := -1
		for i := deep + 1; i <= deep+5; i++ {
			if val(i)-deepest >= 8 {
				arc = i
				break
			}
		}
		if arc < 0 {
			t.Fatalf("dark=%v: no arc step after the shadow at (%d,%d)%+d%+d", dark, x0, y0, dx, dy)
		}
		// The face is read two steps inside the arc, not four: on the top corners
		// the settings panel's own paper edge crosses the diagonal four or five
		// rows in, and the pane's face reads 249 over paper instead of 251 over
		// the glass (a different ground, not a different ink).
		face := lum(after, x0+dx*(arc+2), y0+dy*(arc+2))
		clearance := -1
		for i := arc; i <= arc+4; i++ {
			if d := val(i) - face; d >= -2 && d <= 2 {
				clearance = i
				break
			}
		}
		if clearance < 0 {
			t.Fatalf("dark=%v: the face never settles inside the corner at (%d,%d)", dark, x0, y0)
		}
		return floatCornerArm{
			ground:    lum(before, x0+dx*deep, y0+dy*deep),
			shadow:    deepest,
			arc:       val(arc),
			inside:    val(arc + 1),
			face:      face,
			clearance: clearance,
			arcStep:   arc,
		}
	}
	return floatCornerReading{
		topLeft:     corner(left, top, +1, +1),
		topRight:    corner(right, top, -1, +1),
		bottomLeft:  corner(left, bottom, +1, -1),
		bottomRight: corner(right, bottom, -1, -1),
	}
}

// TestTheFloatingCornerSqueezesFourInkRowsIntoOne is the pin.
//
// The four straight edges of an E2 pane are four rows of ink on the top edge
// (shadow / border / crown / face, §6.155) and three elsewhere (§6.156 / §6.157).
// On the 10u corners those rows collapse into **one** pixel: at 45° a 1px ring
// has about half a pixel of coverage, so the border and the crown underneath it
// blend. What is measured here is that blend — and that no fourth ink grew at the
// corner to compensate for it.
func TestTheFloatingCornerSqueezesFourInkRowsIntoOne(t *testing.T) {
	// P13-gm 定基线（§6.158）：四角此前没有字面量 pin（量于 P13-gm，真 toast，
	// 720×520，默认 47% 框 alpha，45° 对角线的 straight luma）。
	want := map[string]floatCornerReading{
		"light": {
			topLeft:     floatCornerArm{ground: 248, shadow: 220, arc: 238, inside: 251, face: 251, clearance: 4, arcStep: 3},
			topRight:    floatCornerArm{ground: 248, shadow: 220, arc: 238, inside: 251, face: 251, clearance: 4, arcStep: 3},
			bottomLeft:  floatCornerArm{ground: 246, shadow: 194, arc: 233, inside: 249, face: 249, clearance: 4, arcStep: 3},
			bottomRight: floatCornerArm{ground: 246, shadow: 155, arc: 229, inside: 249, face: 249, clearance: 4, arcStep: 3},
		},
		"dark": {
			topLeft:     floatCornerArm{ground: 32, shadow: 28, arc: 61, inside: 27, face: 26, clearance: 4, arcStep: 3},
			topRight:    floatCornerArm{ground: 32, shadow: 28, arc: 61, inside: 27, face: 26, clearance: 4, arcStep: 3},
			bottomLeft:  floatCornerArm{ground: 23, shadow: 18, arc: 38, inside: 24, face: 24, clearance: 4, arcStep: 3},
			bottomRight: floatCornerArm{ground: 31, shadow: 20, arc: 44, inside: 25, face: 24, clearance: 4, arcStep: 3},
		},
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		got := measureFloatCorners(t, dark)
		w := want[name]
		if got != w {
			t.Errorf("%s: the four corners read %+v, want %+v — if the material moved, "+
				"re-baseline §6.158 with the old → new pair rather than letting the corners "+
				"and the four edges drift apart", name, got, w)
			continue
		}

		// The straight edges from the *same* frames, so the blend below is never a
		// cross-render comparison.
		top := measureFloatEdge(t, dark)
		bottom := measureFloatBottom(t, dark)

		for _, c := range []struct {
			name string
			arm  floatCornerArm
			lo   int // the two inks the straight edge puts on either side of the arc
			hi   int
		}{
			{"topLeft", got.topLeft, top.border, top.crown},
			{"topRight", got.topRight, top.border, top.crown},
			{"bottomLeft", got.bottomLeft, bottom.border, bottom.face},
			{"bottomRight", got.bottomRight, bottom.border, bottom.face},
		} {
			lo, hi := c.lo, c.hi
			if lo > hi {
				lo, hi = hi, lo
			}
			// 158-1: the arc pixel is the blend of the two inks the straight edge
			// separates. On the top edge the crown is 1px *inside* the border, so at
			// 45° they land on one pixel; on the bottom there is no crown, so the
			// blend is border → face. Either way the arc is strictly between the two
			// and never a third ink.
			if c.arm.arc <= lo || c.arm.arc >= hi {
				t.Errorf("%s/%s: the arc reads %d, outside the %d…%d the straight edge "+
					"spends on the same two rows (%d and %d) — a corner that reads outside "+
					"its own edge is a fourth ink, not a corner",
					name, c.name, c.arm.arc, lo, hi, c.lo, c.hi)
			}
			if mid := (lo + hi) / 2; c.arm.arc < mid-12 || c.arm.arc > mid+12 {
				t.Errorf("%s/%s: the arc reads %d against the blend's midpoint %d (%d…%d) — "+
					"the 1px ring at 45° has about half coverage, so the corner must read as "+
					"the average of the two rows it merges, not as one of them",
					name, c.name, c.arm.arc, mid, lo, hi)
			}

			// 158-2: exactly one inked row. On the straight edges the ink spans two
			// rows (border + crown on top, §6.155); the diagonal may spend exactly
			// one, and the face must begin right after it. A second inked row would
			// be someone "fixing" the corner by thickening it.
			if c.arm.clearance != c.arm.arcStep+1 {
				t.Errorf("%s/%s: the diagonal spends %d rows of ink (arc at step %d, face at "+
					"step %d) — the corner merges the edge's rows, it does not add one",
					name, c.name, c.arm.clearance-c.arm.arcStep, c.arm.arcStep, c.arm.clearance)
			}
			// 158-3: no lit line inside the arc — §6.157's rule carried around the
			// corner: what sits inside the corner's line is the face itself.
			if d := c.arm.inside - c.arm.face; d < -2 || d > 2 {
				t.Errorf("%s/%s: the pixel inside the arc reads %d against the face %d — "+
					"the corner has no crown of its own; the top edge's crown is what the "+
					"arc pixel has already absorbed", name, c.name, c.arm.inside, c.arm.face)
			}
		}

		// 158-4: one radius, four corners. The 45° clearance is the same on every
		// corner (and on both appearances): `RadiusMD` is one number, so the arc is
		// one arc — asymmetric corners would read as a hand-drawn box.
		for _, c := range []struct {
			name string
			arm  floatCornerArm
		}{{"topLeft", got.topLeft}, {"topRight", got.topRight}, {"bottomLeft", got.bottomLeft}, {"bottomRight", got.bottomRight}} {
			if c.arm.arcStep != got.topLeft.arcStep {
				t.Errorf("%s/%s: the arc sits at diagonal step %d but the top-left corner's "+
					"sits at %d — one `RadiusMD`, four corners, one 45° clearance",
					name, c.name, c.arm.arcStep, got.topLeft.arcStep)
			}
		}

		// 158-5: the corner's shadow follows the light from above, exactly as the
		// straight edges do (§6.156 takes 79 by day / 7 at night below, §6.157 31 on
		// the paper side). Light can spend it, night cannot: by day the bottom
		// corners take several times what the top corners take, at night both
		// saturate.
		for _, pair := range []struct {
			name        string
			top, bottom floatCornerArm
		}{{"left", got.topLeft, got.bottomLeft}, {"right", got.topRight, got.bottomRight}} {
			takeTop := pair.top.ground - pair.top.shadow
			takeBottom := pair.bottom.ground - pair.bottom.shadow
			if takeBottom <= takeTop {
				t.Errorf("%s/%s: the bottom corner's shadow takes %d levels against the top "+
					"corner's %d — the throw is offset downwards; a corner that takes more "+
					"shadow above than below means the light moved",
					name, pair.name, takeBottom, takeTop)
			}
			if !dark && takeBottom*2 < takeTop*3 {
				t.Errorf("%s/%s: on paper the bottom corner takes %d levels against the top "+
					"corner's %d, want at least half again as much — the vertical offset of "+
					"the throw is what makes the box read as lifted", name, pair.name, takeBottom, takeTop)
			}
			if dark && takeBottom > 16 {
				t.Errorf("%s/%s: at night the bottom corner's shadow takes %d levels — this "+
					"appearance has at most 8 to spend on a straight edge (§6.156); a deeper "+
					"corner shadow is a black band, not depth", name, pair.name, takeBottom)
			}
		}
		t.Logf("%s: TL%+v TR%+v BL%+v BR%+v (straight: border %d, crown %d, bottom border %d)",
			name, got.topLeft, got.topRight, got.bottomLeft, got.bottomRight,
			top.border, top.crown, bottom.border)
	}
}

func TestDumpP13GMFloatCorner(t *testing.T) {
	if os.Getenv("FLOTER_P13GM_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13GM_DUMP=1 to write /tmp/floter-P13-gm-floatcorner-*.png")
	}
	for _, dark := range []bool{false, true} {
		tt := toastTester(t, dark)
		tt.SetScale(3)
		tt.Frame()
		name := "/tmp/floter-P13-gm-floatcorner-light.png"
		if dark {
			name = "/tmp/floter-P13-gm-floatcorner-dark.png"
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
