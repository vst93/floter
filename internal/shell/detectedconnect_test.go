package shell

import (
	"testing"

	"floter/internal/settingsui"
)

// P10-d · 发现行的连接请求里那两段 fallback（`connectDetected` 的 12 行算术）
// 今天只由一条异步全链路间接碰过——而 `registerFromLink`（system.go:269）自己就把
// Name 与 VersionArgs 填满了，连那条全链路都走不到它们。这里把算术搬进
// `detectedToolRequest` 之后直接钉住：Name 有值就用、只剩空白才退到路径的命令词、
// VersionArgs 空才补 `--version`。

// 有值就用：只 trim，不剥扩展名、不改大小写、不动内部空白。
func TestADetectedNameIsUsedAsItIs(t *testing.T) {
	cases := []struct{ name, want string }{
		{"ripgrep", "ripgrep"},
		// 前后空白是表单/发现留下的噪声，不是名字的一部分。
		{"  ripgrep  ", "ripgrep"},
		{"\tripgrep\n", "ripgrep"},
		// 内部空白属于名字：`My Tool` 不是两个词，也不能被压成一个。
		{"My Tool", "My Tool"},
		// 发现行给的是可执行名时，扩展名与大小写都留着——剥它们的是 fallback
		// 那一支（`progname.FromPath`），两条路的答案故意不同。
		{"RG.EXE", "RG.EXE"},
		{"工具", "工具"},
	}
	for _, tc := range cases {
		got := detectedToolRequest(settingsui.DetectedTool{
			Name: tc.name,
			Path: "/usr/bin/something-else",
		})
		if got.Name != tc.want {
			t.Errorf("Name %q → %q, want %q", tc.name, got.Name, tc.want)
		}
	}
}

// 只剩空白才退到路径的命令词：`progname.FromPath` = `Stem(filepath.Base(path))`，
// 小写 + 只剥启动器后缀。
func TestABlankDetectedNameFallsBackToThePathWord(t *testing.T) {
	cases := []struct{ name, path, want string }{
		{"", "/usr/bin/rg", "rg"},
		{" ", "/usr/bin/rg", "rg"},
		{"\t", "/usr/bin/rg", "rg"},
		{"\n", "/usr/bin/rg", "rg"},
		// Go 的 TrimSpace 认全角空格（U+3000），所以中文名只剩全角空格也落空。
		{"\u3000", "/usr/bin/rg", "rg"},
		{"  \t ", "/opt/My Tool/bin/gh", "gh"},
		// 命令词那一支小写 + 剥启动器后缀，与「有值就用」那一支相反。
		{"", "/usr/bin/RG.EXE", "rg"},
		{"", "/opt/tools/ripgrep.exe", "ripgrep"},
		// 只剥一层：`rg.exe.txt` 不是启动器名。
		{"", "/usr/bin/rg.exe.txt", "rg.exe.txt"},
		// 分隔符是宿主的（这条路径是宿主自己产出的）：在 Linux 宿主上整条
		// Windows 路径是**一个**文件名，于是小写 + 剥后缀，反斜杠留在名字里。
		{"", "C:\\bin\\rg.exe", "c:\\bin\\rg"},
	}
	for _, tc := range cases {
		got := detectedToolRequest(settingsui.DetectedTool{Name: tc.name, Path: tc.path})
		if got.Name != tc.want {
			t.Errorf("Name %q + Path %q → %q, want %q", tc.name, tc.path, got.Name, tc.want)
		}
	}
}

// fallback 是兜底不是校验：名字与路径都空时它给出的不是空串，而是
// `filepath.Base("")` 自己的答案 `"."`（`progname.FromPath` 一路照搬下来）。
// 拒绝发生在 `PrepareCustom`（`LocalToolFailed`）那一层，这一支产品里不可达
// ——`computeDetectedTools` 先跳过空路径（detectedtools.go:168），register 深链
// 拿到的是绝对路径或解析结果。钉住是为了让这个「`.`」将来变红时有人看一眼。
func TestTheBlankNameFallbackHasItsOwnFloor(t *testing.T) {
	got := detectedToolRequest(settingsui.DetectedTool{Name: "   "})
	if got.Name != "." {
		t.Errorf("Name \"\" + Path \"\" → %q, want \".\" (filepath.Base of nothing)", got.Name)
	}
}

// `--version` 只在发现方什么都没说时才补：说了就原样交出去，不做 trim
// （`customRequest` 自己 `strings.Fields`）。
func TestVersionArgsDefaultOnlyWhenEmpty(t *testing.T) {
	cases := []struct{ versionArgs, want string }{
		{"", "--version"},
		{"-V", "-V"},
		{"version", "version"},
		{"--version --verbose", "--version --verbose"},
		// 文档化：只有空白也算「说了」，于是原样保留——`strings.Fields` 之后
		// 它才变成空表（探针不跑）。产品里没有这条路径（发现方从不设它，
		// register 深链传的是 `--version`）。
		{"   ", "   "},
	}
	for _, tc := range cases {
		got := detectedToolRequest(settingsui.DetectedTool{
			Name:        "rg",
			Path:        "/usr/bin/rg",
			VersionArgs: tc.versionArgs,
		})
		if got.VersionArgs != tc.want {
			t.Errorf("VersionArgs %q → %q, want %q", tc.versionArgs, got.VersionArgs, tc.want)
		}
	}
}

// 请求的其余部分是透传 + 一个必填：路径、描述照搬，平台由宿主填
// （不填就被 `platformRequired` 拒，见函数里那段注释），表单字段保持零值
// ——发现行不经过连接表单，`customRequest` 才补 `Mode: "executable"`。
func TestTheDetectedRequestCarriesThePathDescriptionAndPlatform(t *testing.T) {
	got := detectedToolRequest(settingsui.DetectedTool{
		Name:        "flameshot",
		Path:        "/usr/bin/flameshot",
		Description: "Screenshot tool",
		VersionArgs: "--version",
	})
	if got.Program != "/usr/bin/flameshot" {
		t.Errorf("Program = %q, want the discovered path", got.Program)
	}
	if got.Description != "Screenshot tool" {
		t.Errorf("Description = %q, want the discovered sentence", got.Description)
	}
	if len(got.Platforms) != 1 || got.Platforms[0] != settingsui.CurrentPlatform() {
		t.Errorf("Platforms = %v, want [%s]", got.Platforms, settingsui.CurrentPlatform())
	}
	if got.Mode != "" || got.Command != "" || got.Args != "" || got.Version != "" ||
		got.ScriptLanguage != "" || got.ScriptContent != "" || len(got.Params) != 0 {
		t.Errorf("the detected row must not fill the form's own fields: %+v", got)
	}
}
