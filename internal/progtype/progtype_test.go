package progtype

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeFile drops a plain file with the given mode, so a pin can build a file
// the extension strategy must reject and the bit strategy must judge by mode.
func writeFile(t *testing.T, dir, name string, mode os.FileMode) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x\n"), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestForReadsTheWindowsLadderByExtension is the pin moved out of `tools` when
// its local isExecutable was deleted: on Windows the four extensions are the
// programs and a plain file is not one, whatever its mode; on Unix the bit
// decides and the name means nothing.
func TestForReadsTheWindowsLadderByExtension(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"rg.exe", "run.cmd", "go.bat", "old.com", "RG.EXE"} {
		if !For("windows", writeFile(t, dir, name, 0o644)) {
			t.Errorf("%s was not executable on windows", name)
		}
	}
	// A plain file, whatever its mode, is not a program on Windows.
	for _, name := range []string{"notes.txt", "README", "rg", "makefile"} {
		if For("windows", writeFile(t, dir, name, 0o755)) {
			t.Errorf("%s was executable on windows", name)
		}
	}
	// A directory and a missing path are never executable.
	if For("windows", dir) {
		t.Error("a directory was executable on windows")
	}
	if For("windows", filepath.Join(dir, "absent.exe")) {
		t.Error("a missing path was executable on windows")
	}

	// The Unix side keeps the executable-bit rule: mode decides, the name does
	// not. A Windows host emulates every file as 0666, so the bit rule cannot
	// be exercised there — the block is conditional rather than skipped, so the
	// Windows half above still runs.
	if runtime.GOOS != "windows" {
		if !For("linux", writeFile(t, dir, "rg", 0o755)) {
			t.Error("a 0o755 file was not executable on linux")
		}
		if For("linux", writeFile(t, dir, "quiet", 0o644)) {
			t.Error("a 0o644 file was executable on linux")
		}
		// rg.exe was written 0o644 above (the first loop), so the bit rule must
		// say no: this is the "no extension magic on Unix" half.
		if For("linux", filepath.Join(dir, "rg.exe")) {
			t.Error("rg.exe was executable on linux without the execute bit")
		}
		// A directory carries execute bits of its own; it is still not a
		// program, which is what the regular-file half of the rule is for.
		if For("linux", dir) {
			t.Error("a directory was executable on linux")
		}
		if ForLenient("linux", dir) {
			t.Error("a directory was runnable on linux under the permissive rule")
		}
		// Any of the three execute bits will do, not just the owner's.
		exec := writeFile(t, dir, "group", 0o644)
		if err := os.Chmod(exec, 0o010); err != nil {
			t.Fatal(err)
		}
		if !For("linux", exec) {
			t.Error("a file with only the group execute bit was not executable")
		}
	}
}

// TestHostIsTheHostPlatformRule pins the delegation: Host must be For with the
// host's own key and nothing else, so a call site that says "the host" cannot
// drift away from the platform table.
func TestHostIsTheHostPlatformRule(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeFile(t, dir, "rg", 0o755),
		writeFile(t, dir, "quiet", 0o644),
		writeFile(t, dir, "rg.exe", 0o644),
		dir,
		filepath.Join(dir, "absent"),
	}
	for _, path := range paths {
		if got, want := Host(path), For(runtime.GOOS, path); got != want {
			t.Errorf("Host(%q) = %v, For(%q) = %v", path, got, runtime.GOOS, want)
		}
	}
}

// TestAnEmptyPlatformMeansTheHost pins the compatibility half the call sites
// relied on before the move: an empty key is "whatever this machine is".
func TestAnEmptyPlatformMeansTheHost(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeFile(t, dir, "rg", 0o755),
		writeFile(t, dir, "rg.exe", 0o644),
		dir,
	}
	for _, path := range paths {
		if got, want := For("", path), For(runtime.GOOS, path); got != want {
			t.Errorf("For(\"\", %q) = %v, want %v", path, got, want)
		}
		if got, want := ForLenient("", path), ForLenient(runtime.GOOS, path); got != want {
			t.Errorf("ForLenient(\"\", %q) = %v, want %v", path, got, want)
		}
	}
}

// TestThePermissiveWindowsRuleAcceptsAnyFile pins the second strategy: on
// Windows every existing non-directory counts, because a custom extension's
// path is the user's own pick and Windows decides runnability by content and
// association, not by name. A directory and a missing path are still no.
func TestThePermissiveWindowsRuleAcceptsAnyFile(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"notes.txt", "run.py", "README", "rg.exe"} {
		if !ForLenient("windows", writeFile(t, dir, name, 0o644)) {
			t.Errorf("%s was not runnable on windows under the permissive rule", name)
		}
	}
	if ForLenient("windows", dir) {
		t.Error("a directory was runnable on windows")
	}
	if ForLenient("windows", filepath.Join(dir, "absent")) {
		t.Error("a missing path was runnable on windows")
	}
	// The Unix half is not permissive: it is the same bit rule For uses.
	if runtime.GOOS != "windows" {
		if !ForLenient("linux", writeFile(t, dir, "tool", 0o755)) {
			t.Error("a 0o755 file was not runnable on linux")
		}
		if ForLenient("linux", writeFile(t, dir, "plain", 0o644)) {
			t.Error("a 0o644 file was runnable on linux under the permissive rule")
		}
	}
}

// TestTheStrategiesDisagreeOnAPlainFile is the pin for the reason this package
// has two strategies instead of one: they must answer differently on a plain
// file, and a future "cleanup" that merges them has to fail here first.
func TestTheStrategiesDisagreeOnAPlainFile(t *testing.T) {
	plain := writeFile(t, t.TempDir(), "notes.txt", 0o755)
	if !ForLenient("windows", plain) {
		t.Error("the permissive rule rejected a plain file on windows")
	}
	if For("windows", plain) {
		t.Error("the extension rule accepted a plain file on windows")
	}
	// On Unix there is nothing to disagree about: both read the bit.
	if got, want := ForLenient("linux", plain), For("linux", plain); got != want {
		t.Errorf("the two strategies disagree on linux: %v vs %v", got, want)
	}
}
