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

// P13-bp · 面级 census：一块「有 padding、有圆角、有面」的盘子，不许一条线都没有。
//
// 前三代材质账各有一个盲区，都是同一个形状：
//
//   - `settingsui/facecensus_test.go`（P13-p）的判据是「面 **+ 边**」，没有边框的面不进视野；
//   - `ci/litarm_contract_test.go`（P13-ab）只扫 `Hovered()` / `Pressed()` 臂——一个
//     根本没有指针臂的面（`shell/detach.go` 的 `detachedList` 行盘、不可选的列表）等于不存在。
//
// P13-aa 的「仍待」把这条写得很清楚：「census 还缺一条**反向**判据——『面』出现在
// `Hovered()` / `Pressed()` 臂里就必须有一对线。这一族现在只能靠人肉找。」
// P13-bp 在插件窗口的列表行盘上撞到同一个盲区（实测浅色 冠 / 面 / 底全 **229**、
// 深色全 **20**，两条线都不存在），于是这一轮把那条反向判据补上，并按「面」的
// 形状而不是指针状态来判。
//
// 判据：一个函数体里同时出现
//
//	① `Background(<面 token>)`——它画了一块面；且
//	② `.Padding(` 与 `.Radius(`——它是**盘子**（有内边距与圆角的块），不是一层洗色
//	   （行 hover 的 `SurfaceHover`、选中行整块的 `RowSelected` 都没有这一对）；
//
// 就必须在该函数体里出现至少一条线：
//
//	`LitFace(` / `DimFace(` / `.Lens(` / `LensInset(` / `InsetShadow(` / `Border(`
//
// 一个面**有一条属于自己的边**（哪怕是外圈发丝）就已经在 z 上站住；两条线都没有的
// 面就是 §2 判据要退掉的「一块颜色」。逃生口只有一条：`flatFaceAllow` 里写清理由，
// 条目**已经满足**了就报「过期」并变红（同 `litarm_contract_test.go` 的约定）。

// flatFaceAllow lists the panes that paint a face and deliberately wear no line at
// all. Each entry names the contract line; an empty map is the healthy reading.
var flatFaceAllow = map[string]string{
	"launcher.keycap": "docs/ui-redesign.md: shortcut legends use a flat translucent fill, without embossed edges",
}

func hasAny(s string, marks []string) bool {
	for _, m := range marks {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// sourceOf returns a node's own source text.
func sourceOf(fset *token.FileSet, path string, n ast.Node) string {
	src, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	start := fset.Position(n.Pos()).Offset
	end := fset.Position(n.End()).Offset
	if start < 0 || end > len(src) || start > end {
		return ""
	}
	return string(src[start:end])
}

// lineOf names where a function is declared.
func lineOf(fset *token.FileSet, path string, fn *ast.FuncDecl) string {
	rel := strings.TrimPrefix(filepath.ToSlash(path), "../")
	return rel + ":" + strconv.Itoa(fset.Position(fn.Pos()).Line)
}

// flatFaceTokens are the fills this design calls a face (or a groove).
var flatFaceTokens = []string{
	"Background(tokens.Control)",
	"Background(tokens.ControlHover)",
	"Background(tokens.ControlPress)",
	"Background(tokens.Field)",
	"Background(tokens.RaisedWarm)",
	"Background(tokens.Sunken)",
	"Background(plane)",
}

// flatLineMarks are the shapes that give a pane an edge: the E1 pair, its disabled
// arm, the retired four-layer lens, the groove's own pair, any inset line, or a
// plain border.
var flatLineMarks = []string{
	"LitFace(", "DimFace(", ".Lens(", "LensInset(", "InsetShadow(", "Border(",
}

// TestNoPaddedPaneIsFlat is the reverse census P13-aa asked for.
func TestNoPaddedPaneIsFlat(t *testing.T) {
	var offenders, stale []string
	seen := map[string]bool{}
	scanned := 0
	fset := token.NewFileSet()
	for _, root := range []string{"../launcher", "../settingsui", "../shell", "../terminalui", "../control"} {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("could not read %s: %v", root, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(root, name)
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				t.Fatalf("parse %s: %v", path, perr)
			}
			pkg := file.Name.Name
			for _, d := range file.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				body := sourceOf(fset, path, fn.Body)
				if !hasAny(body, flatFaceTokens) || !strings.Contains(body, "Padding(") || !strings.Contains(body, "Radius(") {
					continue
				}
				scanned++
				key := pkg + "." + fn.Name.Name
				if fn.Recv != nil {
					key = pkg + ".(*App)." + fn.Name.Name
				}
				seen[key] = true
				if hasAny(body, flatLineMarks) {
					continue
				}
				if _, ok := flatFaceAllow[key]; ok {
					continue
				}
				offenders = append(offenders, key+":"+lineOf(fset, path, fn))
			}
		}
	}
	for key := range flatFaceAllow {
		if !seen[key] {
			stale = append(stale, key)
		}
	}
	if scanned < 20 {
		t.Fatalf("the census only found %d padded faces — the scan is broken, not the package", scanned)
	}
	sort.Strings(offenders)
	sort.Strings(stale)
	for _, key := range offenders {
		t.Errorf("%s: this pane paints a face with padding and a corner but no edge at all — "+
			"it is a flat block of colour, not a surface on the z ladder (docs/liquid-glass.md §2: "+
			"a face wears a lit edge on top and a counter-side line below, theme.Tokens.LitFace). "+
			"Paint the pair, or add the entry to flatFaceAllow with the contract line.", key)
	}
	for _, key := range stale {
		t.Errorf("flatFaceAllow lists %q, which no longer paints a padded face — drop the stale entry", key)
	}
}
