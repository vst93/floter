package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTildeExpansionHasOneAnswer pins the收口 P11-t did: "what does a leading
// `~` mean" is answered by `internal/homepath` and nowhere else.
//
// Before the round there were four hand-written expansions and two answers.
// The action bar normalised the separator *before* looking for `~/`, so on
// Windows `~/notes` became `~\notes`, the prefix cut missed, and the row
// offered `C:\Users\<u>\~\notes` as the path to open and to copy. The connect
// link's manifest joined what was left after `~` straight to home, so `~alice`
// meant `home/alice` there and `home/~alice` everywhere else — the reference's
// `resolve_path` (`src-tauri/src/commands/actions.rs:92-105`) answers the
// latter.
//
// The scan looks for the two ways a hand-written expansion starts: cutting the
// `~` off with `strings.CutPrefix` or `strings.TrimPrefix`. A bare
// `strings.HasPrefix(x, "~")` is not flagged: it is how a *classification*
// asks "does this look like a device path" (`internal/extensions/sync.go`) and
// how a terminal link decides a value is absolute-ish (`internal/shell`), and
// neither expands anything.
func TestTildeExpansionHasOneAnswer(t *testing.T) {
	needles := []string{`strings.CutPrefix(`, `strings.TrimPrefix(`}

	offenders := []string{}
	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The single implementation lives here.
				if d.Name() == "homepath" {
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
				if !strings.Contains(line, `"~`) {
					continue
				}
				for _, needle := range needles {
					if !strings.Contains(line, needle) {
						continue
					}
					rel, relErr := filepath.Rel(filepath.Join("..", ".."), path)
					if relErr != nil {
						return relErr
					}
					offenders = append(offenders, rel+": "+strings.TrimSpace(line))
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", root, err)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("a tilde is expanded outside internal/homepath; forward to homepath.Expand/Resolve instead:\n%s",
			strings.Join(offenders, "\n"))
	}
}
