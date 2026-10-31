package settingsui

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
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
	"floter/internal/theme"
)

// P13-bc · accent 主 CTA 的指针阶梯。
//
// `docs/liquid-glass.md` §5 第 2 条把「主 CTA 填充」定成一类，本仓有两枚：About 页的
// 更新键（`updateButton`）与扩展面板的 Save / Run（`extensionPrimaryButton`）。
// 同一类、同一个角色，指针落上去时此前各有各的说法：
//
//	浅色：extensionPrimaryButton 静息面 6/74/166 → hover **6/74/166**（一动不动）
//	      updateButton          静息面 6/74/166 → hover  29/91/173（淡向 247 的地面）
//	深色：extensionPrimaryButton 静息面 143/183/255 → hover **143/183/255**
//	      updateButton          静息面 143/183/255 → hover 130/166/232（淡向 14 的地面）
//
// 三条算术上的事实，各自是一句话：
//
//  1. 一枚 accent 主 CTA **没有 hover 那一档**。一枚能按的面在指针下不动，读起来
//     像「它没收到这个指针」。
//  2. 另一枚用 `.Opacity(0.9)` 表达 hover，而透明度乘的是**整棵子树**——面、冠线、
//     墨一起淡。于是 §3 的「受光边 = 这块面可以被碰的记号」恰好在指针说「可以碰」
//     的那一刻自己熄了一半。
//  3. 同一个淡在两种外观上**方向相反**：浅色淡向 247 的地面 = 变亮（沉进纸里），
//     深色淡向 14 = 变暗（也沉）。这就是 P13-r 在分段控件上判掉的那条算术——
//     **透明度不是一句 z 声明**，它的方向由脚下的地面决定。
//
// `theme` 从 P8-a 起就带着 `AccentHover`（浅 `#0857C4` / 深 `#A3C4FF`）与
// `AccentPressed`（浅 `#053C87` / 深 `#7AA7FF`），生产代码里**零调用点**：阶梯设计
// 好了、从没接上。P13-bc 把它们接上，两枚 CTA 于是与中性胶囊族同一个说法
// （`Control` / `ControlHover` / `ControlPress`）。

// accentCta is one of the class's two members, drawn the way the product draws it.
type accentCta struct {
	name  string
	build func(c *ui.Context, a *App) ui.Element
}

func accentCtas() []accentCta {
	return []accentCta{
		{"updateButton", func(c *ui.Context, a *App) ui.Element {
			// `updateButton` draws into the context and returns nothing, so the
			// wrapper column takes its box.
			return wrapControl(c, func() { a.updateButton(c, "Download", false, nil) })
		}},
		{"extensionPrimaryButton", func(c *ui.Context, a *App) ui.Element {
			return a.extensionPrimaryButton(c, "Save", false)
		}},
	}
}

// accentCtaFrame renders one accent CTA on the real settings-page ground and
// returns the frame, the button's box and the tokens it was drawn with.
func accentCtaFrame(t *testing.T, dark bool, cta accentCta, stage string) (*image.RGBA, ui.Rect, theme.Tokens) {
	t.Helper()
	name := "light"
	if dark {
		name = "dark"
	}
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = name }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	tokens := theme.ForPreferences(store.Snapshot(), dark, ui.Preferences{})
	ground := realPageGround(t, dark)
	var box ui.Rect
	var ref ui.Handle
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(tokens.Theme)
		c.Root().Background(ground)
		// The probe puts the control at (20, 20) rather than centring it: a
		// centred box lands on a fractional row whenever its height is odd, and
		// a 1px line on a half-covered row is a blend — the reading would then
		// grade the rasteriser, not the material.
		ui.Column(c).FillWidth().FillHeight().Padding(20).
			AlignItems(ui.Start).Justify(ui.Start).Children(func() {
			cta.build(c, a).Bind(&ref)
			box = c.Resolve(ref).Bounds()
		})
	}, 400, 160)
	tt.SetDark(dark)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	// The pointer goes to the button's own centre. `Hovered()` is a flag the
	// frame reads, so the move needs a frame to land and then the colour
	// transition (120ms) has to run out before the reading means anything.
	if stage != "rest" {
		tt.Move(float32(box.X+box.W/2), float32(box.Y+box.H/2))
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
	}
	if stage == "press" {
		tt.Press(float32(box.X+box.W/2), float32(box.Y+box.H/2))
		tt.Frame()
		tt.Advance(300 * time.Millisecond)
		tt.Frame()
	}
	return tt.Image(), box, tokens
}

