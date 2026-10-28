package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDesktopEntriesHaveOneReader pins the收口 P11-s did: "what does this
// `.desktop` file say" is answered by `internal/desktopentry` and nowhere else.
//
// Before the round there were two readers and three answers. `internal/apps`
// had the locale ladder, the string escapes, `TryExec` and `DBusActivatable`;
// `internal/inventory` had none of them, took the first value of a repeated key
// instead of the last, and read an `Exec=` line with its own splitter. The
// splitter disagreed with the application scan's on two points that matter: a
// single quote grouped in one and not the other, and a field code the
// specification does not define (`%z`) was dropped by one and made the whole
// line unusable for the other. The reference implementation
// (`src-tauri/src/commands/apps/linux.rs` on `main`) keeps such a code as
// written, which is the reading both call sites now get.
//
// Without this scan the收口 is a convention: a third reader can grow in a
// package that has nothing to do with desktop entries, and every behavioural
// assertion still passes on the fixtures that package happens to use.
//
// The scan looks for the section header, the one string every reader of a
// desktop entry has to match. `internal/desktopentry` is the one allowed
// place. Test files are out of scope: they do not ship, and this scan's own
// needle would match it.
func TestDesktopEntriesHaveOneReader(t *testing.T) {
	const needle = `"[Desktop Entry]"`

	offenders := []string{}
	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The single implementation lives here.
				if d.Name() == "desktopentry" {
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
				if !strings.Contains(line, needle) {
					continue
				}
				rel, relErr := filepath.Rel(filepath.Join("..", ".."), path)
				if relErr != nil {
					rel = path
				}
				offenders = append(offenders, filepath.ToSlash(rel)+": "+strings.TrimSpace(line))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("desktop entries are read in more than one place; move the reading into internal/desktopentry:\n%s",
			strings.Join(offenders, "\n"))
	}
}
