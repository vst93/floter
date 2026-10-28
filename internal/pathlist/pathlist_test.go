package pathlist

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Split is the one way a PATH-shaped value becomes directories, and the two
// things it decides are visible here: the empty string names nothing, and an
// empty entry in the middle names nothing either. The fixture is built with the
// platform's own separator, so the same assertion holds on all three.
func TestSplitDropsTheEmptyEntries(t *testing.T) {
	if got := Split(""); len(got) != 0 {
		t.Errorf("Split(\"\") = %v, want no entries", got)
	}
	sep := string(os.PathListSeparator)
	got := Split(strings.Join([]string{"/one", "", "/two", "", ""}, sep))
	if want := []string{"/one", "/two"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Split = %v, want %v", got, want)
	}
	// No trimming: a directory may legitimately end in a space, and the one
	// caller that wants trimming (the XDG list) does it itself.
	if got := Split("/a b" + sep + " /c"); !reflect.DeepEqual(got, []string{"/a b", " /c"}) {
		t.Errorf("Split trimmed an entry: %v", got)
	}
}

// The reason this package exists: Windows PATH entries may be quoted, and
// `filepath.SplitList` is the only splitter that strips those quotes. On Unix
// there is no quoting to strip, so the entry keeps its characters. Both arms
// are asserted here — no skip, no build tag — so the test runs everywhere and
// says which platform it is on when it fails.
func TestSplitUnquotesOnWindowsOnly(t *testing.T) {
	// The fixture is a single entry on the platform it is written for: a
	// Windows path contains its own colon, so feeding it to the Unix splitter
	// would be testing the colon, not the quoting.
	entry := `"/opt/Program Files/Git/cmd"`
	want := entry
	if runtime.GOOS == "windows" {
		entry = `"C:\Program Files\Git\cmd"`
		want = `C:\Program Files\Git\cmd`
	}
	got := Split(entry)
	if len(got) != 1 {
		t.Fatalf("Split(%q) = %v on %s, want one entry", entry, got, runtime.GOOS)
	}
	if got[0] != want {
		t.Errorf("Split(%q)[0] = %q on %s, want %q", entry, got[0], runtime.GOOS, want)
	}
}

// A name that carries a separator is a path, not a PATH entry. Joining it to
// every directory instead makes `<PATH dir>/bin/foo` a hit for the name
// `bin/foo` — the bug `inventory.LookPath` used to have.
func TestLookupShortCircuitsOnASeparator(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "bin")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(nested, "foo")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if found, ok := lookupIn(dir, filepath.Join("bin", "foo")); ok {
		t.Errorf("a relative name with a separator resolved to %q", found)
	}
	if found, ok := lookupIn(dir, program); !ok || found != program {
		t.Errorf("an absolute program path = (%q, %v), want (%q, true)", found, ok, program)
	}
	// A relative path is judged as a path too — once the working directory is
	// the one that holds it. An empty PATH does *not* stand in for the working
	// directory here: only a bare name asks PATH, and PATH says nothing.
	t.Chdir(nested)
	if found, ok := lookupIn("", "./foo"); !ok || found != "./foo" {
		t.Errorf("a relative path = (%q, %v), want (\"./foo\", true)", found, ok)
	}
	if found, ok := lookupIn("", "foo"); ok {
		t.Errorf("an empty PATH resolved the bare name to %q", found)
	}
}

// PATH is ordered: the first directory that carries the name wins, and the
// name is returned with the directory that supplied it.
func TestLookupWalksPathInOrder(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, dir := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(dir, "tool"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	sep := string(os.PathListSeparator)
	found, ok := lookupIn(strings.Join([]string{first, second}, sep), "tool")
	if !ok || found != filepath.Join(first, "tool") {
		t.Errorf("Lookup = (%q, %v), want (%q, true)", found, ok, filepath.Join(first, "tool"))
	}
	// The empty entry names no directory here; the name is simply absent.
	if found, ok := lookupIn(strings.Join([]string{"", ""}, sep), "tool"); ok {
		t.Errorf("an empty PATH found %q", found)
	}
}

// One answer, not two: the probe the app scan uses and the walk the inventory
// uses are this package's walk.
func TestLookupIsTheSameAnswerEverywhere(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "shared")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	found, ok := Lookup("shared")
	if !ok || found != program {
		t.Fatalf("Lookup = (%q, %v), want (%q, true)", found, ok, program)
	}
	// A directory is not a program, and a name nobody has is not found.
	if err := os.Mkdir(filepath.Join(dir, "adirectory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if found, ok := Lookup("adirectory"); ok {
		t.Errorf("a directory resolved to %q", found)
	}
	if found, ok := Lookup("ghost"); ok {
		t.Errorf("a name absent from PATH resolved to %q", found)
	}
}
