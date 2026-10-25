package apps

import (
	"os"
	"path/filepath"
	"testing"
)

// Roots reads the environment, and this is the half no pin covered: the table
// itself is pinned by value in `TestLinuxRootsFollowTheSearchPath`, but nothing
// said that `Roots` actually *passes* `$XDG_DATA_HOME` and `$XDG_DATA_DIRS` into
// it. A machine whose applications live under a custom prefix is exactly where a
// hard-coded list finds nothing, and the user's own directory has to come first
// or its overrides lose.
func TestRootsReadsTheXDGEnvironment(t *testing.T) {
	dataHome := t.TempDir()
	userApps := filepath.Join(dataHome, "applications")
	if err := os.MkdirAll(userApps, 0o755); err != nil {
		t.Fatal(err)
	}
	extra := t.TempDir()
	extraApps := filepath.Join(extra, "applications")
	if err := os.MkdirAll(extraApps, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("XDG_DATA_DIRS", extra)

	roots := Roots()
	if len(roots) == 0 || roots[0] != userApps {
		t.Fatalf("Roots = %v, want %q first", roots, userApps)
	}
	found := false
	for _, root := range roots {
		if root == extraApps {
			found = true
		}
	}
	if !found {
		t.Fatalf("Roots = %v, want %q in it", roots, extraApps)
	}
}
