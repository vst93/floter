package ci

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// P13-dc · 插件配置浮层的字阶：只许阶梯上的档，不许 body 的偏移量。
//
// 这一族在设置页里已经收过两次（§6.13 的 `t.FontSize-1` → `tokens.Caption`、
// §6.71 的写失败线），但浮层的**分组标题**一直挂着 `t.FontSize+2`。它不是外观问题：
// 偏移量在默认步上恰好落在阶梯上（12 + 2 = `tokens.Title` 的 14），可它**不随界面
// 大小缩放**——用户把「界面大小」调小一步，阶梯缩到 12.6，这一格仍是 12.8；调大一
// 步，阶梯到 15.4，这一格只有 15.2。同一块面上的两级字于是越走越近。
//
// 本轮先修了取景（`pluginSheetFrame` 少了 `c.SetTheme`，量的是框架库存的 14 / 4），
// 才把这一格换到 `tokens.Title`。这条账钉的是**别再长回来**：
//
//	① `internal/settingsui/pluginsheet.go` 里任何 `FontSize(...)` 的参数都不许是算式
//	   （`t.FontSize+2` / `-1` 这一族），必须是阶梯上的档；
//	② 允许两个来源：`tokens.<档>`（浮层里所有字）与 `t.FontSize`（浮层**自己的名字**，
//	   故意留在 body 档，见 §6.71 的「故意不做」）；
//	③ 账不能空转：这个文件里至少要有三条 `FontSize` 才算数到东西。
var fontLadderRungs = map[string]bool{
	"Caption": true, "Body": true, "Emphasis": true, "Title": true, "Display": true,
}

const fontLadderSheetFile = "../settingsui/pluginsheet.go"

// TestThePluginSheetsTextSizesAreLadderRungs is judgement ①②.
func TestThePluginSheetsTextSizesAreLadderRungs(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fontLadderSheetFile, nil, 0)
	if err != nil {
		t.Fatalf("cannot parse %s: %v", fontLadderSheetFile, err)
	}
	seen := 0
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "FontSize" {
			return true
		}
		seen++
		pos := fset.Position(call.Pos())
		arg := call.Args[0]
		switch a := arg.(type) {
		case *ast.BinaryExpr:
			t.Errorf("%s: the size is an expression (`%s`), not a rung of the ladder: an offset off "+
				"the body size does not scale with the interface step, so this line drifts away from "+
				"the ladder the moment the user changes it — use `tokens.<rung>`",
				pos, fontLadderExpr(a))
		case *ast.SelectorExpr:
			ident, ok := a.X.(*ast.Ident)
			if ok && ident.Name == "t" && a.Sel.Name == "FontSize" {
				return true // the sheet's own name, deliberately on the body register
			}
			if !fontLadderRungs[a.Sel.Name] {
				t.Errorf("%s: the size is `%s`, which is not a rung of the text ladder (caption / body / "+
					"emphasis / title / display)", pos, fontLadderExpr(a))
			}
		default:
			t.Errorf("%s: the size is `%s`, neither a ladder rung nor the sheet's own body register",
				pos, fontLadderExpr(arg))
		}
		return true
	})
	if seen < 3 {
		t.Fatalf("only %d `FontSize(...)` calls found in %s: the sheet moved or was renamed, and this "+
			"account is now measuring nothing", seen, fontLadderSheetFile)
	}
}

// fontLadderExpr renders the few expression shapes this account meets, for the
// failure message (the whole point is that a reader sees `t.FontSize+2` itself).
func fontLadderExpr(n ast.Node) string {
	switch e := n.(type) {
	case *ast.BinaryExpr:
		return fontLadderExpr(e.X) + " " + e.Op.String() + " " + fontLadderExpr(e.Y)
	case *ast.SelectorExpr:
		return fontLadderExpr(e.X) + "." + e.Sel.Name
	case *ast.Ident:
		return e.Name
	case *ast.BasicLit:
		return e.Value
	case *ast.CallExpr:
		return fontLadderExpr(e.Fun) + "(…)"
	}
	return strconv.Quote("?")
}