// accentCtaFace is the face of an accent CTA: the mean of the band a few rows
// under the top frame, which is above the label's own line box, so the reading
// grades the material and not the ink on it.
func accentCtaFace(img *image.RGBA, box ui.Rect) ui.Color {
	x0, x1 := int(box.X+box.W*0.25), int(box.X+box.W*0.75)
	faceY := int(box.Y) + 4
	return lensMean(img, x0, faceY, x1, faceY)
}

// accentCtaInk is the label band's furthest pixel from the face, which is the
// word itself.
func accentCtaInk(img *image.RGBA, box ui.Rect, face ui.Color) ui.Color {
	x0, x1 := int(box.X+box.W*0.25), int(box.X+box.W*0.75)
	var ink ui.Color
	best := 0
	for y := int(box.Y) + 5; y < int(box.Y+box.H)-3; y++ {
		for x := x0; x <= x1; x++ {
			p := imagePixel(img, x, y)
			if d := litAbs(litSum(p) - litSum(face)); d > best {
				best, ink = d, p
			}
		}
	}
	return ink
}

func litAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// TestBothAccentCTAsLiftUnderThePointer is the class's own statement: the two
// accent primaries answer the pointer the same way, and that way is the ladder's
// hover rung — not "no answer", and not "fade the whole subtree".
func TestBothAccentCTAsLiftUnderThePointer(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			for _, cta := range accentCtas() {
				rest, box, tokens := accentCtaFrame(t, appearance.dark, cta, "rest")
				hot, _, _ := accentCtaFrame(t, appearance.dark, cta, "hover")
				restFace := accentCtaFace(rest, box)
				hotFace := accentCtaFace(hot, box)
				if d := channelDelta(restFace, tokens.Theme.Accent); d > 12 {
					t.Errorf("[%s] %s: resting face = %v, want the accent %v", appearance.name, cta.name,
						restFace, tokens.Theme.Accent)
				}
				if d := channelDelta(hotFace, tokens.Theme.AccentHover); d > 12 {
					t.Errorf("[%s] %s: hovered face = %v, want the ladder's AccentHover %v — "+
						"the accent primary has no hover rung (liquid-glass.md §5 第 2 条)",
						appearance.name, cta.name, hotFace, tokens.Theme.AccentHover)
				}
			}
		})
	}
}

// TestAnAccentCTAsHoverMovesOnlyItsFace is the *why* of the rule above: the old
// `.Opacity(0.9)` multiplied the face, the crown and the ink together, so the
// lit edge went out exactly when the pointer said this face could be touched,
// and the same fade moved in opposite z directions per appearance (light faded
// towards its 247 ground = brighter = sunk; dark towards its 14 = darker = sunk).
// The ladder's answer moves the face and leaves the crown and the ink alone.
func TestAnAccentCTAsHoverMovesOnlyItsFace(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			for _, cta := range accentCtas() {
				rest, box, _ := accentCtaFrame(t, appearance.dark, cta, "rest")
				hot, _, _ := accentCtaFrame(t, appearance.dark, cta, "hover")
				restFace, hotFace := accentCtaFace(rest, box), accentCtaFace(hot, box)
				// The ink does not move. Under the old fade it did: light
				// `247/249/255` → `226/234/247`, dark `21/23/27` → `32/37/47`.
				restInk, hotInk := accentCtaInk(rest, box, restFace), accentCtaInk(hot, box, hotFace)
				if d := channelDelta(restInk, hotInk); d > 6 {
					t.Errorf("[%s] %s: hover moved the label ink %v → %v (Δ %v) — "+
						"the rung belongs to the face, not to the subtree",
						appearance.name, cta.name, restInk, hotInk, d)
				}
				// And the crown stays lit on the hover rung. Read as the extreme
				// of its first three rows: a 1px line at a fractional y is a blend,
				// and the two members land on different rows (`updateButton`'s
				// 28.8u box puts it on row 0, the 27u primary on row 1).
				crown := imagePixel(hot, int(box.X+box.W/2), int(box.Y))
				for dy := 0; dy <= 2; dy++ {
					if q := imagePixel(hot, int(box.X+box.W/2), int(box.Y)+dy); litSum(q) > litSum(crown) {
						crown = q
					}
				}
				// The threshold is 10 sums (~3/channel): the honest reading on the
				// dark accent face is `LensRim`'s 0.09 white over 163 → **171
				// (+8/+5/0 = 13 sums)**, and the defect it guards against read
				// **exactly 0** (the crown was the face).
				if litSum(crown)-litSum(hotFace) < 10 {
					t.Errorf("[%s] %s: the hovered crown %v is not a lit line over its face %v — "+
						"the mark of a pressable face went out exactly when the pointer landed",
						appearance.name, cta.name, crown, hotFace)
				}
			}
		})
	}
}

