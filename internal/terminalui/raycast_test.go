package terminalui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// The P8-c pins: docs/raycast-design.md §6.2's own numbers, checked against
// what the terminal surface actually paints.
//
// Every pin here is a *positive* statement of the contract's墨与面 on the live
// view (not a helper's return value), because the two rounds before this one
// each lost a mark to a helper that computed the right thing and a view that
// painted another.

// raycastFrame renders the live page in the appearance the store asks for,
// with the resolved tokens installed on the context first: a pixel read then
// compares against the same numbers the view painted with.
func raycastFrame(t *testing.T, a *App, dark bool, w, h int) (*ui.Tester, theme.Tokens) {
	t.Helper()
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = tokensOf(a, c)
		c.SetTheme(tokens.Theme)
		a.View(c)
	}, w, h)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()
	return tt, tokens
}

// pixel reads one pixel of the last frame.
func pixel(tt *ui.Tester, x, y int) ui.Color {
	img := tt.Image()
	if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		return ui.Color{}
	}
	i := y*img.Stride + x*4
	return ui.Color{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
}

// delta is the largest per-channel difference between two colours.
func delta(a, b ui.Color) int {
	d := absInt(int(a.R) - int(b.R))
	if x := absInt(int(a.G) - int(b.G)); x > d {
		d = x
	}
	if x := absInt(int(a.B) - int(b.B)); x > d {
		d = x
	}
	return d
}

