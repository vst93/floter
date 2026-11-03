package ci

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// P13-dc / P13-dd · 设置页的字阶：只许阶梯上的档，不许 body 的偏移量。
//
// 这一族在设置页里已经收过四次（§6.13 的 `t.FontSize-1` → `tokens.Caption`、
// §6.71 的写失败线、§6.72 的浮层分组标题，以及 P13-dd 的整包收口）。它不是外观
// 问题，是**不随界面步长缩放**——实测四个步长上的行盒（`ui.Tester`，参考字体）：
//
//	         tiny 0.8   small 0.9   default 1.0   large 1.1
//	-1  行盒    13          15           17          19
//	caption     14          15           17          19
//	+1  行盒    17          18           20          22
//	emphasis    16          18           20          22
//	+2  行盒    18          19           22          23
//	title       17          19           22          23
//
// 出货档（0.9）上两者恰好重合，所以这些偏移量在默认设置里**看不出来**；用户把
// 「界面大小」调小一格，同一块面上的三级字同时离开阶梯（说明行 13 对 caption 的
// 14、次级标题 17 对 emphasis 的 16、区块标题 18 对 title 的 17）。阶梯是随步长
// 缩放的**名字**，偏移量不是。
//
// 这条账钉三件事：
//
//	① **偏移量这个写法本身**在这个包里不存在：任何 `… .FontSize ± …` 的算式都红
//	   （`size := t.FontSize - 1` 这种绕道也算——它正是本轮被清掉的那种代码），
//	   唯一的例外是 `fontLadderExceptions` 里点名、且自己写了理由的两个具名函数；
//	② `FontSize(...)` 的参数只许是阶梯档（`tokens.<档>` 或任何 `.<档>`）、这个面
//	   自己的正文档（`t.FontSize`）、或者一个**命名的局部量**（由①兜住它从哪来）；
//	③ 账不能空转：整包至少 30 处 `FontSize`，且例外名单里点名的函数仍然存在。
var fontLadderRungs = map[string]bool{
	"Caption": true, "Body": true, "Emphasis": true, "Title": true, "Display": true,
}

// fontLadderExceptions are the two sizes in this package that are deliberately
// **not** text registers. Each is a named helper whose own doc comment says why
// there is no rung for it. The account accepts the *call*, and the closing
// over an offset inside the helper's own body — never at a call site.
var fontLadderExceptions = map[string]string{
	// `●` / `○`: a glyph, not a line; it has to fit inside a caption line.
	"markDot": "a status disc is a glyph, not a register (there is no rung under caption)",
	// The uninstall dialog's hints: the reference's 10.5px step, half a unit
	// under the body *by definition* (`ComponentizedUninstallDialog.css`).
	"uninstallSubBody": "the reference defines this register as body minus half a unit",
}

// fontLadderAccount is one package's entry in the account: the package's own
// production files, the floor below which the account is measuring nothing,
// and the sizes that are deliberately not text registers.
//
// P13-fe（`docs/liquid-glass.md` §6.126）：这一本账从 P13-dc 起只管 `settingsui`，
// §6.74 把启动器那九处量完之后写死的下一步是「先收口取景，再搬这 5 处，再把字阶账
// 从 `internal/settingsui` 扩到 `internal/launcher`——现在扩过去只能给调用点开例外，
// 那是把账写软」。取景已由 P13-ec / P13-ed / P13-ee 收口，启动器也只剩两处偏移量
// （结果行标题 `+2`、输出页状态条 `-1`），于是账本扩成一张表：两个包，同一个判据，
// 零个为启动器开的例外。
type fontLadderAccount struct {
	pkg      string
	minCalls int
	// exceptions are the sizes in this package that are deliberately **not**
	// text registers. Each is a named helper whose own doc comment says why
	// there is no rung for it. The account accepts the *call*, and the closing
	// over an offset inside the helper's own body — never at a call site.
	exceptions map[string]string
}

var fontLadderAccounts = []fontLadderAccount{
	{pkg: "../settingsui", minCalls: 30, exceptions: fontLadderExceptions},
	// The launcher draws ~18 texts across the field row, the result rows, the
	// status band and the captured-output page; ten is the floor that still
	// proves the walk found the package rather than an empty directory.
	{pkg: "../launcher", minCalls: 10},
}

// TestTheSettingsPackagesTextSizesAreLadderRungs is judgements ①②, for every
// package in the account (`fontLadderAccounts`: the settings pages and, since
// P13-fe, the launcher).
func TestTheSettingsPackagesTextSizesAreLadderRungs(t *testing.T) {
	for _, account := range fontLadderAccounts {
		account := account
		t.Run(account.pkg, func(t *testing.T) {
			fontLadderAudit(t, account)
		})
	}
}

