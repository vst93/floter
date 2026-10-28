package launcher

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/browser"
	"floter/internal/calculator"
	clipboardpkg "floter/internal/clipboard"
)

// P11-aw · **插件模式页也不能有两段字压在同一块像素上，也不能有字跑出窗口。**
//
// P11-aj 把「一段面上没有两段字压在同一块像素上」这条不变量落在启动器的三个普通页
// （空页 / 有匹配 / 零匹配）。那三个页共用一张卡、一条列表、一个页脚。**插件模式页**
// （剪贴板、计算器、浏览器）不共用它们：它们各自带一条筛选 chips 带、一个 scope mark
// 顶替放大镜、一条不一样的列表（剪贴板行的缩略图、计算器的两列、浏览器的 tab/书签
// 分组），并且没有页脚。P11-aj 的审计从没走过这几页——`TestTheLauncherPaintsNoTextOverText`
// 的夹具只 type 普通查询。
//
// 这条审计把同一把尺子（`auditLauncherFrame`）量到三张模式页上：两套外观 × 两个窗口
// 尺寸（默认卡，和窗口能缩到的最小高度），每条模式一个「长到会被省略号截断」的标题
// 在夹具里（这正是文字与行尾时间戳/键帽相撞的地方），外加每个模式的空态。
func TestThePluginModesPaintNoTextOverText(t *testing.T) {
	sizes := [][2]int{
		{InputWindowWidth, frameHeight(maxResultSlots)},
		{InputWindowWidth, 360}, // settings.MinTerminalHeight, the shortest the window opens
	}
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		for _, size := range sizes {
			tag := fmt.Sprintf("%s %dx%d", appearance, size[0], size[1])
			t.Run(tag+"/clipboard", func(t *testing.T) {
				a := clipboardModeApp(t)
				tt := renderAt(t, a, size[0], size[1], dark)
				auditLauncherFrame(t, tag+" clipboard", tt)
			})
			t.Run(tag+"/clipboard-empty", func(t *testing.T) {
				a := testApp()
				a.Clipboard = clipboardpkg.NewStore(clipboardpkg.FromConfigRoot(t.TempDir()), clipboardpkg.MaxMaxItems)
				a.EnterClipboard()
				tt := renderAt(t, a, size[0], size[1], dark)
				auditLauncherFrame(t, tag+" clipboard empty", tt)
			})
			t.Run(tag+"/calculator", func(t *testing.T) {
				a := calculatorModeApp(t)
				tt := renderAt(t, a, size[0], size[1], dark)
				auditLauncherFrame(t, tag+" calculator", tt)
			})
			t.Run(tag+"/calculator-empty", func(t *testing.T) {
				a := testApp()
				a.Calculator = &calculatorFixture{}
				a.RefreshCalculator()
				a.EnterCalculator()
				tt := renderAt(t, a, size[0], size[1], dark)
				auditLauncherFrame(t, tag+" calculator empty", tt)
			})
			t.Run(tag+"/browser", func(t *testing.T) {
				a := browserModeApp(t)
				tt := renderAt(t, a, size[0], size[1], dark)
				auditLauncherFrame(t, tag+" browser", tt)
			})
		}
	}
}

// clipboardApp is a clipboard mode holding a handful of entries, one long
// enough that its row ellipsizes before the trailing timestamp.
func clipboardModeApp(t *testing.T) *App {
	t.Helper()
	a := testApp()
	store := clipboardpkg.NewStore(clipboardpkg.FromConfigRoot(t.TempDir()), clipboardpkg.MaxMaxItems)
	for i := 0; i < 6; i++ {
		if _, _, err := store.AddText(fmt.Sprintf(
			"clip %02d — a line long enough that the row's own ellipsis has to cut it before the timestamp", i)); err != nil {
			t.Fatal(err)
		}
	}
	a.Clipboard = store
	a.EnterClipboard()
	return a
}

