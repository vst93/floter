package ci

import (
	"strings"
	"testing"
)

// P13-cm · 会话列表的「失败」臂是一条**接线**，不是一条样式：设置页只能画
// `settingsui.Session.Failed` 告诉它的事，而这一位由 shell 在拼列表时从终端页
// 取。它一旦没人传，字段恒为假，暖色那一档就静默消失——像素 pin 全绿，功能已经
// 没了。所以这一条钉住接线本身。
//
// 形状照 P13-a 的结构账（`termsession_contract_test.go`）：读源文件，问一句
// 「这一位是不是从终端页来」。
func TestTheSessionListTakesItsOutcomeFromTheTerminalSurface(t *testing.T) {
	shell := repoFile(t, "internal/shell/shell.go")
	if !strings.Contains(shell, "Failed:") || !strings.Contains(shell, "a.Terminal.Failed(i)") {
		t.Error("the shell's session list no longer passes the terminal surface's Failed bit; " +
			"the sessions page's warning arm would be dead code")
	}
	if !strings.Contains(shell, "Running:") || !strings.Contains(shell, "!a.Terminal.Exited(i)") {
		t.Error("the shell's session list lost its Running bit")
	}
	// 反方向：设置页的答案来自这一位，不是自己去读退出码。
	settings := repoFile(t, "internal/settingsui/settingsui.go")
	if !strings.Contains(settings, "rowSegmentWarning") {
		t.Error("sessionMeta no longer chooses the warning rung for a failed outcome")
	}
	if strings.Contains(settings, ".ExitCode") {
		t.Error("the settings page started reading an exit code it does not have; " +
			"the outcome belongs to the terminal surface")
	}
}
