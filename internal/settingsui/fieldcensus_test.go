package settingsui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-cc · 字段材质只有一处决定。
//
// `fieldFace` 的注释从 P8-c 起就写着「它是字段材质唯一被决定的地方」，而三块字段面
// 在自己拼材质：`aliasField`（外圈 `Hairline`，唯一合法差）、`sheetFieldSlot` 与
// `configMultiselectBox`（外圈 `ControlEdge`，与 `fieldFace` 逐字节相同的两行）。
// 单行 grep 照不到它们（`Border`/`Background` 不在同一行），所以这三条 pin 都扫
// **函数体**：A 是路由，C 是那条合法差在像素上真的存在。

// fieldFaceOwners is the field family: every function that draws a surface a user
// types into. Each one must hand its face to `fieldFace`/`fieldFaceEdge`; none of
// them may name the lit/dim pair itself.
var fieldFaceOwners = []string{
	"textInputLit",         // 设置页的文本框（`extensionFieldLit` 由它转手）
	"configSelect",         // 设置页的下拉框
	"configMultiselectBox", // 设置页的多选箱
	"sheetFieldSlot",       // 弹层的字段槽
	"aliasField",           // 命令别名编辑器
}

// TestEveryFieldFaceIsDecidedInOnePlace pins the routing.
//
// Mutation: give `sheetFieldSlot` its own `Border(1, tokens.ControlEdge)` +
// `tokens.LitFace` again → the function stops calling `fieldFace` and goes red.
func TestEveryFieldFaceIsDecidedInOnePlace(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			bodies[fn.Name.Name] = string(src[fn.Body.Pos()-1 : fn.Body.End()-1])
		}
	}
	for _, name := range fieldFaceOwners {
		body, ok := bodies[name]
		if !ok {
			t.Fatalf("%q is gone — the field family changed shape, update fieldFaceOwners", name)
		}
		if !strings.Contains(body, "a.fieldFace(") && !strings.Contains(body, "a.fieldFaceEdge(") {
			t.Errorf("%s no longer hands its face to fieldFace/fieldFaceEdge: "+
				"a field's material is decided in one place (§6.46)", name)
		}
		for _, own := range []string{"tokens.LitFace(", "tokens.DimFace("} {
			if strings.Contains(body, own) {
				t.Errorf("%s paints %s itself; the field family's one material is fieldFaceEdge (§6.46)", name, own)
			}
		}
	}
	// The two material functions themselves, so the routing above cannot be
	// satisfied by an empty shell: the shell keeps the family's edge, the edge
	// variant keeps both arms.
	if !strings.Contains(bodies["fieldFace"], "a.fieldFaceEdge(") ||
		!strings.Contains(bodies["fieldFace"], ".ControlEdge") {
		t.Error("fieldFace no longer routes the family's own edge into fieldFaceEdge")
	}
	for _, own := range []string{"tokens.LitFace(", "tokens.DimFace("} {
		if !strings.Contains(bodies["fieldFaceEdge"], own) {
			t.Errorf("fieldFaceEdge lost %s — the material is gone, not moved", own)
		}
	}
}

// fieldFrameColumn renders one field on the probe ground and reads the middle of
// its left frame, i.e. the one line the `edge` parameter chooses.
func fieldFrameColumn(t *testing.T, dark bool, build func(c *ui.Context, a *App)) ui.Color {
	t.Helper()
	tt, ground := lensProbe(t, dark, 380, 140, build)
	img := tt.Image()
	b := paintedBox(img, ground)
	if b.W < 150 || b.H < 16 {
		t.Fatalf("the painted box %+v is not a field", b)
	}
	mid := int(b.Y + b.H/2)
	// The frame is the box's own first column: the probe's ground is the neutral
	// 128, the face is one tint off it, and the 1px edge is the column that is
	// not the face (measured: face 119, `Hairline` frame 108 on the light sheet).
	return lensMean(img, int(b.X), mid, int(b.X), mid)
}

// TestTheAliasFieldKeepsItsOwnFrame pins the one legal difference P13-cc keeps:
// the alias editor's frame is `Hairline` (0.09), the rest of the family's is
// `ControlEdge` (0.06 light / 0.07 dark) — the reference's
// `.extension-command-alias__input { border: 1px solid var(--hairline) }`.
//
// The pair below is deliberately read on **the same ground, at the same size**:
// if the edge parameter were dropped (or the alias silently took the family's
// edge), the two numbers would meet and this pin would go red — which is what
// stops a future material change from leaving the alias on the old rung.
//
// Mutation: pass `tokens.ControlEdge` from `aliasField` → the two frames meet.
func TestTheAliasFieldKeepsItsOwnFrame(t *testing.T) {
	for _, appearance := range appearanceNames {
		t.Run(appearance.name, func(t *testing.T) {
			alias := fieldFrameColumn(t, appearance.dark, func(c *ui.Context, a *App) {
				value := "k"
				a.extensionMetrics(func() { a.aliasField(c, &value).Width(240) })
			})
			field := fieldFrameColumn(t, appearance.dark, func(c *ui.Context, a *App) {
				value := "k"
				a.extensionMetrics(func() { a.extensionField(c, &value, configFieldMetrics).Width(240) })
			})
			gap := channelDelta(alias, field)
			t.Logf("%s: alias frame %v, sibling field frame %v, gap %d", appearance.name, alias, field, gap)
			if gap < 2 {
				t.Fatalf("%s: the alias frame %v and the sibling field's %v are the same line — "+
					"the Hairline parameter is gone (§6.46)", appearance.name, alias, field)
			}
		})
	}
}
