package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeFile drops a plain file with the given mode. Unlike writeExecutable it
// does not chmod 0o755, so a caller can build a file the Windows extension test
// must reject.
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

// TestTheProbeLadderFollowsThePlatformKeyNotTheHost is the pin for the bug this
// round fixes: candidateNames used to read `runtime.GOOS`, so on a Linux host
// the Windows ladder was unreachable and no test could pin it. The ladder now
// reads the key `Look` was given.
func TestTheProbeLadderFollowsThePlatformKeyNotTheHost(t *testing.T) {
	windows := candidateNames(PlatformWindows, "rg")
	want := []string{"rg.exe", "rg.cmd", "rg.bat", "rg.com", "rg"}
	if len(windows) != len(want) {
		t.Fatalf("the windows ladder = %v, want %v", windows, want)
	}
	for i := range want {
		if windows[i] != want[i] {
			t.Fatalf("the windows ladder = %v, want %v", windows, want)
		}
	}
	// Every other platform key is the bare name, in one entry.
	for _, platform := range []string{PlatformLinux, PlatformMacOS} {
		ladder := candidateNames(platform, "rg")
		if len(ladder) != 1 || ladder[0] != "rg" {
			t.Errorf("the %s ladder = %v, want [rg]", platform, ladder)
		}
	}

	// The consequence, not just the table: a probe told "windows" resolves
	// `rg.exe` on this host, and the same probe told "linux" does not.
	dir := t.TempDir()
	writeFile(t, dir, "rg.exe", 0o755)
	var ripgrep Entry
	for _, entry := range Catalog {
		if entry.ID == "ripgrep" {
			ripgrep = entry
		}
	}
	if got := Probe(ripgrep, []string{dir}, PlatformWindows); got != "rg.exe" {
		t.Errorf("a windows probe resolved %q, want rg.exe", got)
	}
	if got := Probe(ripgrep, []string{dir}, PlatformLinux); got != "" {
		t.Errorf("a linux probe resolved %q, want nothing: rg.exe is not rg", got)
	}
}

// TestTheWindowsExecutableTestNeedsAnExecutableExtension pins the second half:
// isExecutable on Windows used to accept any regular file, so `notes.txt`
// counted as a program. It now reads the same four extensions
// `inventory.isExecutable` does.
func TestTheWindowsExecutableTestNeedsAnExecutableExtension(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"rg.exe", "run.cmd", "go.bat", "old.com", "RG.EXE"} {
		if !isExecutable(PlatformWindows, writeFile(t, dir, name, 0o644)) {
			t.Errorf("%s was not executable on windows", name)
		}
	}
	// A plain file, whatever its mode, is not a program on Windows.
	for _, name := range []string{"notes.txt", "README", "rg", "makefile"} {
		if isExecutable(PlatformWindows, writeFile(t, dir, name, 0o755)) {
			t.Errorf("%s was executable on windows", name)
		}
	}
	// A directory and a missing path are never executable.
	if isExecutable(PlatformWindows, dir) {
		t.Error("a directory was executable on windows")
	}
	if isExecutable(PlatformWindows, filepath.Join(dir, "absent.exe")) {
		t.Error("a missing path was executable on windows")
	}

	// The Unix side keeps the executable-bit rule: mode decides, the name does
	// not. A Windows host emulates every file as 0666, so the bit rule cannot
	// be exercised there — the block is conditional rather than skipped, so the
	// Windows half above still runs.
	if runtime.GOOS != "windows" {
		if !isExecutable(PlatformLinux, writeFile(t, dir, "rg", 0o755)) {
			t.Error("a 0o755 file was not executable on linux")
		}
		if isExecutable(PlatformLinux, writeFile(t, dir, "quiet", 0o644)) {
			t.Error("a 0o644 file was executable on linux")
		}
		// rg.exe was written 0o644 above (the first loop), so the bit rule must
		// say no: this is the "no extension magic on Unix" half.
		if isExecutable(PlatformLinux, filepath.Join(dir, "rg.exe")) {
			t.Error("rg.exe was executable on linux without the execute bit")
		}
	}
}

