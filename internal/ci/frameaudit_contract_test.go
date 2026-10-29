package ci

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestTheFrameAuditHasOneRule is the contract `internal/frameaudit` states in its
// package comment: 「没有两段字压在同一块像素上，也没有一段字跑出窗口」 is one
// rule with one implementation.
//
// It exists because the same rule was carried by three test files
// (`internal/launcher/textframe_test.go`, `internal/settingsui/textframe_test.go`,
// `internal/terminalui/draweroverlap_test.go`), each with its own copy of
// `encloses` and its own `if r.W >= w-1 { continue }`. The skip was written as an
// optimisation — a full-window ancestor contains every run, so `Encloses` would
// exempt every pair anyway — but it also dropped the **full-width leaves** (a
// banner spanning the panel), and those were never measured: P11-bx's mutation
// put the settings banner at six times its font size and the whole suite stayed
// green.
//
// So the scan bans, in those three packages' test files:
//
//   - the window-width skip (`w-1` / `w - 1`, however spaced), which is how the
//     blind spot was written both times, and
//   - a local `func encloses`/`func enclosesBox`, which is how the rule grew three
//     answers.
//
// and requires each of the three surfaces to reach the rule through
// `floter/internal/frameaudit`.
func TestTheFrameAuditHasOneRule(t *testing.T) {
	surfaces := map[string]string{
		"launcher/textframe_test.go":       "internal/launcher",
		"settingsui/textframe_test.go":     "internal/settingsui",
		"terminalui/draweroverlap_test.go": "internal/terminalui",
	}
	for file, dir := range surfaces {
		path := filepath.Join("..", "..", "internal", file)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if !strings.Contains(string(data), "\"floter/internal/frameaudit\"") {
			t.Errorf("%s does not use internal/frameaudit: the rule must have one implementation", file)
		}
		if !strings.Contains(string(data), "frameaudit.Audit(") {
			t.Errorf("%s does not call frameaudit.Audit: it carries its own copy of the rule", file)
		}
		for index, line := range strings.Split(string(data), "\n") {
			code := line
			if comment := strings.Index(code, "//"); comment >= 0 {
				code = code[:comment]
			}
			flat := strings.ReplaceAll(code, " ", "")
			where := file + ":" + strconv.Itoa(index+1)
			switch {
			case strings.Contains(flat, "w-1"):
				t.Errorf("%s: %q — the window-width skip is what made full-width runs invisible; "+
					"leave it to frameaudit.Encloses (P11-by)", where, strings.TrimSpace(line))
			case strings.Contains(flat, "funcencloses"):
				t.Errorf("%s: %q — the rule's containment test lives in internal/frameaudit (P11-by)",
					where, strings.TrimSpace(line))
			}
		}
		// The package the file lives in is only named so a future surface that
		// copies this scan has the mapping at hand.
		if _, err := os.Stat(filepath.Join("..", "..", dir)); err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
	}
}
