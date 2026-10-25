package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"floter/internal/i18n"
	"floter/internal/settingsui"
)

// P9-o：完成与失败必须落在**屏幕上那个表面**自己的通知里。
//
// 病灶（改前）：notifyCompletion（shell.go:3246）与设置页五个动作的失败分支
// （installPackageFolder 两处、connectLocalTool 两处）都把句子交给
// a.Launcher.Feedback / WarnFeedback。可那个 docked 行只在启动器视图里画
// （shell.go 的 surface() 只画当前表面，launcher.go:1464 才调 feedbackRow），
// 所以：
//   - 用户在设置页点「连接」，失败时屏幕上一个字都不出现；
//   - 那句警告被存进启动器的 a.toast，用户之后回到启动器时它以 3 秒时效
//     突然弹出来——一条已经过时的、指向别处的句子。
// 深链路径（refuseLink → queueNotice）早已是可见的 toast，两条路径因此不一致。

// 设置页上的完成不再静默：句子要真的出现在这一帧里。
func TestASettingsCompletionSpeaksOnTheSettingsPage(t *testing.T) {
	a := newApp(t)
	a.Open(SurfaceSettings)
	a.onScreen = func() bool { return true }
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 900, 620)
	tt.Frame()

	a.notifyCompletion(func(i18n.Notifications) string { return "the tool is connected" })
	tt.Frame()
	if !tt.HasText("the tool is connected") {
		t.Fatalf("the completion was silent on the settings page: %q", tt.Texts())
	}
}

// 设置页上的失败不再静默：UI 的连接路径与深链路径读到同一句话。
func TestASettingsFailureSpeaksOnTheSettingsPage(t *testing.T) {
	a := newApp(t)
	a.Open(SurfaceSettings)
	a.onScreen = func() bool { return true }
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 900, 620)
	tt.Frame()

	want := i18n.For("en").Settings.LocalToolFailed
	// An empty tool cannot be normalized, so PrepareCustom fails before anything
	// is written — the same failure branch the settings page's Connect button
	// reaches.
	a.connectLocalTool(settingsui.CustomTool{})
	deadline := 0
	for !tt.HasText(want) && deadline < 200 {
		tt.Frame()
		deadline++
	}
	if !tt.HasText(want) {
		t.Fatalf("the failure was silent on the settings page: want %q, got %q", want, tt.Texts())
	}
	// 旧行为：句子被塞进启动器的 docked 行，等用户回到启动器才弹。
	if a.Launcher.FeedbackVisible() {
		t.Error("the settings failure was parked on the launcher's row instead of said where the user is")
	}
}

// 反向 pin：启动器上的完成仍走它自己的 docked 行，不额外多一条 toast。
// 这是 notifyCompletion 原有契约（「屏幕上的窗口用自己的面板回答」），
// 修设置页不能把它改坏。
func TestALauncherCompletionKeepsTheDockedRow(t *testing.T) {
	a := newApp(t)
	a.onScreen = func() bool { return true }
	tt := ui.NewTester(func(c *ui.Context) { a.View(c) }, 720, 520)
	tt.Frame()

	a.notifyCompletion(func(i18n.Notifications) string { return "the tool is connected" })
	if !a.Launcher.FeedbackVisible() {
		t.Fatal("the launcher's own row said nothing")
	}
	if got := pendingNotices(a); got != 0 {
		t.Errorf("a launcher completion queued %d toasts, want 0", got)
	}
	tt.Frame()
	if got := countTexts(tt.Texts(), "the tool is connected"); got != 1 {
		t.Errorf("the completion appears %d times, want 1 (%q)", got, tt.Texts())
	}
}
