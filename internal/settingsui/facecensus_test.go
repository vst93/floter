package settingsui

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

// faceCensusAllow is the list of functions that draw a control face and an edge
// without the E1 pair, each with the reason it deliberately has none. Every
// entry has to name the contract line that says so: a face with no pair is a
// decision, never an oversight.
//
// P13-p: this census exists because the previous three sweeps (§6.2b 9-1…9-10
// in P13-i, 9-13/9-14 in P13-l, 9-15…9-17 in P13-m) were all greps — first on
// `tokens.Lens(`, then on `Border(1,` and `tokens.Control)` *on the same line*.
// Three faces were invisible to both: `sessionIconButton`'s hover rung never had
// a `Lens` to grep for, and the other two write their `Border`/`Background` on
// separate lines. Scanning a whole function body for the material, rather than
// a line for a call, is the only query that cannot miss them.
var faceCensusAllow = map[string]string{
	// 26u 的会话方格（`rows.go` `sessionChip`）与 30u 的包方格（`packageChip`）
	// 是同一类：一块只有 26~30u 的方块上再画两条 1px 线就是噪点，不是层级
	// （§6.2 第 2-9 行、§6.4 第 4-2 行）。
	"packageChip": "§6.4 第 4-2 行：30u 的包方格同一条理由",
	// 4px 高（padding 1u 上下）的权限药丸，同 2-9。
	"approvalPermissionPills": "§6.2 第 2-9 行：4px 高的药丸加一条线就成噪点",
	// 两条警告面：那一行已经被警告色占住，第二个面只说一句话（§6.4 第 4-4 行）。
	"healthFailureCard":   "§6.4 第 4-4 行：警告内环占住 padding 第一行，再叠受光边会把环洗淡",
	"approvalStaleBanner": "§6.4 第 4-4 行：警告前导条是它唯一的身份线",
	// 录制中的两处：身份是那圈 accent 边（「正在录」），同 §6.2b 9-11 的
	// 分段选中段——一个面只说一句话。
	"customCapturePane": "§6.2b 第 9-11/9-14b 行：accent 环就是它的身份线",
	"shortcutControl":   "§6.2b 第 9-11/9-14b 行：accent 环就是它的身份线",
	// 14px 的勾选框：同 2-9，一条 1px 线在 14u 的方块里就是噪点。
	"paramRequiredToggle": "§6.2 第 2-9 行：14u 的勾选框加一条线就成噪点",
	// 警告色的内环/药丸：环已经占了 padding 第一行（`InsetShadow(0,0,0,1,…)`），
	// 再叠受光边会把那一行洗淡（§6.2b 第 9-14b 行、§6.4 第 4-4 行）。
	"noticeLine":              "§6.2b 第 9-14b 行：警告内环占住 padding 第一行",
	"stateMark":               "§6.4 第 4-4 行：破损 chip 的环就是它的身份线",
	"runtimeUnavailableBadge": "§6.4 第 4-4 行：同上，同一个破损 chip 的第二种读法",
	// 4px 高的状态药丸，同 2-9。
	"healthStatusChip": "§6.2 第 2-9 行：4px 高的药丸加一条线就成噪点",
}

// faceMaterial reports whether a function body draws a control face with an
// edge, and whether it also paints the E1 pair (or any other 1px inner line,
// which is the same statement in this codebase's vocabulary).
func faceMaterial(body string) (face, edge, pair bool) {
	face = strings.Contains(body, "Background(tokens.Control)") ||
		strings.Contains(body, "Background(tokens.ControlHover)") ||
		strings.Contains(body, "Background(tokens.ControlPress)") ||
		strings.Contains(body, "Background(tokens.Field)") ||
		strings.Contains(body, "Background(tokens.RaisedWarm)") ||
		strings.Contains(body, "Background(plane)")
	edge = strings.Contains(body, "Border(1") || strings.Contains(body, "InsetShadow(")
	// The pair is the E1 pair, or the four-layer lens's own first line, or the
	// groove's `LensInset` — never a bare `InsetShadow`: a warning ring is an
	// inset line too, and counting it would let a face answer with the ring
	// that §6.4 4-4 says must stay the only thing on that row.
	pair = strings.Contains(body, "LitFace(") || strings.Contains(body, ".Lens(") ||
		strings.Contains(body, "LensInset(") || strings.Contains(body, "InsetShadow(0, 1, 0, 0")
	return face, edge, pair
}

// TestEveryFaceWithAnEdgeWearsTheLitPair is the material census: a function that
// draws a face and an edge must paint the pair, or be listed with a reason.
func TestEveryFaceWithAnEdgeWearsTheLitPair(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	seen := map[string]bool{}
	scanned := 0
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
			if !ok || fn.Body == nil || fn.Recv == nil {
				continue
			}
			body := string(src[fn.Body.Pos()-1 : fn.Body.End()-1])
			face, edge, pair := faceMaterial(body)
			if !face || !edge {
				continue
			}
			scanned++
			name := fn.Name.Name
			seen[name] = true
			reason, allowed := faceCensusAllow[name]
			switch {
			case allowed && pair:
				t.Errorf("%s.%s now paints the E1 pair (%s); the census entry is stale — drop it and cite the pixel pin instead",
					path, name, reason)
			case !allowed && !pair:
				t.Errorf("%s.%s draws a control face and a ControlEdge border but no 1px inner line: "+
					"paint `tokens.LitFace` (E1) or add a reason to faceCensusAllow (%s §2)",
					path, name, "docs/liquid-glass.md")
			}
		}
	}
	if scanned < 15 {
		t.Fatalf("the census only found %d faces — the scan is broken, not the package", scanned)
	}
	for name := range faceCensusAllow {
		if !seen[name] {
			t.Errorf("faceCensusAllow lists %q, which no longer draws a face+edge — drop the stale entry", name)
		}
	}
}
