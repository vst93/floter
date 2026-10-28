package pathid

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The key is the fallback identity, and it folds case in exactly one place:
// Windows, where the filesystem does. On Linux and macOS the spelling keeps
// the user's case, so `/opt/RG` and `/opt/rg` stay two keys there.
func TestKeyFoldsCaseOnlyWhereTheFilesystemDoes(t *testing.T) {
	if got := Key(""); got != "" {
		t.Fatalf("Key(%q) = %q, want the empty path to stay empty", "", got)
	}

	upper := KeyFor("windows", "/tmp/Foo/Bar.exe")
	lower := KeyFor("windows", "/tmp/foo/bar.exe")
	if upper != lower {
		t.Fatalf("Windows keys differ by case: %q vs %q", upper, lower)
	}

	if KeyFor("linux", "/tmp/Foo/Bar.exe") == KeyFor("linux", "/tmp/foo/bar.exe") {
		t.Fatalf("the Linux key folded case: %q", KeyFor("linux", "/tmp/foo/bar.exe"))
	}
	if KeyFor("darwin", "/tmp/Foo/Bar.exe") == KeyFor("darwin", "/tmp/foo/bar.exe") {
		t.Fatalf("the darwin key folded case: %q", KeyFor("darwin", "/tmp/foo/bar.exe"))
	}

	// Cleaning is part of the key: `dir/./x` and `dir/x` are one spelling.
	dir := t.TempDir()
	if Key(filepath.Join(dir, ".", "x")) != Key(filepath.Join(dir, "x")) {
		t.Fatalf("Key did not clean: %q vs %q", Key(filepath.Join(dir, ".", "x")), Key(filepath.Join(dir, "x")))
	}
	// And the host entry point agrees with the named one.
	if Key(dir) != KeyFor(runtime.GOOS, dir) {
		t.Fatalf("Key(%q) = %q, KeyFor(%q, …) = %q", dir, Key(dir), runtime.GOOS, KeyFor(runtime.GOOS, dir))
	}
}

// A symlinked pair is one program, and so is a hardlinked pair — the second is
// the case no string comparison can see, and the reason `Same` asks the
// filesystem instead of only comparing keys.
func TestSameAsksTheFilesystem(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "rg")
	if err := os.WriteFile(real, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "fd")
	if err := os.WriteFile(other, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "rg-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlinks unavailable in the test directory: %v", err)
	}

	for _, testCase := range []struct {
		what        string
		left, right string
		want        bool
	}{
		{"the same path twice", real, real, true},
		{"a symlink and its target", link, real, true},
		{"the target and its symlink", real, link, true},
		{"two different programs", real, other, false},
		{"a path and nothing", real, "", false},
		{"nothing and a path", "", real, false},
		{"nothing and nothing", "", "", false},
		{"one spelling of a missing path", filepath.Join(dir, "gone"), filepath.Join(dir, ".", "gone"), true},
		{"two missing paths", filepath.Join(dir, "gone"), filepath.Join(dir, "gone2"), false},
	} {
		if got := Same(testCase.left, testCase.right); got != testCase.want {
			t.Errorf("%s: Same(%q, %q) = %v, want %v", testCase.what, testCase.left, testCase.right, got, testCase.want)
		}
	}

	// A hardlink shares the inode and nothing else: no spelling, no symlink.
	hard := filepath.Join(dir, "hard-rg")
	if err := os.Link(real, hard); err != nil {
		// Some volumes cannot hardlink; the symlink cases above already pin
		// the resolution half of `Same`.
		t.Logf("hardlink unavailable on this filesystem: %v", err)
	} else if !Same(hard, real) {
		t.Fatalf("Same(%q, %q) = false: a hardlink is the same file and no string key can say so", hard, real)
	}
}

// A case-insensitive volume answers for both spellings; a case-sensitive one
// keeps them apart. Both answers are the filesystem's, so the test asks the
// filesystem what it did rather than assuming a platform.
func TestSameFollowsTheVolumesCaseRules(t *testing.T) {
	dir := t.TempDir()
	upper := filepath.Join(dir, "Settings.json")
	if err := os.WriteFile(upper, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	lower := filepath.Join(dir, "settings.json")
	lowerInfo, lowerErr := os.Stat(lower)
	upperInfo, upperErr := os.Stat(upper)
	if upperErr != nil {
		t.Fatal(upperErr)
	}
	oneFile := lowerErr == nil && os.SameFile(upperInfo, lowerInfo)
	if got := Same(upper, lower); got != oneFile {
		t.Fatalf("Same(%q, %q) = %v, but the filesystem says oneFile = %v", upper, lower, got, oneFile)
	}
}

// Keys has to carry both spellings, or a set built from the integration's
// resolved path will not answer for the search path's symlink.
func TestKeysCarryEverySpelling(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "rg")
	if err := os.WriteFile(real, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "rg-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlinks unavailable in the test directory: %v", err)
	}
	if len(Keys("")) != 0 {
		t.Fatalf("Keys(%q) = %v, want none", "", Keys(""))
	}
	keys := Keys(link)
	if len(keys) != 2 {
		t.Fatalf("Keys(%q) = %v, want the link and its target", link, keys)
	}
	if keys[0] != Key(link) || keys[1] != Key(real) {
		t.Fatalf("Keys(%q) = %v, want [%q %q]", link, keys, Key(link), Key(real))
	}
	if keys := Keys(real); len(keys) != 1 || keys[0] != Key(real) {
		t.Fatalf("Keys(%q) = %v, want just its own key", real, keys)
	}
}

// The spelling answer is for path tables that describe another platform: the
// tool roots are Windows paths read on every host, so the Windows rules
// (either separator, no case) apply wherever the table is evaluated.
func TestSameSpellingFoldsWhatThisFilesystemCannotSee(t *testing.T) {
	for _, testCase := range []struct {
		left, right string
		same        bool
	}{
		{`C:\ProgramData\chocolatey\bin`, `C:\ProgramData\chocolatey\bin`, true},
		{`C:\ProgramData\chocolatey\bin`, `C:\ProgramData/chocolatey/bin`, true},
		{`C:\ProgramData\chocolatey\bin`, `c:\programdata\CHOCOLATEY\bin`, true},
		{`C:\ProgramData\chocolatey\bin`, `C:\ProgramData\chocolatey\bin\.`, true},
		{`C:\ProgramData\chocolatey\bin`, `D:\CorpData\chocolatey\bin`, false},
		{`C:\ProgramData\chocolatey\bin`, `C:\ProgramData\chocolatey`, false},
	} {
		if got := SameSpelling(testCase.left, testCase.right); got != testCase.same {
			t.Fatalf("SameSpelling(%q, %q) = %v, want %v", testCase.left, testCase.right, got, testCase.same)
		}
	}
}
