package extensions

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/progname"
)

// P10-b：连接程序时那个「命令词」是 `progname.FromPath` 的答案，不是
// `apps.fileStem` 的答案，也不是 `.desktop` 别名面（`apps.executableName`）的答案。
// 三个面在同一个文件名上给出三个不同的名字，见
// `internal/apps/filestem_test.go` 的 `TestTheThreeNameFacesDisagreeOnPurpose`；
// 这里钉住的是**产品真的走这条路**：请求没带 Command 时，`normalizeCustom`
// （`custom.go:257`）用它生成命令词，随后 `validCustomCommand`
// （`customvalidate.go:118`）按命令词的字形判它合不合法。
//
// 为什么这个 pin 必须存在：把 `progname.FromPath` 换成剥任意扩展名的
// `apps.fileStem` 会让 `RG.EXE` 派生出 `RG` —— 大写字母过不了命令词字形，
// 用户装好的程序从此连接不上。合并的方向不是「换个拼写」，是产品回归。
func TestTheDerivedCommandIsTheLauncherStemNotTheFileStem(t *testing.T) {
	cases := []struct {
		file      string
		want      string
		wantIssue CustomIssueKey
	}{
		// 一个启动器后缀：剥掉、小写，三条启动器后缀都一样。
		{"RG.EXE", "rg", ""},
		{"rg.cmd", "rg", ""},
		{"rg.com", "rg", ""},
		{"rg.bat", "rg", ""},
		// 没有后缀：原样（已小写）。
		{"mytool", "mytool", ""},
		// 内层点保留 —— progname 只剥**一个启动器后缀**，`rg.exe.txt` 不是
		// 启动器，所以派生出带点的词，被命令词字形拒绝（`fileStem` 会给
		// `rg.exe`，同样带点、同样被拒；两条规则在这条上都不放行）。
		{"rg.exe.txt", "", CustomIssueCommandShape},
		{"report.tar.gz", "", CustomIssueCommandShape},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			dir := t.TempDir()
			program := filepath.Join(dir, tc.file)
			if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			normalized, err := normalizeCustom(CustomRequest{
				Name: "Gadget", ExecutablePath: program, Platforms: testPlatforms(),
			})
			if tc.wantIssue != "" {
				if !errors.Is(err, ErrBadRequest) || !strings.Contains(err.Error(), string(tc.wantIssue)) {
					t.Fatalf("%s: want the %s refusal, got %v", tc.file, tc.wantIssue, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: a legal derived command was refused: %v", tc.file, err)
			}
			if normalized.Command != tc.want {
				t.Errorf("%s: derived command = %q, want %q", tc.file, normalized.Command, tc.want)
			}
		})
	}
}

// 同一个名字在三个面上的答案，以及「哪一个能当命令词」这个后果。别名面
// （`apps.executableName`，`apps.go:732`）一个字符都不剥，词干面
// （`apps.fileStem`，`shortcut.go:226`）剥任意扩展名但保留大小写，命令词面
// （`progname.FromPath`，`progname.go:100`）只剥启动器后缀并小写。
//
// 这条 pin 用 `validCustomCommand` 判的正是 `normalizeCustom` 之后那一关，
// 所以它证明的是产品后果，而不是三个函数碰巧不同：只有命令词面能过。
func TestOnlyTheCommandFaceIsALegalCommandWord(t *testing.T) {
	const path = "/c/bin/RG.EXE"
	faces := []struct {
		name string
		word string
		ok   bool
	}{
		{"alias face (executableName: strips nothing)", "RG.EXE", false},
		{"stem face (fileStem: any extension, case kept)", "RG", false},
		{"command face (progname.FromPath)", progname.FromPath(path), true},
	}
	if got, want := progname.FromPath(path), "rg"; got != want {
		t.Fatalf("progname.FromPath(%q) = %q, want %q", path, got, want)
	}
	for _, face := range faces {
		if got := validCustomCommand(face.word); got != face.ok {
			t.Errorf("%s gives %q: validCustomCommand = %v, want %v", face.name, face.word, got, face.ok)
		}
	}
}
