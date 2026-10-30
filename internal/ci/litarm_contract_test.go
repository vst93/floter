package ci

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// P13-ab · 指针状态的材质账：`Hovered()` / `Pressed()` 臂里出现的面，也必须有一对线。
//
// 前两代 census 都是「面 **+ 边** ⇒ 必须有一对线」：
//
//   - `internal/settingsui/facecensus_test.go`（P13-p）按**函数体**扫，判据要求面**同时**
//     有 `Border(` / `InsetShadow(`；
//   - P13-aa 又量出它的第二个盲区——**没有边**的面不进视野（悬停只有一层 `ControlHover`
//     洗色，一个 `Border` 都没有），四处全靠人肉找。
//
// 这一条把账补到指针状态这一层，并且把判据收到**臂级**：函数级是空账（实测 25 个带指针
// 臂的函数，25 个都有一对线——因为四层 `Lens` / 选中块的 `LitFace` 都在同一个函数里）。
// 真正会漏的是**臂**：一个 `case segment.Hovered():` 里只写一句
// `Background(tokens.ControlHover)`，同一函数里别的一格（选中格）却有 `Lens`。
//
// 三条判据：
//
//	① 「一对线」是**严格**的：`LitFace(` / `.Lens(` / `LensInset(` / `InsetShadow(0, 1, 0, 0`。
//	   裸的 `InsetShadow(0, 1, 0, 0, …)` **不算**——那正是 P13-aa 的 execute 键（只有顶边、
//	   没有对侧），而 P13-p 的 facecensus 把它算成 pair，于是那块「半对」的面从来没进过视野。
//	② 逃生口只有一条：**同一个函数在臂之外的别处**画了一次那一对线。这是本仓的常见写法
//	   （`power.go` 的 `if plane.A > 0 { LitFace }`、`cardRow` 的 `if face.Lit { LitFace }`、
//	   `shortcutRecorder` 的序言 `LitFace`）：一份面，所有状态共用一对线。
//	③ 都不满足 ⇒ 必须在 `pointerFaceAllow` 里写清理由；条目**已经满足**了就报「过期」并变红。
//
// 回测（报告里有实跑）：P13-aa 修掉的 `output.go` / `power.go` 三处，在**修之前**全部会被
// 这条判据抓到；`power.go` 的 execute 键（半对）也会被抓到，因为 ① 不认裸的顶边。

// pointerPairMarks are the shapes that draw E1's pair (or a groove's own pair) as one
// statement. A bare `InsetShadow(0, 1, 0, 0, …)` is deliberately absent: one line is
// not a pair.
var pointerPairMarks = []string{
	"LitFace(",
	".Lens(",
}

// pointerFaceMarks are the material markers that make a pointer arm say "this face can
// be touched": the ladder's three faces plus the two row washes (a row is a reference's
// own flat `:hover` tint, so the row washes also have to be accounted for — either by a
// shared pair or by an allowlist entry that says why).
var pointerFaceMarks = []string{
	"ControlHover",
	"ControlPress",
	"IconSurfaceHover",
	"RaisedWarm",
	"inks.Wash",
}

// pointerFaceAllow lists the pointer arms that paint a face and deliberately do not wear
// a pair. Each entry has to name the contract line; a face with no pair is a decision,
// never an oversight.
var pointerFaceAllow = map[string]string{
	// 凹槽里未被选中的那一格：`ControlHover` 是参考自己的 `.settings-option:hover`
	// 平涂。给它一对线会读成「抬起来的一格」，而那正是**选中格**独占的一句话
	// （`Thumb` + `Lens` + accent 环，P13-r / §6.2b 第 9-11 行）：一个凹槽里只有一格
	// 能站起来。
	"control.SegmentedIndex/Hovered": "§6.2b 第 9-11 行：凹槽里只有选中的那一格能站起来（P13-r）",
	// 启动器这两枚小格的**按下**档：`ControlPress` 面 + `Scale(0.92)`，没有一对线。
	// 这一档要不要保留一对线**需拍板**（P13-aa「仍待」第 2 条：按下是「沉进玻璃」，
	// 参考自己也去掉了影；设置页同族的 `P13-q` 反过来在按下档保留了那一对）。
	// 本表只记账，不改语义——拍板之后删掉这两行、让臂自己满足即可。
	"launcher.rowControl/Pressed":          "需拍板（P13-aa 仍待第 2 条）：按下档的面与几何在 P13-q 已定，一对线未定",
	"launcher.fieldSettingsButton/Pressed": "需拍板（P13-aa 仍待第 2 条）：同上，同一个 28u 小格",
}

// pointerArm is one `case X.Hovered():` / `if X.Pressed() {` block.
type pointerArm struct {
	key   string // pkg.func/Hovered
	where string // file:line
	text  string
	start token.Pos
	end   token.Pos
}

