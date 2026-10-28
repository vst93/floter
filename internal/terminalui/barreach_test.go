package terminalui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
	"floter/internal/theme"
)

// P11-af：浮条是**浮在画布上**的一条 28u 全宽带。它以前整条 `.DragWindow()`，
// 于是画布最上面 28u（第一/第二行输出）收不到任何鼠标事件：点不进、拖不了
// 选区，终端的 select-to-copy 在这两行上失效。这条 pin 钉住两半：
//
//  1. 源码契约：浮条 `PassThrough()`（不是命中目标）、揭示读 `PointerPosition`、
//     `DragWindow()` 只剩一处且落在会话标题上（手柄收窄成 title）。
//  2. 行为：指针停在**标题字外**的带子里（那里没有任何可命中元素）仍会把标题
//     揭示出来——证明揭示走的是指针位置而不是命中；指针离开后重新藏回去。

func barRevealFrame(t *testing.T) (*ui.Tester, theme.Tokens) {
	t.Helper()
	a := New(newStore(), Actions{}, nil)
	withSession(a, &terminal.Terminal{})
	withTitle(a, barTitle)
	copy := i18n.For(a.Store.Snapshot().Language).Terminal
	var tok theme.Tokens
	tt := ui.NewTester(func(c *ui.Context) {
		tok = tokensOf(a, c)
		c.SetTheme(tok.Theme)
		c.Root().Background(ui.RGBA(0, 0, 0, 1))
		a.titleBar(c, copy)
	}, 900, 40)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	return tt, tok
}

func TestTheBarRevealsUnderAPointerItDoesNotCatch(t *testing.T) {
	tt, _ := barRevealFrame(t)
	// Clear of the bar: nothing is revealed, so the band is empty.
	tt.Move(600, 100)
	tt.Frame()
	_, _, restMass := barInkBox(tt, 40, 890, 40)
	if restMass > 200 {
		t.Fatalf("the bar drew ink with the pointer clear of it (mass %d)", restMass)
	}
	// Inside the band, but past the title's own words: `barTitle`'s ink ends
	// around x=300 at the body step, so x=600 is over plain canvas as far as
	// hit-testing goes — and over the bar as far as containment goes. The
	// reveal must follow the pointer, not a hit.
	tt.Move(600, 14)
	tt.Frame()
	minX, maxX, litMass := barInkBox(tt, 40, 890, 40)
	if maxX < 0 {
		t.Fatalf("the bar revealed no title under a pointer it does not catch: %q", tt.Texts())
	}
	if litMass <= restMass+1000 {
		t.Fatalf("the title did not appear under the pointer (rest %d, hovered %d)", restMass, litMass)
	}
	if maxX > 400 {
		t.Fatalf("the title's ink runs to x=%d; the pointer at 600 would be over its words, not the empty band", maxX)
	}
	_ = minX
	// Leaving the band folds it away again: the reveal is the pointer's, not a
	// one-way latch.
	tt.Move(600, 100)
	tt.Frame()
	if _, _, afterMass := barInkBox(tt, 40, 890, 40); afterMass > 200 {
		t.Fatalf("the bar stayed revealed after the pointer left the band (mass %d)", afterMass)
	}
}

// funcBody returns the source of the named method through the ast, so a rename
// elsewhere in the file cannot make a pin read the wrong block.
func funcBody(t *testing.T, name string) string {
	t.Helper()
	src, err := os.ReadFile("terminalui.go") // #nosec G304 -- the file under test, fixed name
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "terminalui.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name || fn.Recv == nil {
			continue
		}
		start := fset.Position(fn.Pos()).Offset
		end := fset.Position(fn.End()).Offset
		return string(src[start:end])
	}
	t.Fatalf("%s is not declared in terminalui.go", name)
	return ""
}

func TestTheTerminalBarIsNotTheWindowsDragHandle(t *testing.T) {
	body := funcBody(t, "titleBar")
	if strings.Contains(body, "AlignItems(ui.Center).DragWindow()") {
		t.Error("the bar itself is a drag handle again: the whole 28u band would take the pointer")
	}
	if !strings.Contains(body, "AlignItems(ui.Center).PassThrough()") {
		t.Error("the bar is not PassThrough: the 28u band over the canvas takes the pointer again")
	}
	if !strings.Contains(body, "Label(identityTitle).PassThrough()") {
		t.Error("the identity row is not PassThrough: its growing box still spans the whole band")
	}
	if !strings.Contains(funcBody(t, "statusDot"), ".PassThrough()") {
		t.Error("the status dot is a hit target: a 7u square of the band still takes the pointer")
	}
	if !strings.Contains(body, "PointerPosition()") {
		t.Error("the bar's reveal does not read the pointer's position; a PassThrough bar has no Hovered() to read")
	}
	if n := strings.Count(body, "DragWindow()"); n != 1 {
		t.Fatalf("the bar has %d DragWindow() calls, want 1 (the session title is the page's title bar)", n)
	}
	i := strings.Index(body, "DragWindow()")
	before := body[:i]
	stmt := before[strings.LastIndex(before, "ui."):]
	if !strings.HasPrefix(stmt, "ui.Text(c, identityTitle)") {
		t.Errorf("the drag handle is not the session title: %q", firstLine(stmt))
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// The P11-af visual evidence: the terminal page at rest and with the pointer in
// the top band. At rest the band paints nothing (the canvas under it is the
// whole page); the pointer reveals the title without the band becoming a hit
// target. Opt-in (`FLOTER_PNG=1`), like the other rounds' dumps.
func TestDumpP11AFBarPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		store := newStore()
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearance }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{}, nil)
		withSession(a, &terminal.Terminal{})
		withTitle(a, barTitle)
		var tok theme.Tokens
		tt := ui.NewTester(func(c *ui.Context) {
			tok = tokensOf(a, c)
			c.SetTheme(tok.Theme)
			c.Root().Background(tok.Theme.Background)
			a.View(c)
		}, 860, 320)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		for _, at := range []struct {
			name string
			y    float32
		}{{"rest", 160}, {"revealed", 14}} {
			tt.Move(600, at.y)
			tt.Frame()
			path := "/tmp/floter-P11-af-bar-" + at.name + "-" + appearance + ".png"
			f, err := os.Create(path) // #nosec G304 -- a fixed /tmp path this test owns
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, tt.Image()); err != nil {
				f.Close()
				t.Fatal(err)
			}
			f.Close()
			t.Logf("%s (%dx%d)", path, tt.Image().Rect.Dx(), tt.Image().Rect.Dy())
		}
	}
}
