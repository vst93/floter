package ci

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// P11-an · 浮动层的投影只有一个读法：`theme.ElevThrow` / `theme.ElevContact`。
//
// `--elev-shadow-scale` 是 `--main-opacity` 的函数（`main:src/styles/base.css:178`），
// 所以同一条 `--elev-3` 在 100% 的框上是 0.135、在 10% 的框上才是 0.405。四个
// 浮动层（启动器的警告状态行、设置页的卸载对话框、终端页的退出便条与失败提示）
// 曾把 scale = 1 那一档的字面值（`ui.RGBA(0, 0, 0, 0.3)` / `0.2`）抄进 `Shadow`，
// 于是透明度滑块只动填充、不动投影——而其中两处的注释还写着「its 0.2 is
// scaled here」，那句话描述了一个从未实现的耦合。
//
// 这条扫描守着「不许再来一次」：三个画字面的包里，`Shadow(` 的 alpha 必须来自
// token 或 `theme.Elev*`，不能再是裸的小数字面量。
func TestFloatingThrowsFollowTheFrameScale(t *testing.T) {
	literal := regexp.MustCompile(`Shadow\(.*ui\.RGBA\(0,\s*0,\s*0,\s*0\.[0-9]`)
	pkgs := []string{"../launcher", "../settingsui", "../terminalui"}
	checked := 0
	for _, pkg := range pkgs {
		entries, err := os.ReadDir(pkg)
		if err != nil {
			t.Fatalf("could not read %s: %v", pkg, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(pkg, name))
			if err != nil {
				t.Fatalf("could not read %s: %v", filepath.Join(pkg, name), err)
			}
			checked++
			for i, line := range strings.Split(string(data), "\n") {
				// A comment may quote the reference's own CSS — that is where the
				// token came from, not a painted value.
				if strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				if loc := literal.FindString(line); loc != "" {
					t.Errorf("%s/%s:%d casts a shadow at a literal alpha (%s); "+
						"paint it with theme.ElevThrow / theme.ElevContact so it follows the "+
						"frame's own transparency", pkg, name, i+1, loc)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("the guard read no source files")
	}
}