func hasMark(s string, marks []string) bool {
	for _, m := range marks {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// isPointerTest reports whether an expression is a `something.Hovered()` /
// `something.Pressed()` call.
func isPointerTest(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return sel.Sel.Name == "Hovered" || sel.Sel.Name == "Pressed"
}

// TestEveryPointerStateFaceNamesItsPair is the arm-level census.
func TestEveryPointerStateFaceNamesItsPair(t *testing.T) {
	pkgs := []string{"../control", "../launcher", "../settingsui", "../shell", "../terminalui"}
	seen := map[string]bool{}
	armsWithFace := 0
	for _, pkg := range pkgs {
		entries, err := os.ReadDir(pkg)
		if err != nil {
			t.Fatalf("could not read %s: %v", pkg, err)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		pkgName := filepath.Base(pkg)
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(pkg, name)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("could not read %s: %v", path, err)
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, src, 0)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			text := func(n ast.Node) string { return string(src[n.Pos()-1 : n.End()-1]) }
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				var arms []pointerArm
				// cut holds every `case` clause (pointer or not): a switch's clauses are
				// mutually exclusive states, so a pair painted in the `i == *selected`
				// clause never covers the `segment.Hovered()` one — that is exactly the
				// hole this census exists to find (P13-r's groove). `if` bodies stay in
				// `outside`, because `if plane.A > 0 { LitFace }` is the shared-pair
				// shape this codebase actually writes.
				var cut []pointerArm
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.CaseClause:
						if node.List != nil {
							cut = append(cut, pointerArm{start: node.Pos(), end: node.End()})
						}
						for _, e := range node.List {
							if !isPointerTest(e) {
								continue
							}
							var sb strings.Builder
							for _, s := range node.Body {
								sb.WriteString(text(s))
								sb.WriteString("\n")
							}
							arms = append(arms, pointerArm{
								key:   pkgName + "." + fn.Name.Name + "/" + pointerMethodName(e),
								where: fset.Position(node.Pos()).String(),
								text:  sb.String(),
								start: node.Pos(),
								end:   node.End(),
							})
						}
					case *ast.IfStmt:
						if isPointerTest(node.Cond) {
							arm := pointerArm{
								key:   pkgName + "." + fn.Name.Name + "/" + pointerMethodName(node.Cond),
								where: fset.Position(node.Pos()).String(),
								text:  text(node.Body),
								start: node.Pos(),
								end:   node.End(),
							}
							arms = append(arms, arm)
							cut = append(cut, arm)
						}
					}
					return true
				})
				if len(arms) == 0 {
					continue
				}
				// The escape hatch: the same function paints the pair somewhere that is
				// not one of its pointer arms (a shared prologue/tail, one pair for every
				// state).
				outside := withoutSpans(text(fn.Body), cut, fn.Body.Pos())
				sharedPair := hasMark(outside, pointerPairMarks)
				for _, arm := range arms {
					if !hasMark(arm.text, pointerFaceMarks) {
						continue
					}
					armsWithFace++
					reason, allowed := pointerFaceAllow[arm.key]
					inArm := hasMark(arm.text, pointerPairMarks)
					seen[arm.key] = true
					switch {
					case allowed && (inArm || sharedPair):
						t.Errorf("%s: the allowlist entry %q is stale — this arm now names its pair (%s); "+
							"drop the entry and cite the pixel pin instead", arm.where, arm.key, reason)
					case allowed:
						// Deliberate: the reason is in the table.
					case !inArm && !sharedPair:
						t.Errorf("%s: %s paints a face in its pointer arm (%s) and never a pair — not in the arm, "+
							"not once for every state elsewhere in the function. Paint `tokens.LitFace` (E1: top "+
							"`LensRim` + bottom `LensEdge`) or add a reason to pointerFaceAllow (%s §2/§3)",
							arm.where, arm.key, arm.text, "docs/liquid-glass.md")
					}
				}
			}
		}
	}
	if armsWithFace < 20 {
		t.Fatalf("the census only found %d pointer-state faces — the scan is broken, not the five packages "+
			"(it found 29 on 2026-10-31)", armsWithFace)
	}
	for key := range pointerFaceAllow {
		if !seen[key] {
			t.Errorf("pointerFaceAllow lists %q, which no longer paints a face in a pointer arm — drop the stale entry", key)
		}
	}
}

// pointerMethodName is the receiver method the arm tests (`Hovered` / `Pressed`).
func pointerMethodName(e ast.Expr) string {
	call := e.(*ast.CallExpr)
	return call.Fun.(*ast.SelectorExpr).Sel.Name
}

// withoutSpans is the function body's source with every pointer arm cut out, so a pair
// found in it belongs to the whole function rather than to one state.
func withoutSpans(body string, arms []pointerArm, bodyStart token.Pos) string {
	base := int(bodyStart - 1) // token.Pos is 1-based
	type span struct{ start, end int }
	spans := make([]span, 0, len(arms))
	for _, arm := range arms {
		spans = append(spans, span{int(arm.start) - 1 - base, int(arm.end) - 1 - base})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var sb strings.Builder
	at := 0
	for _, s := range spans {
		if s.start < at || s.end > len(body) {
			continue
		}
		sb.WriteString(body[at:s.start])
		at = s.end
	}
	sb.WriteString(body[at:])
	return sb.String()
}
