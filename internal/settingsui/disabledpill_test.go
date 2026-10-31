package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-ba · 行忙时那簇键的灯。
//
// `extensionActionFace` 的 `disabled` 臂（P13-s）早就把契约写死了：禁用面「底对侧
// 还在、受光边熄了」，因为受光边是「这块面可以被碰」的记号（`docs/liquid-glass.md`
// §3）。但 `actionButton` → `extensionActionButton` 把那一格**写死成 `false`**，
// 于是扩展页行忙时被锁住的整簇键（导出 / 导入 / Run / 复检 / 检测到的工具行）
// 全是「亮着灯但按不动」的面：它们只被框架的 0.5 淡了一道，冠线照旧在。
//
// 三条判据就是这一族的三条 pin：面还有台阶、顶内线不亮、底对侧还在（复用
// `assertDisabledFace`），加上「禁用后指针不再是手型」。

// rowLockedPill is the product's own draw call for a ring-locked action pill —
// the one `rowAction` / `repairAction` / the Run button take when the row is
// busy. It is called through the product, not rebuilt in the test, so a call
// site that drifts back to `actionButton(...).Disabled(true)` turns this red.
func rowLockedPill(a *App, c *ui.Context) ui.Element {
	return a.disabledActionButton(c, "Check")
}

func TestTheRowLockedActionPillTurnsItsCrownOff(t *testing.T) {
	for _, dark := range []bool{false, true} {
		box, r := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return rowLockedPill(a, c)
		})
		assertDisabledFace(t, "the row-locked action pill", dark, box, r)
	}
}

// census: no draw call in this package may build an action pill and then
// disable it after the fact — the face already has a disabled arm, and
// `.Disabled(...)` on a built pill leaves the lit crown on. This is the pin
// that names the file and line when a call site drifts back.
func TestNoActionPillIsDisabledAfterItIsBuilt(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(src), "\n")
		for i, line := range lines {
			if strings.Contains(line, "//") || !strings.Contains(line, "actionButton(c") {
				continue
			}
			if strings.Contains(line, "disabledActionButton") {
				continue
			}
			// The disable may land a couple of lines later (`button := …` then
			// `if busy { button = button.Disabled(true) }`), so the window is
			// the call line plus the next three.
			for j := i; j < len(lines) && j <= i+3; j++ {
				if strings.Contains(lines[j], ".Disabled(") {
					t.Errorf("%s:%d disables an action pill after building it (%s); take `disabledActionButton` instead — `actionButton` draws the lit face",
						name, j+1, strings.TrimSpace(lines[j]))
					break
				}
			}
		}
	}
}

// The opt-in dump for this round: the row-locked pill over its enabled twin,
// both appearances, with the four readings printed. Run with
// FLOTER_P13BA_TAG=before while `disabledActionButton` is re-injected with the
// old body (`actionButton(...).Disabled(true)`) to get the comparison pair.
func TestDumpP13BARowLockedPill(t *testing.T) {
	if os.Getenv("FLOTER_P13BA_DUMP") == "" {
		t.Skip("set FLOTER_P13BA_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13BA_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		_, locked := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return rowLockedPill(a, c)
		})
		_, live := dimProbe(t, dark, func(a *App, c *ui.Context) ui.Element {
			return a.actionButton(c, "Check")
		})
		t.Logf("%s (%s): locked ground %v face %v crown %v shade %v inkContrast %d | enabled face %v crown %v",
			appearanceName(dark), tag, locked.ground, locked.face, locked.crown, locked.shade, locked.inkContrast,
			live.face, live.crown)
	}
	for _, dark := range []bool{false, true} {
		ground := realPageGround(t, dark)
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		tt := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(a.tokens(c).Theme)
			c.Root().Background(ground)
			ui.Column(c).Padding(20).Gap(8).Width(360).Children(func() {
				wrapControl(c, func() { rowLockedPill(a, c) })
				wrapControl(c, func() { a.actionButton(c, "Check") })
				wrapControl(c, func() { a.busyActionButton(c, "Check", "dump") })
			})
		}, 400, 220)
		tt.SetDark(dark)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		tt.Frame()
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		name := filepath.Join(os.TempDir(), fmt.Sprintf("floter-P13-ba-locked-pill-%s-%s.png", appearance, tag))
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", name)
	}
}
