package progname

import (
	"runtime"
	"testing"
)

// The three directions must agree, because they are one table: a name the
// acceptance rule calls a launcher must be one the strip rule removes and one
// the ladder offers.
func TestTheThreeDirectionsReadOneTable(t *testing.T) {
	for _, suffix := range []string{".exe", ".cmd", ".bat", ".com"} {
		name := "rg" + suffix
		if !IsLauncher(name) {
			t.Errorf("%s was not accepted as a launcher", name)
		}
		if got := Stem(name); got != "rg" {
			t.Errorf("Stem(%s) = %q, want rg", name, got)
		}
		offered := false
		for _, candidate := range Ladder("windows", "rg") {
			if candidate == name {
				offered = true
			}
		}
		if !offered {
			t.Errorf("the windows ladder %v does not offer %s", Ladder("windows", "rg"), name)
		}
	}
}

func TestStemDropsOneLauncherSuffix(t *testing.T) {
	for _, pair := range [][2]string{
		{"rg", "rg"},
		{"rg.exe", "rg"},
		{"RG.EXE", "rg"},
		{"rg.cmd", "rg"},
		{"  rg.bat ", "rg"},
		{"rg.com", "rg"},
		{"rg.exe.txt", "rg.exe.txt"}, // one suffix, and it is not the last one
		{"rgexe", "rgexe"},
		{"", ""},
		{"unknown", "unknown"},
	} {
		if got := Stem(pair[0]); got != pair[1] {
			t.Errorf("Stem(%q) = %q, want %q", pair[0], got, pair[1])
		}
	}
}

// FromPath is the same rule with the directory taken off first: a connected
// program's command word comes from its own file name.
func TestFromPathIsTheStemOfTheBaseName(t *testing.T) {
	for _, pair := range [][2]string{
		{"/usr/bin/rg", "rg"},
		{"/opt/tools/RG.EXE", "rg"},
		{"/home/me/bin/bat.cmd", "bat"},
		{"/usr/local/bin/x.com", "x"},
		{"rg.exe", "rg"},
		{"/usr/bin/", "bin"}, // filepath.Base cleans the trailing separator
	} {
		if got := FromPath(pair[0]); got != pair[1] {
			t.Errorf("FromPath(%q) = %q, want %q", pair[0], got, pair[1])
		}
	}
}

func TestIsLauncherSeesTheFourAndNothingElse(t *testing.T) {
	for _, name := range []string{"rg.exe", "RG.EXE", "rg.cmd", "rg.bat", "rg.com", ".exe"} {
		if !IsLauncher(name) {
			t.Errorf("%q was not a launcher name", name)
		}
	}
	for _, name := range []string{"rg", "rg.txt", "rgexe", "rg.exe.txt", ""} {
		if IsLauncher(name) {
			t.Errorf("%q was taken for a launcher name", name)
		}
	}
}

// The ladder is the probe order: each suffix first, the bare name last, so a
// machine that has both `rg.exe` and `rg` resolves the one the user installed.
func TestLadderIsTheWindowsOneAndTheBareNameEverywhereElse(t *testing.T) {
	windows := Ladder("windows", "rg")
	want := []string{"rg.exe", "rg.cmd", "rg.bat", "rg.com", "rg"}
	if len(windows) != len(want) {
		t.Fatalf("the windows ladder = %v, want %v", windows, want)
	}
	for i := range want {
		if windows[i] != want[i] {
			t.Fatalf("the windows ladder = %v, want %v", windows, want)
		}
	}
	// The ladder must not alias the caller's string or the table.
	if windows[0] != "rg.exe" || launcherSuffixes[0] != ".exe" {
		t.Fatalf("the ladder rewrote its inputs: %v", windows)
	}
	for _, platform := range []string{"linux", "macos", "darwin"} {
		if ladder := Ladder(platform, "rg"); len(ladder) != 1 || ladder[0] != "rg" {
			t.Errorf("the %s ladder = %v, want [rg]", platform, ladder)
		}
	}
	// An empty key means this machine.
	want = []string{"rg"}
	if runtime.GOOS == "windows" {
		want = []string{"rg.exe", "rg.cmd", "rg.bat", "rg.com", "rg"}
	}
	if got := Ladder("", "rg"); len(got) != len(want) {
		t.Fatalf("the empty key gave %v, want %v", got, want)
	}
	for i := range want {
		if Ladder("", "rg")[i] != want[i] {
			t.Fatalf("the empty key gave %v, want %v", Ladder("", "rg"), want)
		}
	}
}
