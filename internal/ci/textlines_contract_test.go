package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTextLinesHaveOneReader pins the收口 P11-r did: "a text blob, as lines" is
// answered by `internal/textlines.Split` and nowhere else.
//
// Before the round there were seven answers — six `strings.Split(text, "\n")`
// and one `bufio.Scanner` — and they disagreed on the two framing bytes a
// platform adds: the CR of a Windows CRLF, and the UTF-8 BOM a Windows editor
// writes. Two call sites hand-trimmed the CR, one hand-trimmed the BOM, four
// trimmed neither, and the consequences were real:
//
//   - `--help` output from a Windows host lost every option's value-taking
//     status (and every description-less switch disappeared outright), because
//     the CR glued itself onto the last token of the line;
//   - a `.desktop` file with a BOM vanished from the application list while the
//     tools list still showed it, because U+FEFF is not whitespace.
//
// Without this scan the收口 is a convention: a new parser can split on "\n"
// again and every behavioural assertion still passes on Linux, where the CR
// never appears.
//
// The scan looks for the three spellings of the old framing:
//
//	strings.Split(<x>, "\n")      — a line split that ignores the CR
//	strings.TrimSuffix(<x>, "\r") — a hand-rolled CR trim
//	strings.TrimPrefix(<x>, "\ufeff") — a hand-rolled BOM trim
//
// `internal/textlines` is the one allowed place. Test files are out of scope:
// they do not ship, and this scan's own needles would match it.
func TestTextLinesHaveOneReader(t *testing.T) {
	needles := []string{
		`strings.Split(`,
		`TrimSuffix(`,
		`TrimPrefix(`,
	}
	// The argument each needle must be looking at to count as line framing.
	arguments := []string{
		`"\n"`,
		`"\r"`,
		`"\ufeff"`,
	}

	roots := []string{"internal", "cmd"}
	offenders := []string{}
	for _, root := range roots {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// The single implementation lives here.
				if d.Name() == "textlines" {
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
				for i, needle := range needles {
					if strings.Contains(line, needle) && strings.Contains(line, arguments[i]) {
						rel, relErr := filepath.Rel(filepath.Join("..", ".."), path)
						if relErr != nil {
							rel = path
						}
						offenders = append(offenders, filepath.ToSlash(rel)+": "+strings.TrimSpace(line))
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	for _, offender := range offenders {
		t.Errorf("text lines are read outside internal/textlines: %s", offender)
	}
}
