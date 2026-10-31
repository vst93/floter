package terminalui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settings"
)

// P13-bd · 终端页空态的三件东西：**一句陈述 + 两条出路**，而不是「一句灰提示 +
// 两条亮线」。
//
// `docs/raycast-design.md` §4.7（= `docs/liquid-glass.md` §6.1 表 1-6）给空态钉的是
// 一句陈述：`Emphasis` 步 + `TextStrong`，无底板、无图标、无 accent。P8-b 把
// `settingsui/empty.go` 抬到了这一档（会话页 / 自定义快捷键 / 集成页三个空态）。
//
// 终端页是同一族里**唯一没抬的那一个**，而且它把阶梯**上下颠倒**了：标题
// （`terminalui.go` 的 `View` 空态臂）画在 `TextMuted`（§4.7 的**旧值**，合约原话
// 「读起来像一条错误提示」），而两条出路（`emptyLine`）画在 `TextTertiary` —— 比
// 标题**更亮**。实测（860×600，本轮改前）：
//
//	              标题 lo    出路 lo    页面 lo
//	浅色           114        142        252
//	深色            21         21         21
//
// 浅色档标题是全页**最暗的一件**（出路比它亮 28 级）；深色档两件同值（`textMuted`
// `#85858D` 与 `textTertiary` `#6E6E76` 在这块面板上都压到 21）。
//
// 改后（同一帧）：标题走 `tokens.Emphasis` + `tokens.TextStrong`，出路走
// `tokens.TextSecondary`（hover `TextStrong`）。三档于是成阶梯：
// 陈述 ≫ 可以碰 ≫ 键帽的图例。**字号与盒子一个字节没动**——`t.FontSize + 1` 就是
// `tokens.Title`（14 × 0.9 = 12.6），而 `pageheight_test.go` 把这一行钉在 23。
//
// 四条账，深浅两套外观各跑一遍：
//
//	① 标题是这一页**最重**的一件：标题 lo ≤ 出路 lo − 20 级；
//	② 标题在「陈述」的对比档上：标题 lo ≤ 面板 lo − 100 级（浅）/ ≥ 面板 + 100（深）；
//	③ 出路不是陈述：出路 lo ≥ 标题 lo + 20 级（写成独立断言，不让一侧退化成另一侧的复制）；
//	④ hover 抬一档：指针落在出路行上时它的墨**更亮**，且键帽的冠线一个字节不动。
//
// 第 ② 条的「逐字节等于 `TextStrong`」那一半拿不到参照物（同一帧里没有另一件
// `TextStrong` 墨可借），所以它拆成两条：像素级的对比下界（上面）+ AST 级的 token
// 等值（`TestTheEmptyStateRegisterIsTheStatementsRegister`）。这是本轮的一处取舍，
// 写在报告里，而不是造一条写死像素值的脆弱断言。

// inkLevel is the glyph's own luma inside a text box: the darkest pixel on the
// light sheet (ink sits below the page) and the brightest on the dark one (ink
// sits above it). Reading `min` on both appearances is the mistake this round
// tripped over first: on a dark page the minimum is the *background*, so every
// box measures the same 21 and the ladder looks flat when it is not.
func inkLevel(img *image.RGBA, r ui.Rect, dark bool) int {
	lo, hi := 999, -1
	x0, x1 := int(r.X), int(r.X+r.W)
	y0, y1 := int(r.Y), int(r.Y+r.H)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := y*img.Stride + x*4
			if img.Pix[i+3] == 0 {
				continue
			}
			v := (int(img.Pix[i])*299 + int(img.Pix[i+1])*587 + int(img.Pix[i+2])*114) / 1000
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if dark {
		return hi
	}
	return lo
}

