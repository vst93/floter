package shell

import (
	"testing"

	"floter/internal/inventory"
	"floter/internal/tools"
)

// P10-b：发现行的安装配方是按**命令词面**的名字去目录里找的
// （`detectedInstallCommand`，`shell.go:2557` 的 `progname.FromPath`），
// 目录里的探针名全是小写。所以大写文件名也必须命中——这正是
// `progname` 小写、而别名面（`apps.executableName`，一个字符都不剥）与
// 词干面（`apps.fileStem`，剥扩展名但保留大小写）给不出的答案。
//
// 已有的 pin（`detected_test.go`）只用全小写路径，区分不了三条规则；
// 这条补上大小写这一维：换成词干面 `Flameshot` 就匹配不到 `flameshot`，
// 安装按钮会静默消失。
func TestTheDetectedRowMatchesTheCatalogByTheCommandWord(t *testing.T) {
	cases := []struct {
		what string
		path string
		want string
	}{
		{
			what: "a Windows executable with the installer's own capitalisation",
			path: "/c/Program Files/Flameshot/Flameshot.EXE",
			want: "winget install --id Flameshot.Flameshot -e",
		},
		{
			what: "a launcher-less program with capitalisation",
			path: "/opt/apps/Flameshot",
			want: "winget install --id Flameshot.Flameshot -e",
		},
		{
			what: "all caps still folds",
			path: "/opt/apps/FLAMESHOT",
			want: "winget install --id Flameshot.Flameshot -e",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.what, func(t *testing.T) {
			got := detectedInstallCommand(
				inventory.Candidate{Name: "whatever", Path: tc.path},
				tools.PlatformWindows, []string{"winget"},
			)
			if got != tc.want {
				t.Errorf("detectedInstallCommand(%q) = %q, want %q (the catalog's probes are lowercase)",
					tc.path, got, tc.want)
			}
		})
	}
}