// TestABareNameIsNotInstalledOnWindows is the user-visible consequence: a
// `rg` with no extension in the search path must not light up the Windows
// catalog, and `rg.exe` must.
func TestABareNameIsNotInstalledOnWindows(t *testing.T) {
	var ripgrep Entry
	for _, entry := range Catalog {
		if entry.ID == "ripgrep" {
			ripgrep = entry
		}
	}
	dir := t.TempDir()
	writeFile(t, dir, "rg", 0o755) // a Unix build, not a Windows program
	if Installed(ripgrep, []string{dir}, PlatformWindows) {
		t.Error("a bare rg counted as installed on windows")
	}
	writeFile(t, dir, "rg.exe", 0o755)
	if !Installed(ripgrep, []string{dir}, PlatformWindows) {
		t.Error("rg.exe did not count as installed on windows")
	}
	// The same directory read as linux sees the bare name. A Windows host
	// emulates every file as 0666, so the bit rule cannot be exercised there.
	if runtime.GOOS != "windows" {
		if !Installed(ripgrep, []string{dir}, PlatformLinux) {
			t.Error("a bare rg did not count as installed on linux")
		}
	}
}

// TestTheManagerLadderFollowsThePlatformKey covers the manager half: winget is
// a Windows name, so a `winget.exe` in the path means winget only when the key
// says windows.
func TestTheManagerLadderFollowsThePlatformKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "winget.exe", 0o755)
	writeFile(t, dir, "apt", 0o755)

	windows := DetectManagers([]string{dir}, PlatformWindows)
	if !contains(windows, "winget") {
		t.Errorf("winget.exe was not detected on windows: %v", windows)
	}
	if contains(windows, "apt") {
		t.Errorf("a bare apt was detected on windows: %v", windows)
	}
	linux := DetectManagers([]string{dir}, PlatformLinux)
	if contains(linux, "winget") {
		t.Errorf("winget.exe was detected on linux: %v", linux)
	}
	// A Windows host emulates every file as 0666, so the Unix bit rule cannot
	// be exercised there; the bare-name rejection above still runs everywhere.
	if runtime.GOOS != "windows" && !contains(linux, "apt") {
		t.Errorf("apt was not detected on linux: %v", linux)
	}
}

// TestLookWiresThePlatformKeyThrough is the end-to-end pin: the key handed to
// Look is the one the probes and the manager ladder see. Before this round the
// wiring existed but the two leaves ignored it, so this is the seam that makes
// the fix observable from the public entry point.
func TestLookWiresThePlatformKeyThrough(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "rg.exe", 0o755)
	writeFile(t, dir, "winget.exe", 0o755)

	states := Look([]string{dir}, PlatformWindows)
	byID := map[string]State{}
	for _, state := range states {
		byID[state.ID] = state
	}
	if !byID["ripgrep"].Installed {
		t.Error("Look(windows) did not see rg.exe")
	}
	if byID["ripgrep"].Manager != "winget" {
		t.Errorf("Look(windows) chose %q, want winget", byID["ripgrep"].Manager)
	}

	// The same path read as linux sees neither: the key really is the input.
	linux := Look([]string{dir}, PlatformLinux)
	for _, state := range linux {
		if state.Installed {
			t.Errorf("Look(linux) saw %s in a windows-only path", state.ID)
		}
	}
}

// TestAnEmptyPlatformKeyFallsBackToTheHost pins the compatibility half: an
// empty key means "whatever this machine is", so existing callers that pass
// nothing keep working.
func TestAnEmptyPlatformKeyFallsBackToTheHost(t *testing.T) {
	want := []string{"rg"}
	if runtime.GOOS == "windows" {
		want = []string{"rg.exe", "rg.cmd", "rg.bat", "rg.com", "rg"}
	}
	got := candidateNames("", "rg")
	if len(got) != len(want) {
		t.Fatalf("the empty key gave %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the empty key gave %v, want %v", got, want)
		}
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
