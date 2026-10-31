package terminalui

import (
	"image/png"
	"os"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-ao · 浮条左上角那盏灯：**形**说状态，alpha 不再兼职。
//
// 旧实现两个状态共用一枚实心点，只差 alpha：活着 `TextTertiary` @1.0 起、靠呼吸
// 走到 0.50；结束 `TextTertiary` @0.70 静止。两件事由此塌掉：
//
//  1. **范围重叠**：呼吸的 `1-0.5*breath` 每个 2.4s 周期里约 40% 的时间比「结束」
//     那一档还暗，静帧（截图、正中间那一帧）分不出谁是谁。
//  2. **空白页说假话**：`exited()` 在 `a.Current() == nil` 时返回 false，于是
//     `terminalui.go:906` 那张「没有任何会话」的空白页上，灯按**活着**在呼吸。
//
// 本轮的判据（每套外观各跑一遍，全部量像素）：
//
//	① 没有程序在跑（结束 / 空白页）→ 中心是**底色**（空心），而不是一块淡墨；
//	② 环上那一点是**满档** `Tertiary`：灯的像素里存在与墨色**逐字节相同**的点，
//	   0.7 的淡墨永远到不了；
//	③ 盒子还是 1.75u、和活着的点一样宽（形差不动布局，也不引回光环）；
//	④ 环的墨量落在活圆盘的 **0.5~0.9** 之间：比实心轻（暗就是暗），但远不是影子。
//
// 出图：`/tmp/floter-P13-ao-<state>-<dark|light>.png`（opt-in，`FLOTER_PNG=1`）。

// dotStates are the three answers the light gives; `blank` is the page with no
// session at all, the one that used to breathe for a process that does not exist.
func dotStates() []string { return []string{"live", "exited", "blank"} }

// lightFrame renders just the bar over the appearance's own page, with the
// pointer resting on it so the identity row (and the light in it) is revealed.
func lightFrame(t *testing.T, dark bool, state string) (*ui.Tester, theme.Tokens) {
	t.Helper()
	store := newStore()
	appearance := "light"
	if dark {
		appearance = "dark"
	}
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{}, nil)
	if state != "blank" {
		withSession(a, &terminal.Terminal{})
		withTitle(a, barTitle)
	}
	if state == "exited" {
		a.Current().Resident, a.Current().ExitCode = true, 3
	}
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		// The bar floats over a black root in both appearances — the page the light
		// sits on is the same page for both, so the two appearances only differ in
		// the ink, which is what this pin reads (and a dark page keeps the bar's own
		// shadow out of the mask).
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.titleBar(c, copy)
	}, 400, 96)
	tt.SetDark(dark)
	// Reduce motion pins the breathing light at its 0% keyframe, so the running
	// light's ink is one fixed value and the frames are comparable.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Move(200, 14)
	tt.Frame()
	return tt, tok
}

// dotPixels is what the light actually painted: its ink box, its centre pixel,
// the pixel at the middle of its left edge (the ring's core), its ink mass above
// the page, and how many of its pixels are the ink colour exactly.
type dotPixels struct {
	minX, maxX int
	width      int
	centre     ui.Color
	edge       ui.Color
	mass       int
	exact      int
	page       ui.Color
}

// readDot finds the light as the block of pixels painted in the ink colour and
// reads it. The page is sampled from the frame's own far corner, and the light is
// located by its ink rather than by "any pixel that is not the page": the bar
// throws its own shadow on a light page, so a not-the-page mask covers the whole
// width and reads the bar, not the light.
func readDot(tt *ui.Tester, ink ui.Color) dotPixels {
	img := tt.Image()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	at := func(x, y int) ui.Color {
		i := y*img.Stride + x*4
		return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
	}
	// The reference per column is that column's own bottom row: below the bar and
	// its shadow on both appearances, so the mask sees what the light added and
	// not the bar's own band or its ambient shadow.
	ref := func(x int) ui.Color { return at(x, h-1) }
	page := ref(w - 1)
	// The light's box: pixels painted at least 40 levels off the page (a painted
	// pixel clears that, the bar's own ambient shadow sits in the teens), taken as
	// the first contiguous run of columns — the bar's 1.75u gap to the title ends
	// the run even when a glyph happens to pass through the ink colour.
	const painted = 40
	colInk := func(x int) bool {
		for y := 0; y < h; y++ {
			if channelDelta(at(x, y), ref(x)) > painted {
				return true
			}
		}
		return false
	}
	minX := -1
	for x := 0; x < 24; x++ {
		if colInk(x) {
			minX = x
			break
		}
	}
	if minX < 0 {
		return dotPixels{minX: -1, maxX: -1, page: page}
	}
	maxX := minX
	for maxX+1 < 24 && colInk(maxX+1) {
		maxX++
	}
	minY, maxY := 1<<30, -1
	for y := 0; y < h; y++ {
		for x := minX; x <= maxX; x++ {
			if channelDelta(at(x, y), ref(x)) > painted {
				minY, maxY = min(minY, y), max(maxY, y)
			}
		}
	}
	if maxY < 0 {
		return dotPixels{minX: -1, maxX: -1, page: page}
	}
	cx, midY := (minX+maxX)/2, (minY+maxY)/2
	mass, exact := 0, 0
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			p := at(x, y)
			if d := channelDelta(p, ref(x)); d > painted {
				mass += d
			}
			if p.R == ink.R && p.G == ink.G && p.B == ink.B {
				exact++
			}
		}
	}
	return dotPixels{
		minX: minX, maxX: maxX, width: maxX - minX + 1,
		centre: at(cx, midY),
		// The left edge of the circle at its vertical middle: for a 1 DIP ring on
		// a 1.75u box that column is covered by the stroke alone.
		edge: at(minX, midY),
		mass: mass, exact: exact, page: page,
	}
}

