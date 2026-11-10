package shell

import (
	"image"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-gj · E2 的顶边：影、描边、冠，三段里谁在干活（`docs/liquid-glass.md` §6.155）。
//
// §6.154（P13-gi）把 **E1**（长在地面上的面）的顶边拆成两条线：一条 `ControlEdge`
// 发丝把它从**地面**上分出来，一条 `LensRim` 冠线点亮它自己。E2（浮层）用的是另外
// 一套四件东西（§2 判据 3）：`Float` 面 + `Border(1, InputStroke)` + 一行
// `EdgeHighlight` 冠线 + `ElevThrow` 两层影。本轮要回答三件事：
//
//  1. E2 的「分离」由谁做——那 1px 描边，还是它头上那层影？
//  2. 冠线在深浅两套外观下是不是同号。
//  3. 一整条顶边读成几条线。
//
// 量的是真 toast（`internal/shell/notices.go`：`Float` + `InputStroke` +
// `EdgeHighlight` + `ElevThrow`，四件齐全，且完整画在窗口里、影不会被裁），
// settings 面，720×520，入场弹簧 settle 之后。
//
// 量法两件事值得先说：
//
//   - 窗玻璃是半透明的（默认 47%），`ui.Tester` 的缓冲是**预乘**的，所以同一列上
//     alpha 不同的两行不能直接比亮度——`straightLuma` 先把框自己的 alpha 除掉。
//   - 盒子的边界用 **alpha 台阶**定位（`Float` 面叠在 47% 的框上是 196，纸面是 127），
//     不用亮度：浅色档上盒沿的亮度只比纸面高 3 级，而 alpha 有 70 级。alpha 是量尺的
//     定位器，不是设计事实（合成到桌面上之后它不存在）。

// floatEdgeArm is one column through a floating pane's top edge, in straight
// luminance levels: the page at the shadow row (read from the frame **before**
// the toast existed), the same row once the pane's cast shadow is on it, the
// pane's own border row, the crown row under it, and the face.
type floatEdgeArm struct {
	ground int // 同一行、同一列，但取「还没有这张浮层时」的帧
	shadow int // 影最重的那一行：紧贴盒上沿
	border int // 盒子自己那一行：Border(1, InputStroke) 压在 Float 面上
	crown  int // 它下面那一行：InsetShadow 顶 EdgeHighlight
	face   int // 面
}

// floatEdgeFrame renders the settings surface, freezes it, then queues one
// notice on the app's injected clock and freezes the settled frame. The two
// frames share the pane's position, so every row can be read twice: with and
// without the floating pane on it.
func floatEdgeFrame(t *testing.T, dark bool) (before, after *image.RGBA, box ui.Rect, paneTop int) {
	t.Helper()
	a := newApp(t)
	if dark {
		if err := a.Store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Unix(0, 0)
	a.toastClock = func() time.Time { return now }
	a.Open(SurfaceSettings)
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.SetDark(dark)
	tt.Frame()
	before = shellFrameCopy(tt.Image())

	a.warnNotice("Where am I")
	tt.Frame()
	now = now.Add(toastEnter)
	tt.Frame()
	now = now.Add(toastEnter)
	tt.Frame()

	dismiss, ok := tt.Find(i18n.For("en").Settings.DismissNotice)
	if !ok {
		t.Fatalf("dark=%v: the toast has no dismiss control: %q", dark, tt.Texts())
	}
	after = shellFrameCopy(tt.Image())
	x := int(dismiss.X + dismiss.W/2)
	// Walk up from the ✕'s first row: the pane's box starts where the frame's
	// alpha steps (the pane is a face laid over the glass), which is the one
	// edge that is unambiguous in both appearances.
	top := int(dismiss.Y)
	for dy := 0; dy > -40; dy-- {
		if alphaAt(after, x, top+dy) >= 250 && alphaAt(after, x, top+dy-1) < 250 {
			paneTop = top + dy
			break
		}
	}
	if paneTop == 0 {
		t.Fatalf("dark=%v: no pane edge in the column through the ✕ (x=%d)", dark, x)
	}
	return before, after, dismiss, paneTop
}

// measureFloatEdge reads the five rows one floating pane's top edge owns.
func measureFloatEdge(t *testing.T, dark bool) floatEdgeArm {
	t.Helper()
	before, after, dismiss, top := floatEdgeFrame(t, dark)
	x := int(dismiss.X + dismiss.W/2)
	row := func(img *image.RGBA, y int) int { return straightLuma(shellPixelAt(img, x, y)) }
	return floatEdgeArm{
		ground: row(before, top-1),
		shadow: row(after, top-1),
		border: row(after, top),
		crown:  row(after, top+1),
		face:   row(after, top+2),
	}
}

// straightLuma is a pixel's luma with the frame's own alpha divided back out:
// the settings surface is drawn at the window's glass alpha (47%), so the
// tester's buffer is premultiplied and two rows with different alpha are not
// comparable until they are put back on the same scale.
func straightLuma(c ui.Color) int {
	if c.A == 0 {
		return 0
	}
	r := int(c.R) * 255 / int(c.A)
	g := int(c.G) * 255 / int(c.A)
	b := int(c.B) * 255 / int(c.A)
	// The usual 299/587/114 weights over 1000: a neutral grey at level v reads v.
	return (r*299 + g*587 + b*114) / 1000
}

// alphaAt reads one pixel's alpha; outside the frame there is nothing, i.e. 0.
func alphaAt(img *image.RGBA, x, y int) int {
	if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		return 0
	}
	return int(img.Pix[y*img.Stride+x*4+3])
}

// TestTheFloatingTopEdgeIsThreePartsAndTheySwapRoles is the pin. E2's top edge
// is **three** rows, and which one separates the pane from the page flips with
// the ground's polarity — paper's ground is 248, so a cast shadow can take 30
// levels off it and the 1px line only lifts 5; night's ground is 32, so the
// shadow can take 4 and the line has to carry the boundary (+24). The crown is
// lit in both, but on paper it is clipped at pure white (255) and therefore
// only 4 levels above the face.
func TestTheFloatingTopEdgeIsThreePartsAndTheySwapRoles(t *testing.T) {
	// P13-gj 重定基线（§6.155）：这五行此前没有字面量 pin（量于 P13-gj，
	// 真 toast，720×520，默认 47% 框 alpha，straight luma）。
	want := map[string]floatEdgeArm{
		"light": {ground: 248, shadow: 237, border: 227, crown: 255, face: 250},
		"dark":  {ground: 24, shadow: 23, border: 46, crown: 71, face: 25},
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		got := measureFloatEdge(t, dark)
		if got != want[name] {
			t.Errorf("%s: the top edge reads %+v, want %+v — if the material moved, "+
				"re-baseline §6.155 with the old → new pair rather than letting the "+
				"three rows drift apart", name, got, want[name])
		}
		// 155-1: the two lit rows never merge. This is 140-5's failure mode
		// measured on E2 instead of E1: a border row painted with the crown's
		// own ink turns the pair into one two-row white band (§1 禁令 3).
		if lift := got.crown - got.border; lift < 16 {
			t.Errorf("%s: the crown sits %+d levels over the border row (%d vs %d), want ≥ 16 — "+
				"the pane's own edge and its lit row must stay two readings, not one band",
				name, lift, got.crown, got.border)
		}
		// 155-2: the crown lights the face in both appearances — the same sign
		// E1's `LensRim` has. On paper the headroom is gone: the face is 251 and
		// the lit row lands on 255, i.e. the ink is clipped, which is why the
		// floor here is 3 rather than E1's 8.
		if lift := got.crown - got.face; lift < 3 {
			t.Errorf("%s: the crown sits %+d levels over the face (%d vs %d), want ≥ +3 — "+
				"the lit edge is the mark of a face that can be touched (§3), in both appearances",
				name, lift, got.crown, got.face)
		}
		if !dark {
			// Paper: the shadow does the separating, and it must beat the line —
			// if the line ever overtakes it, the pane's depth is being faked by
			// a stroke again.
			byShadow := got.ground - got.shadow
			byLine := got.border - got.shadow
			if byShadow < 10 {
				t.Errorf("light: the cast shadow takes only %d levels off the page (%d → %d), want ≥ 10 — "+
					"on paper the fill is 3 levels above the page, so the shadow is the whole of the pane's depth",
					byShadow, got.ground, got.shadow)
			}
			if byShadow <= byLine {
				t.Errorf("light: the border row separates %d levels against the shadow's %d — "+
					"on paper the hairline is the weaker reading and must stay so", byLine, byShadow)
			}
			// The fill step: the floating pane is not separated from the page by
			// its own colour on paper. Stated as a range, because that is the
			// fact this round is about (it is 3 levels, not 30).
			if step := got.face - got.ground; step < 0 || step > 6 {
				t.Errorf("light: the float face sits %+d levels against the page (%d vs %d), want 0…+6 — "+
					"paper expresses elevation through the cast shadow, not through a brighter fill",
					step, got.face, got.ground)
			}
			t.Logf("light: shadow %d levels, border %+d over the shadow, crown %+d over the face (clipped at %d), face %+d over the page",
				byShadow, byLine, got.crown-got.face, got.crown, got.face-got.ground)
			continue
		}
		// Night: the ground is already near black, so the cast shadow cannot
		// separate anything (it manages 4 levels) and the border row has to.
		byShadow := got.ground - got.shadow
		if byShadow > 8 {
			t.Errorf("dark: the cast shadow takes %d levels off the page (%d → %d), more than the 8 "+
				"this appearance has to spend — the night ground is nearly black and a deeper shadow "+
				"would be a black band, not depth", byShadow, got.ground, got.shadow)
		}
		if byLine := got.border - got.shadow; byLine < 16 {
			t.Errorf("dark: the border row separates only %d levels (border %d / shadow %d), want ≥ 16 — "+
				"at night the shadow cannot carry the boundary, so the line must", byLine, got.border, got.shadow)
		}
		// The night face is *darker* than the page: smoked glass. That is the
		// other half of "elevation is not a brighter fill".
		if got.face > got.ground+2 {
			t.Errorf("dark: the float face reads %d against the page's %d — at night a floating pane is "+
				"smoked glass, darker than the sheet it covers", got.face, got.ground)
		}
		t.Logf("dark: shadow %d levels, border %+d over the shadow, crown %+d over the face, face %+d against the page",
			byShadow, got.border-got.shadow, got.crown-got.face, got.face-got.ground)
	}
}

// TestDumpP13GjFloatEdge writes the pane the round measured. Opt-in, as every
// dump in this package is:
//
//	FLOTER_P13GJ_DUMP=1 go test -count=1 -run TestDumpP13GjFloatEdge ./internal/shell/
func TestDumpP13GjFloatEdge(t *testing.T) {
	if os.Getenv("FLOTER_P13GJ_DUMP") == "" {
		t.Skip("opt-in screenshot dump: set FLOTER_P13GJ_DUMP=1 to write /tmp/floter-P13-gj-floatedge-*.png")
	}
	for _, dark := range []bool{false, true} {
		tt := toastTester(t, dark)
		tt.SetScale(3)
		tt.Frame()
		name := "/tmp/floter-P13-gj-floatedge-light.png"
		if dark {
			name = "/tmp/floter-P13-gj-floatedge-dark.png"
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