// pageLevel is the empty page's own face, sampled far from the copy: the page
// keeps the window's colour (see `panelFill`), so this is the panel's luma.
func pageLevel(img *image.RGBA, dark bool) int {
	lo, hi := 999, -1
	for y := 500; y < 560; y++ {
		for x := 100; x < 220; x++ {
			i := y*img.Stride + x*4
			if img.Pix[i+3] == 0 {
				continue
			}
			v := (int(img.Pix[i])*299 + int(img.Pix[i+1])*587 + int(img.Pix[i+2])*114) / 1000
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if dark {
		return hi
	}
	return lo
}

// contrast is how far a glyph's own level sits from the page: always positive,
// whichever way the appearance resolves.
func contrast(level, page int) int {
	if level < page {
		return page - level
	}
	return level - page
}

// emptyRegisterFrame draws the terminal's empty page with the pointer resting,
// and returns the tester plus the three boxes the pins read.
func emptyRegisterFrame(t *testing.T, dark bool) (*ui.Tester, i18n.Terminal, map[string]ui.Rect) {
	t.Helper()
	store := newStore()
	if err := store.Update(func(s *settings.Settings) {
		if dark {
			s.Theme = "dark"
		} else {
			s.Theme = "light"
		}
	}); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{Close: func() {}, NewCommand: func() {}}, nil)
	tt := render(t, a, 860, 600)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	term := i18n.For("en").Terminal
	boxes := map[string]ui.Rect{}
	for _, want := range []string{term.EmptyTitle, term.EmptyNew, term.EmptyBack} {
		box, ok := tt.Find(want)
		if !ok {
			t.Fatalf("the empty page does not draw %q: %v", want, tt.Texts())
		}
		boxes[want] = box
	}
	return tt, term, boxes
}

// TestTheEmptyStateTitleIsTheStatement pins ① and ②.
func TestTheEmptyStateTitleIsTheStatement(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tt, term, boxes := emptyRegisterFrame(t, dark)
			img := tt.Image()
			page := pageLevel(img, dark)
			title := contrast(inkLevel(img, boxes[term.EmptyTitle], dark), page)
			line := contrast(inkLevel(img, boxes[term.EmptyNew], dark), page)
			t.Logf("%s: title contrast=%d, line contrast=%d, page=%d", name, title, line, page)

			if title < line+20 {
				t.Errorf("%s: the title carries %d levels of contrast and the ways out %d — the page's one "+
					"statement is weaker than its two exits by %d levels, less than the 20 the register "+
					"needs: the ladder is upside down (want title ≥ line + 20)", name, title, line, line-title)
			}
			// ② the statement's own contrast step: a statement is not a hint.
			if title < 100 {
				t.Errorf("%s: the title carries %d levels of contrast over a page at %d — less than the 100 "+
					"a statement needs on this appearance", name, title, page)
			}
		})
	}
}

// TestTheEmptyStateWaysOutAreNotStatements pins ③: the mirror of ①, written
// separately so neither side can degrade into a copy of the other.
func TestTheEmptyStateWaysOutAreNotStatements(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tt, term, boxes := emptyRegisterFrame(t, dark)
			img := tt.Image()
			page := pageLevel(img, dark)
			title := contrast(inkLevel(img, boxes[term.EmptyTitle], dark), page)
			for _, want := range []string{term.EmptyNew, term.EmptyBack} {
				line := contrast(inkLevel(img, boxes[want], dark), page)
				if line > title-20 {
					t.Errorf("%s: %q carries %d levels of contrast against the title's %d — only %d levels "+
						"weaker: a way out is not a statement and must sit at least 20 below it "+
						"(want line ≤ title − 20)", name, want, line, title, title-line)
				}
			}
		})
	}
}

// TestAWayOutLiftsUnderThePointer pins ④: a line the pointer can land on lifts
// one rung, and the keycap beside it does not move — the cap is an E1 face
// whose crown is already drawn (P13-d / §6.3 表 3-8), and a hover that fades
// its subtree would be the `.Opacity(` arithmetic P13-bc rejected.
func TestAWayOutLiftsUnderThePointer(t *testing.T) {
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			tt, term, boxes := emptyRegisterFrame(t, dark)
			img := tt.Image()
			page := pageLevel(img, dark)
			rest := contrast(inkLevel(img, boxes[term.EmptyNew], dark), page)

			// Put the pointer on the first way out's own row and redraw.
			box := boxes[term.EmptyNew]
			tt.Move(box.X+box.W/2, box.Y+box.H/2)
			tt.Frame()
			tt.Frame()
			hovered := contrast(inkLevel(tt.Image(), boxes[term.EmptyNew], dark), page)
			t.Logf("%s: way out at rest %d, hovered %d (page %d)", name, rest, hovered, page)

			// The ink lifts *away* from the page on both appearances.
			if hovered < rest+10 {
				t.Errorf("%s: the way out carries %d levels of contrast at rest and %d under the pointer — "+
					"the ink moved %d levels, not the 10 a hover rung needs (want hovered ≥ rest + 10)",
					name, rest, hovered, hovered-rest)
			}

			// The keycap's crown, sampled on the cap's top inner row, is the E1
			// `LensRim` line. It is the one thing on this row that must not move:
			// it says "this face can be touched" and the pointer just said so.
			capBox, ok := tt.Find("\u21a9")
			if !ok {
				t.Skip("the keycap's legend is not in the tester's text list; the crown pin needs its box")
			}
			crownRest := crownLuma(img, capBox)
			crownHover := crownLuma(tt.Image(), capBox)
			t.Logf("%s: keycap crown %d → %d", name, crownRest, crownHover)
			if crownRest != crownHover {
				t.Errorf("%s: the keycap's crown moved %d → %d under the pointer: the hover is fading the "+
					"cap's own face, which is the `.Opacity(` arithmetic this round rejects",
					name, crownRest, crownHover)
			}
		})
	}
}

