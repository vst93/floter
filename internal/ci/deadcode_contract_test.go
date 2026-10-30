package ci

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// deadCodeBaselineEntries is the account P11-cr wrote: the functions
// `deadcode` measured as having no caller (24 of them — 18 in production code,
// 6 private helpers in test files), which that round deleted.
//
// The number is pinned on purpose. `TestTheDeadCodeBaselineStaysDeleted` reads
// the ledger next to it and requires:
//
//   - **every entry still has no `func` definition** in the package it names,
//     so re-adding one of these orphans turns the suite red instead of waiting
//     for the next person to remember to run `deadcode`; and
//   - **the ledger lists exactly this many entries**, so the list cannot be
//     widened (a new "dead" claim needs its own evidence and its own round) or
//     quietly trimmed (deleting a line is how a pin gets around itself).
//
// Regenerating the list needs the tool, which needs the network and does not
// run on a cross-compiled GOOS:
//
//	go run golang.org/x/tools/cmd/deadcode@v0.52.0 -test ./...
//
// P11-cr measured 25 lines → 1 after the deletions. The remaining line is a
// **false positive** and is deliberately not in the ledger: `shell`'s
// `dressWindowsFrame` is the `!windows` stub of a function `chrome_windows.go`
// calls, and `deadcode` compiles for Linux so it cannot see the windows arm
// (see the note at the bottom of the ledger).
const deadCodeBaselineEntries = 24

// deadCodeBaseline is the ledger's path, relative to this package.
const deadCodeBaseline = "deadcode_baseline.txt"

// TestTheDeadCodeBaselineStaysDeleted is P11-cr's account of the repository's
// dead code: the 24 zero-caller functions that round deleted stay deleted.
//
// It is a source scan, not a `deadcode` run. The tool needs the network (it is
// a `go run …@version`) and cannot run for a cross-compiled GOOS (P11-cr
// measured `fork/exec …deadcode.exe: exec format error` under `GOOS=windows`),
// so a CI-shaped pin cannot invoke it. What the tool found once is therefore
// written down as a ledger and pinned here — and what the pin guards is the
// thing that actually rots: someone re-adding one of these functions, or
// someone editing the ledger to make a red go away.
func TestTheDeadCodeBaselineStaysDeleted(t *testing.T) {
	data, err := os.ReadFile(deadCodeBaseline)
	if err != nil {
		t.Fatalf("the dead-code ledger is unreadable: %v", err)
	}
	type entry struct{ path, symbol string }
	var entries []entry
	seen := map[string]bool{}
	for index, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("%s:%d: want `<path> <symbol>`, got %q", deadCodeBaseline, index+1, line)
		}
		key := fields[0] + " " + fields[1]
		if seen[key] {
			t.Errorf("%s:%d: duplicated entry %q", deadCodeBaseline, index+1, key)
		}
		seen[key] = true
		entries = append(entries, entry{path: fields[0], symbol: fields[1]})
	}
	if len(entries) != deadCodeBaselineEntries {
		t.Errorf("the dead-code ledger lists %d entries, this round wrote %d: "+
			"adding a claim needs its own `deadcode` measurement, and removing one "+
			"is how the pin would get around itself", len(entries), deadCodeBaselineEntries)
	}

	for _, e := range entries {
		path := filepath.Join("..", "..", e.path)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: the ledger names a file that does not exist: %v", e.path, err)
			continue
		}
		// The declaration, in any receiver shape: `func Name(`, `func (p T) Name(`.
		decl := regexp.MustCompile(`func\s+(\([^)]*\)\s*)?` + regexp.QuoteMeta(e.symbol) + `\s*\(`)
		dir := filepath.Dir(path)
		matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		sort.Strings(matches)
		for _, file := range matches {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			for index, line := range strings.Split(string(source), "\n") {
				if !decl.MatchString(line) {
					continue
				}
				t.Errorf("%s:%d: `%s` is back — the ledger says it has no caller "+
					"(P11-cr measured it with `deadcode -test ./...`); delete it again, "+
					"or give it a caller and take it off the ledger in the same round",
					file, index+1, e.symbol)
			}
		}
	}
}

// TestTheDeadCodeFalsePositiveStaysOut is the other half of the account: the
// one `deadcode` line P11-cr left standing must stay standing, and must stay
// off the ledger.
//
// `dressWindowsFrame` lives in `chrome_other.go` (the `!windows` arm) and does
// nothing. `deadcode` compiles for Linux, so it cannot see `chrome_windows.go`
// — whose `stripSystemChrome` calls it. Deleting the stub would break the
// Windows build, so the tool's line is a blind spot, not a finding. That
// judgement has to be written down or the next `deadcode` run will "rediscover"
// it and someone will delete a function the Windows build needs.
func TestTheDeadCodeFalsePositiveStaysOut(t *testing.T) {
	ledger, err := os.ReadFile(deadCodeBaseline)
	if err != nil {
		t.Fatalf("the dead-code ledger is unreadable: %v", err)
	}
	for index, line := range strings.Split(string(ledger), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			// The ledger's own note names this symbol to explain why it is
			// absent; only entry lines are claims.
			continue
		}
		if strings.Contains(line, "dressWindowsFrame") {
			t.Errorf("%s:%d: `dressWindowsFrame` is the `!windows` stub of a function "+
				"`chrome_windows.go` calls — it is not dead and must not be on the ledger",
				deadCodeBaseline, index+1)
		}
	}

	stub := filepath.Join("..", "shell", "chrome_other.go")
	source, err := os.ReadFile(stub)
	if err != nil {
		t.Fatalf("%s: %v", stub, err)
	}
	if !strings.Contains(string(source), "func dressWindowsFrame(") {
		t.Errorf("%s dropped the `!windows` stub of `dressWindowsFrame`: the windows "+
			"arm (`chrome_windows.go`) calls it, and there is nothing to call there "+
			"without it", stub)
	}
	// And the arm that calls it must still be there, or the stub really is dead.
	user := filepath.Join("..", "shell", "chrome_windows.go")
	windows, err := os.ReadFile(user)
	if err != nil {
		t.Fatalf("%s: %v", user, err)
	}
	if !strings.Contains(string(windows), "dressWindowsFrame(hwnd)") {
		t.Errorf("%s no longer calls `dressWindowsFrame`: the header's premise is gone",
			user)
	}
}
