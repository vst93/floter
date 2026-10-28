package ci

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPathListsHaveOneSplitter is the contract `internal/pathlist` states in
// its package comment: a PATH-shaped value is split in exactly one place.
//
// It exists because the repository used to carry three splitters for the same
// question — `filepath.SplitList` in three files, and
// `strings.Split(value, string(os.PathListSeparator))` in three more. They
// disagree on Windows, where only `filepath.SplitList` strips the surrounding
// quotes an installer may write around a PATH entry (`"C:\Program Files\…"`),
// so the same machine found `git` through one code path and not through
// another. A future caller that spells its own split brings that disagreement
// back; this scan is what makes it fail here instead of on a user's Windows box.
//
// The banned spellings are the *splitters*, not the separator:
//
//   - `filepath.SplitList` and a `strings.Split` on the list separator name
//     directories. Banned outside `internal/pathlist`.
//   - `strings.Join(<dirs>, string(os.PathListSeparator))` writes a list back
//     out (`extensions.SearchPath`); it is the mirror of splitting and has no
//     second answer. Allowed.
//   - `os.PathSeparator` (singular) asks "does this name carry a separator",
//     which is a different question. Allowed.
func TestPathListsHaveOneSplitter(t *testing.T) {
	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The single implementation lives here.
				if d.Name() == "pathlist" {
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
			for index, line := range strings.Split(string(data), "\n") {
				code := line
				if comment := strings.Index(code, "//"); comment >= 0 {
					code = code[:comment]
				}
				where := rel + ":" + strconv.Itoa(index+1)
				switch {
				case strings.Contains(code, "filepath.SplitList"):
					t.Errorf("%s calls filepath.SplitList: split PATH-shaped values with pathlist.Split (see internal/pathlist)", where)
				case (strings.Contains(code, "os.PathListSeparator") || strings.Contains(code, "filepath.ListSeparator")) &&
					!strings.Contains(code, "strings.Join"):
					t.Errorf("%s splits on the list separator: split PATH-shaped values with pathlist.Split (see internal/pathlist)", where)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
}
