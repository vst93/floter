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

// P13-gd · 受光边只有一个来源：视图里画的每一条内线，颜色必须来自 token。
//
// `docs/liquid-glass.md` §3 说受光边是 E1 的**结构线**而不是装饰，§1 又把它从
// 「无光泽上限 0.04」重定基线到 0.09 / 0.24 / 0.20 / 0.06（P12-a、P13-n、P13-ag、
// P13-ah 四次）。四次治的是同一种病：一个视图**手写**了一条线，于是 token 再改
// 也改不到它。最刺眼的一次是浅色档的 `LensRim`——0.09 的白压在 221 上只抬 3 级，
// 「这块面可以被碰」的记号是一个看不见的像素，而**测试全绿**。
//
// 判据（一条）：生产文件里每一个 `.InsetShadow(...)` 的**颜色实参**不许是字面量
// （`ui.RGBA(...)` / `ui.Hex(...)` / `ui.RGB(...)` / `ui.Color{...}`）。
//
// 为什么只扫视图层：`internal/theme` 就是 token 的定义处，`LensInset` 那一对字面值
// （`0.85` / `0.06`）是凹槽镜片的**原始数值**，把这个文件排除在外才是对的账。
//
// 前置断言：扫到的站点数必须 ≥ 18。账目本身是 21 条（P13-gd 实测；视图树的
// grep 命中是 26 行，其中 5 行只在散文注释里），一条扫描器写坏了（改名、换 API、walk 少了一个根）
// 就必须**大声红**，而不是安静地变成一张空表——那是本仓 census 一族的老毛病。
//
// 逃生口只有一条：`litedgeLiteralAllow` 写清理由；条目**已经满足**了就报「过期」
// 并变红（同 `litarm_contract_test.go` / `flatface_contract_test.go` 的约定）。
var litedgeLiteralAllow = map[string]string{}

// litedgeViewRoots are the trees that paint with the token layer, and the one
// tree that *is* the token layer.
var litedgeViewRoots = []string{"..", "../../cmd"}

// litedgeTokenRoot is the definition site: literals are the point there.
const litedgeTokenRoot = "../theme"

// litedgeMinSites is the population the scan must still see (21 at P13-gd:
// 26 grep hits in the view trees minus 5 that only exist in prose comments).
const litedgeMinSites = 18

// TestEveryLitEdgeTakesItsColourFromAToken pins the one source of a lit edge.
func TestEveryLitEdgeTakesItsColourFromAToken(t *testing.T) {
	var files []string
	for _, root := range litedgeViewRoots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if filepath.ToSlash(path) == litedgeTokenRoot || info.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if len(files) == 0 {
		t.Fatal("the scan found no production files at all")
	}

	fset := token.NewFileSet()
	sites, violations := 0, 0
	seen := map[string]bool{}
	var bad []string
	for _, path := range files {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "InsetShadow" || len(call.Args) == 0 {
				return true
			}
			sites++
			colour := call.Args[len(call.Args)-1]
			if !isLiteralColour(colour) {
				return true
			}
			violations++
			pos := fset.Position(sel.Pos())
			rel := filepath.ToSlash(strings.TrimPrefix(pos.Filename, "../"))
			where := rel + ":" + strconv.Itoa(pos.Line)
			seen[where] = true
			if reason, allowed := litedgeLiteralAllow[where]; allowed {
				_ = reason
				return true
			}
			bad = append(bad, where+"  <- "+strings.TrimSpace(sourceOf(fset, path, colour)))
			return true
		})
	}

	if sites < litedgeMinSites {
		t.Fatalf("the scan only saw %d inset lines (want >= %d): the census went blind, not the app", sites, litedgeMinSites)
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("a lit edge takes its colour from a token, not a literal: %s — §3 的受光边只有一个来源", b)
	}
	for where := range litedgeLiteralAllow {
		if !seen[where] {
			t.Errorf("stale allowlist entry %q: this site no longer writes a literal colour", where)
		}
	}
}

// isLiteralColour reports whether a colour argument is written as a value
// instead of read from a token: `ui.RGBA(...)`, `ui.RGB(...)`, `ui.Hex(...)`
// or `ui.Color{...}`.
func isLiteralColour(n ast.Expr) bool {
	switch e := n.(type) {
	case *ast.CallExpr:
		sel, ok := e.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "ui" {
			return false
		}
		switch sel.Sel.Name {
		case "RGBA", "RGB", "Hex":
			return true
		}
	case *ast.CompositeLit:
		sel, ok := e.Type.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "ui" && sel.Sel.Name == "Color" {
			return true
		}
	case *ast.UnaryExpr:
		return isLiteralColour(e.X)
	}
	return false
}
