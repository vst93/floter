package terminalui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-am · 终端浮条上的五个 28u 键：指针按下去的那一帧此前与悬停帧**逐像素相同**。
//
// P13-al 把「按下」这一档补到了 `internal/shell` 的四个键上（插件窗口 bar / 便条 ✕ /
// toast ✕ 与动作胶囊），并在 `docs/liquid-glass.md` §6.7b 第 7-9 行把这一页记成
// 「故意不做（本轮）」+ 下一轮候选，理由是：这几个键的墨来自**画布**
// （`canvasInks` / `barInks`），板子里没有「按下的洗色」，直接借 `ControlPress`
// 会在命名调色板的深色画布上压出一块窗口灰。本轮补的就是 `barInks` 的第六档
// （`Press`），所以这一页可以按同一条 §5 的规矩收尾。
//
// 三条判据（每套外观 × 两套墨源各跑一遍，全部量像素）：
//
//	① 悬停真的画了面（悬停面 ≠ 浮条自己的面）；
//	② 按下真的画了**另一个**面（按下面 ≠ 悬停面）——旧实现在这一条上逐像素相同，
//	   这是本轮的根因；
//	③ 按下的面在**浮条面与悬停面之间**（第三档是「往静息的回退」，不是第三种颜色）；
//	④ 按下只改自己的那一块：换了面、没有搬盒子（悬停/按下两帧的图上差异全部落在
//	   这枚键的盒子内，旁边的键与浮条的标题一个像素没动）。
//
// 为什么 ④ 不是「盒子收一格」：族里那一步是 `Scale(0.92)`（P13-q / P13-al），
// 本轮实测它在本页会把一整个点击丢掉——浮条上的键在抽屉滑入的那两帧里只有 2 DIP
// 宽（实测盒子 `{858 314 2 28}`），绕中心缩 0.92 后左边缘往里让 0.08 DIP，正好把
// 落在中心线上的点击点挤出命中框，释放时 `contains` 为假，`drawer_test.go` 的两条
// B pin 变红。命中的账属于框架，不属于一次只改墨与面的视觉轮，所以这一页只收面，
// 几何写成 `liquid-glass.md` §6.7b 第 7-10 行的「需拍板」。
//
// 墨源分两套，因为 `barInks` 有两条来路：`inherit`（窗口自己的控制阶梯，
// `Wash = SurfaceHover` / `Press = SurfacePressed`）与命名调色板（调色板前景的
// 两层洗色，`0.10` / `0.035`）。只测一套会让另一套静默掉队。
//
// 出图：`/tmp/floter-P13-am-<case>-<hover|press>.png`（opt-in，`FLOTER_PNG=1`）。

// pressCase is one bar: an appearance and the ink source its controls draw with.
type pressCase struct {
	name string
	id   string // stored palette id; "" or "inherit" = the window's own rungs
	dark bool
}

func pressCases() []pressCase {
	return []pressCase{
		{name: "inherit-light", id: "", dark: false},
		{name: "inherit-dark", id: "", dark: true},
		// A dark palette inside a light window: the case that forbade borrowing
		// `ControlPress` — a window-grey press face would show up here as a
		// plate *lighter* than the canvas it sits on.
		{name: "ink-lightwindow", id: "ink", dark: false},
		// And a light palette inside a dark window, the same question the other
		// way round.
		{name: "paper-darkwindow", id: "paper", dark: true},
	}
}

// pressKeys are the bar's five controls, in bar order.
func pressKeys(copy i18n.Terminal) []string {
	return []string{copy.SettingsOpen, copy.Pin, copy.NewCommand, copy.OpenExternal, copy.Close}
}