// calculatorApp is a calculator mode holding a handful of history entries with
// a long expression and a wide result.
func calculatorModeApp(t *testing.T) *App {
	t.Helper()
	a := testApp()
	entries := make([]calculator.Entry, 0, 6)
	for i := 0; i < 6; i++ {
		entries = append(entries, calculator.Entry{
			ID:         fmt.Sprintf("e%d", i),
			Expression: fmt.Sprintf("(%d + %d) * (%d - %d) / %d", i, i+1, i+2, i+3, i+4),
			Result:     fmt.Sprintf("%d.%04d", i*7, i),
			CreatedAt:  int64(100 - i),
		})
	}
	a.Calculator = &calculatorFixture{entries: entries}
	a.RefreshCalculator()
	a.EnterCalculator()
	return a
}

// browserApp is a browser mode with a history entry, a bookmark and an open
// tab, all with long titles.
func browserModeApp(t *testing.T) *App {
	t.Helper()
	a := testApp()
	found := []browser.Result{
		{URL: "https://go.dev/doc/effective_go", Title: "Effective Go — a documentation page whose title is long enough to be ellipsized in its row", Kind: "history", BrowserID: "chrome", Browser: "Chrome", Visited: time.Unix(1_700_000_000, 0)},
		{URL: "https://rust-lang.org/learn", Title: "Rust — Learn", Kind: "bookmark", BrowserID: "chrome", Browser: "Chrome"},
	}
	tabs := []browser.Tab{
		{BrowserID: "chrome", Index: 0, Title: "Rust Playground — an open tab with a long title too", URL: "https://play.rust-lang.org"},
	}
	a.Actions.SearchBrowser = func(query string, done func(BrowserResults)) {
		done(BrowserResults{Found: true, Results: found, Tabs: tabs})
	}
	a.EnterBrowser()
	return a
}

// renderAt draws one frame at a given size and appearance. The mode's first
// frame asks the shell for its rows; the caller's fixture answers synchronously,
// so a second frame draws them.
func renderAt(t *testing.T, a *App, w, h int, dark bool) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, w, h)
	tt.SetDark(dark)
	tt.Frame()
	tt.Frame()
	return tt
}

// The audit is only meaningful if each mode actually draws a list; a mode that
// silently renders empty would pass the overlap check for the wrong reason.
func TestThePluginModesAuditDrawsRealRows(t *testing.T) {
	a := clipboardModeApp(t)
	tt := renderAt(t, a, InputWindowWidth, frameHeight(maxResultSlots), false)
	if !tt.HasText("clip 00") {
		t.Fatalf("the clipboard mode drew no row: %q", tt.Texts())
	}
	c := calculatorModeApp(t)
	ct := renderAt(t, c, InputWindowWidth, frameHeight(maxResultSlots), false)
	if len(ct.Texts()) == 0 {
		t.Fatal("the calculator mode drew nothing")
	}
	b := browserModeApp(t)
	bt := renderAt(t, b, InputWindowWidth, frameHeight(maxResultSlots), false)
	if !bt.HasText("Rust — Learn") && !bt.HasText("Rust Playground — an open tab with a long title too") {
		t.Fatalf("the browser mode drew no row: %q", bt.Texts())
	}
}

// TestDumpPluginModePNGs is the round's visual evidence: the three plugin mode
// pages in both appearances, written to /tmp. Opt-in, as the other dumps are.
func TestDumpPluginModePNGs(t *testing.T) {
	if os.Getenv("FLOTER_PNG") == "" {
		t.Skip("set FLOTER_PNG=1 to write the round's visual evidence")
	}
	builders := map[string]func(*testing.T) *App{
		"clipboard":  clipboardModeApp,
		"calculator": calculatorModeApp,
		"browser":    browserModeApp,
	}
	for _, mode := range []string{"clipboard", "calculator", "browser"} {
		for _, dark := range []bool{false, true} {
			appearance := "light"
			if dark {
				appearance = "dark"
			}
			a := builders[mode](t)
			tt := renderAt(t, a, InputWindowWidth, frameHeight(maxResultSlots), dark)
			img := tt.Image()
			if img == nil {
				t.Fatalf("%s/%s: no frame", mode, appearance)
			}
			path := filepath.Join("/tmp", "floter-P11-aw-"+mode+"-"+appearance+".png")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, img); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", path)
		}
	}
}
