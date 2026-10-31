package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// P13-ad · 指针的颜色答案只有一步。
//
// `docs/liquid-glass.md` §5 的表格第一行此前写着「颜色过渡（行、控件）90ms
// ease-out」，`docs/raycast-design.md:231` 更具体地写成「颜色过渡：
// `rowTransition` = 90ms ease-out（Colors only）」。**两句话都不是真的**：
//
//   - `rowTransition`（`internal/launcher/launcher.go`）是行的**入场**
//     （`Colors: false`、`Enter: {Opacity: 0}`），不是颜色过渡；
//   - 90ms 在 `internal/` 与 `cmd/` 里一次都没出现过；
//   - 实际是 11 处手写的 120ms 加 1 处 150ms（`settingsui/uninstall.go`
//     的组件行，照抄那个组件自己的 `transition: background 150ms ease`）。
//
// 于是同一个指针动作在应用里有三种速度，而对话框里那一族最慢——一张浮纸里的
// 行比它脚下的列表慢 25%。这一轮把 12 处收成 `theme.ColorStep`，这条账守两件事：
//
//	① 常量本身是参考 CSS 的 `--dur-2`，写死 120ms。**不能拿 ColorStep 判 ColorStep**
//	   （P13-ac 的第一次变异就是这么没红的：常量判常量，改常量时 pin 跟着改）。
//	② 任何 `Colors: true` 的过渡都只许写 `theme.ColorStep`，且**位置表过期即红**：
//	   删掉一条颜色过渡也是红，因为「这个面回答指针」是产品行为，不是可选项。
func TestTheColorStepIsTheReferenceDurTwo(t *testing.T) {
	src := repoFile(t, "internal/theme/theme.go")
	if !strings.Contains(src, "const ColorStep = 120 * time.Millisecond") {
		t.Errorf("theme.ColorStep is not the reference's `--dur-2` (120ms) spelled out; " +
			"if the pointer's colour step really moved, say so in liquid-glass.md §5 and here, " +
			"as an explicit re-baseline rather than a silent drift")
	}
}

// colorStepSites is the census: which file answers a pointer with colours, and
// how many faces it does it on. A file that gains or loses one is a red here
// until the table is updated — that is the point.
var colorStepSites = map[string]int{
	"internal/launcher/filters.go":       1, // the filter row's chips
	"internal/launcher/launcher.go":      2, // the row's 20u control, the field's settings key
	"internal/launcher/power.go":         1, // the power confirmation's rows
	"internal/settingsui/parameditor.go": 1, // the argument editor's stepper
	"internal/settingsui/rows.go":        3, // the two action pills and the bare note button
	"internal/settingsui/settingsui.go":  2, // the sidebar's row and the run pill
	"internal/settingsui/uninstall.go":   2, // the dialog's action pill and its component card
}

func TestEveryColorTransitionRidesOneStep(t *testing.T) {
	seen := map[string]int{}
	var offenders []string
	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The one definition lives here; its own line is the constant.
				if root == "internal" && d.Name() == "theme" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			slashed := filepath.ToSlash(path)
			slashed = strings.TrimPrefix(slashed, "../../")
			for i, line := range strings.Split(string(data), "\n") {
				if !strings.Contains(line, "Colors: true") {
					continue
				}
				seen[slashed]++
				if strings.Contains(line, "time.") {
					offenders = append(offenders,
						slashed+":"+itoa(i+1)+": a colour transition with its own duration — the pointer has one step (theme.ColorStep)")
					continue
				}
				if !strings.Contains(line, "theme.ColorStep") {
					offenders = append(offenders,
						slashed+":"+itoa(i+1)+": a colour transition that does not name theme.ColorStep")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	for _, o := range offenders {
		t.Error(o)
	}

	// The census has to match exactly: a missing entry means a face stopped
	// answering the pointer (or a new one appeared without a decision), and a
	// stale entry means this table is describing a file that no longer does.
	for file, want := range colorStepSites {
		if got := seen[file]; got != want {
			t.Errorf("%s: %d colour transitions, want %d — the table and the code disagree", file, got, want)
		}
	}
	for file, got := range seen {
		if _, ok := colorStepSites[file]; !ok {
			t.Errorf("%s: %d colour transitions and no line in the table — add one rather than letting it ride an unrecorded duration", file, got)
		}
	}
}

// itoa is strconv.Itoa without pulling the import into this file's header for
// one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
