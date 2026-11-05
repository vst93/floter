package shell

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"floter/internal/ansi"
	"floter/internal/extensions"
	"floter/internal/launcher"
	"floter/internal/settings"
)

// P13-gw · 捕获输出窗口的**复用分支**（`detachOutput` 命中 `findDetached` 的那条）
// 与便条窗口的几何键。
//
// 这一轮接续的是 2026-11-05 11:03 的一条抢救 stash（`stash@{2}` = `8a69c7f8` 之后的
// 快照）：那条半成品里有 5 项从未进过 HEAD——`detached` 复用窗口只换了 `view`，其余
// 每一样都属于**上一份内容**；分离窗口把 `view.Text` 当正文画（命令的颜色全丢）；
// `Empty` 的句子在窗口里画成输出；所有便条共用 `"pinned"` 一格几何。下面每个
// `Test…` 一条，都是「现象 → 改动 → 验证」里那条验证。
//
// 复用窗口为什么必须清自己的状态：它是**同一个窗口换了一份内容**（R91「窗口属于
// 它的内容」的另一半），而 `detachedOutput` 的字段全是「这一份内容」的账。

// detachReuseApp is the shell with a detached-window hook that opens nothing: the
// window itself needs a display, so the test reads the state the shell keeps.
func detachReuseApp(t *testing.T) *App {
	t.Helper()
	return New(Options{
		Store: settings.NewStore(settings.Default()),
		OpenDetached: func(launcher.OutputView) *mygo.Window {
			return nil
		},
	})
}

// detachEntry is one command, so every detach below has the same content key and
// therefore hits the reuse branch.
func detachEntry() *extensions.CommandEntry {
	return &extensions.CommandEntry{
		IntegrationID: "local.tool",
		Command:       extensions.Command{ID: "jv", Name: "JSON Viewer"},
	}
}

// detachInkTopOfBody is the first pixel row of the window body that holds ink:
// the bar's own title ends well above this, and the body starts under it.
func detachInkTopOfBody(img *image.RGBA) int {
	for y := 70; y < img.Rect.Dy()-2; y++ {
		if pluginEdgeSpread(img, y, 10, img.Rect.Dx()-10) > 100 {
			return y
		}
	}
	return -1
}

// A new run of the same command replaces the window's content. The scroll
// offset belonged to the *old* content, so the new one starts at its own
// beginning — the rule `showOutput` already follows for the panel's own
// scroller (P11-ar) and the plugin sheet follows for its scroll (P13-gu).
func TestAReusedWindowStartsItsNewContentAtTheTop(t *testing.T) {
	a := detachReuseApp(t)
	entry := detachEntry()
	a.detachOutput(launcher.OutputView{Title: "jv --floter", Text: pluginEdgeText(), Entry: entry})
	if len(a.detached) != 1 {
		t.Fatalf("the window is not held: %d", len(a.detached))
	}
	detached := a.detached[0]
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	tt.Frame()
	if detached.scroll.MaxY <= 0 {
		t.Fatalf("the body does not overflow this window (MaxY = %v): the pin below would pass on "+
			"a window that cannot scroll", detached.scroll.MaxY)
	}
	top := detachInkTopOfBody(tt.Image())
	if top < 0 {
		t.Fatalf("no body ink at all: %q", tt.Texts())
	}

	// A wheel down the body: the ink must move, or the reading below proves
	// nothing about the offset.
	detached.scroll.Y = 300
	tt.Frame()
	scrolled := detachInkTopOfBody(tt.Image())
	if scrolled == top {
		t.Fatalf("scrolling to Y=300 moved no ink (still %d): the pin cannot see an offset", top)
	}

	// The same command runs again: same window, new content.
	a.detachOutput(launcher.OutputView{Title: "jv --again", Text: pluginEdgeText(), Entry: entry})
	if got := detached.scroll; got != (ui.ScrollState{}) {
		t.Fatalf("the replaced window kept the old content's offset: Y=%v MaxY=%v", got.Y, got.MaxY)
	}
	tt.Frame()
	tt.Frame()
	if got := detachInkTopOfBody(tt.Image()); got != top {
		t.Fatalf("the new content starts at ink row %d, not where a fresh window starts it (%d)", got, top)
	}
}

