package extensions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// One process, one rule: every extension process is created and waited for by
// `proctree.go`. A new `exec.Command` in another file is how the six spawn
// sites drifted apart in the first place (some in a session, some in no group,
// all of them killing only the direct child), so the rule is a source pin
// rather than a convention.
func TestEveryExtensionProcessGoesThroughOneConstructor(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"exec.Command(", "exec.CommandContext(", ".Run()", ".Output()"}
	var offenders []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// proctree.go is the constructor itself; proctree_unix.go and
		// proctree_windows.go are its platform arms. Nothing else may spawn.
		if strings.HasPrefix(name, "proctree") {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(".", name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, line := range strings.Split(string(body), "\n") {
			for _, pattern := range forbidden {
				if strings.Contains(line, pattern) {
					offenders = append(offenders, name+": "+strings.TrimSpace(line))
				}
			}
		}
	}
	if len(offenders) > 0 {
		t.Errorf("these spawn or wait outside proctree.go:\n%s", strings.Join(offenders, "\n"))
	}
}