// crownLuma is the mean luma of the cap's top inner row: the row the E1
// `LensRim` line paints.
func crownLuma(img *image.RGBA, capBox ui.Rect) int {
	y := int(capBox.Y) + 1
	if y >= img.Rect.Dy() {
		y = img.Rect.Dy() - 1
	}
	sum, n := 0, 0
	for x := int(capBox.X) + 2; x < int(capBox.X+capBox.W)-2; x++ {
		i := y*img.Stride + x*4
		sum += (int(img.Pix[i])*299 + int(img.Pix[i+1])*587 + int(img.Pix[i+2])*114) / 1000
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

// TestTheEmptyStateRegisterIsTheStatementsRegister is pin ② 's second half: the
// title's ink must be the theme's own `TextStrong`, read off the source rather
// than off a pixel value. `go/ast`, not a line grep (the query shape P13-p /
// P13-bc adopted): a call chain wraps, and the defect under test is a chain.
func TestTheEmptyStateRegisterIsTheStatementsRegister(t *testing.T) {
	file, fset, err := parseTerminalSource("terminalui.go")
	if err != nil {
		t.Fatalf("parse terminalui.go: %v", err)
	}
	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "TextColor" || len(call.Args) != 1 {
			return true
		}
		arg, ok := call.Args[0].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// Only the title's own line: its argument is `tokens.TextStrong` and the
		// receiver is a `ui.Text` whose first argument is `copy.EmptyTitle`.
		if arg.Sel.Name != "TextStrong" {
			return true
		}
		if !paintsEmptyTitle(sel.X) {
			return true
		}
		found = true
		return true
	})
	if !found {
		t.Errorf("the empty state's title is not painted in `tokens.TextStrong` (§4.7's statement "+
			"register, the same one `settingsui.emptyState` uses): %s", fset.Position(file.Pos()))
	}
}

// paintsEmptyTitle walks a receiver chain of calls (`ui.Text(...).FontSize(...).TextColor(...)`)
// down to the innermost call and reports whether it is `ui.Text(c, copy.EmptyTitle)`.
func paintsEmptyTitle(expr ast.Expr) bool {
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if sel.Sel.Name == "Text" {
			// `ui.Text(c, copy.EmptyTitle)`: the context is the first argument, so
			// the copy is any later one.
			for _, arg := range call.Args {
				ident, ok := arg.(*ast.SelectorExpr)
				if ok && ident.Sel.Name == "EmptyTitle" {
					return true
				}
			}
			return false
		}
		expr = sel.X
	}
}

func parseTerminalSource(name string) (*ast.File, *token.FileSet, error) {
	src, err := os.ReadFile(filepath.Join(".", name))
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	return file, fset, err
}

// TestDumpTheEmptyStateRegister writes the round's visual evidence (opt-in, like
// every other dump: `FLOTER_P13BD_DUMP=1`).
func TestDumpTheEmptyStateRegister(t *testing.T) {
	if os.Getenv("FLOTER_P13BD_DUMP") == "" {
		t.Skip("set FLOTER_P13BD_DUMP=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		tt, _, _ := emptyRegisterFrame(t, dark)
		path := filepath.Join("/tmp", fmt.Sprintf("floter-P13-bd-terminal-empty-%s-after.png", name))
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
	}
}
