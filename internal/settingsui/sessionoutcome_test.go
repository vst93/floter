package settingsui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"floter/internal/settings"
)

// P13-cm · 会话列表的状态词此前只有两档：运行中（`rowSegmentStrong`）与
// 「结束了」（`rowSegmentTertiary`）。可是「结束」不是一件事——程序以 0 退出
// 与以 3 退出在列表里**逐像素相同**，只差 `· exit 3` 那三个字符。
//
// 同一件事实在终端页已经分了档：`residentNote` 对 `ExitCode != 0` 用 `t.Warning`
// （`internal/terminalui/terminalui.go:1315-1318`）。P13-bg 也已经为捕获输出页
// 立过同一条判据（`docs/liquid-glass.md` §6.26：三种结局逐像素相同是缺陷，
// 色相是语义不是强调）。这一条把第三处并进同一档。
//
// 量的是**像素**，不是枚举：角色加了而画笔没接（M2）必须同样变红。
func TestTheFailedSessionEndsInTheWarningHue(t *testing.T) {
	for _, dark := range []bool{false, true} {
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		inks, grounds := sessionOutcomeInks(t, dark)
		running, clean, failed := inks["running"], inks["clean"], inks["failed"]
		cleanSize, failedSize := inks["cleanSize"], inks["failedSize"]
		ground := grounds["cleanSize"]
		t.Logf("%s: running %v | clean %v | failed %v | cleanSize %v | failedSize %v | ground %v",
			appearance, running, clean, failed, cleanSize, failedSize, ground)

		// ① 失败臂戴暖色相，干净臂与运行中臂是中性墨（§6.26 的同一个门槛：
		// 暖 = r−b ≥ 40，中性 = |r−b| ≤ 12）。
		if warm := int(failed.R) - int(failed.B); warm < 40 {
			t.Errorf("%s: the failed state word is not warm: %v (r−b = %d)", appearance, failed, warm)
		}
		if skew := int(clean.R) - int(clean.B); skew > 12 || skew < -12 {
			t.Errorf("%s: a clean exit took a hue: %v (r−b = %d)", appearance, clean, skew)
		}
		if skew := int(running.R) - int(running.B); skew > 12 || skew < -12 {
			t.Errorf("%s: a running session took a hue: %v (r−b = %d)", appearance, running, skew)
		}
		// ② 两个结局不许再逐像素相同。
		if colorClose(clean, failed, 12) {
			t.Errorf("%s: a clean exit and a failed one still render the same colour: %v vs %v",
				appearance, clean, failed)
		}
		// ③ 失败臂要比它自己的静息邻居（屏幕尺寸 / 开始时间）更离地：
		// 结局是这一行要读的东西，事实是背景。
		failedOff := colorDistance(failed, ground)
		quietOff := colorDistance(failedSize, ground)
		if failedOff <= quietOff {
			t.Errorf("%s: the failed ending does not stand off the ground more than its own meta text: "+
				"failed %v (%d) vs size %v (%d)", appearance, failed, failedOff, failedSize, quietOff)
		}
		// ④ 干净臂与它的邻居同档（P8-d 的那条关系不许被这一轮带跑）。
		if !colorClose(clean, cleanSize, 12) {
			t.Errorf("%s: a clean exit no longer joins its quiet meta text: %v vs %v",
				appearance, clean, cleanSize)
		}
	}
}

// 角色账：`sessionMeta` 是纯函数，这里钉住三选一，不经过画笔。
func TestTheSessionMetaChoosesItsRoleFromTheOutcome(t *testing.T) {
	clean := sessionMeta(Session{Note: "Exited", Size: "80×24"})
	failed := sessionMeta(Session{Note: "Exited · exit 3", Size: "80×24", Failed: true})
	running := sessionMeta(Session{Note: "Running · attached", Size: "80×24", Running: true})
	// 一个既 Running 又 Failed 的会话（不可能出现，但角色顺序要有答案）：
	// 运行中压过失败——程序还在跑，就没有「结局」可说。
	both := sessionMeta(Session{Note: "Running", Running: true, Failed: true})

	if got := clean[0].Role; got != rowSegmentTertiary {
		t.Errorf("clean exit role = %v, want the quiet grey", got)
	}
	if got := failed[0].Role; got != rowSegmentWarning {
		t.Errorf("failed exit role = %v, want the warning rung", got)
	}
	if got := running[0].Role; got != rowSegmentStrong {
		t.Errorf("running role = %v, want the strong neutral rung", got)
	}
	if got := both[0].Role; got != rowSegmentStrong {
		t.Errorf("a session that is both running and failed = %v, want running to win", got)
	}
}

