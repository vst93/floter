package extensions

import (
	"os"
	"path/filepath"
	"testing"

	"floter/internal/progtype"
	"floter/internal/tools"
)

// P10-f · 「这个名字是不是程序」：名字探针用严格判据，用户点选的路径才用宽松判据。
//
// 病灶：`lookToolIn` / `lookToolsIn` 是**名字探针**（系统替用户按名字找程序），却用了
// `progtype.ForLenient`——Windows 上「存在且不是目录」就算程序。于是搜索路径里一个
// 无扩展名的普通文件会被 `floter://register?cmd=…` 连成集成、被 `.desktop` 的裸
// `Exec=` 当成落地文件、被 `SystemRuntimeCandidates` 塞进选择器。
//
// 同一个问题在本仓其它三处名字探针上早就是严格判据（`tools.Probe` → `progtype.For`，
// `apps.executableAvailable` 与 `inventory.executableFile` → `progtype.Host`），参考的
// PATH 发现也逐条目过滤（inventory.rs:439，判据 :1088）。宽松判据只属于用户自己点选了
// 一个已存在路径的三处（provider.go:53 / reconnect.go:68 / custom.go:253）。
//
// 这些用例用 `platform` 参数，所以 Linux 的 CI 也能钉住 Windows 的答案。

// TestANameProbeRefusesAFileWindowsCannotRun is the round's headline, and it
// says both sentences at once: the same file is refused by the name probe and
// accepted by the picked-path rule. That disagreement is deliberate and located
// — it is why `progtype` has two strategies at all — so a future round cannot
// "fix" one of them without seeing the other.
func TestANameProbeRefusesAFileWindowsCannotRun(t *testing.T) {
	dir := t.TempDir()
	notAProgram := filepath.Join(dir, "notes.txt")
	bareName := filepath.Join(dir, "tool")
	writeProbeFixture(t, notAProgram)
	writeProbeFixture(t, bareName)

	for _, name := range []string{"notes.txt", "tool"} {
		if found, ok := lookToolIn("windows", []string{dir}, name); ok {
			t.Errorf("windows lookToolIn(%q) = %q; a file with no launcher suffix is not a program", name, found)
		}
		if got := lookToolsIn("windows", []string{dir}, name); len(got) != 0 {
			t.Errorf("windows lookToolsIn(%q) = %v; a chooser must not offer a file Windows cannot run", name, got)
		}
	}

	// The other sentence: the user picking that same file by hand still gets a
	// yes, because hiding the program they just chose is the worse mistake.
	for _, path := range []string{notAProgram, bareName} {
		if !progtype.ForLenient("windows", path) {
			t.Errorf("ForLenient(\"windows\", %q) = false; a picked path stays permissive", path)
		}
	}

	// The control: a real launcher in the same directory is still found, so the
	// refusal above is about the suffix and nothing else.
	writeProbeFixture(t, filepath.Join(dir, "notes.txt.exe"))
	if found, ok := lookToolIn("windows", []string{dir}, "notes.txt"); !ok || found != filepath.Join(dir, "notes.txt.exe") {
		t.Errorf("windows lookToolIn(\"notes.txt\") = %q, %v; want notes.txt.exe", found, ok)
	}
}

// TestTheCandidateListUsesTheSameJudgementAsTheLookup pins that the two doors
// into one name agree: whatever `LookTool` refuses to resolve, `LookTools` must
// not offer. They had separate judgements before this round, so a file could be
// unresolvable by name and still appear in the chooser.
func TestTheCandidateListUsesTheSameJudgementAsTheLookup(t *testing.T) {
	dir := t.TempDir()
	writeProbeFixture(t, filepath.Join(dir, "tool"))
	writeProbeFixture(t, filepath.Join(dir, "tool.exe.txt"))

	for _, name := range []string{"tool", "tool.exe"} {
		_, lookupOK := lookToolIn("windows", []string{dir}, name)
		listed := len(lookToolsIn("windows", []string{dir}, name)) > 0
		if lookupOK != listed {
			t.Errorf("name %q: lookToolIn ok = %v, lookToolsIn listed = %v; one name, one judgement", name, lookupOK, listed)
		}
	}

	// And the file that is a program is still listed under its own name.
	writeProbeFixture(t, filepath.Join(dir, "tool.exe"))
	got := lookToolsIn("windows", []string{dir}, "tool")
	if len(got) != 1 || got[0].Name != "tool.exe" {
		t.Fatalf("lookToolsIn = %v; want the one launcher", got)
	}
}

