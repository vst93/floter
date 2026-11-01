package launcher

import (
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// P13-cd · 同一张卡的两个空态，说的必须是同一档话。
//
// 启动器卡片里有两个页面可以「什么都没有」：结果区（查不到 / 插件模式没内容）
// 与捕获输出页（命令写了个空）。P13-cc 之后这一族只剩捕获输出页那一半没量：
// 它此前把那句话画成**正文**——12px `t.Text`、`Selectable()`、贴在正文第一行
// （实测墨盒 `{X:12 Y:123 W:188 H:16}`，720×488 帧），与页头那行命令行同档墨，
// 与「命令真的打印了这句话」在界面上无法区分。
//
// 两条账，一条像素一条结构：
//   - 像素：这句话的**墨**必须居中在那条正文带里（不是贴着左内边距、不是落在
//     第一行），深浅两套各量一次。
//   - 结构：这句话只有**一处**决定（`App.quietStatement`），两个空态都调它；
//     本包其余函数不许再自己拼 `Emphasis` + `TextStrong` 这一对。

// inkBox is the bounding box of the pixels that differ from the face by at
// least 40 levels — the glyphs, not the line box the layout handed out. A
// left-aligned sentence and a centred one have the *same* box here (both fill
// the width), so the box says nothing; the ink does.
func inkBox(img *image.RGBA, rect ui.Rect) (ui.Rect, int) {
	face := lumaAt(img, int(rect.X)+2, int(rect.Y)+2)
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	n := 0
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			d := lumaAt(img, x, y) - face
			if d < 0 {
				d = -d
			}
			if d < 40 {
				continue
			}
			n++
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	return ui.Rect{X: float32(minX), Y: float32(minY), W: float32(maxX - minX + 1), H: float32(maxY - minY + 1)}, n
}

// lastInkRow is the lowest row above `before` that carries ink anywhere in the
// frame — the page strip's own last line (its status sentence). The band the
// empty state is centred in starts below it.
func lastInkRow(img *image.RGBA, before int) int {
	face := lumaAt(img, 6, before/2)
	last := -1
	for y := 0; y < before; y++ {
		for x := 0; x < img.Bounds().Max.X; x++ {
			d := lumaAt(img, x, y) - face
			if d < 0 {
				d = -d
			}
			if d >= 40 {
				last = y
				break
			}
		}
	}
	return last
}

func TestTheCapturedOutputsEmptyPageIsAStatement(t *testing.T) {
	for _, dark := range []bool{false, true} {
		dark := dark
		name := "light"
		if dark {
			name = "dark"
		}
		t.Run(name, func(t *testing.T) {
			a := testApp()
			tt := runListOutput(t, a, "")
			tt.SetDark(dark)
			tt.Frame()
			if a.output == nil || !a.output.Empty {
				t.Fatalf("the empty run did not take the empty page: %+v", a.output)
			}
			rect, ok := tt.Find(a.output.Text)
			if !ok {
				t.Fatalf("the empty state's sentence is missing: %q", tt.Texts())
			}
			img := tt.Image()
			box, n := inkBox(img, rect)
			if n == 0 {
				t.Fatalf("the sentence carries no ink in its own box %+v", rect)
			}
			bounds := img.Bounds()
			w, h := float32(bounds.Max.X), float32(bounds.Max.Y)

			// ① 水平：墨盒的中线落在帧心上（结果区那一句是 360.5 对 360）。
			centerX := box.X + box.W/2
			if got, want := centerX, w/2; got < want-3 || got > want+3 {
				t.Errorf("the empty state is not centred: ink centre x=%v, want %v (ink box %+v)", got, want, box)
			}
			// ② 它真的离开了正文的左内边距（改前墨盒左沿 = 12）。
			if box.X <= w/3 {
				t.Errorf("the empty state still sits on the body's left inset: ink x=%v of a %v frame", box.X, w)
			}
			// ③ 竖直：不在正文第一行上（改前墨盒顶 = 123，帧高 488）。
			if box.Y <= h*0.30 {
				t.Errorf("the empty state still rides the body's first row: ink y=%v of a %v frame", box.Y, h)
			}
			// ④ 竖直：居中在「页头条子下沿 → 面板底内边距」这条带里。
			top := float32(lastInkRow(img, int(box.Y)))
			band := (top + (h - 4)) / 2 // 4px = the body's bottom inset (`t.Space(1)`)
			centerY := box.Y + box.H/2
			if centerY < band-12 || centerY > band+12 {
				t.Errorf("the empty state is centred at y=%v, want %v (band %v..%v)", centerY, band, top, h-4)
			}
			// ⑤ 寄存器：这句话是 `Emphasis` 那一阶，不是正文那一阶。行盒的高度
			// 就是字号的行度量：`Emphasis` 18px，正文 12px 是 22px（改前实测），
			// 两者相差 4px，阈值取中间。
			if rect.H > 20 {
				t.Errorf("the empty state wears the body's size: its line box is %vpx tall, want the emphasis step's 18", rect.H)
			}
		})
	}
}

// The results page's own empty state is the same grammar, and it is the one the
// shared function was extracted from: pin it so the extraction cannot move the
// page it came from.
func TestTheResultsPageStillCentresItsEmptyState(t *testing.T) {
	a := testApp()
	a.Query = "zzzz"
	tt := render(t, a)
	rect, ok := tt.Find("No results")
	if !ok {
		t.Fatalf("the results page's empty message is missing: %q", tt.Texts())
	}
	box, n := inkBox(tt.Image(), rect)
	if n == 0 {
		t.Fatalf("the empty page's sentence carries no ink at %+v", rect)
	}
	if got, want := box.X+box.W/2, float32(tt.Image().Bounds().Max.X)/2; got < want-3 || got > want+3 {
		t.Errorf("the results page's empty state moved off centre: ink centre x=%v, want %v", got, want)
	}
}

// The census: one function decides what an empty sentence looks like, and both
// pages of this card call it. A third grammar (or a re-inlined pair) is what
// P13-cd exists to prevent.
func TestTheLauncherCardsEmptyStatesComeFromOneFunction(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	callers := map[string]int{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			emphasis, strong, calls := false, false, 0
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "quietStatement":
					calls++
				case "FontSize":
					emphasis = emphasis || selectorArg(call, "Emphasis")
				case "TextColor":
					strong = strong || selectorArg(call, "TextStrong")
				}
				return true
			})
			if calls > 0 {
				callers[name]++
			}
			if emphasis && strong && fn.Name.Name != "quietStatement" {
				t.Errorf("%s: %s builds its own Emphasis + TextStrong sentence; "+
					"an empty state has one decision point (`App.quietStatement`, §6.47)",
					name, fn.Name.Name)
			}
		}
	}
	if callers["launcher.go"] != 1 || callers["output.go"] != 1 || len(callers) != 2 {
		t.Errorf("the empty-state sentence is called from %v, want exactly launcher.go and output.go", callers)
	}
}

// selectorArg reports whether one of the call's arguments is `X.<name>`.
func selectorArg(call *ast.CallExpr, name string) bool {
	for _, arg := range call.Args {
		sel, ok := arg.(*ast.SelectorExpr)
		if ok && sel.Sel.Name == name {
			return true
		}
	}
	return false
}

// P13-cd visual evidence: the captured-output page's empty state in both
// appearances. Opt-in (`FLOTER_PNG=1`) for the same reason as the other dumps.
// `FLOTER_PNG_TAG` names the file's suffix, so the round can dump the same
// frame before and after the change from one test.
func TestDumpP13CDEmptyOutputPNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_PNG_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		name := "light"
		if dark {
			name = "dark"
		}
		a := testApp()
		tt := runListOutput(t, a, "")
		tt.SetDark(dark)
		tt.Frame()
		path := filepath.Join("/tmp", "floter-P13-cd-empty-output-"+name+"-"+tag+".png")
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
		if err := png.Encode(file, tt.Image()); err != nil {
			t.Fatalf("encode %s: %v", path, err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
	}
}
