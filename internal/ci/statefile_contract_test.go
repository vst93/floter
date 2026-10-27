package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStateFilesHaveOneWriter pins the收口 P11-k started and P11-l finished:
// every state file the app persists goes through `statefile.Write`, the single
// `temp → write → fsync → chmod → rename → dir fsync` implementation. The
// P11-k round rewrote four call sites onto it; P11-l moved the remaining four
// (calculator, extensions config ×2, extensions repository, terminalui).
//
// Without this scan the收口 is only a convention: a caller can quietly grow a
// hand-rolled temp block again and every existing assertion (mode, no leftover
// temp file, rollback-on-failure) still passes, because none of them look at
// how the file was written — only that it was.
//
// One `os.CreateTemp` is allowed outside `statefile`: `internal/apps/icns.go`
// makes a *scratch* PNG in the OS temp dir for the `sips` conversion, reads it
// back, and removes it. That is not a state file — nothing is persisted, the
// dir is `""` (the OS temp dir, not a config root), and the file is gone before
// the call returns — so it is not the pattern this pin is about.
func TestStateFilesHaveOneWriter(t *testing.T) {
	allowed := map[string]string{
		// file (slash path, relative to the repo root) → why it may keep one.
		"internal/apps/icns.go": "a scratch PNG in the OS temp dir for sips, read back and removed — not a persisted state file",
	}

	roots := []string{"internal", "cmd"}
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The single implementation lives here; it is the one place
				// allowed to spell the temp-file dance.
				if d.Name() == "statefile" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			// Test files cannot persist production state (they do not ship and
			// nothing in the product calls them), so a scratch temp file in a
			// test is out of scope — and this scan's own needle would match
			// itself.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if !strings.Contains(string(data), "os.CreateTemp") {
				return nil
			}
			// Normalise to a slash path relative to the repo root so the
			// allow-list reads the same on every platform.
			rel := filepath.ToSlash(path)
			rel = strings.TrimPrefix(rel, "../../")
			if _, ok := allowed[rel]; ok {
				return nil
			}
			t.Errorf("%s calls os.CreateTemp: state files must go through statefile.Write (see internal/statefile); if this is a genuine scratch file, add it to the allow-list with a reason", rel)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}