// pressFrame renders the terminal bar over the face the canvas would paint.
func pressFrame(t *testing.T, k pressCase) *ui.Tester {
	t.Helper()
	store := newStore()
	if err := store.Update(func(s *settings.Settings) {
		s.TerminalTheme = k.id
		appearance := "light"
		if k.dark {
			appearance = "dark"
		}
		s.Theme = appearance
	}); err != nil {
		t.Fatal(err)
	}
	// Every action wired: a nil one removes its key from the bar, and this pin
	// is about all five.
	a := New(store, Actions{
		Settings:     func() {},
		Pin:          func(string, string) {},
		NewCommand:   func() {},
		OpenExternal: func() {},
		Close:        func() {},
	}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, barTitle)
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	tt := ui.NewTester(func(c *ui.Context) {
		tokens := tokensOf(a, c)
		c.SetTheme(tokens.Theme)
		c.Root().Background(pressCanvas(tokens, k.id))
		a.titleBar(c, copy)
	}, 900, 60)
	tt.SetDark(k.dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	return tt
}

// pressCanvas is the face under the bar: the palette's background when the canvas
// belongs to one, the window's own page otherwise.
func pressCanvas(tokens theme.Tokens, id string) ui.Color {
	if spec, ok := palettes[id]; ok {
		return packed(spec.bg)
	}
	if tokens.Theme != nil {
		return tokens.Theme.Background
	}
	return ui.RGB(18, 19, 22)
}

// pressRead parks the pointer and reads the pixel the material paints at the key's
// own padding column (clear of the 16u glyph).
//
// state: "rest" parks the pointer on the bar but on no button (the bar's own face,
// the ground both faces are graded against), "hover" rests it on the key,
// "press" holds the button down.
func pressRead(t *testing.T, k pressCase, label, state string) (ui.Color, ui.Rect) {
	t.Helper()
	tt := pressFrame(t, k)
	box, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%s: the bar has no %q control: %v", k.name, label, tt.Texts())
	}
	x, y := int(box.X)+pressProbe, int(box.Y+box.H/2)
	switch state {
	case "rest":
		// Mid-bar, over neither the identity line (left) nor the controls
		// (right): the pointer reveals the bar, which is the only state the
		// controls are drawn in.
		tt.Move(400, 14)
		tt.Frame()
	case "hover":
		tt.Move(float32(x), float32(y))
		tt.Frame()
	case "press":
		tt.Move(float32(x), float32(y))
		tt.Frame()
		tt.Press(float32(x), float32(y))
		// No colour transition is set on these keys, so the frame after the
		// press is the settled face; the second frame only proves it is a
		// state, not a one-frame artefact of the dispatch.
		tt.Frame()
		tt.Frame()
	default:
		t.Fatalf("unknown state %q", state)
	}
	return pressPixelAt(tt.Image(), x, y), box
}

// pressProbe is how far inside the key's left edge the material is read: the key
// is 28u with a 16u mark centred, so 5u is the face, never the glyph.
const pressProbe = 5

func pressPixelAt(img image.Image, x, y int) ui.Color {
	r, g, b, _ := img.At(x, y).RGBA()
	return ui.RGBA(uint8(r>>8), uint8(g>>8), uint8(b>>8), 255)
}

// pressDelta is how far apart two colours are, in levels of their widest channel.
func pressDelta(a, b ui.Color) int {
	d := func(x, y uint8) int {
		if x > y {
			return int(x) - int(y)
		}
		return int(y) - int(x)
	}
	dr, dg, db := d(a.R, b.R), d(a.G, b.G), d(a.B, b.B)
	if dg > dr {
		dr = dg
	}
	if db > dr {
		dr = db
	}
	return dr
}

// pressLuma is the perceptual weight of a colour, used only to ask "which side of
// which" — never to assert an exact value the renderer composites.
func pressLuma(c ui.Color) int { return (int(c.R)*299 + int(c.G)*587 + int(c.B)*114) / 1000 }

// pressBetween reports whether mid's luma sits strictly between a and b.
func pressBetween(a, mid, b ui.Color) bool {
	lo, hi := pressLuma(a), pressLuma(b)
	if lo > hi {
		lo, hi = hi, lo
	}
	return pressLuma(mid) > lo && pressLuma(mid) < hi
}

// TestTheTerminalKeysSinkWhenThePointerGoesDown is ② and ③: the third rung exists
// and it is the ladder stepping back down, not a third colour.
func TestTheTerminalKeysSinkWhenThePointerGoesDown(t *testing.T) {
	copy := i18n.For("en").Terminal
	for _, k := range pressCases() {
		for _, label := range pressKeys(copy) {
			rest, _ := pressRead(t, k, label, "rest")
			hover, _ := pressRead(t, k, label, "hover")
			pressed, _ := pressRead(t, k, label, "press")
			t.Logf("%s/%s: bar %v hover %v pressed %v", k.name, label, rest, hover, pressed)
			if pressDelta(hover, rest) < 4 {
				t.Errorf("%s/%s: the hovered face %v is the bar's own face %v — the hover arm painted "+
					"nothing, so this pin would be grading the bar, not the key", k.name, label, hover, rest)
				continue
			}
			if pressDelta(pressed, hover) < 3 {
				t.Errorf("%s/%s: the key under the pointer is %v whether or not the button is down (bar %v): "+
					"the press rung paints no face of its own — one step back toward the bar's own face is "+
					"what `--glass-control-press` means (docs/liquid-glass.md §5)", k.name, label, pressed, rest)
				continue
			}
			if !pressBetween(rest, pressed, hover) {
				t.Errorf("%s/%s: the pressed face %v is not between the bar's own face %v and the hovered %v — "+
					"the third rung is a step back toward rest, not a third colour",
					k.name, label, pressed, rest, hover)
			}
		}
	}
}

