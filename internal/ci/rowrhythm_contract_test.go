package ci

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// P13-bq · 设置页间距节奏的第一张账：行的纵向刻度只能是一档。
//
// `.pending-bugs.md` 从 P13-bo 起连着两轮把「通用队列 · 间距节奏」列成下一格，并且
// 明写「先量设置卡片里 8/9/11/13u 的混用（**不许先改**）」。量下来那半张猜想在这条
// 链上不成立（`docs/rounds/P13-bq-spec.md` 有全量读数）：设置卡的**行**只有一档
// `t.Space(2.25)` = 9u / 36px，由 `cardRow` / `stackedRow` / `actionRow` 三条路共享；
// `Space(2.75)`(11u) 与 `Space(3.25)`(13u) 在 `internal/settingsui` 里**一次都没出现作间距**
// （全仓那两处都是 `Size(t.Space(3.25), t.Space(3.25))`，那是图标尺寸）。
//
// 所以这一条钉的不是「改哪一格」，是**别再长出第二格**：
//
//	① 三条行路的第一条间距语句必须是同一个 `Space(...)` 字面值（同族两个值 = 同一张卡
//	   里两行不一样高，用户读到的是「这两行不是一种东西」）；
//	② 它必须是 `Space(2.25)`（9u，参考 `.settings-row` 自己的那一档，见
//	   `docs/raycast-design.md` §4 的既落地条目）；
//	③ `internal/settingsui` 的**间距**参数里不许出现 `Space(2.75)` / `Space(3.25)`——
//	   这两档在这个包里没有角色，出现即为「顺手加了一个中间值」。
//
// 判据是「方法名 + 参数位」，不是字面文本：只有 `Padding` / `Gap` / `GapX` / `GapY` /
// `Margin` / `InsetShadow` 这些**间距方法**的第一参数位上的 `Space(...)` 进账，所以
// `Size(t.Space(3.25), t.Space(3.25))`（图标）与 `max(t.Space(11), rowFloorPx)`
// （行的**高度下限**，不是行的内边距）都不进这张账 —— 同一个数字在三条轴上不是一个东西。

// rowRhythmFns are the settings card's three row paths: they are one rhythm and must
// keep the same first vertical padding argument.
var rowRhythmFns = []string{"cardRow", "stackedRow", "actionRow"}

// rowRhythmPadMethod is the one method the row step is read from: a row's *vertical*
// padding. A row's `Gap` is the air between its own parts and a row's `min-height` is
// its floor — neither is the step this account is about.
const rowRhythmPadMethod = "Padding"

// rowRhythmSpacingMethods are the methods whose first argument is a measuring step.
var rowRhythmSpacingMethods = []string{
	"Padding", "Gap", "GapX", "GapY", "Margin", "MarginX", "MarginY", "InsetShadow",
}

// rowRhythmRowStep is the one vertical step a settings card row may wear (9u / 36px).
const rowRhythmRowStep = 2.25

// rowRhythmSpaceAllow lists `<path>:<line>` of spacing arguments allowed to be one of the
// two steps that have no role in this package. Empty, and it should stay that way: the
// fix is always to use one of the steps that already exist (8 / 9 / 10 / 12 / 16 / 24u).
var rowRhythmSpaceAllow = map[string]string{}