// ① §6.2 第 5 条 · The live status dot carries **no halo**: the 1.75u disc is
// the whole mark, and the pixels just around it are the bar's own backdrop.
//
// The old mark was base.css's `--elev-halo` — `0 0 0 2px var(--accent-halo)`,
// the accent at 0.2 — a ring three columns wider than the dot on each side.
// §1 caps gloss at 0.04 and §5 gives the accent no decorative role, so the
// halo is gone and the dot alone says "this session is live".
//
// Mutation M1 (put `dot.Shadow(..., AccentHalo)` back) turns this red.
//
// P8-e 重定基线（S）：这条 pin 的**结构**换了——它原先靠「bar 里唯一的暖墨就是
// 那个点」来定位（`R−B ≥ 40` 的像素盒），点转中性后这个前提不存在了。新的定位
// 是「不是笔杆底色的像素盒」：点仍在每行高度内，它的墨色不参与定位。断言量的事
// （盒子是 1.75u 的圆盘、盒外一圈就是笔杆底色）一字未改。
// 旧结构 → 新结构：accent-hued 像素盒 → 与 backdrop 有差的像素盒。
func TestTheStatusDotCarriesNoHalo(t *testing.T) {
	live := New(newStore(), Actions{}, nil)
	withSession(live, &terminal.Terminal{})
	tt, _ := raycastFrame(t, live, true, 300, 80)
	// The bar's identity row is revealed only while the pointer is on it or
	// something in it has the keyboard.
	tt.Move(150, 14)
	tt.Frame()

	// The dot's box is the extent of the ink standing off the bar's own row,
	// read inside the bar's leading inset and clear of the title: the ink is a
	// neutral now, so the probe cannot be a hue — it is the ink's own contrast
	// against the column to its right, which is bar and nothing else.
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	for y := 0; y < 24; y++ {
		bar := pixel(tt, 20, y)
		for x := 0; x < 20; x++ {
			if delta(pixel(tt, x, y), bar) >= 40 {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if maxX < 0 {
		t.Fatalf("the live bar drew no status dot: %q", tt.Texts())
	}
	// The box is the bare 1.75u disc: 7 columns, as the token paints it. A 2u
	// `AccentHalo` ring — the accent at 0.2 — spread it to 11.
	if w, h := maxX-minX+1, maxY-minY+1; w > 8 || h > 8 {
		t.Errorf("the live dot's box is %dx%d; the 1.75u disc is 7x7 and a 2u halo made it 11x11", w, h)
	}
	// And the pixels a ring would paint — two rows above and below the disc, at
	// its own centre column, and two columns either side — are the bar's own
	// backdrop again: the same colour the bar paints where nothing is drawn.
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	backdrop := pixel(tt, 280, cy)
	if delta(backdrop, ui.Color{}) == 0 {
		t.Fatalf("the bar's backdrop read as nothing at (280,%d)", cy)
	}
	for _, at := range [][2]int{{cx, minY - 2}, {cx, maxY + 1}, {minX - 2, cy}, {maxX + 1, cy}} {
		if at[0] < 0 || at[1] < 0 {
			continue
		}
		p := pixel(tt, at[0], at[1])
		if d := delta(p, backdrop); d > 3 {
			t.Errorf("the pixel %v just outside the live dot is %v, not the bar's backdrop %v: the 2u halo is back",
				at, p, backdrop)
		}
	}
}

// ② §6.2 第 7 条 · The drawer's live readout is **neutral ink**, not the accent.
//
// The readout answers "what is this slider's value", not "which one is
// chosen", so §5 does not let it wear the accent. The register is untouched:
// body step, monospace, `tnum`, weight 600 — its strongest pixel is a grey,
// i.e. no channel stands off the others the way the accent's red does.
//
// Mutation M2 (paint it `tokens.Theme.Accent` again) turns this red.
func TestTheDrawerReadoutWearsNeutralInk(t *testing.T) {
	store := newStore()
	if err := store.Update(func(s *settings.Settings) { s.Theme = "dark" }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	var tokens theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tokens = tokensOf(a, c)
		c.SetTheme(tokens.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.drawer(c, i18n.For("en").Terminal)
	}, 900, 400)
	tt.SetDark(true)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Frame()

	rect := mustBox(t, tt, "14 px")
	label := mustBox(t, tt, i18n.For("en").Settings.FontSize)
	brightest, hue, best := ui.Color{}, 0, -1
	for y := int(rect.Y); y < int(rect.Y+rect.H)+2; y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			p := pixel(tt, x, y)
			if s := int(p.R) + int(p.G) + int(p.B); s > best {
				best = s
				brightest = p
				hue = delta(p, ui.Color{R: p.G, G: p.G, B: p.G, A: p.A})
			}
		}
	}
	if best < 0 {
		t.Fatal("the readout drew no ink in its own box")
	}
	// A grey: no channel stands off the others the way the accent's red does
	// (the accent's own R−B is ~150, so even a lightly covered glyph shows
	// tens of levels).
	if hue > 12 {
		t.Errorf("the readout's brightest pixel %v is %d levels off grey: it carries a hue (the accent is back)",
			brightest, hue)
	}
	// And it is the strong neutral: brighter than the label beside it, so the
	// value still reads as a value (the register — body step, mono, tnum,
	// weight 600 — is untouched).
	labelBest := -1
	for y := int(label.Y); y < int(label.Y+label.H)+2; y++ {
		for x := int(label.X); x < int(label.X+label.W); x++ {
			if s := int(pixel(tt, x, y).R) + int(pixel(tt, x, y).G) + int(pixel(tt, x, y).B); s > labelBest {
				labelBest = s
			}
		}
	}
	if best <= labelBest {
		t.Errorf("the readout's ink (%d) is not brighter than its label's (%d): the value and its name read at one level",
			best, labelBest)
	}
}

// ③ §6.2 第 4 条 · The panel's feedback notice carries a **neutral** ring, not
// an accent one.
//
// The mark was `Border(1, tokens.AccentEdge)`. The notice says "an action the
// panel owns could not run" — that is not a selection and not a focus ring, so
// §5 allows it no accent; its warning ink and its alert glyph already say what
// it is. The ring is `ControlEdge`, and the accent's own red-vs-blue gap
// (~150) is nowhere on it.
func TestTheFeedbackNoticeRingIsNotAccent(t *testing.T) {
	a := New(newStore(), Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	const message = "could not continue in the system terminal"
	a.ShowFeedback(message)
	tt, tokens := raycastFrame(t, a, true, 500, 200)
	box := mustBox(t, tt, message)

	// Walk left from the notice's own text to its painted edge: the first
	// column that is neither the panel under it nor the floater's face is the
	// ring. The notice is 20u from the panel's left edge and 7u/10u padded, so
	// the walk is short.
	midY := int(box.Y + box.H/2)
	panel := pixel(tt, 2, midY)
	ringX := -1
	for x := 2; x < int(box.X); x++ {
		p := pixel(tt, x, midY)
		if delta(p, panel) > 6 {
			ringX = x
			break
		}
	}
	if ringX < 0 {
		t.Fatalf("the notice drew no left edge between the panel and its text at x<%v", box.X)
	}
	ring := pixel(tt, ringX, midY)
	face := pixel(tt, ringX+3, midY)
	if delta(ring, face) < 4 {
		t.Fatalf("the notice's left edge %v is its own face %v: the ring is not drawn", ring, face)
	}
	// The ring is the neutral control edge over the floater's fill, so its
	// red-minus-blue gap is the floater's own and nothing more.
	if warm := int(ring.B) - int(ring.R); warm > 16 {
		t.Errorf("the notice's ring %v is %d levels warm (R−B): the accent keyline is back", ring, warm)
	}
	// The accent is the counter-example the pin measures against: its own R−B
	// is ~150, so a 16-level threshold cannot pass it.
	if warm := int(tokens.Theme.Accent.B) - int(tokens.Theme.Accent.R); warm < 100 {
		t.Fatalf("the accent is not the red this pin measures against (%d)", warm)
	}
}

// ④ §6.2 第 1/2 条 · The panel is the window's corner (12) and the session
// mount inside it is a *block* (10): two different rungs on the same frame.
//
// base.css drew the mount at `--window-radius` too, so a block inside the panel
// read as a second window; §2 gives radius 12 only to the faces that *are*
// windows. The pin reads two facts, because a radius is not a token a frame
// reports: the two helpers the view actually calls, and the arc each rung draws
// — the panel's corner must trace 12's arc and the mount's 10's.
func TestThePanelAndTheMountTakeDifferentCorners(t *testing.T) {
	empty := New(newStore(), Actions{}, nil)
	_, tokens := raycastFrame(t, empty, true, 200, 120)
	if tokens.WindowRadius != 12 || tokens.RadiusMD != 10 {
		t.Fatalf("the ladder is WindowRadius=%v RadiusMD=%v, want 12 and 10", tokens.WindowRadius, tokens.RadiusMD)
	}
	if panelCorner(tokens) != tokens.WindowRadius {
		t.Errorf("the panel's corner = %v, want the window's %v (§2: the panel *is* the window)",
			panelCorner(tokens), tokens.WindowRadius)
	}
	if mountCorner(tokens) != tokens.RadiusMD {
		t.Errorf("the mount's corner = %v, want RadiusMD %v (§2: a block inside a panel)",
			mountCorner(tokens), tokens.RadiusMD)
	}
	if panelCorner(tokens) == mountCorner(tokens) {
		t.Errorf("the panel and the mount share one corner (%v): the panel-in-a-panel reading is back",
			panelCorner(tokens))
	}

	// The arc each rung draws, read off the same box at both radii. A radius
	// is only real if the frame shows it, and the two arcs differ in their
	// first row and in how fast they reach the box's edge.
	arc := func(r float32, top ui.Color) []int {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(top)
			ui.Box(c).Fill().Radius(r).Background(tokens.Theme.Background.Alpha(1))
		}, 200, 120)
		tt.SetDark(true)
		tt.Frame()
		return arcProfile(tt, 10)
	}
	at10, at12 := arc(tokens.RadiusMD, ui.RGBA(255, 255, 255, 1)), arc(tokens.WindowRadius, ui.RGBA(255, 255, 255, 1))
	if sameProfile(at10, at12) {
		t.Fatalf("the two rungs draw the same arc (%v): the probe cannot tell 10 from 12", at10)
	}
	if at10[0] >= at12[0] {
		t.Errorf("radius 10's first row starts %d px in and radius 12's %d: the wider radius must bite deeper",
			at10[0], at12[0])
	}
}