// sessionOutcomeInks renders the sessions page with one row of each outcome and
// reads the ink of each state word and of its quiet neighbours.
func sessionOutcomeInks(t *testing.T, dark bool) (map[string]ui.Color, map[string]ui.Color) {
	t.Helper()
	store := settings.NewStore(settings.Default())
	if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
		t.Fatal(err)
	}
	a := New(store, Actions{})
	a.Sessions = func() []Session { return sessionOutcomeRows() }
	tt := renderOnPanelTheme(t, a, 720, 760)
	if err := tt.Click("Sessions"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	labels := map[string]string{
		"running":    "Running · attached",
		"clean":      "Exited",
		"failed":     "Exited · exit 3",
		"cleanSize":  "100×30",
		"failedSize": "120×40",
	}
	rects := map[string]ui.Rect{}
	for key, label := range labels {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing %q in %q", label, tt.Texts())
		}
		rects[key] = r
	}
	inks := map[string]ui.Color{}
	grounds := map[string]ui.Color{}
	for key, r := range rects {
		inks[key] = inkColor(tt, r)
		grounds[key], _ = modalColor(tt, r)
	}
	return inks, grounds
}

func sessionOutcomeRows() []Session {
	created := time.Date(2024, 1, 2, 15, 4, 0, 0, time.Local)
	return []Session{
		{ID: 1, Title: "zsh", Running: true, Active: true, Index: 0,
			Note: "Running · attached", Size: "80×24", Created: created},
		{ID: 2, Title: "build", Index: 1,
			Note: "Exited", Size: "100×30", Created: created},
		{ID: 3, Title: "deploy", Index: 2, Failed: true,
			Note: "Exited · exit 3", Size: "120×40", Created: created},
	}
}

// 出图（opt-in）：三行并排，深浅各一张。`FLOTER_P13CM_TAG=before` 时用来抓
// 改前那一帧（把失败臂退回 tertiary 再跑）。
func TestDumpP13CMSessionOutcome(t *testing.T) {
	if os.Getenv("FLOTER_P13CM_DUMP") == "" {
		t.Skip("set FLOTER_P13CM_DUMP=1 to write the round's visual evidence")
	}
	tag := os.Getenv("FLOTER_P13CM_TAG")
	if tag == "" {
		tag = "after"
	}
	for _, dark := range []bool{false, true} {
		store := settings.NewStore(settings.Default())
		if err := store.Update(func(s *settings.Settings) { s.Theme = appearanceName(dark) }); err != nil {
			t.Fatal(err)
		}
		a := New(store, Actions{})
		a.Sessions = func() []Session { return sessionOutcomeRows() }
		tt := renderOnPanelTheme(t, a, 720, 760)
		if err := tt.Click("Sessions"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		appearance := "light"
		if dark {
			appearance = "dark"
		}
		name := filepath.Join(os.TempDir(),
			fmt.Sprintf("floter-P13-cm-sessionoutcome-%s-%s.png", appearance, tag))
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

// renderOnPanelTheme is `render` with the panel's own theme on the context. The
// sessions page paints a failed ending in `c.Theme().Warning`, so a frame whose
// context still wears the framework's stock theme would measure the stock
// warning (`#d97706` / `#f59e0b`) instead of this design's (`#9C1F18` / a pale
// peach).
func renderOnPanelTheme(t *testing.T, a *App, w, h int) *ui.Tester {
	t.Helper()
	skipWithoutTheReferenceFont(t)
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.tokens(c).Theme)
		a.View(c)
	}, w, h)
	tt.Frame()
	return tt
}
