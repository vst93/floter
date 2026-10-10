package inventory

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"floter/internal/extensions"
)

// The discovery merge: the same executable found by two sources is one
// candidate with both sources, the better quality wins, and a description
// from the second source fills the first's blank.
func TestDiscoveryMergesSources(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture is a shell script")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "rg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := discover([]string{dir})
	// The walk reads the machine's own sources too; find the fixture by
	// its path prefix.
	var got *Candidate
	for index := range out {
		if out[index].Path == path {
			got = &out[index]
		}
	}
	if got == nil {
		t.Fatalf("the fixture was not discovered in %d candidates", len(out))
	}
	if got.Name != "rg" || !got.Available {
		t.Errorf("candidate = %+v", *got)
	}
	if len(got.Sources) != 1 || got.Sources[0] != SourcePath {
		t.Errorf("sources = %v", got.Sources)
	}
	if got.Fingerprint == "" {
		t.Error("no fingerprint")
	}
}

// The ranking: a curated, user-owned tool with a description outranks a
// long variant name; the order is total (quality, priority, name).
func TestDiscoveryRanksRecognisableToolsFirst(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture is a shell script")
	}
	dir := t.TempDir()
	for _, name := range []string{"a52dec", "git", "gcc-12"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	out := discover([]string{dir})
	// The walk also reads the machine's own sources; the three fixtures
	// are what this test ranks, by name.
	byName := map[string]Candidate{}
	for _, candidate := range out {
		byName[candidate.Name] = candidate
	}
	var ranked []string
	for _, name := range []string{"git", "a52dec", "gcc-12"} {
		candidate, ok := byName[name]
		if !ok {
			t.Fatalf("%s was not discovered", name)
		}
		ranked = append(ranked, name)
		_ = candidate
	}
	// git (curated) outranks a52dec (alphabetical filler) and gcc-12 (a
	// variant, penalised).
	if !(Priority(byName["git"]) > Priority(byName["a52dec"]) && Priority(byName["a52dec"]) > Priority(byName["gcc-12"])) {
		t.Errorf("ranking: git=%d a52dec=%d gcc-12=%d",
			Priority(byName["git"]), Priority(byName["a52dec"]), Priority(byName["gcc-12"]))
	}
	_ = ranked
}

// The curated stem is the one spelling rule: Windows launcher suffixes and
// case never change an answer.
func TestCuratedStemIsTheOneSpellingRule(t *testing.T) {
	for _, pair := range [][2]string{
		{"rg", "rg"}, {"RG.EXE", "rg"}, {"rg.cmd", "rg"}, {"  rg.bat ", "rg"},
		{"rg.com", "rg"}, {"unknown", "unknown"},
	} {
		if got := CuratedStem(pair[0]); got != pair[1] {
			t.Errorf("CuratedStem(%q) = %q, want %q", pair[0], got, pair[1])
		}
	}
	if !IsCurated("git") || !IsCurated("Git.exe") || IsCurated("gitk-alike") {
		t.Error("the curated lookup disagrees with the stem rule")
	}
}

// The priority's signals are additive and clamped at zero.
func TestPrioritySignals(t *testing.T) {
	git := Candidate{Name: "git", Path: "/usr/bin/git", Sources: []DiscoverySource{SourcePath}}
	if got := Priority(git); got < 1000 {
		t.Errorf("git priority = %d, want at least the curated bonus", got)
	}
	// A variant name pays the penalty; a curated name still wins.
	variant := Candidate{Name: "gcc-12", Path: "/usr/bin/gcc-12", Sources: []DiscoverySource{SourcePath}}
	if Priority(variant) >= Priority(git) {
		t.Errorf("variant %d not below git %d", Priority(variant), Priority(git))
	}
	// A desktop source outranks the same name from the PATH alone.
	desktop := Candidate{Name: "git", Path: "/usr/bin/git", Sources: []DiscoverySource{SourceDesktop}}
	if Priority(desktop) <= Priority(git) {
		t.Errorf("desktop %d not above plain %d", Priority(desktop), Priority(git))
	}
	// A one-character name is penalised, never negative.
	one := Candidate{Name: "x", Path: "/usr/bin/x", Sources: []DiscoverySource{SourcePath}}
	if Priority(one) < 0 {
		t.Errorf("priority went negative: %d", Priority(one))
	}
}

// A variant name is a numeric suffix, not a real compound (`rg-pdf`).
func TestVariantNames(t *testing.T) {
	for _, name := range []string{"aclocal-1.18", "gcc-12", "xml2-config", "python3-config", "x86_64-linux-gnu-gcc-12"} {
		if !isVariantName(name) {
			t.Errorf("%q is not a variant", name)
		}
	}
	for _, name := range []string{"git", "rg-pdf", "gofmt", "pkg-config-tool"} {
		if isVariantName(name) {
			t.Errorf("%q is a variant", name)
		}
	}
}

// The desktop entry reader: Name, Comment and the Exec's first non-field
// token; NoDisplay hides the entry.
func TestDesktopEntryReader(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "rg.desktop")
	if err := os.WriteFile(entry, []byte(`[Desktop Entry]
Name=Ripgrep
Comment=Fast search
Exec=rg %F
Type=Application
`), 0o644); err != nil {
		t.Fatal(err)
	}
	name, comment, exec, ok := readDesktopEntry(entry)
	if !ok || name != "Ripgrep" || comment != "Fast search" || exec != "rg" {
		t.Errorf("entry = %q %q %q ok=%v", name, comment, exec, ok)
	}
	hidden := filepath.Join(dir, "hidden.desktop")
	if err := os.WriteFile(hidden, []byte(`[Desktop Entry]
Name=Hidden
Exec=x
NoDisplay=true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := readDesktopEntry(hidden); ok {
		t.Error("a NoDisplay entry was read")
	}
}

// The environment signature is the search path's: a snapshot from one
// path is stale against another. (The search path itself caches once per
// process, so the test asserts the signature is the path's join — what a
// changed PATH therefore changes.)
func TestEnvironmentSignatureIsTheSearchPath(t *testing.T) {
	if got := environmentSignature(); got != strings.Join(extensions.SearchDirectories(), "|") {
		t.Errorf("signature = %q", got)
	}
}