// arcProfile is, for each of the box's first rows, the first column whose pixel
// leaves the page: the corner's arc traced row by row.
func arcProfile(tt *ui.Tester, rows int) []int {
	img := tt.Image()
	out := make([]int, rows)
	for y := 0; y < rows && y < img.Rect.Dy(); y++ {
		out[y] = -1
		for x := 0; x < 40 && x < img.Rect.Dx(); x++ {
			i := y*img.Stride + x*4
			if int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2]) < 3*250 {
				out[y] = x
				break
			}
		}
	}
	return out
}

// sameProfile reports whether two corner arcs trace the same curve.
func sameProfile(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ⑤ §6.2 第 6 条 · The bar's hover shadow is read from the tokens, not from the
// literal `rgba(0, 0, 0, 0.18)` the port carried: the same throw is 0.40 in the
// dark appearance and 0.10 in the light one, and a literal pins a *third* rung
// that is heavier than the contract in one appearance and lighter in the other.
//
// P13-d 重定基线（V，`docs/liquid-glass.md` §6.3 第 3-4 行）：旧值 → 新值 =
// 「宽影 `0 6px 16px` 配紧贴边墨 `ShadowContact`」→「宽影配 `ShadowAmbient`
// **加**一条 `0 1px 2px` 的 `ShadowContact` 接触影」（§2 判据 3：E2 必须两层影）。
// 那条单层的 `ShadowContact` 是**旧值**，所以它从「被钉住的那一条」变成
// 「必须偏离的那一条」——pin 的结构（参照帧逐像素相减）与那条
// 「不是字面值」的断言一字未改。
//
// The pin renders the live bar twice (pointer on it, pointer clear), then the
// same bar with each candidate stack, and holds the frames against each other
// pixel for pixel.
func TestTheTitleBarHoverShadowIsShadowContact(t *testing.T) {
	canvas := ui.RGBA(184, 184, 184, 1)
	a := New(newStore(), Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, barTitle)
	var tokens theme.Tokens
	bar := func(hover bool) *ui.Tester {
		tt := ui.NewTester(func(c *ui.Context) {
			tokens = tokensOf(a, c)
			c.SetTheme(tokens.Theme)
			c.Root().Background(canvas)
			a.titleBar(c, i18n.For("en").Terminal)
		}, 900, 120)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		if hover {
			tt.Move(450, 14)
		} else {
			tt.Move(450, 96)
		}
		tt.Frame()
		return tt
	}
	hovered := bar(true)
	idle := bar(false)

	// The shadow only exists while the pointer is on the bar.
	floor := int(tokens.Theme.Space(7))
	if delta(pixel(hovered, 450, floor), pixel(idle, 450, floor)) < 6 {
		t.Fatalf("the bar cast nothing under the pointer: idle and hovered agree at the row below it")
	}

	// A candidate is a **stack** of throws, painted in the order the view paints
	// them, so the pin can hold the live bar against the token pair (E2) and
	// against the two stacks it must not be: the single-layer token throw the
	// previous generation carried, and the port's literal.
	probe := func(throws ...ui.Color) *ui.Tester {
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(tokens.Theme)
			c.Root().Background(canvas)
			strip := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(tokens.Theme.Space(7))
			strip.Shadow(0, tokens.Theme.Space(1.5), tokens.Theme.Space(4), 0, throws[0])
			if len(throws) > 1 {
				strip.Shadow(0, tokens.Theme.Space(0.25), tokens.Theme.Space(0.5), 0, throws[1])
			}
		}, 900, 120)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		return tt
	}
	pair := probe(tokens.ShadowAmbient, tokens.ShadowContact)
	oldContact := probe(tokens.ShadowContact)
	literal := probe(ui.RGBA(0, 0, 0, 0.18))
	under := func(x0, y0, x1, y1 int, other *ui.Tester) int {
		ia, ib := hovered.Image(), other.Image()
		n := 0
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				i := y*ia.Stride + x*4
				if absInt(int(ia.Pix[i])-int(ib.Pix[i])) > 1 ||
					absInt(int(ia.Pix[i+1])-int(ib.Pix[i+1])) > 1 ||
					absInt(int(ia.Pix[i+2])-int(ib.Pix[i+2])) > 1 {
					n++
				}
			}
		}
		return n
	}
	toBePair := under(100, floor, 800, floor+30, pair)
	toBeOld := under(100, floor, 800, floor+30, oldContact)
	toBeLiteral := under(100, floor, 800, floor+30, literal)
	if toBePair != 0 {
		t.Errorf("%d pixels under the bar differ from the E2 pair (ShadowAmbient + ShadowContact); "+
			"liquid-glass.md §6.3 第 3-4 行 / §2 判据 3 names the two layers", toBePair)
	}
	if toBeOld == 0 {
		t.Errorf("the bar's hover shadow is byte-identical to the single-layer ShadowContact throw " +
			"the previous generation painted: the contact layer is missing (§2 判据 3)")
	}
	if toBeLiteral == 0 {
		t.Errorf("the bar's hover shadow is byte-identical to a literal rgba(0, 0, 0, 0.18): " +
			"the token that follows the appearance is not being read (both frames would differ)")
	}
}

