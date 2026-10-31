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

// P13-aq · 一次淡的账：`Disabled` 与产品自己的 `Opacity(<1)` 不许写在同一条链上。
//
// 框架的画笔给 `Disabled` 的整棵子树乘 **0.5**（`mygo@v0.7.3/ui/paint.go:257-259`），
// 产品再叠一个 `.Opacity(0.x)` 就是两次乘——有效 0.2~0.3。P13-s / P13-t / P13-u 在胶囊、
// 锁定表单、卸载对话框上判掉了这条，并写下「两道淡乘起来不是设计，是一道淡画两遍」；
// P13-aq 把同一句话收口到最后四处（extensions 图标键 / 配置页 Save / About 的更新 CTA /
// 会话页的 kill 药丸）。
//
// 判据是**链级**的，不是行级的：这个仓的调用链常年跨行
// （`pill := ui.ButtonBase(...).\n\tMinHeight(...).\n\tOpacity(0.4)...Disabled(true)`），
// 行级 grep 照不到。`Disabled(true)` 与 `Opacity(f)`（f < 1）出现在同一条方法链上即红。
//
// 逃生口只有一条：`disabledFadeAllow` 里写清理由。条目**已经满足**了就报「过期」并变红
// （同 `litarm_contract_test.go` 的约定），所以这张表只能变小，不会长草。

// disabledFadeAllow lists `<path>:<line>` of chains that are allowed to fade twice, with
// the reason. Empty, and it should stay that way: the fix is always to delete the app's
// own fade and let the framework's 0.5 be the only one.
var disabledFadeAllow = map[string]string{}

// TestNoDisabledChainFadesTwice walks the non-test sources under the repo's own
// internal/ and cmd/ trees and reports every method chain that both disables an
// element and fades it by hand.
func TestNoDisabledChainFadesTwice(t *testing.T) {
	root := filepath.Join("..", "..")
	var offenders, stale []string
	fset := token.NewFileSet()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			base := info.Name()
			if base == ".git" || base == "docs" || base == "testdata" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasPrefix(rel, "internal/") && !strings.HasPrefix(rel, "cmd/") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return perr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			disabled, faded := chainCalls(call)
			if !disabled || !faded {
				return true
			}
			line := fset.Position(call.Lparen).Line
			key := rel + ":" + strconv.Itoa(line)
			if _, ok := disabledFadeAllow[key]; ok {
				return true
			}
			offenders = append(offenders, key)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking the source tree: %v", err)
	}
	// An allowlist entry whose chain no longer fades twice is stale: the fix landed,
	// the excuse has to go.
	for key := range disabledFadeAllow {
		found := false
		for _, o := range offenders {
			if o == key {
				found = true
			}
		}
		if !found {
			stale = append(stale, key)
		}
	}
	sort.Strings(offenders)
	sort.Strings(stale)
	for _, key := range offenders {
		t.Errorf("%s: this chain both disables the element and fades it by hand — the framework already multiplies a disabled subtree by 0.5 (mygo ui/paint.go:257-259), so this is one fade drawn twice (P13-aq, docs/liquid-glass.md §6.2b-6). Delete the Opacity, or add the chain to disabledFadeAllow with the reason.", key)
	}
	for _, key := range stale {
		t.Errorf("%s is in disabledFadeAllow but its chain no longer fades twice — drop the entry", key)
	}
}

// chainCalls reports whether the receiver chain of call contains a `Disabled(true)`
// and an `Opacity(f)` with f < 1.
func chainCalls(call *ast.CallExpr) (disabled, faded bool) {
	for cur := call; cur != nil; {
		sel, ok := cur.Fun.(*ast.SelectorExpr)
		if !ok {
			return disabled, faded
		}
		switch sel.Sel.Name {
		case "Disabled":
			if len(cur.Args) == 1 {
				if id, ok := cur.Args[0].(*ast.Ident); ok && id.Name == "true" {
					disabled = true
				}
			}
		case "Opacity":
			if len(cur.Args) == 1 {
				if lit, ok := cur.Args[0].(*ast.BasicLit); ok && lit.Kind == token.FLOAT {
					if f, err := strconv.ParseFloat(lit.Value, 64); err == nil && f < 1 {
						faded = true
					}
				}
			}
		}
		next, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return disabled, faded
		}
		cur = next
	}
	return disabled, faded
}
