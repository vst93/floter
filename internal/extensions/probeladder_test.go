package extensions

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"floter/internal/progname"
)

// P10-e · 「用一个名字找程序」在 Windows 上只有一个答案。
//
// 病灶：`lookToolIn` / `lookToolsIn` 曾经直接 `filepath.Join(dir, name)` 再 stat，
// 而 Windows 的程序叫 `rg.exe`——于是裸名在 Linux/macOS 上解析得到、在 Windows 上
// 一个都解析不到。目录探针（`tools.Probe` → `progname.Ladder`）早就有梯子，参考
// 也是同一个答案（`executable_name_matches`，resolver.rs:118-133：期望名不带扩展名时
// 额外接受四个启动器拼写）。本轮把 `extensions` 的两条也接上同一个梯子。
//
// 这些用例用 `platform` 参数而不是 `runtime.GOOS`，所以 Linux 的 CI 也能钉住 Windows 的
// 答案（`progtype.For("windows", …)` 只按扩展名判，与宿主无关）。

// writeProbeFixture writes a file the way an installer would leave a program:
// present, not a directory. On Windows the name is the whole story; on Unix the
// execute bit is set so the same fixture also works as a host-platform program.
func writeProbeFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestABareNameFindsTheLauncherOnWindowsOnly is the round's headline: `rg`
// resolves to `rg.exe` on Windows, and the same fixture stays unresolvable off
// Windows, where the program really is called `rg`. The disagreement is
// deliberate — a ladder on Unix would invent names the filesystem does not have.
func TestABareNameFindsTheLauncherOnWindowsOnly(t *testing.T) {
	dir := t.TempDir()
	writeProbeFixture(t, filepath.Join(dir, "rg.exe"))

	found, ok := lookToolIn("windows", []string{dir}, "rg")
	if !ok || found != filepath.Join(dir, "rg.exe") {
		t.Errorf("windows lookToolIn(\"rg\") = %q, %v; want the .exe", found, ok)
	}
	for _, platform := range []string{"linux", "darwin"} {
		if found, ok := lookToolIn(platform, []string{dir}, "rg"); ok {
			t.Errorf("%s lookToolIn(\"rg\") = %q; a Unix program is named rg, not rg.exe", platform, found)
		}
	}
	// The same fixture found by its real name is the control: the ladder is the
	// only difference between the two answers.
	if found, ok := lookToolIn("linux", []string{dir}, "rg.exe"); !ok || found != filepath.Join(dir, "rg.exe") {
		t.Errorf("linux lookToolIn(\"rg.exe\") = %q, %v; the file is there under its own name", found, ok)
	}
}

// TestTheInterpreterNamesResolveThroughTheLadder pins the caller this bug bit
// hardest: `FindInterpreter("powershell")` asks for the bare `pwsh` /
// `powershell`, which on Windows only ever exists as `pwsh.exe`.
func TestTheInterpreterNamesResolveThroughTheLadder(t *testing.T) {
	dir := t.TempDir()
	writeProbeFixture(t, filepath.Join(dir, "pwsh.exe"))

	names := InterpreterNames("powershell")
	found, ok := lookToolIn("windows", []string{dir}, names...)
	if !ok || found != filepath.Join(dir, "pwsh.exe") {
		t.Fatalf("windows lookToolIn(%v) = %q, %v; want pwsh.exe", names, found, ok)
	}
	if found, ok := lookToolIn("linux", []string{dir}, names...); ok {
		t.Errorf("linux lookToolIn(%v) = %q; the ladder must not run there", names, found)
	}
}

// TestTheLadderOrderIsTheSuffixOrderThenTheBareName pins which spelling wins
// when a directory holds several: `.exe` first, then `.cmd`, `.bat`, `.com`.
// Each case removes the winner, so the order is measured rather than asserted
// once. The ladder's last rung is the bare name; P10-f moved that rung out of
// this loop because on Windows it is table shape, not an answer — see the bare
// name block below.
func TestTheLadderOrderIsTheSuffixOrderThenTheBareName(t *testing.T) {
	order := []string{"tool.exe", "tool.cmd", "tool.bat", "tool.com"}
	for start := range order {
		dir := t.TempDir()
		for _, name := range order[start:] {
			writeProbeFixture(t, filepath.Join(dir, name))
		}
		found, ok := lookToolIn("windows", []string{dir}, "tool")
		want := filepath.Join(dir, order[start])
		if !ok || found != want {
			t.Errorf("with %v present, lookToolIn(\"tool\") = %q, %v; want %q", order[start:], found, ok, want)
		}
	}
	// P10-f 重定判据：旧断言「只剩裸名时仍解析」→ 新断言「裸名是梯子最后一格，但 Windows
	// 判据拒绝一个无扩展名的文件」。The rung is still generated (`progname.Ladder`
	// appends it, and on Unix it is the only rung), yet on Windows a file with no
	// launcher suffix is not a program — the reference's PATH discovery only ever
	// yields entries its extension strategy accepts (inventory.rs:439, :1088), and
	// every other name probe in this build asks `progtype.For` the same way.
	bare := t.TempDir()
	writeProbeFixture(t, filepath.Join(bare, "tool"))
	if ladder := progname.Ladder("windows", "tool"); ladder[len(ladder)-1] != "tool" {
		t.Errorf("ladder = %v; the bare name is its last rung", ladder)
	}
	if found, ok := lookToolIn("windows", []string{bare}, "tool"); ok {
		t.Errorf("lookToolIn(\"tool\") = %q; a file Windows cannot run by name is not a program", found)
	}
	// On Unix that same file is the program: the rung is not dead there.
	if found, ok := lookToolIn("linux", []string{bare}, "tool"); !ok || found != filepath.Join(bare, "tool") {
		t.Errorf("linux lookToolIn(\"tool\") = %q, %v; the bare name is the Unix spelling", found, ok)
	}
	// Nothing on the ladder is not a hit: a file the suffixes do not cover is
	// never renamed into one.
	dir := t.TempDir()
	writeProbeFixture(t, filepath.Join(dir, "tool.exe.txt"))
	if found, ok := lookToolIn("windows", []string{dir}, "tool"); ok {
		t.Errorf("lookToolIn(\"tool\") = %q; tool.exe.txt is not a launcher", found)
	}
}