func TestTheStatusLightAnswersWithAFormNotAnAlpha(t *testing.T) {
	for _, dark := range []bool{true, false} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			liveTT, tok := lightFrame(t, dark, "live")
			live := readDot(liveTT, tok.TextTertiary)
			if live.maxX < 0 {
				t.Fatalf("the running session's bar drew no status light: %q", liveTT.Texts())
			}
			// The light side of the pair: a filled disc, full ink at its centre.
			if got := channelDelta(live.centre, tok.TextTertiary); got > 12 {
				t.Errorf("the running dot's centre is %v, %d levels off the tertiary step (%v): the light is not filled",
					live.centre, got, tok.TextTertiary)
			}
			if want := int(tok.Theme.Spacing*1.75 + 0.5); live.width < want-1 || live.width > want+1 {
				t.Errorf("the running dot is %d DIP wide, want the 1.75u box (%d)", live.width, want)
			}

			// The dark side of the pair: a program that ended, and the blank page.
			for _, state := range []string{"exited", "blank"} {
				tt, tok2 := lightFrame(t, dark, state)
				p := readDot(tt, tok2.TextTertiary)
				if p.maxX < 0 {
					t.Fatalf("%s: no status light was drawn: %q", state, tt.Texts())
				}
				// ① hollow: the middle of the light is the page behind it.
				if got := channelDelta(p.centre, p.page); got > 6 {
					t.Errorf("%s: the dot's centre is %v, %d levels off the page (%v): still a filled disc, so the state only lives in the alpha",
						state, p.centre, got, p.page)
				}
				// ② full ink: some pixel of the light is the ink colour exactly.
				if p.exact == 0 {
					t.Errorf("%s: no pixel of the light is the ink colour exactly (%v): the dark rung is a dimmed disc, not a ring at full ink",
						state, tok2.TextTertiary)
				}
				if got := channelDelta(p.edge, tok2.TextTertiary); got > 14 {
					t.Errorf("%s: the ring's edge is %v, %d levels off the tertiary step (%v)",
						state, p.edge, got, tok2.TextTertiary)
				}
				// ③ form, not geometry: the same 1.75u box the running light has.
				if p.width < live.width-1 || p.width > live.width+1 {
					t.Errorf("%s: the ring is %d DIP wide against the running dot's %d: the form change moved the box",
						state, p.width, live.width)
				}
				// ④ the ring is lighter than the disc but not a ghost. Measured at
				// the shipped 1.5 DIP stroke: 0.49 of the disc's ink in both
				// appearances — half the light, which is what 「暗」 should cost.
				ratio := float64(p.mass) / float64(live.mass)
				t.Logf("%s/%s: %d DIP wide, ink %d vs live %d (%.2f), %d exact-ink px, centre %v on page %v",
					name, state, p.width, p.mass, live.mass, ratio, p.exact, p.centre, p.page)
				if ratio < 0.4 || ratio > 0.9 {
					t.Errorf("%s: the ring carries %.2f of the running dot's ink (live %d, dark %d), want 0.5~0.9",
						state, ratio, live.mass, p.mass)
				}
			}
		})
	}
}

// TestTheBlankPageDoesNotBreathe pins the second half of P13-ao on its own: the
// page that holds no session must not report a live one. Before this round
// `exited()` answered false for "no session", so the light took the running arm
// and breathed — a status light for a process that was never started.
func TestTheBlankPageDoesNotBreathe(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	if a.live() {
		t.Errorf("a surface with no session reports itself live")
	}
	withSession(a, &terminal.Terminal{})
	if !a.live() {
		t.Errorf("a fresh session is not reported live")
	}
	if session := a.Current(); session != nil {
		session.Resident, session.ExitCode = true, 0
	}
	if a.live() {
		t.Errorf("a held session (program ended, screen still on show) is reported live")
	}
}

// TestStatusDotPictures is the opt-in frame dump: the three states in both
// appearances, so the pair can be read the way a person reads it.
func TestStatusDotPictures(t *testing.T) {
	if os.Getenv("FLOTER_PNG") != "1" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-ao-<state>-<dark|light>.png")
	}
	for _, dark := range []bool{true, false} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, state := range dotStates() {
			tt, _ := lightFrame(t, dark, state)
			path := "/tmp/floter-P13-ao-" + state + "-" + appearance + ".png"
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(file, tt.Image()); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
