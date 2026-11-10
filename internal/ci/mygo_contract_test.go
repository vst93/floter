package ci

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// auditedFramework is the minimum release required by the integration audit
// in docs/rounds/P14-mygo-upgrade-report.md. The launcher filters physical-only
// key events introduced in 0.8; terminal prompt integration, live regions and
// custom animation settling use the newer APIs. Keep accidental downgrades
// from silently invalidating those contracts.
const auditedFramework = "v0.8.3"

// TestTheFrameworkIsPinnedAtOrAboveTheAuditedVersion fails when `go.mod` pins
// mygo below the audited release, and names both versions so that the fix is
// obvious: either raise the pin, or raise this constant after auditing again.
func TestTheFrameworkIsPinnedAtOrAboveTheAuditedVersion(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	pinned := ""
	for _, line := range strings.Split(string(data), "\n") {
		// Both spellings `go mod` writes: a line of its own, and a line of a
		// `require ( … )` block, whose fields start at the module path.
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == "require" {
			fields = fields[1:]
		}
		if len(fields) >= 2 && fields[0] == "github.com/egoist/mygo" {
			pinned = fields[1]
			break
		}
	}
	if pinned == "" {
		t.Fatalf("go.mod no longer requires github.com/egoist/mygo: the audit in docs/rounds/P14-mygo-upgrade-report.md assumes it does")
	}
	if cmp := compareVersions(pinned, auditedFramework); cmp < 0 {
		t.Fatalf("mygo is pinned at %s, below the audited %s: raise the pin, or re-audit ui/ and raise auditedFramework", pinned, auditedFramework)
	}
}

// TestTheScrollHandlersStayOutOfTheWheelPath is the contract P11-br states for
// the one framework change with no path here: from v0.7.3 the wheel walks the
// hit chain innermost out through `HandleInput`, so a handler that answers an
// `InputScroll` would swallow the wheel for every scroll container it sits in.
// This app has no such handler — the wheel belongs to the containers
// (`ListState`/`ScrollState`, pinned by `TestTheWheelScrollsTheListUnderThePointer`
// and its siblings). A first `ui.InputScroll` anywhere in the product is the
// moment that audit must be redone, not a line to slip in.
func TestTheScrollHandlersStayOutOfTheWheelPath(t *testing.T) {
	for _, root := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			// The ban is on product handlers: this scan and the pins around it
			// are tests, and the string they look for is in them.
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(data), "\n") {
				if strings.Contains(line, "InputScroll") {
					rel, _ := filepath.Rel(filepath.Join("..", ".."), path)
					t.Errorf("%s:%d asks for the wheel: %s\n`HandleInput` sees `InputScroll` before the scroll containers do (ui/input.go, mygo %s+). Re-audit that chain — or drop the handler — before pinning it here.", rel, i+1, strings.TrimSpace(line), auditedFramework)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scanning %s: %v", root, err)
		}
	}
}

var versionPattern = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)`)

// compareVersions compares two `vX.Y.Z` module versions, prerelease and build
// suffixes ignored, as this repository pins releases only.
func compareVersions(a, b string) int {
	pa, pb := versionPattern.FindStringSubmatch(a), versionPattern.FindStringSubmatch(b)
	if pa == nil || pb == nil {
		return 0
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x - y
		}
	}
	return 0
}