// TestThePressRungStaysInsideItsKey is ④: the press paints its own plate and
// nothing else — the layout box the pointer has to hit stays where it was, no
// neighbour and no other part of the bar moves, and the plate really did change.
// (That box step the family takes elsewhere is exactly what this page cannot do:
// see the note at the top of the file.)
func TestThePressRungStaysInsideItsKey(t *testing.T) {
	copy := i18n.For("en").Terminal
	for _, k := range pressCases() {
		for _, label := range pressKeys(copy) {
			_, hoverBox := pressRead(t, k, label, "hover")
			_, pressBox := pressRead(t, k, label, "press")
			if hoverBox != pressBox {
				t.Errorf("%s/%s: the key's layout box is %v hovered and %v pressed — the press rung must be a "+
					"material change, not a moved box", k.name, label, hoverBox, pressBox)
			}
			changed, outside := pressChangedPixels(t, k, label)
			if changed < 40 {
				t.Errorf("%s/%s: only %d pixels differ between the hovered and the held key — that is not a plate",
					k.name, label, changed)
			}
			if outside != 0 {
				t.Errorf("%s/%s: %d changed pixels fall outside the key's own box %v — the press rung is drawing "+
					"on a neighbour, or the block moved under it", k.name, label, outside, pressBox)
			}
		}
	}
}

// pressChangedPixels renders one bar hovered and held on the same key and counts
// the pixels that differ, split into those inside the key's box (grown by one DIP
// for the antialiased edge) and those outside it.
func pressChangedPixels(t *testing.T, k pressCase, label string) (inside, outside int) {
	t.Helper()
	hoverFrame := pressFrame(t, k)
	box, ok := hoverFrame.Find(label)
	if !ok {
		t.Fatalf("%s: the bar has no %q control", k.name, label)
	}
	x, y := int(box.X)+pressProbe, int(box.Y+box.H/2)
	hoverFrame.Move(float32(x), float32(y))
	hoverFrame.Frame()
	held := pressFrame(t, k)
	held.Move(float32(x), float32(y))
	held.Frame()
	held.Press(float32(x), float32(y))
	held.Frame()

	a, b := hoverFrame.Image(), held.Image()
	bounds := a.Bounds()
	left, top := int(box.X)-1, int(box.Y)-1
	right, bottom := int(box.X+box.W)+1, int(box.Y+box.H)+1
	for py := bounds.Min.Y; py < bounds.Max.Y; py++ {
		for px := bounds.Min.X; px < bounds.Max.X; px++ {
			if pressPixelAt(a, px, py) == pressPixelAt(b, px, py) {
				continue
			}
			if px >= left && px <= right && py >= top && py <= bottom {
				inside++
			} else {
				outside++
			}
		}
	}
	return inside, outside
}

// pressPaintedWidth counts the pixels along the key's middle row that are not the
// bar's own face: the key's painted box, read off the frame.
func pressPaintedWidth(t *testing.T, k pressCase, label, state string) int {
	t.Helper()
	tt := pressFrame(t, k)
	box, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%s: the bar has no %q control: %v", k.name, label, tt.Texts())
	}
	x, y := int(box.X)+pressProbe, int(box.Y+box.H/2)
	// The bar's own face, taken well left of the key (the controls sit in a
	// trailing group, so this is the bar's empty run).
	barFace := pressPixelAt(tt.Image(), int(box.X)-8, y)
	switch state {
	case "hover":
		tt.Move(float32(x), float32(y))
		tt.Frame()
	case "press":
		tt.Move(float32(x), float32(y))
		tt.Frame()
		tt.Press(float32(x), float32(y))
		tt.Frame()
	default:
		t.Fatalf("unknown state %q", state)
	}
	img := tt.Image()
	width := 0
	for px := int(box.X) - 8; px <= int(box.X+box.W)+8; px++ {
		if pressDelta(pressPixelAt(img, px, y), barFace) > 5 {
			width++
		}
	}
	return width
}

// TestDumpP13AMPressRungs is the opt-in frame dump: one bar per case, the pointer
// resting on the last key (close) and then holding it, so the plate, its lit crown
// and its counter-side line are all in the picture.
func TestDumpP13AMPressRungs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") != "1" {
		t.Skip("opt-in: FLOTER_PNG=1 writes /tmp/floter-P13-am-<case>-<hover|press>.png")
	}
	copy := i18n.For("en").Terminal
	keys := pressKeys(copy)
	for _, k := range pressCases() {
		for _, state := range []string{"hover", "press"} {
			tt := pressFrame(t, k)
			box, ok := tt.Find(keys[len(keys)-1])
			if !ok {
				t.Fatalf("%s: no close control", k.name)
			}
			x, y := int(box.X+box.W/2), int(box.Y+box.H/2)
			tt.Move(float32(x), float32(y))
			tt.Frame()
			if state == "press" {
				tt.Press(float32(x), float32(y))
				tt.Frame()
				tt.Frame()
			}
			path := filepath.Join("/tmp", "floter-P13-am-"+k.name+"-"+state+".png")
			pressWritePNG(t, tt.Image(), path)
		}
	}
}

func pressWritePNG(t *testing.T, img image.Image, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("could not write %s: %v", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("could not encode %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
}