// TestBothAccentCTAsSinkWhenPressed is the third rung: the accent family had no
// press rung either, while the neutral pills have had one since P13-q
// (`ControlPress` + `Scale(0.97)`). A key that does not move when pressed reads
// as a key that did not take the click.
func TestBothAccentCTAsSinkWhenPressed(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			for _, cta := range accentCtas() {
				hot, box, tokens := accentCtaFrame(t, appearance.dark, cta, "hover")
				press, _, _ := accentCtaFrame(t, appearance.dark, cta, "press")
				hotFace, pressFace := accentCtaFace(hot, box), accentCtaFace(press, box)
				if d := channelDelta(pressFace, tokens.Theme.AccentPressed); d > 12 {
					t.Errorf("[%s] %s: pressed face = %v, want the ladder's AccentPressed %v — "+
						"the press rung is the ladder stepping back down",
						appearance.name, cta.name, pressFace, tokens.Theme.AccentPressed)
				}
				if d := channelDelta(hotFace, pressFace); d < 10 {
					t.Errorf("[%s] %s: pressed face %v is within %v of the hovered one %v — "+
						"the press frame is the hover frame", appearance.name, cta.name,
						pressFace, d, hotFace)
				}
			}
		})
	}
}

// TestNoAccentFaceExpressesDepthThroughOpacity is the census behind the fix: the
// bug was not one call site, it was the *idea* that a face can say "under the
// pointer" by fading its subtree. `AccentHover` / `AccentPressed` exist for
// exactly this; a production function that paints an accent face and also calls
// `.Opacity(` is doing the arithmetic P13-r rejected.
//
// It reads the package with `go/ast` rather than by grepping lines (the query
// shape P13-p adopted for material censuses): a line-based query misses a call
// chain that wraps, and the defect under test is exactly a call chain.
func TestNoAccentFaceExpressesDepthThroughOpacity(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	var offenders []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !hasSuffix(name, ".go") || hasSuffix(name, "_test.go") {
			continue
		}
		file, fset, err := parseGoFile(name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, fn := range file.Decls {
			decl, ok := fn.(*ast.FuncDecl)
			if !ok || decl.Body == nil {
				continue
			}
			paints, fades := false, false
			ast.Inspect(decl.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "Background":
					if len(call.Args) == 1 && isAccentSelector(call.Args[0]) {
						paints = true
					}
				case "Opacity":
					fades = true
				}
				return true
			})
			if paints && fades {
				offenders = append(offenders, fmt.Sprintf("%s:%d %s", name, fset.Position(decl.Pos()).Line, decl.Name.Name))
			}
		}
	}
	if len(offenders) > 0 {
		t.Errorf("an accent face says its depth through `.Opacity(`, which resolves against whatever ground is "+
			"underneath (liquid-glass.md §5 第 2 条 / P13-r) — use AccentHover / AccentPressed:\n  %s",
			joinLines(offenders))
	}
}

// isAccentSelector reports whether an expression names one of the accent ink
// fields — `t.Accent`, `tokens.Accent`, or `theme.Accent`.
func isAccentSelector(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Accent" || sel.Sel.Name == "AccentHover" || sel.Sel.Name == "AccentPressed"
}

func parseGoFile(name string) (*ast.File, *token.FileSet, error) {
	src, err := os.ReadFile(filepath.Join(".", name))
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	return file, fset, err
}

func hasSuffix(s, suffix string) bool {
	if len(suffix) > len(s) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}

func joinLines(lines []string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n  "
		}
		out += line
	}
	return out
}

// TestDumpP13BCAccentCtaStages writes the round's visual evidence: both accent
// CTAs, in three stages, both appearances.
func TestDumpP13BCAccentCtaStages(t *testing.T) {
	if os.Getenv("FLOTER_P13BC_DUMP") == "" {
		t.Skip("set FLOTER_P13BC_DUMP=1 to write the round's visual evidence")
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, stage := range []string{"rest", "hover", "press"} {
			img, _, _ := accentCtaFrame(t, dark, accentCtas()[0], stage)
			name := filepath.Join(os.TempDir(),
				fmt.Sprintf("floter-P13-bc-accentcta-%s-%s.png", appearance, stage))
			f, err := os.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
			f.Close()
			t.Logf("wrote %s", name)
		}
	}
}