// A fault is the sentence of a run that never answered. It is drawn *before* the
// text (the body's switch), so a window that kept it covered the new run's
// output with the old run's failure.
func TestAReusedWindowForgetsTheLastRunsFault(t *testing.T) {
	a := detachReuseApp(t)
	entry := detachEntry()
	a.detachOutput(launcher.OutputView{Title: "jv --floter", Text: "one", Entry: entry})
	detached := a.detached[0]
	detached.failed = "the host refused to start it"

	a.detachOutput(launcher.OutputView{Title: "jv --again", Text: "fresh output", Entry: entry})
	if got := detached.failed; got != "" {
		t.Fatalf("the replaced window keeps the old run's fault: %q", got)
	}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	if !tt.HasText("fresh output") {
		t.Fatalf("the new output is not drawn: %q", tt.Texts())
	}
	if tt.HasText("the host refused to start it") {
		t.Fatalf("the old fault is drawn over the new output: %q", tt.Texts())
	}
}

// `frames` counts the body's layouts; `detachedAskForMore` reads `frames <= 1` as
// "not measured yet". A reused window was measured long ago, so a fresh paged
// list's first frame (MaxY still 0) looked like its end and pulled page two at
// once — a page the user never reached.
func TestAReusedWindowIsMeasuredAgain(t *testing.T) {
	a := detachReuseApp(t)
	entry := detachEntry()
	a.detachOutput(launcher.OutputView{Title: "jv --floter", Text: "one", Entry: entry})
	detached := a.detached[0]
	detached.frames = 9

	first := `{"output":[` + detachRows(20) + `],"page":{"cursor":"c1","hasMore":true}}`
	view := a.Launcher.OutputFor(extensions.CapturedRun{Stdout: first}, nil, entry)
	if !view.Declared || view.Page == nil || !view.Page.HasMore {
		t.Fatalf("the self-paging emission was not read as one: %+v", view.Page)
	}
	var asks int
	a.Launcher.Actions.LoadOutputPage = func(_ extensions.CommandEntry, _ []string, _ string, done func(extensions.CapturedRun, error)) {
		asks++
		done(extensions.CapturedRun{Stdout: `{"output":[],"page":null}`}, nil)
	}
	a.detachOutput(view)
	if got := detached.frames; got != 0 {
		t.Fatalf("the replaced window is still holding %d measured layouts", got)
	}
	tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
	tt.Frame()
	if asks != 0 {
		t.Fatalf("a window that had not measured its new content asked for page two on its first frame")
	}
}

// The copy-on-select ledger belongs to the selection it saw. A fresh content
// whose first selection reads the same characters as the old one was treated as
// already copied: the user selected, let go, and nothing reached the clipboard.
func TestAReusedWindowDoesNotThinkTheNewSelectionWasCopied(t *testing.T) {
	a := detachReuseApp(t)
	entry := detachEntry()
	a.detachOutput(launcher.OutputView{Title: "jv --floter", Text: "same words", Entry: entry})
	detached := a.detached[0]
	detached.selectionText = "same words"
	detached.selectionCopied = "same words"
	detached.selectionAt = time.Now()

	a.detachOutput(launcher.OutputView{Title: "jv --again", Text: "same words", Entry: entry})
	if detached.selectionText != "" || detached.selectionCopied != "" || !detached.selectionAt.IsZero() {
		t.Fatalf("the replaced window still believes the old selection was copied: %q / %q / %v",
			detached.selectionText, detached.selectionCopied, detached.selectionAt)
	}
}

// The window draws the runs the command styled. It used to draw `view.Text`, the
// same characters with every sequence stripped, so a coloured `ls` or a diff
// arrived in one flat ink: one view, one reading of its styles (`TextSpans`).
func TestTheDetachedWindowKeepsTheRunsTheCommandStyled(t *testing.T) {
	const styled = "\x1b[38;2;200;40;40mRED\x1b[0m plain"
	for _, appearance := range []struct {
		name string
		dark bool
	}{{"light", false}, {"dark", true}} {
		t.Run(appearance.name, func(t *testing.T) {
			a := barApp(t, appearance.dark)
			detached := &detachedOutput{
				title: "clr --floter",
				ran:   true,
				view: launcher.OutputView{
					Title: "clr --floter",
					Text:  "RED plain",
					Spans: ansi.Parse(styled),
				},
			}
			tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
			tt.SetDark(appearance.dark)
			tt.Frame()
			tt.Frame()
			// One text element carries both runs, so the reading splits its own
			// box: the first third is `RED`, the rest is ` plain`.
			line, ok := tt.Find("RED plain")
			if !ok {
				t.Fatalf("the line is not drawn: %q", tt.Texts())
			}
			img := tt.Image()
			red := ui.Rect{X: line.X, Y: line.Y, W: line.W / 3, H: line.H}
			plain := ui.Rect{X: line.X + line.W*2/3, Y: line.Y, W: line.W / 3, H: line.H}
			redStep, plainStep := hueStep(img, red), hueStep(img, plain)
			t.Logf("%s: red run R-G %d, plain run R-G %d", appearance.name, redStep, plainStep)
			if redStep < plainStep+40 {
				t.Fatalf("the red run is drawn in the body's ink (R-G %d against the plain run's %d): "+
					"the command's colour is gone", redStep, plainStep)
			}
		})
	}
}

