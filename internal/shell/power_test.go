//go:build unix

package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A missing preferred utility falls through to the next candidate: that is what
// lets one Linux table cover both systemd and SysV systems. Mirrors the
// reference's `missing_preferred_utility_falls_back`.
func TestRunPowerCandidatesFallsThroughAMissingProgram(t *testing.T) {
	candidates := [][]string{
		{"floter-command-that-does-not-exist"},
		{"sh", "-c", "exit 0"},
	}
	if err := runPowerCandidates(candidates, time.Second); err != nil {
		t.Fatalf("a missing first candidate should fall through: %v", err)
	}
}

// A program that starts and refuses is authoritative: its non-zero exit ends
// the search, and the message carries both the status and the utility's own
// stderr. The second candidate writes a sentinel, so "did not run it" is
// observable. Mirrors the reference's
// `refusal_does_not_try_a_more_privileged_fallback`.
func TestRunPowerCandidatesStopsAtARefusal(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "second-ran")
	candidates := [][]string{
		{"sh", "-c", "printf 'permission denied' >&2; exit 7"},
		{"sh", "-c", "touch " + sentinel},
	}
	err := runPowerCandidates(candidates, time.Second)
	if err == nil {
		t.Fatal("a refusal should be reported, not swallowed")
	}
	if !strings.Contains(err.Error(), "exit code 7") {
		t.Errorf("error should carry the exit code: %v", err)
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error should carry the utility's stderr: %v", err)
	}
	if _, statErr := os.Stat(sentinel); !os.IsNotExist(statErr) {
		t.Errorf("the more privileged fallback ran after a refusal: %v", statErr)
	}
}

// A utility that hangs is cut off by the timeout rather than leaving the
// launcher hidden indefinitely. Mirrors the reference's `hung_utility_times_out`.
func TestRunPowerCandidatesTimesOut(t *testing.T) {
	candidates := [][]string{{"sh", "-c", "sleep 5"}}
	err := runPowerCandidates(candidates, 20*time.Millisecond)
	if err == nil {
		t.Fatal("a hung utility should time out")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should say it timed out: %v", err)
	}
}

// A non-zero exit with no stderr still names the status, so the message is
// never empty. Mirrors the reference's status-only branch.
func TestRunPowerCandidatesNamesTheStatusWithoutStderr(t *testing.T) {
	err := runPowerCandidates([][]string{{"sh", "-c", "exit 3"}}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "exit code 3") {
		t.Fatalf("error = %v, want it to name exit code 3", err)
	}
}

// An unknown action is rejected before any command is chosen. Mirrors the
// reference's `unknown_actions_are_rejected`.
func TestPowerCommandsRejectUnknownActions(t *testing.T) {
	if _, err := powerCommands("hibernate", "linux"); err == nil {
		t.Fatal("an unknown action should be rejected")
	}
}

// A platform with no candidates reports that the action is unsupported rather
// than a nil error (the reference's fall-through message).
func TestRunPowerCandidatesReportAnEmptyTable(t *testing.T) {
	err := runPowerCandidates(nil, time.Second)
	if err == nil {
		t.Fatal("an empty candidate table should report an error")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("error = %v, want the unsupported-platform message", err)
	}
}
