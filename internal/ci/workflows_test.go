package ci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoFile reads a file from the repository root. Tests run in their package
// directory, so internal/ci sits two levels below the root.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	path := filepath.Join("..", "..", filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// TestWindowsSmokeIsDispatchOnly pins the reason windows-smoke.yml exists: it
// is a manual, human-viewed diagnostic, never a gate. If a trigger other than
// workflow_dispatch sneaks in, the job starts running on every push and its
// screenshot gets mistaken for a pass/fail signal.
func TestWindowsSmokeIsDispatchOnly(t *testing.T) {
	workflow := repoFile(t, ".github/workflows/windows-smoke.yml")

	if !strings.Contains(workflow, "workflow_dispatch") {
		t.Fatal("windows-smoke.yml must be dispatchable by hand")
	}
	for _, banned := range []string{"push:", "pull_request:", "release:", "schedule:"} {
		if strings.Contains(workflow, banned) {
			t.Errorf("windows-smoke.yml must not trigger on %q", banned)
		}
	}
	// The staged build path this repo actually ships on Windows.
	if !strings.Contains(workflow, "windows/amd64") {
		t.Error("windows-smoke.yml must build windows/amd64")
	}
	// The evidence: a screenshot that is uploaded rather than discarded.
	if !strings.Contains(workflow, "CopyFromScreen") {
		t.Error("windows-smoke.yml must capture the desktop")
	}
	if !strings.Contains(workflow, "smoke-desktop.png") {
		t.Error("windows-smoke.yml must save the capture")
	}
	if !strings.Contains(workflow, "upload-artifact") {
		t.Error("windows-smoke.yml must upload the capture")
	}
}

// TestWindowsSmokeStaysOutOfTheReleaseChain mirrors the original repo's "must
// never call it" comment: the Go gate workflow may not depend on the smoke job,
// or every release build starts waiting on a 40-minute Windows screenshot.
func TestWindowsSmokeStaysOutOfTheReleaseChain(t *testing.T) {
	goWorkflow := repoFile(t, ".github/workflows/go.yml")
	if strings.Contains(goWorkflow, "windows-smoke") {
		t.Fatal("go.yml must not reference the windows-smoke job")
	}
}
