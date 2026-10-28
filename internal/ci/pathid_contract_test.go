package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPathIdentityHasOneAnswer is the contract `internal/pathid` states in its
// package comment: "do these two strings name the same file" is asked in one
// place.
//
// The repository used to answer it six ways — `sameExecutable` twice (one said
// `("", "")` was the same program, the other said it was not), a hand-built
// spelling set in the Detected list, `normalized`, `sameRoot`, and a
// `strings.EqualFold` in the settings migration that folded case on Linux where
// the filesystem does not. Two of those answers were live bugs. This scan fails
// on the seventh spelling instead of on a user's machine.
//
// What is banned, and why the exceptions are exceptions:
//
//   - A function named `sameExecutable` / `samePath` outside `internal/pathid`
//     is a second answer to the identity question.
//   - `filepath.EvalSymlinks` outside `internal/pathid` is the raw material of
//     that second answer. Three callers keep it legitimately:
//     `inventory.fingerprint` (a *content* key — resolved path + size + mtime —
//     not a comparison), `clipboard.canonicalPath` (the resolved spelling is
//     what the files list hashes, not a pair being compared) and `pathid`
//     itself.
//   - The call sites have to stay wired to `pathid`, or the seam is decorative.
func TestPathIdentityHasOneAnswer(t *testing.T) {
	// The raw-material exception list, as repo-relative slash paths.
	allowEvalSymlinks := map[string]bool{
		"internal/inventory/inventory.go": true,
		"internal/clipboard/files.go":     true,
	}
	// The call sites that used to spell their own comparison.
	mustUsePathid := []string{
		"internal/extensions/candidates.go",
		"internal/inventory/inventory.go",
		"internal/inventory/sources.go",
		"internal/settings/migrate.go",
		"internal/shell/detectedtools.go",
		"internal/shell/system.go",
	}
	usesPathid := map[string]bool{}

	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The single implementation lives here.
				if d.Name() == "pathid" {
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
			rel := strings.TrimPrefix(filepath.ToSlash(path), "../../")
			source := string(data)
			if strings.Contains(source, "floter/internal/pathid") {
				usesPathid[rel] = true
			}
			for _, line := range strings.Split(source, "\n") {
				code := line
				if comment := strings.Index(code, "//"); comment >= 0 {
					code = code[:comment]
				}
				switch {
				case strings.Contains(code, "func sameExecutable("),
					strings.Contains(code, "func samePath("):
					t.Errorf("%s defines a second identity comparison: compare with pathid.Same (see internal/pathid)", rel)
				case strings.Contains(code, "filepath.EvalSymlinks") && !allowEvalSymlinks[rel]:
					t.Errorf("%s resolves symlinks itself: compare with pathid.Same, or take every spelling from pathid.Keys (see internal/pathid)", rel)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	for _, rel := range mustUsePathid {
		if !usesPathid[rel] {
			t.Errorf("%s no longer imports internal/pathid: the identity seam has to stay wired", rel)
		}
	}
}