// mustBox finds one text's box, failing the test when the view did not draw it.
func mustBox(t *testing.T, tt *ui.Tester, text string) ui.Rect {
	t.Helper()
	box, ok := tt.Find(text)
	if !ok {
		t.Fatalf("the frame does not draw %q: %v", text, tt.Texts())
	}
	return box
}

// The task book's guard: the terminal views paint from tokens, never from a
// literal colour. `internal/theme` is the one place a value is written down; a
// `#RRGGBB` or an `rgba(` in a terminal view is a token that was not hoisted.
//
// This is the third such guard — `internal/settingsui` and `internal/launcher`
// each carry one — and it is what keeps §6.2's rerungs from re-introducing the
// `rgba(0, 0, 0, 0.18)` the title bar's shadow used to spell.
func TestTheTerminalViewsPaintFromTokensNotLiterals(t *testing.T) {
	hex := regexp.MustCompile(`#[0-9A-Fa-f]{6}\b`)
	rgba := regexp.MustCompile(`rgba\s*\(`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("could not read the package: %v", err)
	}
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("could not read %s: %v", name, err)
		}
		checked++
		for i, line := range strings.Split(string(data), "\n") {
			// A comment may quote the reference's own CSS — that is
			// documentation of where a token came from, not a painted value.
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if loc := hex.FindString(line); loc != "" {
				t.Errorf("%s:%d paints a hex literal (%s); read a token from internal/theme instead",
					name, i+1, loc)
			}
			if loc := rgba.FindString(line); loc != "" {
				t.Errorf("%s:%d paints an rgba() literal; read a token from internal/theme instead",
					name, i+1)
			}
		}
	}
	if checked == 0 {
		t.Fatal("the guard read no source files")
	}
}
