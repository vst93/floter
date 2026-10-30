package ci

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// P13-a · 会话宿主的合约：宿主是**这个二进制的子命令**，而且它在单实例锁之前分流。
//
// 这一条不是设计切片，是**结构账**。宿主进程有两件事必须成立，否则「退出 app 程序还在」
// 这条承诺会在下一次重构里静默失效：
//
//	① 宿主必须在 `mygo.App.RequestSingleInstanceLock()` **之前**被认出来。锁一旦先跑，
//	   在跑的 app 会把这个宿主当成「第二次启动」——宿主被拒绝、参数被转交给 app，
//	   用户按下终端时得到的是一台没有会话的窗口。
//	② 宿主是同一个可执行文件的子命令，不是第二个二进制：Windows 的安装包是一个 exe，
//	   macOS 的 bundle 签名的是里面的东西，多一个可执行文件就是多一件要打包、要签名、
//	   要公证的产物。
//
// 另外钉住「不新增外部依赖」：`internal/termsession` 只许 import 标准库与 `floter/`
// 自己的包。用户 2026-10-30 的取舍里写死了「不引 herdr-core、不新增 cgo、不新增外部
// 二进制」，而最容易破的方式就是某天顺手 `go get` 一个 pty 库。
func TestTheSessionHostIsASubcommandBeforeTheSingleInstanceLock(t *testing.T) {
	main := repoFile(t, "cmd/floter/main.go")
	lock := strings.Index(main, "RequestSingleInstanceLock()")
	if lock < 0 {
		t.Fatal("cmd/floter/main.go has no single-instance lock")
	}
	host := strings.Index(main, "termsession.IsCommand(")
	if host < 0 {
		t.Fatal("cmd/floter/main.go never asks whether it was started as the session host")
	}
	if host > lock {
		t.Errorf("the host subcommand is dispatched at offset %d, after the single-instance lock at %d: "+
			"a running app would refuse the host and take its arguments", host, lock)
	}
	// The host is this binary: the dispatch is a subcommand of main, not a
	// second `main` package.
	entries, err := filepath.Glob(filepath.Join("..", "..", "cmd", "*", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("cmd has %d entry points %v, want one: the host is a subcommand, not a second binary", len(entries), entries)
	}
}

// TestTheSessionHostAddsNoDependency pins the pure-Go promise: the host's
// package imports the standard library and this repo's own packages, nothing
// else. A pseudo-terminal from a module would be a dependency on every
// platform the app builds for, which is exactly what this slice refused.
func TestTheSessionHostAddsNoDependency(t *testing.T) {
	dir := filepath.Join("..", "..", "internal", "termsession")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		files++
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, spec := range f.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if isStdlibOrOwn(path) {
				continue
			}
			t.Errorf("%s imports %q: the session host is standard library and this repo only "+
				"(no cgo, no external binary, no pty module)", name, path)
		}
	}
	if files == 0 {
		t.Fatal("internal/termsession has no Go files")
	}
	// `import "C"` is how a cgo dependency arrives without a module.
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"C"`) {
			t.Errorf("%s imports C: the host is pure Go", entry.Name())
		}
	}
}

// isStdlibOrOwn reports whether an import path is the standard library or one
// of this repo's packages.
func isStdlibOrOwn(path string) bool {
	if strings.HasPrefix(path, "floter/") {
		return true
	}
	// A standard-library path has no dot in its first element; a module path
	// always does (`github.com/…`). This is the rule the go tool applies.
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
