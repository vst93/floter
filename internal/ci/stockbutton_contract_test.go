package ci

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// P13-ai · 控件一律由 theme token 画，框架的默认按钮面不是本仓的面。
//
// 框架的 `styleButton`（`mygo@v0.7.3/ui/widgets.go:152-186`）给的是「`Surface`
// 平涂 + 四边一圈同值的 `stroke`」，**没有受光边、没有底对侧、没有字重**：
// 实测（中灰 128 地面）浅色面 119 / 四条边全 108，深色面 141 / 四条边全 151。
// 四条边一个值就是 §2 判据要退掉的「靠画框分层」——盒子有边界，但边界不指向
// 任何方向，读不出「它在 z 上」。
//
// 这一族此前是全仓最后四枚（会话页页头与空态各一枚、自定义快捷键草稿行的
// 「绑定」、自绘驻留的「应用」），P13-ai 把它们收成 `settingsui.pageButton`
// （`Control` 三档 + `ControlEdge` + `LitFace`）之后，`internal/` 与 `cmd/` 的
// 非测试源码里**一枚都不剩**。这条账守的就是这个零：谁再写一枚框架默认按钮，
// 它就红，并且点名文件与行。
//
// 唯一的豁免是在那行行尾写 `// stock-button: <为什么这个面必须由框架画>`：
// 本仓从 P8 起就没有一个表面靠框架的默认面说话，所以豁免表现在是空的——
// 空表是这条账的取值，不是它的漏洞。
func TestNoControlWearsTheFrameworksStockButton(t *testing.T) {
	var offenders []string
	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			slashed := strings.TrimPrefix(filepath.ToSlash(path), "../../")
			for i, line := range strings.Split(string(data), "\n") {
				if !strings.Contains(line, "ui.Button(") && !strings.Contains(line, "ui.PrimaryButton(") {
					continue
				}
				if strings.Contains(line, "// stock-button:") {
					continue
				}
				offenders = append(offenders, slashed+":"+strconv.Itoa(i+1))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("the framework's stock button paints a four-sided frame with no lit line "+
			"(liquid-glass.md §2 / §6.8); these controls wear it:\n  %s\n"+
			"draw the face from the tokens instead (settingsui.pageButton, launcher.rowControl, "+
			"theme.Tokens.LitFace), or justify it on the line with `// stock-button: <why>`",
			strings.Join(offenders, "\n  "))
	}
}