// TestTheSearchPathOrderOutranksTheLadder pins the loop nesting: the directory
// is the outer loop, so a name found earlier in the search path wins even when a
// later directory holds a better suffix. Suffix order only decides inside one
// directory — the search path's own priority is not up for grabs.
func TestTheSearchPathOrderOutranksTheLadder(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeProbeFixture(t, filepath.Join(first, "tool.cmd"))
	writeProbeFixture(t, filepath.Join(second, "tool.exe"))

	found, ok := lookToolIn("windows", []string{first, second}, "tool")
	if !ok || found != filepath.Join(first, "tool.cmd") {
		t.Errorf("lookToolIn = %q, %v; the first directory wins over the better suffix", found, ok)
	}
	found, ok = lookToolIn("windows", []string{second, first}, "tool")
	if !ok || found != filepath.Join(second, "tool.exe") {
		t.Errorf("reversed lookToolIn = %q, %v; want the .exe from the first directory", found, ok)
	}
}

// TestTheCandidateIsNamedAfterTheFileNotTheProbe pins the name a chooser row
// carries: the file that was found (`rg.exe`), which is what PATH discovery
// calls it (`merge(path, entry.Name(), …)`) and what the register link names the
// integration (`filepath.Base` of the resolved path). It is **not** the command
// word (`progname.FromPath` → `rg`), because those two faces answer different
// questions and P10-b already pinned their disagreement.
func TestTheCandidateIsNamedAfterTheFileNotTheProbe(t *testing.T) {
	dir := t.TempDir()
	writeProbeFixture(t, filepath.Join(dir, "rg.exe"))

	got := lookToolsIn("windows", []string{dir}, "rg")
	if len(got) != 1 {
		t.Fatalf("lookToolsIn = %v; want one candidate", got)
	}
	if want := "rg.exe"; got[0].Name != want {
		t.Errorf("candidate Name = %q; want the file's own name %q", got[0].Name, want)
	}
	// The row titles the path it will spawn: `registerFromLink` names the card
	// `filepath.Base` of that same path, so the two doors agree by construction.
	if base := filepath.Base(got[0].Path); got[0].Name != base {
		t.Errorf("candidate Name = %q; register would call it %q", got[0].Name, base)
	}
	if stem := progname.FromPath(got[0].Path); stem == got[0].Name {
		t.Errorf("candidate Name = %q; the command word is %q, the two faces must not be merged", got[0].Name, stem)
	}
	// Known limitation, pinned rather than hidden: the ladder is spelled from the
	// name the user gave, so it is lowercase. On Windows' case-insensitive
	// filesystem `RG.EXE` is the same file; on this case-sensitive fixture it is
	// not. Fixing that would cost a directory read per probe, which this round
	// does not spend.
	upper := t.TempDir()
	writeProbeFixture(t, filepath.Join(upper, "RG.EXE"))
	found, ok := lookToolIn("windows", []string{upper}, "rg")
	if _, caseInsensitive := os.Stat(filepath.Join(upper, "rg.exe")); caseInsensitive == nil {
		// A case-insensitive filesystem (Windows): the same file, reached under
		// the probe's spelling.
		if !ok || found != filepath.Join(upper, "rg.exe") {
			t.Errorf("case-insensitive lookToolIn(\"rg\") = %q, %v; want the .exe", found, ok)
		}
	} else if ok {
		t.Errorf("lookToolIn(\"rg\") = %q; the probe spelling is lowercase", found)
	}
	// One file, one row: the ladder's four spellings of the same name must not
	// each add a candidate.
	if again := lookToolsIn("windows", []string{dir}, "rg", "rg.exe"); len(again) != 1 {
		t.Errorf("lookToolsIn with both spellings = %v; want one row per file", again)
	}
}

// TestAnEmptyNameNeverResolves pins the guard that has to stay in front of the
// ladder: `progname.Ladder("windows", "")` is the four bare suffixes, so a
// directory holding a file literally called `.exe` would otherwise answer a
// question nobody asked.
func TestAnEmptyNameNeverResolves(t *testing.T) {
	dir := t.TempDir()
	writeProbeFixture(t, filepath.Join(dir, ".exe"))

	if found, ok := lookToolIn("windows", []string{dir}, ""); ok {
		t.Errorf("lookToolIn(\"\") = %q; an empty name names nothing", found)
	}
	if got := lookToolsIn("windows", []string{dir}, "", "rg"); len(got) != 0 {
		t.Errorf("lookToolsIn = %v; the empty name contributed a candidate", got)
	}
}

// TestTheHostLookupIsUnchangedForUnixNames is the control for the platform
// argument: the exported entry points pass `runtime.GOOS`, so a Unix host still
// resolves the bare names it always did and never invents a suffix.
func TestTheHostLookupIsUnchangedForUnixNames(t *testing.T) {
	dir := t.TempDir()
	writeProbeFixture(t, filepath.Join(dir, "tool"))
	if found, ok := lookToolIn(runtime.GOOS, []string{dir}, "tool"); !ok || found != filepath.Join(dir, "tool") {
		t.Errorf("host lookToolIn(\"tool\") = %q, %v", found, ok)
	}
}
