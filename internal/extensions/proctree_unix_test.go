//go:build !windows

package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The two answers this file pins, both from the reference's `ChildCleanup`
// (`main:src-tauri/src/extensions/process_cleanup.rs`):
//
//   - a timeout kills the process *group*, so a script's grandchildren die with
//     it (`kill_and_reap`);
//   - a normal exit sweeps the group too, so a script that left a background
//     child behind does not leak it into the next run (`finish_after_wait`).
//
// The grandchild writes a marker file half a second after it starts. If it is
// still alive when its deadline passes, the marker appears — that is the
// assertion, and it does not depend on how quickly the OS reaps a killed
// process.

// markerGrandchild is a shell snippet that starts a background subshell which
// writes `marker` after half a second. Its streams are redirected away so the
// subshell does not hold the run's pipes (the sweep, not the pipe timeout, is
// what has to kill it).
func markerGrandchild(marker string) string {
	return "( sleep 0.5; : > " + marker + " ) >/dev/null 2>&1 &"
}

// markerAppeared reports whether the grandchild got to write its marker: it
// waits well past the half second the snippet sleeps, then looks.
func markerAppeared(t *testing.T, marker string) bool {
	t.Helper()
	deadline := time.Now().Add(1200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

// A run that times out kills the script's grandchildren with it.
func TestATimedOutRunKillsTheScriptsGrandchildren(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "grandchild-ran")
	script := markerGrandchild(marker) + "\nsleep 300\n"

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	run, err := RunCaptured(ctx, runEntry(script), nil)
	if err == nil {
		t.Fatal("a hung command reported success")
	}
	if !run.TimedOut {
		t.Fatalf("run = %+v", run)
	}
	if markerAppeared(t, marker) {
		t.Error("the timed-out run left its grandchild alive")
	}
}

// A run that exits on its own still sweeps its process group: a background
// child the script left behind does not outlive the run.
func TestAFinishedRunSweepsItsProcessGroup(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "grandchild-ran")
	script := markerGrandchild(marker) + "\nexit 0\n"

	run, err := RunCaptured(context.Background(), runEntry(script), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !run.Success || run.ExitCode != 0 {
		t.Fatalf("run = %+v", run)
	}
	if markerAppeared(t, marker) {
		t.Error("the finished run left its grandchild alive")
	}
}

// The two constructors differ in exactly one bit: a probe also gets its own
// session (no controlling terminal), everything else only its own process
// group. Both are group-killable, which is why the bit may not drift.
func TestTheTwoConstructorsDifferOnlyInTheSession(t *testing.T) {
	grouped := newExtensionCommand(context.Background(), "true")
	if grouped.SysProcAttr == nil {
		t.Fatal("a run has no SysProcAttr")
	}
	if !grouped.SysProcAttr.Setpgid {
		t.Error("a run is not in its own process group")
	}
	if grouped.SysProcAttr.Setsid {
		t.Error("a run asked for a new session")
	}
	if grouped.Cancel == nil {
		t.Fatal("a run has no Cancel")
	}

	probe := newProbeCommand(context.Background(), "true")
	if probe.SysProcAttr == nil || !probe.SysProcAttr.Setsid {
		t.Error("a probe did not ask for its own session")
	}
	if probe.Cancel == nil {
		t.Fatal("a probe has no Cancel")
	}
}