func fontLadderAudit(t *testing.T, account fontLadderAccount) {
	files := fontLadderFiles(t, account.pkg)
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}
	for _, name := range files {
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("cannot parse %s: %v", name, err)
		}
		parsed[name] = file
	}
	exceptionSpans := fontLadderExceptionSpans(fset, parsed, account.exceptions)

	seen := 0
	for _, file := range parsed {
		ast.Inspect(file, func(n ast.Node) bool {
			if binary, ok := n.(*ast.BinaryExpr); ok {
				if !fontLadderOffsetsABodySize(binary) {
					return true
				}
				if fontLadderInsideSpan(fset, binary.Pos(), exceptionSpans) {
					return true
				}
				t.Errorf("%s: the size is an expression (`%s`), not a rung of the ladder: an offset off "+
					"the body size does not scale with the interface step, so this line drifts away from "+
					"the ladder the moment the user changes it — use `tokens.<rung>`",
					fset.Position(binary.Pos()), fontLadderExpr(binary))
				return true
			}
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
				// An offset off a ladder rung (`tokens.Caption + 1`) is the same
				// defect as an offset off the body: ① only sees `.FontSize`, so
				// this arm has to name the rest. When ① already named this
				// line, stay silent and do not report one defect twice.
				if !fontLadderOffsetsABodySize(a) {
					t.Errorf("%s: the size is an expression (`%s`), not a rung of the ladder: an offset off a\n"+
						"rung (or off the body) does not scale with the interface step — name the rung itself",
						pos, fontLadderExpr(a))
				}
			case *ast.CallExpr:
				fun, ok := a.Fun.(*ast.Ident)
				if !ok {
					t.Errorf("%s: the size is `%s`, not a named exception this account knows",
						pos, fontLadderExpr(a))
					return true
				}
				if _, isException := account.exceptions[fun.Name]; !isException {
					t.Errorf("%s: the size comes from `%s`, which is neither a ladder rung nor an "+
						"exception with a written reason (see fontLadderExceptions)", pos, fun.Name)
				}
			case *ast.SelectorExpr:
				ident, ok := a.X.(*ast.Ident)
				if ok && ident.Name == "t" && a.Sel.Name == "FontSize" {
					return true // the surface's own body register
				}
				if !fontLadderRungs[a.Sel.Name] {
					t.Errorf("%s: the size is `%s`, which is not a rung of the text ladder (caption / body / "+
						"emphasis / title / display)", pos, fontLadderExpr(a))
				}
			case *ast.Ident:
				// A named local (`step := tokens.Caption`). Where it comes from
				// is judgement ①'s business, and ① sees the whole package.
			default:
				t.Errorf("%s: the size is `%s`, neither a ladder rung nor a named register",
					pos, fontLadderExpr(arg))
			}
			return true
		})
	}
	if seen < account.minCalls {
		t.Fatalf("only %d `FontSize(...)` calls found in %s: the package moved or was renamed, and this "+
			"account is now measuring nothing", seen, account.pkg)
	}
}

// fontLadderFiles lists one package's production files.
func fontLadderFiles(t *testing.T, pkg string) []string {
	t.Helper()
	all, err := filepath.Glob(filepath.Join(pkg, "*.go"))
	if err != nil {
		t.Fatalf("cannot list %s: %v", pkg, err)
	}
	out := make([]string, 0, len(all))
	for _, name := range all {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		t.Fatalf("%s holds no production files: the package moved or was renamed", pkg)
	}
	return out
}

// fontLadderExceptionSpans is where the named exceptions are allowed to close
// over an offset: their own function bodies, nowhere else.
func fontLadderExceptionSpans(fset *token.FileSet, files map[string]*ast.File, exceptions map[string]string) []span {
	var spans []span
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if _, isException := exceptions[fn.Name.Name]; !isException {
				continue
			}
			spans = append(spans, span{from: fset.Position(fn.Pos()).Offset, to: fset.Position(fn.End()).Offset})
		}
	}
	return spans
}

type span struct{ from, to int }

func fontLadderInsideSpan(fset *token.FileSet, pos token.Pos, spans []span) bool {
	at := fset.Position(pos).Offset
	for _, s := range spans {
		if at >= s.from && at < s.to {
			return true
		}
	}
	return false
}

// fontLadderOffsetsABodySize reports whether the expression is `<something>.FontSize ± …`:
// the one idiom this account exists to keep out of the package.
func fontLadderOffsetsABodySize(binary *ast.BinaryExpr) bool {
	if binary.Op != token.ADD && binary.Op != token.SUB {
		return false
	}
	found := false
	ast.Inspect(binary, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == "FontSize" {
			found = true
		}
		return !found
	})
	return found
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

// TestTheFontLadderExceptionsAreStillNamed is judgement ③'s companion: the
// exception list must keep naming helpers that still exist, or it is a list of
// reasons for code that is gone.
func TestTheFontLadderExceptionsAreStillNamed(t *testing.T) {
	for _, account := range fontLadderAccounts {
		for name, reason := range account.exceptions {
			if strings.TrimSpace(reason) == "" {
				t.Errorf("%s: the exception `%s` has no written reason", account.pkg, name)
			}
			found := false
			for _, file := range fontLadderFiles(t, account.pkg) {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatalf("cannot read %s: %v", file, err)
				}
				if strings.Contains(string(data), "func "+name+"(") ||
					strings.Contains(string(data), "func (a *App) "+name+"(") {
					found = true
				}
			}
			if !found {
				t.Errorf("the exception `%s` is no longer a function in %s: drop the exception (or rename it)",
					name, account.pkg)
			}
		}
	}
}
