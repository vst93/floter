package terminalui

import (
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"

	"floter/internal/i18n"
)

// P11-bd · **终端页上没有动作的浮层不该是命中目标。**
//
// 参考给这三样东西都写了 `pointer-events: none`：常驻便条
// （`main:src/styles/terminal.css:303`）、失败提示（`:414`）、以及浮条里未揭示的
// 按钮组（`:501-505`，注释原话 “While hidden the buttons must not catch clicks
// meant for the terminal underneath”）。本仓三条都漏在同一个地方：`mygo` 的
// `PassThrough` **不沿子树传播**（与 CSS 的 `pointer-events: none` 不同），框架的
// 命中表逐个元素记，而一句 `Text` 自己就是可选中面 = 命中目标。于是「把外层标成
// 透明」这条修法在这三处都只修了一半：
//
//   - 常驻便条（P11-bc 标的）：带子的**字**仍然吃指针；
//   - 失败提示（从来没标）：整块 4.5 秒盖住前两行输出的左半边；
//   - 浮条里未揭示的按钮（只标了 `actions` 那一行）：**看不见的关闭键**还在，
//     静态下点右上角 `(834,14)` 把终端关了（实测 `closed=1`）。
//
// 判据一条：**浮在画布上、自己不接动作的东西，指针要穿过去落到画布上**——画布
// 是这一页唯一会响应指针的东西（聚焦、选区、滚轮回滚）。所以每条 pin 的观测都
// 是同一个：点在那个位置之后，画布有没有拿到键盘。

// canvasHoldsKeyboard reports whether a click at (x, y) reached the canvas: the
// terminal's view is the page's one focusable surface, so a pointer that lands
// on it hands it the keyboard.
func canvasHoldsKeyboard(t *testing.T, a *App, x, y float32) bool {
	t.Helper()
	tt := render(t, a, 860, 600)
	tt.ClickAt(x, y)
	tt.Frame()
	return tt.Focused(i18n.For(a.Store.Snapshot().Language).Terminal.Input)
}

// TestTheHiddenBarButtonsTakeNoClick：浮条未揭示时，右上角那几个 28u 的方格里
// 没有任何东西被画出来——点在那里必须落到画布上，而不是关掉终端 / 打开设置。
// 揭示之后它们是真按钮，点击照旧生效（这条 pin 不能靠「把按钮删掉」变绿）。
func TestTheHiddenBarButtonsTakeNoClick(t *testing.T) {
	copy := i18n.For("en").Terminal
	newApp := func(closed, settings *int) *App {
		a := New(newStore(), Actions{
			Close:    func() { *closed = *closed + 1 },
			Settings: func() { *settings = *settings + 1 },
		}, nil)
		withSession(a, &terminal.Terminal{})
		return a
	}

	closed, settings := 0, 0
	tt := render(t, newApp(&closed, &settings), 860, 600)
	closeBox, ok := tt.Find(copy.Close)
	if !ok {
		t.Fatalf("the close control is not on the frame: %q", tt.Texts())
	}
	gearBox, ok := tt.Find(copy.SettingsOpen)
	if !ok {
		t.Fatalf("the settings control is not on the frame: %q", tt.Texts())
	}
	// At rest the bar's content is transparent: the box holds no more than the
	// canvas under it.
	img := tt.Image()
	if got, canvas := img.RGBAAt(int(closeBox.X+closeBox.W/2), int(closeBox.Y+closeBox.H/2)), img.RGBAAt(400, 300); got != canvas {
		t.Fatalf("the bar drew its controls at rest: %v over a canvas of %v", got, canvas)
	}
	tt.ClickAt(closeBox.X+closeBox.W/2, closeBox.Y+closeBox.H/2)
	tt.Frame()
	if closed != 0 {
		t.Fatalf("a click on the hidden close control closed the terminal (%d times)", closed)
	}
	if !tt.Focused(copy.Input) {
		t.Errorf("a click on the hidden close control did not reach the canvas")
	}

	// The gear, from a fresh frame: the click that arrives with the pointer (the
	// frame before the reveal lands) is the one the rule is about — it belongs to
	// the canvas, not to a control nobody has seen yet.
	closed, settings = 0, 0
	ttGear := render(t, newApp(&closed, &settings), 860, 600)
	gearBox, ok = ttGear.Find(copy.SettingsOpen)
	if !ok {
		t.Fatalf("the settings control is not on the frame: %q", ttGear.Texts())
	}
	ttGear.ClickAt(gearBox.X+gearBox.W/2, gearBox.Y+gearBox.H/2)
	ttGear.Frame()
	if settings != 0 {
		t.Fatalf("a click on the hidden settings control opened the drawer (%d times)", settings)
	}

	// Revealed, they are buttons again: the reveal is what makes them live.
	closed, settings = 0, 0
	tt2 := render(t, newApp(&closed, &settings), 860, 600)
	box, ok := tt2.Find(copy.Close)
	if !ok {
		t.Fatalf("the close control is not on the frame: %q", tt2.Texts())
	}
	tt2.Move(box.X+box.W/2, box.Y+box.H/2)
	tt2.Frame()
	tt2.ClickAt(box.X+box.W/2, box.Y+box.H/2)
	tt2.Frame()
	if closed != 1 {
		t.Fatalf("a click on the revealed close control did not close the terminal (%d times)", closed)
	}
}

