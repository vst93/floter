package terminalui

import "testing"

// P13-cm：会话列表要回答「这次结束是不是失败」，而这件事这一页本来就知道
// （`residentNote` 从 `ExitCode != 0` 取 `t.Warning`）。`Failed` 把同一个判断
// 摊成一条可问的规则，供设置页拼 `settingsui.Session` 时读。
//
// 三条边界：还没结束的会话没有结局可判；结束且码非零才是失败；越界不 panic。
func TestFailedIsOnlyTrueForAnEndedProgramWithANonZeroCode(t *testing.T) {
	a := &App{Sessions: []*Session{
		{ID: 1, Resident: true, ExitCode: 0},
		{ID: 2, Resident: true, ExitCode: 3},
		{ID: 3, Resident: true, ExitCode: -1},
	}}
	for _, tt := range []struct {
		name  string
		index int
		want  bool
	}{
		{"clean exit", 0, false},
		{"exit 3", 1, true},
		{"killed by a signal", 2, true},
		{"before the first session", -1, false},
		{"past the last session", 3, false},
	} {
		if got := a.Failed(tt.index); got != tt.want {
			t.Errorf("%s: Failed(%d) = %v, want %v", tt.name, tt.index, got, tt.want)
		}
	}
}

// A session that has not ended never fails, whatever its code field holds.
// (`Exited` treats a session with no terminal as ended, so the running case is
// exercised where a real terminal exists; this file pins the index and code
// boundaries that the settings page can hit.)