// TestTheNameProbesAgreeWithTheToolCatalog compares this package's lookup with
// the tool catalog's, cell by cell: the same directory, the same requested name,
// the same platform. Two name probes that disagree about one file are how the
// Detected row and the catalog row end up contradicting each other.
func TestTheNameProbesAgreeWithTheToolCatalog(t *testing.T) {
	cases := []struct {
		request string
		files   []string
	}{
		{"tool", []string{"tool.exe"}},
		{"tool", []string{"tool"}},
		{"tool", []string{"tool.exe.txt"}},
		{"tool", []string{"tool.cmd", "tool.exe"}},
		{"notes.txt", []string{"notes.txt"}},
		{"notes.txt", []string{"notes.txt.exe"}},
		{"tool", nil},
	}
	for _, testCase := range cases {
		dir := t.TempDir()
		for _, file := range testCase.files {
			writeProbeFixture(t, filepath.Join(dir, file))
		}
		entry := tools.Entry{ID: "fixture", Name: "Fixture", Probes: []string{testCase.request}}
		probe := tools.Probe(entry, []string{dir}, "windows")
		found, ok := lookToolIn("windows", []string{dir}, testCase.request)
		if (probe != "") != ok {
			t.Errorf("files %v, request %q: tools.Probe = %q, lookToolIn = %q, %v; the two name probes must agree",
				testCase.files, testCase.request, probe, found, ok)
			continue
		}
		if ok && filepath.Base(found) != probe {
			t.Errorf("files %v, request %q: tools.Probe picked %q, lookToolIn picked %q",
				testCase.files, testCase.request, probe, filepath.Base(found))
		}
	}
}

// TestTheSuffixIsNotInventedFromAnotherFile pins the boundary of the ladder: a
// probe spelling is never satisfied by a file that merely contains it. Windows
// resolves `tool` through four suffixes and the bare name, nothing else.
func TestTheSuffixIsNotInventedFromAnotherFile(t *testing.T) {
	dir := t.TempDir()
	writeProbeFixture(t, filepath.Join(dir, "tool.exe.txt"))
	writeProbeFixture(t, filepath.Join(dir, "tool.exe.old"))

	if found, ok := lookToolIn("windows", []string{dir}, "tool"); ok {
		t.Errorf("lookToolIn(\"tool\") = %q; neither file is a launcher", found)
	}
	if got := lookToolsIn("windows", []string{dir}, "tool"); len(got) != 0 {
		t.Errorf("lookToolsIn = %v; neither file is a launcher", got)
	}
}

// TestTheUnixHalfIsUnchangedByTheJudgement is the control: on Unix the two
// strategies are one rule, so this round changed nothing there — an executable
// bare name resolves, a non-executable one does not.
func TestTheUnixHalfIsUnchangedByTheJudgement(t *testing.T) {
	dir := t.TempDir()
	runnable := filepath.Join(dir, "tool")
	writeProbeFixture(t, runnable) // 0o755

	plain := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(plain, []byte("notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, platform := range []string{"linux", "darwin"} {
		if found, ok := lookToolIn(platform, []string{dir}, "tool"); !ok || found != runnable {
			t.Errorf("%s lookToolIn(\"tool\") = %q, %v; an executable bare name is the Unix spelling", platform, found, ok)
		}
		if found, ok := lookToolIn(platform, []string{filepath.Dir(plain)}, "tool"); ok {
			t.Errorf("%s lookToolIn(\"tool\") = %q; no execute bit, no program", platform, found)
		}
	}
	// One rule on Unix: the strict and the permissive strategy answer the same.
	if progtype.For("linux", plain) != progtype.ForLenient("linux", plain) {
		t.Error("For and ForLenient disagree on Unix; there is nothing permissive to be about a real execute bit")
	}
}