// TestTheNoticeTakesNoPointer：失败提示（`.terminal-feedback`）的整块矩形——它的
// 字与它的内边距——都要让指针穿到画布。它 4.5 秒里盖着前两行输出的左半边。
func TestTheNoticeTakesNoPointer(t *testing.T) {
	copy := i18n.For("en").Terminal
	newApp := func() *App {
		a := New(newStore(), Actions{Settings: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		a.ShowFeedback("no emulator")
		return a
	}
	tt := render(t, newApp(), 860, 600)
	box, ok := tt.Find("no emulator")
	if !ok {
		t.Fatalf("the notice is not on the frame: %q", tt.Texts())
	}
	tt.ClickAt(box.X+box.W/2, box.Y+box.H/2)
	tt.Frame()
	if !tt.Focused(copy.Input) {
		t.Errorf("a click on the notice's words did not reach the canvas (box %v)", box)
	}
	// Its own padding, inside the note's face but outside its words: the same.
	if !canvasHoldsKeyboard(t, newApp(), 20, 30) {
		t.Errorf("a click on the notice's padding did not reach the canvas")
	}
}

// TestTheResidentNoteTakesNoPointer：常驻便条（`.terminal-resident`）的句子也要让
// 指针穿过去——它贴在画布底边、整宽，盖住的正是输出的最后一行。
func TestTheResidentNoteTakesNoPointer(t *testing.T) {
	copy := i18n.For("en").Terminal
	a := New(newStore(), Actions{Settings: func() {}}, nil)
	withSession(a, &terminal.Terminal{})
	a.ResidentAt(0, 3)
	tt := render(t, a, 860, 600)
	box, ok := tt.Find(copy.ProcessExitedHint)
	if !ok {
		t.Fatalf("the resident note is not on the frame: %q", tt.Texts())
	}
	tt.ClickAt(box.X+box.W/2, box.Y+box.H/2)
	tt.Frame()
	if !tt.Focused(copy.Input) {
		t.Errorf("a click on the resident note's words did not reach the canvas (box %v)", box)
	}
}

// TestTheCanvasOverlaysGoQuietWhenHidden：源码契约。它就是本轮三个 bug 的共同
// 根因——**`PassThrough` 要逐层标**，只标外层会静默地漏掉子元素。
func TestTheCanvasOverlaysGoQuietWhenHidden(t *testing.T) {
	button := funcBody(t, "barButton")
	if !strings.Contains(button, "live bool") {
		t.Fatal("barButton no longer takes the reveal state: a hidden control cannot be told from a live one")
	}
	if !strings.Contains(button, "button.PassThrough()") {
		t.Error("the hidden control is not PassThrough: an invisible 28u square still catches the click")
	}
	if !strings.Contains(button, "icon.PassThrough()") {
		t.Error("the hidden control's glyph is not PassThrough: PassThrough does not reach children")
	}
	bar := funcBody(t, "titleBar")
	if got := strings.Count(bar, "revealed, a.Actions"); got != 4 {
		t.Errorf("titleBar hands the reveal state to %d of the 4 controls it builds directly", got)
	}
	if !strings.Contains(bar, "revealed, func() { a.Actions.Pin") {
		t.Error("the pin control does not read the reveal state")
	}
	for _, name := range []string{"feedbackNote", "residentNote"} {
		body := funcBody(t, name)
		// The band itself plus every word it paints: a `Text`/`Icon` is its own
		// hit target, so each one has to carry the flag.
		want := 1 + strings.Count(body, "ui.Text(") + strings.Count(body, "ui.Icon(")
		if got := strings.Count(body, "PassThrough()"); got < want {
			t.Errorf("%s marks %d of its %d layers PassThrough: the rest still take the pointer", name, got, want)
		}
	}
}

// TestDumpP11BDPointerFrames is opt-in (FLOTER_DUMP_P11BD=1): it writes the two
// frames this round touches. The fix changes hit flags and nothing else, so the
// PNGs are byte-identical to the pre-fix build — that is the point of the dump
// (see the report's §5).
func TestDumpP11BDPointerFrames(t *testing.T) {
	if os.Getenv("FLOTER_DUMP_P11BD") == "" {
		t.Skip("opt-in: set FLOTER_DUMP_P11BD=1 to write the frames")
	}
	dir := os.Getenv("FLOTER_DUMP_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	copy := i18n.For("en").Terminal
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		a := New(newStore(), Actions{Settings: func() {}, Close: func() {}}, nil)
		withSession(a, &terminal.Terminal{})
		a.ResidentAt(0, 3)
		a.ShowFeedback("no emulator")
		tt := render(t, a, 860, 600)
		if dark {
			tt.SetDark(true)
			tt.Frame()
		}
		if !tt.HasText(copy.ProcessExitedHint) || !tt.HasText("no emulator") {
			t.Fatalf("%s: the frame is missing a note: %q", appearance, tt.Texts())
		}
		f, err := os.Create(dir + "/floter-P11-bd-notes-" + appearance + ".png") // #nosec G304 -- fixed name in a dump dir
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