// TestTheSettingsRowsKeepOneVerticalStep is judgement ① and ②: the three row paths
// share one first vertical padding argument, and that argument is the row step.
func TestTheSettingsRowsKeepOneVerticalStep(t *testing.T) {
	root := filepath.Join("..", "settingsui")
	steps := map[string]map[float64]bool{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if perr != nil {
			return perr
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !rowRhythmHas(rowRhythmFns, fn.Name.Name) {
				continue
			}
			name := fn.Name.Name
			if steps[name] == nil {
				steps[name] = map[float64]bool{}
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || !rowRhythmIsMethod(call, rowRhythmPadMethod) {
					return true
				}
				if v, ok := rowRhythmSpaceArg(call.Args[0]); ok {
					steps[name][v] = true
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range rowRhythmFns {
		got := steps[name]
		if len(got) == 0 {
			t.Errorf("%s: no `%s(t.Space(...))` found — the row path moved or was renamed, "+
				"and this account is now measuring nothing", name, rowRhythmPadMethod)
			continue
		}
		if len(got) > 1 {
			t.Errorf("%s: the row path wears %d different vertical steps (%s); one row "+
				"path is one rhythm", name, len(got), rowRhythmJoinSteps(got))
		}
		for v := range got {
			if v != rowRhythmRowStep {
				t.Errorf("%s: the row's vertical step is Space(%s), want Space(2.25) — the "+
					"reference's own `.settings-row` step (9u / 36px). Changing it moves "+
					"every row's height, and the height ladder with it",
					name, rowRhythmFormat(v))
			}
		}
	}
	// The three paths must agree with each other, which the per-function loop above
	// cannot see on its own (each may hold exactly one value and still differ).
	for _, v := range rowRhythmSteps(steps[rowRhythmFns[0]]) {
		for _, name := range rowRhythmFns[1:] {
			if !steps[name][v] {
				t.Errorf("the row paths disagree: %s has Space(%s) but %s does not — the "+
					"settings card's three row shapes are one rhythm",
					rowRhythmFns[0], rowRhythmFormat(v), name)
			}
		}
	}
}

// TestTheSettingsPackageKeepsItsSpacingSteps is judgement ③: no spacing argument in
// `internal/settingsui` wears one of the two steps that have no role there.
func TestTheSettingsPackageKeepsItsSpacingSteps(t *testing.T) {
	root := filepath.Join("..", "settingsui")
	var offenders, stale []string
	fset := token.NewFileSet()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = "internal/settingsui/" + filepath.ToSlash(rel)
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || !rowRhythmIsSpacing(call) {
				return true
			}
			v, ok := rowRhythmSpaceArg(call.Args[0])
			if !ok || (v != 2.75 && v != 3.25) {
				return true
			}
			key := rel + ":" + strconv.Itoa(fset.Position(call.Lparen).Line)
			if _, ok := rowRhythmSpaceAllow[key]; ok {
				return true
			}
			offenders = append(offenders, key+" (Space("+rowRhythmFormat(v)+"))")
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for key := range rowRhythmSpaceAllow {
		used := false
		for _, o := range offenders {
			if strings.HasPrefix(o, key+" ") {
				used = true
			}
		}
		if !used {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("these allowlist entries no longer name an offender: %s", strings.Join(stale, ", "))
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("`Space(2.75)` / `Space(3.25)` have no role as spacing in this package "+
			"(they are icon sizes here): %s", strings.Join(offenders, ", "))
	}
}

// rowRhythmIsSpacing reports whether a call is one of the measuring methods.
func rowRhythmIsSpacing(call *ast.CallExpr) bool {
	for _, m := range rowRhythmSpacingMethods {
		if rowRhythmIsMethod(call, m) {
			return true
		}
	}
	return false
}

// rowRhythmIsMethod reports whether a call is `<recv>.<name>(...)`.
func rowRhythmIsMethod(call *ast.CallExpr, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == name && len(call.Args) > 0
}

// rowRhythmSpaceArg reports the float literal behind a `t.Space(<n>)` argument.
func rowRhythmSpaceArg(arg ast.Expr) (float64, bool) {
	call, ok := arg.(*ast.CallExpr)
	if !ok {
		return 0, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Space" || len(call.Args) != 1 {
		return 0, false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || (lit.Kind != token.INT && lit.Kind != token.FLOAT) {
		return 0, false
	}
	v, err := strconv.ParseFloat(lit.Value, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// rowRhythmFormat prints a step the way the source writes it (`2.25`, not `2.250000`).
func rowRhythmFormat(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// rowRhythmSteps returns a set's members in a stable order.
func rowRhythmSteps(set map[float64]bool) []float64 {
	out := make([]float64, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Float64s(out)
	return out
}

// rowRhythmJoinSteps renders a set for a failure message.
func rowRhythmJoinSteps(set map[float64]bool) string {
	var parts []string
	for _, v := range rowRhythmSteps(set) {
		parts = append(parts, "Space("+rowRhythmFormat(v)+")")
	}
	return strings.Join(parts, ", ")
}

// rowRhythmHas is a local helper: this package's other files do not need one, and the
// standard library's `slices.Contains` is not used in this tree yet.
func rowRhythmHas(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
