package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floter/internal/extensions"
	"floter/internal/versioning"
)

// TestVersionOrderingHasOneAnswer pins the收口 P11-u did: "of two version
// numbers, which is newer" is answered by `internal/semver` and nowhere else.
//
// Before the round there were two implementations and they disagreed. A
// manifest's version was ordered the way semantic versioning 2.0.0 says
// (`internal/extensions/semver.go` `comparePrerelease`: numeric identifiers
// compare as numbers), while a release tag's version compared the prerelease
// suffix as text (`internal/versioning` `Less`: `v.Pre < other.Pre`). So
// `0.3.26-preview.10` was newer than `0.3.26-preview.9` to one package and
// older to the other. Only the numeric core decides what `Next` computes, so
// the wrong answer never reached a release — but the package whose whole reason
// for existing is "a version number must have one meaning" cannot keep a second
// ordering rule of its own.
//
// The scan looks for the shape the bug had: a comparison of the prerelease
// field as a string. `internal/semver` is where that comparison now lives, so
// it is skipped — everything else must call it.
func TestVersionOrderingHasOneAnswer(t *testing.T) {
	needles := []string{".Pre <", ".Pre >", ".Prerelease <", ".Prerelease >", "func comparePrerelease"}

	offenders := []string{}
	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The single implementation lives here.
				if d.Name() == "semver" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, line := range strings.Split(string(data), "\n") {
				for _, needle := range needles {
					if !strings.Contains(line, needle) {
						continue
					}
					rel, relErr := filepath.Rel(filepath.Join("..", ".."), path)
					if relErr != nil {
						rel = path
					}
					offenders = append(offenders, rel+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("a prerelease is ordered outside internal/semver:\n%s\n"+
			"versions are ordered by semver.Compare (P11-u)", strings.Join(offenders, "\n"))
	}
}

// TestTheTwoVersionPackagesOrderTheSameWay is the behavioural half: the two
// parsers keep their own grammars (a release tag may be `mygo-v0.3.25`, an
// extension may write `1.0`), but every pair they both accept must be ordered
// the same way. A table that only one side agrees with is the bug P11-u fixed.
func TestTheTwoVersionPackagesOrderTheSameWay(t *testing.T) {
	pairs := [][2]string{
		{"1.0.0", "1.0.1"},
		{"1.9.0", "1.10.0"},
		{"0.3.26", "0.3.27-preview.1"},
		{"0.3.26-preview.1", "0.3.26"},
		{"0.3.26-preview.9", "0.3.26-preview.10"},
		{"0.3.26-preview.10", "0.3.26-preview.9"},
		{"1.0.0-rc.2", "1.0.0-rc.10"},
		{"1.0.0-1", "1.0.0-alpha"},
		{"1.0.0-alpha", "1.0.0-alpha.1"},
		{"2.0.0", "2.0.0"},
	}
	for _, pair := range pairs {
		manifestA, err := extensions.ParseSemver(pair[0])
		if err != nil {
			t.Fatalf("extensions.ParseSemver(%q): %v", pair[0], err)
		}
		manifestB, err := extensions.ParseSemver(pair[1])
		if err != nil {
			t.Fatalf("extensions.ParseSemver(%q): %v", pair[1], err)
		}
		tagA, err := versioning.Parse(pair[0])
		if err != nil {
			t.Fatalf("versioning.Parse(%q): %v", pair[0], err)
		}
		tagB, err := versioning.Parse(pair[1])
		if err != nil {
			t.Fatalf("versioning.Parse(%q): %v", pair[1], err)
		}
		if got, want := tagOrder(tagA, tagB), manifestA.Compare(manifestB); got != want {
			t.Errorf("%s vs %s: versioning says %d, extensions says %d", pair[0], pair[1], got, want)
		}
	}
}

// tagOrder is `versioning`'s answer as a sign, so it can be compared with
// `extensions`' `Compare`.
func tagOrder(a, b versioning.Version) int {
	switch {
	case a.Less(b):
		return -1
	case b.Less(a):
		return 1
	default:
		return 0
	}
}