// detachInk is the body's ink as a reader sees it: how tall the tallest drawn
// line is, and how dark the darkest of its pixels is on a light surface.
type detachInk struct {
	height int
	luma   int
}

// detachInkOfRect reads one text element's own ink: the rows its glyphs cover
// (the element's box is the whole column and would read the same either way) and
// the darkest pixel in it.
func detachInkOfRect(img *image.RGBA, rect ui.Rect) detachInk {
	out := detachInk{luma: 1 << 20}
	started, last := -1, -1
	x0, x1 := int(rect.X), int(rect.X+rect.W)
	for y := int(rect.Y); y < int(rect.Y+rect.H)+1; y++ {
		row := false
		for x := x0; x < x1; x++ {
			v := pluginEdgeLuma(shellPixelAt(img, x, y))
			if v > 200 {
				continue
			}
			row = true
			if v < out.luma {
				out.luma = v
			}
		}
		if !row {
			if started >= 0 {
				break
			}
			continue
		}
		if started < 0 {
			started = y
		}
		last = y
	}
	if started < 0 {
		return detachInk{luma: 1 << 20}
	}
	out.height = last - started + 1
	return out
}

// hueStep is the strongest red-vs-green separation inside one drawn run: a
// red pixel reads high, the body's neutral ink reads near zero.
func hueStep(img *image.RGBA, rect ui.Rect) int {
	best := 0
	x0, y0 := int(rect.X), int(rect.Y)
	x1, y1 := int(rect.X+rect.W)+1, int(rect.Y+rect.H)+1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := shellPixelAt(img, x, y)
			if step := int(c.R) - int(c.G); step > best {
				best = step
			}
		}
	}
	return best
}

// `Empty` says the text is not output at all: it is the sentence the view wrote
// in place of an answer. The panel has drawn that sentence in the statement
// register since P13-cd; this window drew it as output — selectable body text at
// the body's own size, the same ink as a real answer.
func TestTheDetachedWindowDrawsAnEmptyAnswerAsAStatement(t *testing.T) {
	sentence := "the command wrote nothing"
	// The two registers differ in ink and in size, and both are read off the
	// pixels: the statement is `tokens.Emphasis` weight 580 in `TextStrong`, the
	// body is `tokens.Body` weight 400 in `TextSecondary`. A text element's box is
	// the whole column either way, so the reading has to be the ink itself.
	render := func(empty bool) (ink detachInk) {
		a := barApp(t, false)
		detached := &detachedOutput{
			title: "jv --floter",
			ran:   true,
			view:  launcher.OutputView{Title: "jv --floter", Text: sentence, Empty: empty},
		}
		tt := ui.NewTester(func(c *ui.Context) { a.detachedView(c, detached) }, 720, 460)
		tt.Frame()
		tt.Frame()
		rect, ok := tt.Find(sentence)
		if !ok {
			t.Fatalf("the sentence is not drawn (empty=%v): %q", empty, tt.Texts())
		}
		return detachInkOfRect(tt.Image(), rect)
	}
	asOutput, asStatement := render(false), render(true)
	if asStatement.luma >= asOutput.luma {
		t.Fatalf("the empty answer keeps the body's ink: luma %d against the body's %d",
			asStatement.luma, asOutput.luma)
	}
	if asStatement.height <= asOutput.height {
		t.Fatalf("the empty answer keeps the body's size: %d ink rows against the body's %d",
			asStatement.height, asOutput.height)
	}
}

// A note is a window holding *this* text under *this* title, so its remembered
// geometry is its own: one shared `"pinned"` key made every note open on the
// last note's rectangle and fight over it (R91: a window belongs to its content).
func TestANotesGeometryIsItsOwn(t *testing.T) {
	first, second := pinnedStateKey("note A"), pinnedStateKey("note B")
	if first == second {
		t.Fatalf("two notes share one geometry key: %q", first)
	}
	if !strings.HasPrefix(first, settings.DetachedStateKeyPrefix) {
		t.Fatalf("a note's key is outside the detached namespace: %q", first)
	}
	if pinnedStateKey(" note A ") != first {
		t.Errorf("the same title with stray spaces gets a second key: %q", pinnedStateKey(" note A "))
	}
	// And it is not a command's key: the text arm and the command arm are two
	// contents, so they cannot answer to one geometry.
	if first == detachStateKey("note A") {
		t.Errorf("a note and a command-less run share one geometry key: %q", first)
	}
}
